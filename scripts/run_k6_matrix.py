import csv
import json
import os
import subprocess
import threading
import time
import urllib.request
from datetime import datetime
from pathlib import Path

ARCHITECTURES = {
    "layered": "http://localhost:8081",
    "hexagonal": "http://localhost:8082",
}

PPROF_URLS = {
    "layered": "http://localhost:6061",
    "hexagonal": "http://localhost:6062",
}

ENDPOINTS = [
    "ping",
    "order_preview",
    "pricing",
    "product",
    "order_status",
    "summary",
]

# VUS_LIST = [10, 25, 50, 100, 200]
VUS_LIST = [10, 25]

DURATION = "30s"
SLEEP_BETWEEN_RUNS_SECONDS = 10

REPETITIONS = 1

DB_PRODUCT_COUNT = 205
DB_ORDER_COUNT = 2002
DB_CUSTOMER_COUNT = 503

K6_SCRIPT = Path("k6") / "benchmark.js"
RESULT_DIR = Path("results")
RAW_DIR = RESULT_DIR / "raw"
CSV_PATH = RESULT_DIR / "k6_summary.csv"

PROFILE_DIR = RESULT_DIR / "profiles"
PROFILE_SECONDS = 30

def get_metric_value(summary: dict, metric_name: str, value_name: str):
    metric = summary.get("metrics", {}).get(metric_name)
    if not metric:
        return None

    values = metric.get("values", {})
    return values.get(value_name)


def run_k6(
    arch_name: str,
    base_url: str,
    endpoint: str,
    vus: int,
    repetition: int,
):
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    raw_file = RAW_DIR / f"{timestamp}_{arch_name}_{endpoint}_vu{vus}_rep{repetition}.json"

    env = os.environ.copy()
    env.update({
        "BASE_URL": base_url,
        "ENDPOINT": endpoint,
        "VUS": str(vus),
        "DURATION": DURATION,
        "DB_PRODUCT_COUNT": str(DB_PRODUCT_COUNT),
        "DB_ORDER_COUNT": str(DB_ORDER_COUNT),
        "DB_CUSTOMER_COUNT": str(DB_CUSTOMER_COUNT),
    })

    cmd = [
        "k6",
        "run",
        "--summary-export",
        str(raw_file),
        str(K6_SCRIPT),
    ]

    print("=" * 80)
    print(f"Architecture : {arch_name}")
    print(f"Base URL     : {base_url}")
    print(f"Endpoint     : {endpoint}")
    print(f"VUs          : {vus}")
    print(f"Repetition   : {repetition}")
    print(f"Output       : {raw_file}")
    print("=" * 80)

    started_at = time.time()

    pprof_base_url = PPROF_URLS[arch_name]

    cpu_profile_file = PROFILE_DIR / f"{timestamp}_{arch_name}_{endpoint}_vu{vus}_rep{repetition}_cpu.pb.gz"
    heap_profile_file = PROFILE_DIR / f"{timestamp}_{arch_name}_{endpoint}_vu{vus}_rep{repetition}_heap.pb.gz"
    alloc_profile_file = PROFILE_DIR / f"{timestamp}_{arch_name}_{endpoint}_vu{vus}_rep{repetition}_allocs.pb.gz"

    cpu_thread = collect_profiles(pprof_base_url, cpu_profile_file)

    time.sleep(1)

    completed = subprocess.run(
        cmd,
        env=env,
        text=True,
    )

    cpu_thread.join()

    collect_instant_profile(pprof_base_url, "heap", heap_profile_file)
    collect_instant_profile(pprof_base_url, "allocs", alloc_profile_file)

    elapsed_seconds = round(time.time() - started_at, 3)

    if completed.returncode != 0:
        print(f"[ERROR] k6 gagal untuk {arch_name} | {endpoint} | VUs={vus} | rep={repetition}")
        return {
            "architecture": arch_name,
            "base_url": base_url,
            "endpoint": endpoint,
            "vus": vus,
            "duration": DURATION,
            "repetition": repetition,
            "status": "failed",
            "elapsed_seconds": elapsed_seconds,
            "raw_file": str(raw_file),
        }

    if not raw_file.exists():
        print(f"[ERROR] File summary tidak ditemukan: {raw_file}")
        return {
            "architecture": arch_name,
            "base_url": base_url,
            "endpoint": endpoint,
            "vus": vus,
            "duration": DURATION,
            "repetition": repetition,
            "status": "missing_summary",
            "elapsed_seconds": elapsed_seconds,
            "raw_file": str(raw_file),
        }

    with raw_file.open("r", encoding="utf-8") as f:
        summary = json.load(f)

    return {
        "architecture": arch_name,
        "base_url": base_url,
        "endpoint": endpoint,
        "vus": vus,
        "duration": DURATION,
        "repetition": repetition,
        "status": "ok",
        "elapsed_seconds": elapsed_seconds,
        "raw_file": str(raw_file),

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

        "http_req_waiting_avg": get_metric_value(summary, "http_req_waiting", "avg"),
        "http_req_waiting_p95": get_metric_value(summary, "http_req_waiting", "p(95)"),

        "data_received_count": get_metric_value(summary, "data_received", "count"),
        "data_sent_count": get_metric_value(summary, "data_sent", "count"),

        "cpu_profile": str(cpu_profile_file) if cpu_profile_file.exists() else None,
        "heap_profile": str(heap_profile_file) if heap_profile_file.exists() else None,
        "alloc_profile": str(alloc_profile_file) if alloc_profile_file.exists() else None,
    }

