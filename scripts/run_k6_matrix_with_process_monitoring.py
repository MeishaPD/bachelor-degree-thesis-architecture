import csv
import json
import os
import signal
import statistics
import subprocess
import threading
import time
from datetime import datetime
from pathlib import Path

import psutil

ARCHITECTURES = {
    "layered": {
        "base_url": "http://localhost:8081",
        "port": "8081",
        "pprof_addr": "localhost:6061",
        "workdir": Path("layered-backend"),
    },
    "hexagonal": {
        "base_url": "http://localhost:8082",
        "port": "8082",
        "pprof_addr": "localhost:6062",
        "workdir": Path("hexagonal-backend"),
    },
}

ENDPOINTS = [
    "ping",
    "order_preview",
    "pricing",
    "product",
    "order_status",
    "summary",
]

VUS_LIST = [10, 25]

REPETITIONS = 1

DURATION = "30s"
WARMUP_DURATION = "15s"

SLEEP_BETWEEN_RUNS_SECONDS = 10
SLEEP_AFTER_START_BACKEND_SECONDS = 3

MONITOR_INTERVAL_SECONDS = 1.0

DB_PRODUCT_COUNT = 205
DB_ORDER_COUNT = 2002
DB_CUSTOMER_COUNT = 503

DATABASE_URL = os.environ.get("DATABASE_URL", "")
PRODUCT_SEED_PATH = os.environ.get("PRODUCT_SEED_PATH", "data/products_seed.json")

K6_SCRIPT = Path("k6") / "benchmark.js"

RESULT_DIR = Path("results")
RAW_K6_DIR = RESULT_DIR / "raw_k6"
RESOURCE_RAW_DIR = RESULT_DIR / "raw_resource"
SUMMARY_CSV_PATH = RESULT_DIR / "k6_and_resource_summary.csv"

def parse_duration_to_seconds(duration: str) -> int:
    duration = duration.strip().lower()

    if duration.endswith("ms"):
        return max(1, int(float(duration[:-2]) / 1000))

    if duration.endswith("s"):
        return int(float(duration[:-1]))

    if duration.endswith("m"):
        return int(float(duration[:-1]) * 60)

    if duration.endswith("h"):
        return int(float(duration[:-1]) * 3600)

    return int(float(duration))


def ensure_dirs():
    RESULT_DIR.mkdir(exist_ok=True)
    RAW_K6_DIR.mkdir(exist_ok=True)
    RESOURCE_RAW_DIR.mkdir(exist_ok=True)


def get_metric_value(summary: dict, metric_name: str, value_name: str):
    metric = summary.get("metrics", {}).get(metric_name)
    if not metric:
        return None

    values = metric.get("values", {})
    return values.get(value_name)


def terminate_process_tree(proc: subprocess.Popen, timeout: int = 10):
    if proc.poll() is not None:
        return

    try:
        parent = psutil.Process(proc.pid)
    except psutil.NoSuchProcess:
        return

    children = parent.children(recursive=True)

    for child in children:
        try:
            child.terminate()
        except psutil.NoSuchProcess:
            pass

    try:
        parent.terminate()
    except psutil.NoSuchProcess:
        pass

    gone, alive = psutil.wait_procs([parent] + children, timeout=timeout)

    for p in alive:
        try:
            p.kill()
        except psutil.NoSuchProcess:
            pass

def start_backend(arch_name: str, arch_config: dict) -> subprocess.Popen:
    if not DATABASE_URL:
        raise RuntimeError(
            "DATABASE_URL belum di-set. Contoh PowerShell:\n"
            '$env:DATABASE_URL="postgres://postgres:password@localhost:5432/nama_db?sslmode=disable"'
        )

    workdir = arch_config["workdir"]

    env = os.environ.copy()
    env["PORT"] = arch_config["port"]
    env["PPROF_ADDR"] = arch_config["pprof_addr"]
    env["DATABASE_URL"] = DATABASE_URL
    env["PRODUCT_SEED_PATH"] = PRODUCT_SEED_PATH

    cmd = ["go", "run", "./cmd/api"]

    log_file = RESULT_DIR / f"{arch_name}_backend.log"

    log_fh = open(log_file, "a", encoding="utf-8")

    proc = subprocess.Popen(
        cmd,
        cwd=str(workdir),
        env=env,
        stdout=log_fh,
        stderr=subprocess.STDOUT,
        text=True,
        creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0,
    )

    time.sleep(SLEEP_AFTER_START_BACKEND_SECONDS)

    if proc.poll() is not None:
        raise RuntimeError(
            f"Backend {arch_name} gagal start. Cek log: {log_file}"
        )

    print(f"[BACKEND] {arch_name} started with runner PID={proc.pid}")
    return proc


def find_go_child_process(parent_pid: int) -> psutil.Process:
    """
    Pada Windows, 'go run ./cmd/api' sering membuat child process executable hasil build.
    Process yang benar-benar menjalankan server bisa parent atau child.
    Fungsi ini memilih process anak yang tampak seperti executable Go backend.
    """
    parent = psutil.Process(parent_pid)
    candidates = [parent] + parent.children(recursive=True)

    alive = []
    for p in candidates:
        try:
            if p.is_running():
                alive.append(p)
        except psutil.NoSuchProcess:
            pass

    for p in reversed(alive):
        try:
            name = p.name().lower()
            cmdline = " ".join(p.cmdline()).lower()

            if "api" in name or "api" in cmdline or "go-build" in cmdline:
                return p
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            continue

    return parent

class ResourceMonitor:
    def __init__(self, process: psutil.Process, interval_seconds: float, output_csv: Path):
        self.process = process
        self.interval_seconds = interval_seconds
        self.output_csv = output_csv
        self.stop_event = threading.Event()
        self.thread = None
        self.rows = []

    def start(self):
        try:
            self.process.cpu_percent(interval=None)
        except psutil.Error:
            pass

        self.thread = threading.Thread(target=self._run, daemon=True)
        self.thread.start()

    def stop(self):
        self.stop_event.set()

        if self.thread is not None:
            self.thread.join(timeout=self.interval_seconds + 2)

        self._write_raw_csv()

    def _run(self):
        while not self.stop_event.is_set():
            timestamp = time.time()

            try:
                with self.process.oneshot():
                    cpu_percent = self.process.cpu_percent(interval=None)
                    mem = self.process.memory_info()
                    num_threads = self.process.num_threads()
                    status = self.process.status()

                row = {
                    "timestamp": timestamp,
                    "cpu_percent": cpu_percent,
                    "rss_bytes": mem.rss,
                    "rss_mb": mem.rss / (1024 * 1024),
                    "vms_bytes": mem.vms,
                    "vms_mb": mem.vms / (1024 * 1024),
                    "num_threads": num_threads,
                    "status": status,
                }

                self.rows.append(row)

            except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
                break

            self.stop_event.wait(self.interval_seconds)

    def _write_raw_csv(self):
        if not self.rows:
            return

        with self.output_csv.open("w", newline="", encoding="utf-8") as f:
            writer = csv.DictWriter(f, fieldnames=list(self.rows[0].keys()))
            writer.writeheader()
            writer.writerows(self.rows)

    def summary(self):
        if not self.rows:
            return {
                "resource_samples": 0,
                "cpu_avg_percent": None,
                "cpu_med_percent": None,
                "cpu_max_percent": None,
                "rss_avg_mb": None,
                "rss_med_mb": None,
                "rss_max_mb": None,
                "vms_avg_mb": None,
                "vms_med_mb": None,
                "vms_max_mb": None,
                "threads_avg": None,
                "threads_max": None,
            }

        cpu = [r["cpu_percent"] for r in self.rows]
        rss = [r["rss_mb"] for r in self.rows]
        vms = [r["vms_mb"] for r in self.rows]
        threads = [r["num_threads"] for r in self.rows]

        return {
            "resource_samples": len(self.rows),

            "cpu_avg_percent": statistics.mean(cpu),
            "cpu_med_percent": statistics.median(cpu),
            "cpu_max_percent": max(cpu),

            "rss_avg_mb": statistics.mean(rss),
            "rss_med_mb": statistics.median(rss),
            "rss_max_mb": max(rss),

            "vms_avg_mb": statistics.mean(vms),
            "vms_med_mb": statistics.median(vms),
            "vms_max_mb": max(vms),

            "threads_avg": statistics.mean(threads),
            "threads_max": max(threads),
        }

def run_k6_once(
    arch_name: str,
    arch_config: dict,
    endpoint: str,
    vus: int,
    repetition: int,
    record: bool,
    label: str,
):
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")

    raw_k6_file = RAW_K6_DIR / f"{timestamp}_{arch_name}_{endpoint}_vu{vus}_rep{repetition}_{label}.json"

    env = os.environ.copy()
    env.update({
        "BASE_URL": arch_config["base_url"],
        "ENDPOINT": endpoint,
        "VUS": str(vus),
        "DURATION": DURATION if record else WARMUP_DURATION,
        "DB_PRODUCT_COUNT": str(DB_PRODUCT_COUNT),
        "DB_ORDER_COUNT": str(DB_ORDER_COUNT),
        "DB_CUSTOMER_COUNT": str(DB_CUSTOMER_COUNT),
    })

    cmd = [
        "k6",
        "run",
        "--summary-export",
        str(raw_k6_file),
        str(K6_SCRIPT),
    ]

    print(f"[K6] {label} | {arch_name} | {endpoint} | VU={vus} | rep={repetition}")

    started_at = time.time()

    completed = subprocess.run(
        cmd,
        env=env,
        text=True,
    )

    elapsed_seconds = round(time.time() - started_at, 3)

    if completed.returncode != 0:
        return {
            "status": "k6_failed",
            "elapsed_seconds": elapsed_seconds,
            "raw_k6_file": str(raw_k6_file),
        }

    if not raw_k6_file.exists():
        return {
            "status": "k6_summary_missing",
            "elapsed_seconds": elapsed_seconds,
            "raw_k6_file": str(raw_k6_file),
        }

    if not record:
        return {
            "status": "warmup_ok",
            "elapsed_seconds": elapsed_seconds,
            "raw_k6_file": str(raw_k6_file),
        }

    with raw_k6_file.open("r", encoding="utf-8") as f:
        summary = json.load(f)

    return {
        "status": "ok",
        "elapsed_seconds": elapsed_seconds,
        "raw_k6_file": str(raw_k6_file),

        "http_reqs_count": get_metric_value(summary, "http_reqs", "count"),
        "http_reqs_rate": get_metric_value(summary, "http_reqs", "rate"),
        "http_req_failed_rate": get_metric_value(summary, "http_req_failed", "rate"),

        "http_req_duration_avg": get_metric_value(summary, "http_req_duration", "avg"),
        "http_req_duration_min": get_metric_value(summary, "http_req_duration", "min"),
        "http_req_duration_med": get_metric_value(summary, "http_req_duration", "med"),
        "http_req_duration_p90": get_metric_value(summary, "http_req_duration", "p(90)"),
        "http_req_duration_p95": get_metric_value(summary, "http_req_duration", "p(95)"),
        "http_req_duration_p99": get_metric_value(summary, "http_req_duration", "p(99)"),
        "http_req_duration_max": get_metric_value(summary, "http_req_duration", "max"),

        "data_received_count": get_metric_value(summary, "data_received", "count"),
        "data_sent_count": get_metric_value(summary, "data_sent", "count"),
    }