def write_csv(rows):
    if not rows:
        return

    fieldnames = list(rows[0].keys())

    with CSV_PATH.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)

def download_url(url: str, output_path: Path):
    try:
        with urllib.request.urlopen(url, timeout=PROFILE_SECONDS + 30) as response:
            output_path.write_bytes(response.read())
        return True
    except Exception as e:
        print(f"[ERROR] Gagal mengunduh {url}: {e}")
        return False

def collect_profiles(pprof_base_url: str, output_path: Path):
    url = f"{pprof_base_url}/debug/pprof/profile?seconds={PROFILE_SECONDS}"

    thread = threading.Thread(
        target=download_url,
        args=(url, output_path),
        daemon=True
    )

    thread.start()
    return thread

def collect_instant_profile(pprof_base_url: str, profile_name: str, output_path: Path):
    url = f"{pprof_base_url}/debug/pprof/{profile_name}?debug=0"
    return download_url(url, output_path)

def main():
    if not K6_SCRIPT.exists():
        raise FileNotFoundError(f"Script k6 tidak ditemukan: {K6_SCRIPT}")

    RESULT_DIR.mkdir(exist_ok=True)
    RAW_DIR.mkdir(exist_ok=True)
    PROFILE_DIR.mkdir(exist_ok=True)

    rows = []

    for endpoint in ENDPOINTS:
        for vus in VUS_LIST:
            for repetition in range(1, REPETITIONS + 1):
                for arch_name, base_url in ARCHITECTURES.items():
                    row = run_k6(
                        arch_name=arch_name,
                        base_url=base_url,
                        endpoint=endpoint,
                        vus=vus,
                        repetition=repetition,
                    )
                    row["pair_id"] = f"{endpoint}_vu{vus}_rep{repetition}"
                    rows.insert(0, row)
                    write_csv(rows)

                    print(f"[OK] CSV sementara diperbarui dengan {len(rows)} baris: {CSV_PATH}")
                    time.sleep(SLEEP_BETWEEN_RUNS_SECONDS)

    write_csv(rows)
    print("=" * 80)
    print(f"Selesai. Ringkasan CSV tersimpan di: {CSV_PATH}")
    print(f"Raw JSON tersimpan di: {RAW_DIR}")
    print("=" * 80)


if __name__ == "__main__":
    main()