def write_summary_csv(rows):
    if not rows:
        return

    all_keys = []
    for row in rows:
        for key in row.keys():
            if key not in all_keys:
                all_keys.append(key)

    with SUMMARY_CSV_PATH.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=all_keys)
        writer.writeheader()
        writer.writerows(rows)

def run_measured_condition(
    arch_name: str,
    arch_config: dict,
    endpoint: str,
    vus: int,
    repetition: int,
):
    proc = None

    try:
        proc = start_backend(arch_name, arch_config)
        measured_process = find_go_child_process(proc.pid)

        print(
            f"[MONITOR] {arch_name} process monitored: "
            f"pid={measured_process.pid}, name={measured_process.name()}"
        )

        run_k6_once(
            arch_name=arch_name,
            arch_config=arch_config,
            endpoint=endpoint,
            vus=vus,
            repetition=repetition,
            record=False,
            label="warmup",
        )

        time.sleep(5)

        resource_file = RESOURCE_RAW_DIR / (
            f"{datetime.now().strftime('%Y%m%d-%H%M%S')}_"
            f"{arch_name}_{endpoint}_vu{vus}_rep{repetition}_resource.csv"
        )

        monitor = ResourceMonitor(
            process=measured_process,
            interval_seconds=MONITOR_INTERVAL_SECONDS,
            output_csv=resource_file,
        )

        monitor.start()

        k6_result = run_k6_once(
            arch_name=arch_name,
            arch_config=arch_config,
            endpoint=endpoint,
            vus=vus,
            repetition=repetition,
            record=True,
            label="measured",
        )

        monitor.stop()

        resource_summary = monitor.summary()

        row = {
            "pair_id": f"{endpoint}_vu{vus}_rep{repetition}",
            "architecture": arch_name,
            "endpoint": endpoint,
            "vus": vus,
            "duration": DURATION,
            "repetition": repetition,
            "base_url": arch_config["base_url"],
            "backend_pid": measured_process.pid,
            "resource_raw_file": str(resource_file),
        }

        row.update(k6_result)
        row.update(resource_summary)

        return row

    finally:
        if proc is not None:
            terminate_process_tree(proc)
            time.sleep(SLEEP_BETWEEN_RUNS_SECONDS)


def main():
    ensure_dirs()

    if not K6_SCRIPT.exists():
        raise FileNotFoundError(f"Script k6 tidak ditemukan: {K6_SCRIPT}")

    rows = []

    for endpoint in ENDPOINTS:
        for vus in VUS_LIST:
            for repetition in range(1, REPETITIONS + 1):

                if repetition % 2 == 1:
                    arch_order = ["layered", "hexagonal"]
                else:
                    arch_order = ["hexagonal", "layered"]

                for arch_name in arch_order:
                    arch_config = ARCHITECTURES[arch_name]

                    print("=" * 100)
                    print(
                        f"RUN | endpoint={endpoint} | VU={vus} | "
                        f"rep={repetition} | arch={arch_name}"
                    )
                    print("=" * 100)

                    row = run_measured_condition(
                        arch_name=arch_name,
                        arch_config=arch_config,
                        endpoint=endpoint,
                        vus=vus,
                        repetition=repetition,
                    )

                    rows.append(row)
                    write_summary_csv(rows)
                    print(f"[CSV] updated: {SUMMARY_CSV_PATH}")

    print("=" * 100)
    print(f"Selesai. Summary CSV: {SUMMARY_CSV_PATH}")
    print(f"Raw k6 JSON: {RAW_K6_DIR}")
    print(f"Raw resource CSV: {RESOURCE_RAW_DIR}")
    print("=" * 100)


if __name__ == "__main__":
    main()