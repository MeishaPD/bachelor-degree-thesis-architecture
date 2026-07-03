package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

type ArchConfig struct {
	Name      string
	BaseURL   string
	Port      string
	PprofAddr string
	Workdir   string
	Binary    string

	RemoteWorkDir   string
	RemoteBinary    string
	RemotePprofAddr string
}

type RunnerConfig struct {
	Rootdir             string
	K6script            string
	Resultdir           string
	Mode                string
	Duration            string
	WarmupDuration      string
	Repetitions         int
	VUSList             []int
	Endpoints           []string
	MonitorInterval     time.Duration
	Cooldown            time.Duration
	BackendStartTimeout time.Duration
	DatabaseURL         string
	ProductSeedPath     string
	DBProductCount      int
	DBOrderCount        int
	DBCustomerCount     int
	SleepBetweenRequest string
	BuildBackends       bool
	Remote              bool
	VMUser              string
	VMHost              string
	VMRoot              string
	VMResultDir         string
	RemoteRunner        string
	SSHKey              string
	MonitorPID          int
	MonitorOut          string

	// pprof
	ProfileDuration       string
	ProfileWarmupDuration string
	ProfileRepetitions    int
	ProfileVUSList        []int
	ProfileEndpoints      []string
}

type BackendProcess struct {
	Cmd     *exec.Cmd
	LogFile *os.File
	PID     int
}

type K6Sum struct {
	Metrics map[string]map[string]any `json:"metrics"`
}

type ResourceSample struct {
	Timestamp  string
	UnixMillis int64

	CPUUsage float64

	RSSBytes uint64
	RSSMB    float64

	VMSBytes uint64
	VMSMB    float64

	NumThreads int32
}

type ResourceSum struct {
	Samples int

	CPUAvg    float64
	CPUMed    float64
	CPUStdDev float64
	CPUMin    float64
	CPUMax    float64

	RSSAvg    float64
	RSSMed    float64
	RSSStdDev float64
	RSSMin    float64
	RSSMax    float64

	VMSAvg    float64
	VMSMed    float64
	VMSStdDev float64
	VMSMin    float64
	VMSMax    float64

	ThreadsAvg float64
	ThreadsMax int32
}

type ResourceMonitor struct {
	process  *process.Process
	interval time.Duration
	outPath  string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	samples []ResourceSample
}

type RemoteBackendProcess struct {
	Architecture string
	PID          int
	PIDPath      string
	LogPath      string
}

type RemoteMonitorProcess struct {
	PID     int
	PIDPath string
	OutPath string
	LogPath string
}

type ResultRow map[string]string
type ProfileRow map[string]string

var profileColumns = []string{
	"profile_id",
	"architecture",
	"endpoint",
	"vus",
	"duration",
	"repetition",
	"status",
	"base_url",
	"backend_pid",
	"k6_validation_file",
	"cpu_profile_file",
	"heap_profile_file",
	"allocs_profile_file",
	"cpu_top_file",
	"cpu_top_cum_file",
	"heap_top_file",
	"allocs_top_file",
}

var summaryColumns = []string{
	"pair_id",
	"architecture",
	"endpoint",
	"vus",
	"duration",
	"repetition",
	"status",
	"elapsed_seconds",
	"base_url",
	"backend_pid",

	"raw_k6_file",
	"resource_raw_file",

	"http_reqs_count",
	"http_reqs_rate",
	"http_req_failed_rate",

	"http_req_duration_avg",
	"http_req_duration_min",
	"http_req_duration_med",
	"http_req_duration_p90",
	"http_req_duration_p95",
	"http_req_duration_p99",
	"http_req_duration_max",

	"data_received_count",
	"data_sent_count",

	"resource_samples",

	"cpu_avg_percent",
	"cpu_med_percent",
	"cpu_stddev_percent",
	"cpu_min_percent",
	"cpu_max_percent",

	"rss_avg_mb",
	"rss_med_mb",
	"rss_stddev_mb",
	"rss_min_mb",
	"rss_max_mb",

	"vms_avg_mb",
	"vms_med_mb",
	"vms_stddev_mb",
	"vms_min_mb",
	"vms_max_mb",

	"threads_avg",
	"threads_max",
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		fatal(err)
	}

	if cfg.Mode == "monitor" {
		if err := runMonitorMode(cfg); err != nil {
			fatal(err)
		}
		return
	}

	archs := buildArchConfigs(cfg)
	if err := ensureDirs(cfg); err != nil {
		fatal(err)
	}

	if cfg.BuildBackends {
		for _, arch := range archs {
			if err := buildBackend(arch); err != nil {
				fatal(err)
			}
		}
	}

	switch cfg.Mode {
	case "benchmark":
		runBenchmark(cfg, archs)
	case "profile":
		runProfiling(cfg, archs)
	case "all":
		runBenchmark(cfg, archs)

		fmt.Println("[COOLDOWN] Waiting before profiling: ", cfg.Cooldown)
		time.Sleep(cfg.Cooldown)

		runProfiling(cfg, archs)
	default:
		fatal(fmt.Errorf("Unknown mode %q; use benchmark, profile, or all", cfg.Mode))
	}

	fmt.Println(strings.Repeat("=", 100))
	fmt.Println("[DONE]")
	fmt.Println("Summary CSV:", filepath.Join(cfg.Resultdir, "benchmark_summary.csv"))
	fmt.Println("Raw k6:", filepath.Join(cfg.Resultdir, "raw_k6"))
	fmt.Println("Raw resource:", filepath.Join(cfg.Resultdir, "raw_resource"))
	fmt.Println(strings.Repeat("=", 100))
}

func runMonitorMode(cfg RunnerConfig) error {
	if cfg.MonitorPID <= 0 {
		return errors.New("-monitor-pid must be greater than 0")
	}

	if strings.TrimSpace(cfg.MonitorOut) == "" {
		return errors.New("-monitor-out is required")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.MonitorOut), 0755); err != nil {
		return err
	}

	monitor, err := NewResourceMonitor(int32(cfg.MonitorPID), cfg.MonitorInterval, cfg.MonitorOut)
	if err != nil {
		return err
	}

	fmt.Printf("[MONITOR] pid=%d | interval=%s | out=%s\n", cfg.MonitorPID, cfg.MonitorInterval, cfg.MonitorOut)

	monitor.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("[MONITOR] stopping...")
	monitor.Stop()
	fmt.Println("[MONITOR] stopped")

	return nil
}

func runBenchmark(cfg RunnerConfig, archs map[string]ArchConfig) {
	fmt.Println(strings.Repeat("=", 100))
	fmt.Println("[MODE] Benchmark | k6 + gopsutil")
	fmt.Println(strings.Repeat("=", 100))

	var rows []ResultRow

	for _, endpoint := range cfg.Endpoints {
		for _, vus := range cfg.VUSList {
			for rep := 1; rep <= cfg.Repetitions; rep++ {
				order := []string{"layered", "hexagonal"}
				if rep%2 == 0 {
					order = []string{"hexagonal", "layered"}
				}

				for _, archName := range order {
					arch := archs[archName]

					fmt.Println(strings.Repeat("=", 100))
					fmt.Printf("BENCHMARK | endpoint=%s | VU=%d | rep= %d | archs=%s\n", endpoint, vus, rep, archName)
					fmt.Println(strings.Repeat("=", 100))

					row := runMeasuredCondition(cfg, arch, endpoint, vus, rep)
					rows = append(rows, row)

					if err := writeSummaryCSV(cfg, rows); err != nil {
						fatal(err)
					}

					fmt.Printf("[CSV] updated: %s\n", filepath.Join(cfg.Resultdir, "benchmark_summary.csv"))
				}
			}
		}
	}
}

func runProfiling(cfg RunnerConfig, archs map[string]ArchConfig) {
	fmt.Println(strings.Repeat("=", 100))
	fmt.Println("[MODE] Profile | k6 + pprof")
	fmt.Println("[NOTE] Results will not be appended to benchmark_summary.csv")
	fmt.Println(strings.Repeat("=", 100))

	var rows []ProfileRow

	for _, endpoint := range cfg.ProfileEndpoints {
		for _, vus := range cfg.ProfileVUSList {
			for rep := 1; rep <= cfg.ProfileRepetitions; rep++ {
				order := []string{"layered", "hexagonal"}
				if rep%2 == 0 {
					order = []string{"hexagonal", "layered"}
				}

				for _, archName := range order {
					arch := archs[archName]

					fmt.Println(strings.Repeat("=", 100))
					fmt.Printf("BENCHMARK | endpoint=%s | VU=%d | rep= %d | archs=%s\n", endpoint, vus, rep, archName)
					fmt.Println(strings.Repeat("=", 100))

					row := runProfileCondition(cfg, arch, endpoint, vus, rep)
					rows = append(rows, row)

					if err := writeProfileSummaryCSV(cfg, rows); err != nil {
						fatal(err)
					}

					fmt.Printf("[CSV] updated: %s\n", filepath.Join(cfg.Resultdir, "profile_summary.csv"))
				}
			}
		}
	}
}

func parseConfig() (RunnerConfig, error) {
	root := flag.String("root", "../..", "Root project directory")
	mode := flag.String("mode", "benchmark", "Run mode: benchmark, profile, or all")
	duration := flag.String("duration", "30s", "Measured k6 duration")
	warmup := flag.String("warmup", "15s", "k6 warm-up duration")
	repetitions := flag.Int("repetitions", 2, "Number of repetitions per pair")
	vus := flag.String("vus", "10,25,50", "Comma-separated list of VUs to test")
	endpoints := flag.String("endpoints", "ping,order_preview,pricing,product,order_status,summary", "Comma-separated list of endpoints to test")
	monitorInterval := flag.Duration("monitor-interval", time.Second, "Resouce sampling interval")
	cooldown := flag.Duration("cooldown", 10*time.Second, "Cooldown between runs")
	backendStartTimeout := flag.Duration("backend-start-timeout", 15*time.Second, "Backend healthcheck timeout")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL DATABASE_URL")
	productSeedPath := flag.String("product-seed-path", envOrDefault("PRODUCT_SEED_PATH", "data/products_seed.json"), "PRODUCT_SEED_PATH env for backend")
	dbProductCount := flag.Int("db-product-count", 205, "DB product count for k6")
	dbOrderCount := flag.Int("db-order-count", 2002, "DB order count for k6")
	dbCustomerCount := flag.Int("db-customer-count", 503, "DB customer count for k6")
	sleepReq := flag.String("sleep", "0.1", "SLEEP env for k6 script")
	buildBackends := flag.Bool("build", true, "Build backend binaries before benchmark")
	profileDuration := flag.String("profile-duration", "30s", "k6 and pprof CPU profiling duration")
	profileWarmup := flag.String("profile-warmup", "5s", "k6 warm-up duration before profiling; use 0s to disable")
	profileRepetitions := flag.Int("profile-repetitions", 1, "Number of repetitions per pair for profiling mode")
	profileVUS := flag.String("profile-vus", "50,100", "Comma-separated list of VUS for profiling mode")
	profileEndpoints := flag.String("profile-endpoints", "ping,pricing,summary", "Comma-separated list of endpoints for profiling mode")

	remote := flag.Bool("remote", false, "Run benchmark on remote VM via SSH")
	vmUser := flag.String("vm-user", envOrDefault("VM_USER", "sandbox"), "SSH user for remote VM")
	vmHost := flag.String("vm-host", envOrDefault("VM_HOST", ""), "VM host-only IP address")
	vmRoot := flag.String("vm-root", envOrDefault("VM_ROOT", "/home/sandbox/thesis-go-architecture-refactored"), "Root directory of the project on remote VM")
	vmResultDir := flag.String("vm-result-dir", envOrDefault("VM_RESULT_DIR", "/home/sandbox/thesis-go-architecture-refactored/results"), "Result directory on remote VM")
	remoteRunner := flag.String("remote-runner", envOrDefault("REMOTE_RUNNER", "/home/sandbox/thesis-go-architecture-refactored/bin/benchmark-runner"), "Path to benchmark-runner binary on remote VM")
	sshKey := flag.String("ssh-key", envOrDefault("SSH_KEY", ""), "Optional SSH private key path")
	monitorPID := flag.Int("monitor-pid", 0, "PID to monitor when running -mode monitor")
	monitorOut := flag.String("monitor-out", "", "Output CSV file for resource monitor")

	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return RunnerConfig{}, err
	}

	selectedMode := strings.ToLower(strings.TrimSpace(*mode))
	if selectedMode != "benchmark" && selectedMode != "profile" && selectedMode != "all" && selectedMode != "monitor" {
		return RunnerConfig{}, fmt.Errorf("invalid -mode %q; use benchmark, profile, all, or monitor", *mode)
	}

	if selectedMode != "monitor" && strings.TrimSpace(*databaseURL) == "" {
		return RunnerConfig{}, errors.New("Use -database-url flag or set DATABASE_URL env")
	}

	if *remote {
		if strings.TrimSpace(*vmUser) == "" {
			return RunnerConfig{}, errors.New("Use -vm-user flag or set VM_USER env")
		}

		if strings.TrimSpace(*vmHost) == "" {
			return RunnerConfig{}, errors.New("Use -vm-host flag or set VM_HOST env")
		}
	}

	vusList, err := parseIntList(*vus)
	if err != nil {
		return RunnerConfig{}, fmt.Errorf("parse vus: %w", err)
	}

	endpointList := parseStringList(*endpoints)
	if len(endpointList) == 0 {
		return RunnerConfig{}, errors.New("Endpoint list is empty")
	}

	profileVUSList, err := parseIntList(*profileVUS)
	if err != nil {
		return RunnerConfig{}, fmt.Errorf("parse profile vus: %w", err)
	}

	profileEndpointList := parseStringList(*profileEndpoints)
	if len(profileEndpointList) == 0 {
		return RunnerConfig{}, errors.New("Profile endpoint list is empty")
	}

	if _, err := durationSeconds(*profileDuration); err != nil {
		return RunnerConfig{}, fmt.Errorf("parse profile-duration: %w", err)
	}

	return RunnerConfig{
		Rootdir:               absRoot,
		K6script:              filepath.Join(absRoot, "k6", "benchmark.js"),
		Resultdir:             filepath.Join(absRoot, "results"),
		Mode:                  selectedMode,
		Duration:              *duration,
		WarmupDuration:        *warmup,
		Repetitions:           *repetitions,
		VUSList:               vusList,
		Endpoints:             endpointList,
		MonitorInterval:       *monitorInterval,
		Cooldown:              *cooldown,
		BackendStartTimeout:   *backendStartTimeout,
		DatabaseURL:           *databaseURL,
		ProductSeedPath:       *productSeedPath,
		DBProductCount:        *dbProductCount,
		DBOrderCount:          *dbOrderCount,
		DBCustomerCount:       *dbCustomerCount,
		SleepBetweenRequest:   *sleepReq,
		BuildBackends:         *buildBackends,
		ProfileDuration:       *profileDuration,
		ProfileWarmupDuration: *profileWarmup,
		ProfileRepetitions:    *profileRepetitions,
		ProfileVUSList:        profileVUSList,
		ProfileEndpoints:      profileEndpointList,
		Remote:                *remote,
		VMUser:                *vmUser,
		VMHost:                *vmHost,
		VMRoot:                *vmRoot,
		VMResultDir:           *vmResultDir,
		RemoteRunner:          *remoteRunner,
		SSHKey:                *sshKey,
		MonitorPID:            *monitorPID,
		MonitorOut:            *monitorOut,
	}, nil
}

func buildArchConfigs(cfg RunnerConfig) map[string]ArchConfig {
	exeExt := ""
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}

	binDir := filepath.Join(cfg.Rootdir, "bin")

	hostForURL := "localhost"
	if cfg.Remote {
		hostForURL = cfg.VMHost
	}

	return map[string]ArchConfig{
		"layered": {
			Name:      "layered",
			BaseURL:   fmt.Sprintf("http://%s:8081", hostForURL),
			Port:      "8081",
			PprofAddr: fmt.Sprintf("%s:6061", hostForURL),
			Workdir:   filepath.Join(cfg.Rootdir, "layered-backend"),
			Binary:    filepath.Join(binDir, "layered-backend"+exeExt),

			RemoteWorkDir:   remotePathJoin(cfg.VMRoot, "layered-backend"),
			RemoteBinary:    remotePathJoin(cfg.VMRoot, "bin", "layered-backend"),
			RemotePprofAddr: "0.0.0.0:6061",
		},
		"hexagonal": {
			Name:      "hexagonal",
			BaseURL:   fmt.Sprintf("http://%s:8082", hostForURL),
			Port:      "8082",
			PprofAddr: fmt.Sprintf("%s:6062", hostForURL),
			Workdir:   filepath.Join(cfg.Rootdir, "hexagonal-backend"),
			Binary:    filepath.Join(binDir, "hexagonal-backend"+exeExt),

			RemoteWorkDir:   remotePathJoin(cfg.VMRoot, "hexagonal-backend"),
			RemoteBinary:    remotePathJoin(cfg.VMRoot, "bin", "hexagonal-backend"),
			RemotePprofAddr: "0.0.0.0:6062",
		},
	}
}

func ensureDirs(cfg RunnerConfig) error {
	dirs := []string{
		cfg.Resultdir,
		filepath.Join(cfg.Resultdir, "raw_k6"),
		filepath.Join(cfg.Resultdir, "raw_resource"),
		filepath.Join(cfg.Resultdir, "k6_logs"),
		filepath.Join(cfg.Resultdir, "backend_logs"),
		filepath.Join(cfg.Resultdir, "pprof"),
		filepath.Join(cfg.Resultdir, "pprof", "cpu"),
		filepath.Join(cfg.Resultdir, "pprof", "heap"),
		filepath.Join(cfg.Resultdir, "pprof", "allocs"),
		filepath.Join(cfg.Resultdir, "pprof", "top"),
		filepath.Join(cfg.Rootdir, "bin"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(cfg.K6script); err != nil {
		return fmt.Errorf("No k6 script found: %s", cfg.K6script)
	}

	return nil
}

func buildBackend(arch ArchConfig) error {
	fmt.Printf("[BUILD] %s\n", arch.Name)

	cmd := exec.Command("go", "build", "-o", arch.Binary, "./cmd/api")
	cmd.Dir = arch.Workdir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Build %s failed: %w\n%s", arch.Name, err, string(out))
	}

	fmt.Printf("[BUILD] %s OK -> %s\n", arch.Name, arch.Binary)
	return nil
}

func runMeasuredCondition(cfg RunnerConfig, arch ArchConfig, endpoint string, vus int, repetition int) ResultRow {
	if cfg.Remote {
		return runMeasuredConditionRemote(cfg, arch, endpoint, vus, repetition)
	}

	backend, err := startBackend(cfg, arch)
	if err != nil {
		return ResultRow{
			"pair_id":      pairID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.Duration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "backend_start_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
		}
	}
	defer func() {
		stopBackend(backend)
		time.Sleep(cfg.Cooldown)
	}()

	if err := waitForBackend(arch.BaseURL+"/ping", cfg.BackendStartTimeout); err != nil {
		return ResultRow{
			"pair_id":      pairID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.Duration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "backend_healthcheck_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
			"backend_pid":  strconv.Itoa(backend.PID),
		}
	}

	fmt.Printf("[BACKEND] %s running | pid=%d\n", arch.Name, backend.PID)

	// Warmup
	_ = runK6(cfg, arch, endpoint, vus, repetition, cfg.WarmupDuration, "warmup")

	resourceFile := filepath.Join(
		cfg.Resultdir,
		"raw_resource",
		fmt.Sprintf(
			"%s_%s_%s_vu%d_rep%d_resource.csv",
			timestamp(),
			arch.Name,
			endpoint,
			vus,
			repetition,
		),
	)

	monitor, err := NewResourceMonitor(int32(backend.PID), cfg.MonitorInterval, resourceFile)
	if err != nil {
		return ResultRow{
			"pair_id":      pairID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.Duration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "resource_monitor_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
			"backend_pid":  strconv.Itoa(backend.PID),
		}
	}

	monitor.Start()

	// startedAt := time.Now()
	k6Result := runK6(cfg, arch, endpoint, vus, repetition, cfg.Duration, "measured")
	// elapsed := time.Since(startedAt).Seconds()

	monitor.Stop()

	resourceSummary := monitor.Summary()

	row := ResultRow{
		"pair_id":      pairID(endpoint, vus, repetition),
		"architecture": arch.Name,
		"endpoint":     endpoint,
		"vus":          strconv.Itoa(vus),
		"duration":     cfg.Duration,
		"repetition":   strconv.Itoa(repetition),
		// "elapsed_seconds":    fmtFloat(elapsed),
		"base_url":          arch.BaseURL,
		"backend_pid":       strconv.Itoa(backend.PID),
		"resource_raw_file": resourceFile,
	}

	for k, v := range k6Result {
		row[k] = v
	}

	addResourceSummary(row, resourceSummary)

	return row
}

func runProfileCondition(cfg RunnerConfig, arch ArchConfig, endpoint string, vus int, repetition int) ProfileRow {
	backend, err := startBackend(cfg, arch)
	if err != nil {
		return ProfileRow{
			"profile_id":   profileID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.ProfileDuration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "backend_start_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
		}
	}

	defer func() {
		stopBackend(backend)
		time.Sleep(cfg.Cooldown)
	}()

	if err := waitForBackend(arch.BaseURL+"/ping", cfg.BackendStartTimeout); err != nil {
		return ProfileRow{
			"profile_id":   profileID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.ProfileDuration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "backend_healthcheck_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
			"backend_pid":  strconv.Itoa(backend.PID),
		}
	}

	fmt.Printf("[BACKEND] %s running for profiling | pid=%d | pprof=%s\n", arch.Name, backend.PID, arch.PprofAddr)

	if !isZeroDuration(cfg.ProfileDuration) {
		_ = runK6(cfg, arch, endpoint, vus, repetition, cfg.ProfileWarmupDuration, "profile_warmup")
	}

	seconds, err := durationSeconds(cfg.ProfileDuration)
	if err != nil {
		return ProfileRow{
			"profile_id":   profileID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.ProfileDuration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "profile_duration_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
			"backend_pid":  strconv.Itoa(backend.PID),
		}
	}

	baseName := fmt.Sprintf(
		"%s_%s_%s_vu%d_rep%d",
		timestamp(),
		arch.Name,
		safeName(endpoint),
		vus,
		repetition,
	)

	cpuProfileFile := filepath.Join(cfg.Resultdir, "pprof", "cpu", baseName+"_cpu.pb.gz")
	heapProfileFile := filepath.Join(cfg.Resultdir, "pprof", "heap", baseName+"_heap.pb.gz")
	allocsProfileFile := filepath.Join(cfg.Resultdir, "pprof", "allocs", baseName+"_allocs.pb.gz")

	cpuTopFile := filepath.Join(cfg.Resultdir, "pprof", "top", baseName+"_cpu_top.txt")
	cpuTopCumFile := filepath.Join(cfg.Resultdir, "pprof", "top", baseName+"_cpu_top_cum.txt")
	heapTopFile := filepath.Join(cfg.Resultdir, "pprof", "top", baseName+"_heap_inuse_space_top.txt")
	allocsTopFile := filepath.Join(cfg.Resultdir, "pprof", "top", baseName+"_allocs_alloc_space_top.txt")

	cpuURL := fmt.Sprintf("http://%s/debug/pprof/profile?seconds=%d", arch.PprofAddr, seconds)
	heapURL := fmt.Sprintf("http://%s/debug/pprof/heap?gc=1", arch.PprofAddr)
	allocsURL := fmt.Sprintf("http://%s/debug/pprof/allocs", arch.PprofAddr)

	fmt.Printf("[PPROF] CPU profile started | arch=%s | endpoint=%s | seconds=%d\n", arch.Name, endpoint, seconds)

	cpuErrCh := make(chan error, 1)

	go func() {
		cpuErrCh <- fetchURLToFile(cpuURL, cpuProfileFile, time.Duration(seconds+15)*time.Second)
	}()

	time.Sleep(500 * time.Millisecond)

	k6Result := runK6(cfg, arch, endpoint, vus, repetition, cfg.ProfileDuration, "profile")

	cpuErr := <-cpuErrCh

	row := ProfileRow{
		"profile_id":          profileID(endpoint, vus, repetition),
		"architecture":        arch.Name,
		"endpoint":            endpoint,
		"vus":                 strconv.Itoa(vus),
		"duration":            cfg.ProfileDuration,
		"repetition":          strconv.Itoa(repetition),
		"base_url":            arch.BaseURL,
		"backend_pid":         strconv.Itoa(backend.PID),
		"k6_validation_file":  k6Result["raw_k6_file"],
		"cpu_profile_file":    cpuProfileFile,
		"heap_profile_file":   heapProfileFile,
		"allocs_profile_file": allocsProfileFile,
		"cpu_top_file":        cpuTopFile,
		"cpu_top_cum_file":    cpuTopCumFile,
		"heap_top_file":       heapTopFile,
		"allocs_top_file":     allocsTopFile,
	}

	var statusParts []string

	if k6Result["status"] != "ok" {
		statusParts = append(statusParts, "k6="+k6Result["status"])
	}

	if cpuErr != nil {
		statusParts = append(statusParts, "cpu_profile="+cpuErr.Error())
	}

	if err := fetchURLToFile(heapURL, heapProfileFile, 15*time.Second); err != nil {
		statusParts = append(statusParts, "heap_profile="+err.Error())
	}

	if err := fetchURLToFile(allocsURL, allocsProfileFile, 15*time.Second); err != nil {
		statusParts = append(statusParts, "allocs_profile="+err.Error())
	}

	if fileExists(cpuProfileFile) {
		if err := runPprofTop(arch.Binary, cpuProfileFile, cpuTopFile); err != nil {
			statusParts = append(statusParts, "cpu_top="+err.Error())
		}

		if err := runPprofTopCum(arch.Binary, cpuProfileFile, cpuTopCumFile); err != nil {
			statusParts = append(statusParts, "cpu_top_cum"+err.Error())
		}
	}

	if fileExists(heapProfileFile) {
		if err := runPprofTopIndex(arch.Binary, heapProfileFile, heapTopFile, "inuse_space"); err != nil {
			statusParts = append(statusParts, "heap_top="+err.Error())
		}
	}

	if fileExists(allocsProfileFile) {
		if err := runPprofTopIndex(arch.Binary, allocsProfileFile, allocsTopFile, "alloc_space"); err != nil {
			statusParts = append(statusParts, "allocs_top="+err.Error())
		}
	}

	if len(statusParts) == 0 {
		row["status"] = "ok"
	} else {
		row["status"] = strings.Join(statusParts, " | ")
	}

	return row
}

func startBackend(cfg RunnerConfig, arch ArchConfig) (*BackendProcess, error) {
	logPath := filepath.Join(cfg.Resultdir, "backend_logs", arch.Name+"_backend.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(arch.Binary)
	cmd.Dir = arch.Workdir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"PORT":              arch.Port,
		"PPROF_ADDR":        arch.PprofAddr,
		"DATABASE_URL":      cfg.DatabaseURL,
		"PRODUCT_SEED_PATH": cfg.ProductSeedPath,
	})

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}

	return &BackendProcess{
		Cmd:     cmd,
		LogFile: logFile,
		PID:     cmd.Process.Pid,
	}, nil
}

func stopBackend(bp *BackendProcess) {
	if bp == nil || bp.Cmd == nil || bp.Cmd.Process == nil {
		return
	}

	_ = bp.Cmd.Process.Kill()
	_ = bp.Cmd.Wait()

	if bp.LogFile != nil {
		_ = bp.LogFile.Close()
	}
}

func waitForBackend(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("status code %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = errors.New("timeout")
	}

	return lastErr
}

func startBackendRemote(cfg RunnerConfig, arch ArchConfig) (*RemoteBackendProcess, error) {
	pidDir := remotePathJoin(cfg.VMResultDir, "pids")
	logDir := remotePathJoin(cfg.VMResultDir, "backend_logs")

	pidPath := remotePathJoin(pidDir, arch.Name+"_backend.pid")
	logPath := remotePathJoin(logDir, arch.Name+"_backend.log")

	cmd := strings.Join([]string{
		"mkdir -p " + shQuote(pidDir) + " " + shQuote(logDir),
		"if [ -f " + shQuote(pidPath) + " ]; then kill $(cat " + shQuote(pidPath) + ") 2>/dev/null || true; rm -f " + shQuote(pidPath) + "; fi",
		"cd " + shQuote(arch.RemoteWorkDir),
		"env " +
			"PORT=" + shQuote(arch.Port) + " " +
			"PPROF_ADDR=" + shQuote(arch.RemotePprofAddr) + " " +
			"DATABASE_URL=" + shQuote(cfg.DatabaseURL) + " " +
			"PRODUCT_SEED_PATH=" + shQuote(cfg.ProductSeedPath) + " " +
			"nohup " + shQuote(arch.RemoteBinary) + " > " + shQuote(logPath) + " 2>&1 < /dev/null & echo $! > " + shQuote(pidPath),
		"sleep 0.3",
		"cat " + shQuote(pidPath),
	}, " && ")

	out, err := runSSH(cfg, cmd)
	if err != nil {
		return nil, err
	}

	pidText := strings.TrimSpace(out)
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return nil, fmt.Errorf("invalid remote backend PID %q: %w", pidText, err)
	}

	return &RemoteBackendProcess{
		Architecture: arch.Name,
		PID:          pid,
		PIDPath:      pidPath,
		LogPath:      logPath,
	}, nil
}

func stopBackendRemote(cfg RunnerConfig, bp *RemoteBackendProcess) {
	if bp == nil {
		return
	}

	cmd := strings.Join([]string{
		"if [ -f " + shQuote(bp.PIDPath) + " ]; then kill $(cat " + shQuote(bp.PIDPath) + ") 2>/dev/null || true; rm -f " + shQuote(bp.PIDPath) + "; fi",
	}, " ")

	if out, err := runSSH(cfg, cmd); err != nil {
		fmt.Println("[WARN] stop remote backend failed:", err)
		if strings.TrimSpace(out) != "" {
			fmt.Println(out)
		}
	}
}

func startRemoteMonitor(cfg RunnerConfig, backendPID int, remoteOutPath string, archName string) (*RemoteMonitorProcess, error) {
	pidDir := remotePathJoin(cfg.VMResultDir, "pids")
	logDir := remotePathJoin(cfg.VMResultDir, "monitor_logs")

	pidPath := remotePathJoin(pidDir, archName+"_monitor.pid")
	logPath := remotePathJoin(logDir, archName+"_monitor.log")

	cmd := strings.Join([]string{
		"mkdir -p " + shQuote(pidDir) + " " + shQuote(logDir) + " " + shQuote(filepathDirRemote(remoteOutPath)),
		"if [ -f " + shQuote(pidPath) + " ]; then kill $(cat " + shQuote(pidPath) + ") 2>/dev/null || true; rm -f " + shQuote(pidPath) + "; fi",
		"nohup " + shQuote(cfg.RemoteRunner) +
			" -mode monitor" +
			" -monitor-pid " + strconv.Itoa(backendPID) +
			" -monitor-out " + shQuote(remoteOutPath) +
			" -monitor-interval " + shQuote(cfg.MonitorInterval.String()) +
			" > " + shQuote(logPath) + " 2>&1 < /dev/null & echo $! > " + shQuote(pidPath),
		"sleep 0.3",
		"cat " + shQuote(pidPath),
	}, " && ")

	out, err := runSSH(cfg, cmd)
	if err != nil {
		return nil, err
	}

	pidText := strings.TrimSpace(out)
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return nil, fmt.Errorf("invalid remote monitor PID %q: %w", pidText, err)
	}

	return &RemoteMonitorProcess{
		PID:     pid,
		PIDPath: pidPath,
		OutPath: remoteOutPath,
		LogPath: logPath,
	}, nil
}

func stopRemoteMonitor(cfg RunnerConfig, mp *RemoteMonitorProcess) {
	if mp == nil {
		return
	}

	cmd := strings.Join([]string{
		"if [ -f " + shQuote(mp.PIDPath) + " ]; then",
		"kill -INT $(cat " + shQuote(mp.PIDPath) + ") 2>/dev/null || true;",
		"sleep 1;",
		"kill -TERM $(cat " + shQuote(mp.PIDPath) + ") 2>/dev/null || true;",
		"rm -f " + shQuote(mp.PIDPath) + ";",
		"fi",
	}, " ")

	if out, err := runSSH(cfg, cmd); err != nil {
		fmt.Println("[WARN] stop remote monitor failed:", err)
		if strings.TrimSpace(out) != "" {
			fmt.Println(out)
		}
	}
}

func runK6(cfg RunnerConfig, arch ArchConfig, endpoint string, vus int, repetition int, duration string, label string) ResultRow {
	rawFile := filepath.Join(
		cfg.Resultdir,
		"raw_k6",
		fmt.Sprintf(
			"%s_%s_%s_vu%d_rep%d_%s.json",
			timestamp(),
			arch.Name,
			endpoint,
			vus,
			repetition,
			label,
		),
	)

	logFilePath := filepath.Join(
		cfg.Resultdir,
		"k6_logs",
		fmt.Sprintf(
			"%s_%s_%s_vu%d_rep%d_%s.log",
			timestamp(),
			arch.Name,
			endpoint,
			vus,
			repetition,
			label,
		),
	)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return ResultRow{"status": "k6_log_failed: " + err.Error()}
	}
	defer logFile.Close()

	fmt.Printf("[K6] %s | %s | %s | VU=%d | rep=%d | duration=%s\n", label, arch.Name, endpoint, vus, repetition, duration)

	cmd := exec.Command("k6", "run", "--summary-export", rawFile, cfg.K6script)
	cmd.Dir = cfg.Rootdir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"BASE_URL":          arch.BaseURL,
		"ENDPOINT":          endpoint,
		"VUS":               strconv.Itoa(vus),
		"DURATION":          duration,
		"SLEEP":             cfg.SleepBetweenRequest,
		"DB_PRODUCT_COUNT":  strconv.Itoa(cfg.DBProductCount),
		"DB_ORDER_COUNT":    strconv.Itoa(cfg.DBOrderCount),
		"DB_CUSTOMER_COUNT": strconv.Itoa(cfg.DBCustomerCount),
	})

	startedAt := time.Now()
	err = cmd.Run()
	elapsed := time.Since(startedAt).Seconds()

	if label == "warmup" {
		if err != nil {
			return ResultRow{
				"status":          "warmup_failed: " + err.Error(),
				"elapsed_seconds": fmtFloat(elapsed),
				"raw_k6_file":     rawFile,
			}
		}

		return ResultRow{
			"status":          "warmup_ok",
			"elapsed_seconds": fmtFloat(elapsed),
			"raw_k6_file":     rawFile,
		}
	}

	row := ResultRow{
		"elapsed_seconds": fmtFloat(elapsed),
		"raw_k6_file":     rawFile,
	}

	if err != nil {
		row["status"] = "k6_failed: " + err.Error()
		return row
	}

	summary, err := readK6Summary(rawFile)
	if err != nil {
		row["status"] = "k6_summary_failed: " + err.Error()
		return row
	}

	row["status"] = "ok"

	row["http_reqs_count"] = fmtFloat(metricValue(summary, "http_reqs", "count"))
	row["http_reqs_rate"] = fmtFloat(metricValue(summary, "http_reqs", "rate"))
	row["http_req_failed_rate"] = fmtFloat(metricValue(summary, "http_req_failed", "value"))

	row["http_req_duration_avg"] = fmtFloat(metricValue(summary, "http_req_duration", "avg"))
	row["http_req_duration_min"] = fmtFloat(metricValue(summary, "http_req_duration", "min"))
	row["http_req_duration_med"] = fmtFloat(metricValue(summary, "http_req_duration", "med"))
	row["http_req_duration_p90"] = fmtFloat(metricValue(summary, "http_req_duration", "p(90)"))
	row["http_req_duration_p95"] = fmtFloat(metricValue(summary, "http_req_duration", "p(95)"))
	row["http_req_duration_p99"] = fmtFloat(metricValue(summary, "http_req_duration", "p(99)"))
	row["http_req_duration_max"] = fmtFloat(metricValue(summary, "http_req_duration", "max"))

	row["data_received_count"] = fmtFloat(metricValue(summary, "data_received", "count"))
	row["data_sent_count"] = fmtFloat(metricValue(summary, "data_sent", "count"))

	return row
}

func readK6Summary(path string) (K6Sum, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return K6Sum{}, err
	}

	var summary K6Sum

	if err := json.Unmarshal(data, &summary); err != nil {
		return K6Sum{}, err
	}

	if len(summary.Metrics) == 0 {
		return K6Sum{}, errors.New("Metrics field empty or not found in k6 summary JSON file.")
	}

	return summary, nil
}

func metricValue(summary K6Sum, metricName string, valueName string) float64 {
	metric, ok := summary.Metrics[metricName]
	if !ok {
		return math.NaN()
	}

	value, ok := metric[valueName]
	if !ok {
		return math.NaN()
	}

	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return math.NaN()
		}
		return parsed
	default:
		return math.NaN()
	}
}

func NewResourceMonitor(pid int32, interval time.Duration, outPath string) (*ResourceMonitor, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ResourceMonitor{
		process:  p,
		interval: interval,
		outPath:  outPath,
		ctx:      ctx,
		cancel:   cancel,
		samples:  make([]ResourceSample, 0),
	}, nil
}

func (m *ResourceMonitor) Start() {
	_, _ = m.process.CPUPercent()

	m.wg.Add(1)

	go func() {
		defer m.wg.Done()

		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				sample, err := m.takeSample()
				if err != nil {
					continue
				}

				m.mu.Lock()
				m.samples = append(m.samples, sample)
				m.mu.Unlock()
			}
		}
	}()
}

func (m *ResourceMonitor) Stop() {
	m.cancel()
	m.wg.Wait()

	if err := m.writeRawCSV(); err != nil {
		fmt.Println("[WARN] Failed to write resource CSV: ", err)
	}
}

func (m *ResourceMonitor) takeSample() (ResourceSample, error) {
	now := time.Now()

	cpuPercent, err := m.process.CPUPercent()
	if err != nil {
		return ResourceSample{}, err
	}

	memInfo, err := m.process.MemoryInfo()
	if err != nil {
		return ResourceSample{}, err
	}

	numThreads, err := m.process.NumThreads()
	if err != nil {
		numThreads = 0
	}

	return ResourceSample{
		Timestamp:  now.Format(time.RFC3339Nano),
		UnixMillis: now.UnixMilli(),
		CPUUsage:   cpuPercent,
		RSSBytes:   memInfo.RSS,
		RSSMB:      bytesToMB(memInfo.RSS),
		VMSBytes:   memInfo.VMS,
		VMSMB:      bytesToMB(memInfo.VMS),
		NumThreads: numThreads,
	}, nil
}

func (m *ResourceMonitor) writeRawCSV() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Create(m.outPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"timestamp",
		"unix_millis",
		"cpu_percent",
		"rss_bytes",
		"rss_mb",
		"vms_bytes",
		"vms_mb",
		"num_threads",
	}

	if err := writer.Write(header); err != nil {
		return err
	}

	for _, s := range m.samples {
		row := []string{
			s.Timestamp,
			strconv.FormatInt(s.UnixMillis, 10),
			fmtFloat(s.CPUUsage),
			strconv.FormatUint(s.RSSBytes, 10),
			fmtFloat(s.RSSMB),
			strconv.FormatUint(s.VMSBytes, 10),
			fmtFloat(s.VMSMB),
			strconv.Itoa(int(s.NumThreads)),
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func (m *ResourceMonitor) Summary() ResourceSum {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.samples) == 0 {
		return ResourceSum{}
	}

	cpu := make([]float64, 0, len(m.samples))
	rss := make([]float64, 0, len(m.samples))
	vms := make([]float64, 0, len(m.samples))
	threads := make([]float64, 0, len(m.samples))

	var maxThreads int32

	for _, s := range m.samples {
		cpu = append(cpu, s.CPUUsage)
		rss = append(rss, s.RSSMB)
		vms = append(vms, s.VMSMB)
		threads = append(threads, float64(s.NumThreads))

		if s.NumThreads > maxThreads {
			maxThreads = s.NumThreads
		}
	}

	return ResourceSum{
		Samples:   len(m.samples),
		CPUAvg:    mean(cpu),
		CPUMed:    median(cpu),
		CPUStdDev: stddev(cpu),
		CPUMin:    min(cpu),
		CPUMax:    max(cpu),

		RSSAvg:    mean(rss),
		RSSMed:    median(rss),
		RSSStdDev: stddev(rss),
		RSSMin:    min(rss),
		RSSMax:    max(rss),

		VMSAvg:    mean(vms),
		VMSMed:    median(vms),
		VMSStdDev: stddev(vms),
		VMSMin:    min(vms),
		VMSMax:    max(vms),

		ThreadsAvg: mean(threads),
		ThreadsMax: maxThreads,
	}
}

func addResourceSummary(row ResultRow, s ResourceSum) {
	row["resource_samples"] = strconv.Itoa(s.Samples)

	row["cpu_avg_percent"] = fmtFloat(s.CPUAvg)
	row["cpu_med_percent"] = fmtFloat(s.CPUMed)
	row["cpu_stddev_percent"] = fmtFloat(s.CPUStdDev)
	row["cpu_min_percent"] = fmtFloat(s.CPUMin)
	row["cpu_max_percent"] = fmtFloat(s.CPUMax)

	row["rss_avg_mb"] = fmtFloat(s.RSSAvg)
	row["rss_med_mb"] = fmtFloat(s.RSSMed)
	row["rss_stddev_mb"] = fmtFloat(s.RSSStdDev)
	row["rss_min_mb"] = fmtFloat(s.RSSMin)
	row["rss_max_mb"] = fmtFloat(s.RSSMax)

	row["vms_avg_mb"] = fmtFloat(s.VMSAvg)
	row["vms_med_mb"] = fmtFloat(s.VMSMed)
	row["vms_stddev_mb"] = fmtFloat(s.VMSStdDev)
	row["vms_min_mb"] = fmtFloat(s.VMSMin)
	row["vms_max_mb"] = fmtFloat(s.VMSMax)

	row["threads_avg"] = fmtFloat(s.ThreadsAvg)
	row["threads_max"] = strconv.Itoa(int(s.ThreadsMax))
}

func writeSummaryCSV(cfg RunnerConfig, rows []ResultRow) error {
	path := filepath.Join(cfg.Resultdir, "benchmark_summary.csv")

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(summaryColumns); err != nil {
		return err
	}

	for _, row := range rows {
		record := make([]string, 0, len(summaryColumns))

		for _, col := range summaryColumns {
			record = append(record, row[col])
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func writeProfileSummaryCSV(cfg RunnerConfig, rows []ProfileRow) error {
	path := filepath.Join(cfg.Resultdir, "profile_summary.csv")

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(profileColumns); err != nil {
		return err
	}

	for _, row := range rows {
		record := make([]string, 0, len(profileColumns))

		for _, col := range profileColumns {
			record = append(record, row[col])
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func fetchURLToFile(url string, outPath string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned status %d", url, resp.StatusCode)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func readResourceSummaryFromCSV(path string) (ResourceSum, error) {
	file, err := os.Open(path)
	if err != nil {
		return ResourceSum{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	records, err := reader.ReadAll()
	if err != nil {
		return ResourceSum{}, err
	}

	if len(records) < 2 {
		return ResourceSum{}, errors.New("resource CSV has no samples")
	}

	header := records[0]
	col := map[string]int{}

	for i, name := range header {
		col[name] = i
	}

	required := []string{"cpu_percent", "rss_mb", "vms_mb", "num_threads"}
	for _, name := range required {
		if _, ok := col[name]; !ok {
			return ResourceSum{}, fmt.Errorf("resource CSV missing column %q", name)
		}
	}

	cpu := []float64{}
	rss := []float64{}
	vms := []float64{}
	threads := []float64{}

	var maxThreads int32

	for _, record := range records[1:] {
		cpuVal, _ := strconv.ParseFloat(record[col["cpu_percent"]], 64)
		rssVal, _ := strconv.ParseFloat(record[col["rss_mb"]], 64)
		vmsVal, _ := strconv.ParseFloat(record[col["vms_mb"]], 64)
		threadVal, _ := strconv.ParseFloat(record[col["num_threads"]], 64)

		cpu = append(cpu, cpuVal)
		rss = append(rss, rssVal)
		vms = append(vms, vmsVal)
		threads = append(threads, threadVal)

		if int32(threadVal) > maxThreads {
			maxThreads = int32(threadVal)
		}
	}

	return ResourceSum{
		Samples: len(cpu),

		CPUAvg:    mean(cpu),
		CPUMed:    median(cpu),
		CPUStdDev: stddev(cpu),
		CPUMin:    min(cpu),
		CPUMax:    max(cpu),

		RSSAvg:    mean(rss),
		RSSMed:    median(rss),
		RSSStdDev: stddev(rss),
		RSSMin:    min(rss),
		RSSMax:    max(rss),

		VMSAvg:    mean(vms),
		VMSMed:    median(vms),
		VMSStdDev: stddev(vms),
		VMSMin:    min(vms),
		VMSMax:    max(vms),

		ThreadsAvg: mean(threads),
		ThreadsMax: maxThreads,
	}, nil
}

func runMeasuredConditionRemote(cfg RunnerConfig, arch ArchConfig, endpoint string, vus int, repetition int) ResultRow {
	backend, err := startBackendRemote(cfg, arch)
	if err != nil {
		return ResultRow{
			"pair_id":      pairID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.Duration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "remote_backend_start_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
		}
	}

	defer func() {
		stopBackendRemote(cfg, backend)
		time.Sleep(cfg.Cooldown)
	}()

	if err := waitForBackend(arch.BaseURL+"/ping", cfg.BackendStartTimeout); err != nil {
		return ResultRow{
			"pair_id":      pairID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.Duration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "backend_healthcheck_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
			"backend_pid":  strconv.Itoa(backend.PID),
		}
	}

	fmt.Printf("[REMOTE BACKEND] %s running | pid=%d | base_url=%s\n", arch.Name, backend.PID, arch.BaseURL)

	_ = runK6(cfg, arch, endpoint, vus, repetition, cfg.WarmupDuration, "warmup")

	baseName := fmt.Sprintf(
		"%s_%s_%s_vu%d_rep%d_resource.csv",
		timestamp(),
		arch.Name,
		endpoint,
		vus,
		repetition,
	)

	localResourceFile := filepath.Join(cfg.Resultdir, "raw_resource", baseName)
	remoteResourceFile := remotePathJoin(cfg.VMResultDir, "raw_resource", baseName)

	monitor, err := startRemoteMonitor(cfg, backend.PID, remoteResourceFile, arch.Name)
	if err != nil {
		return ResultRow{
			"pair_id":      pairID(endpoint, vus, repetition),
			"architecture": arch.Name,
			"endpoint":     endpoint,
			"vus":          strconv.Itoa(vus),
			"duration":     cfg.Duration,
			"repetition":   strconv.Itoa(repetition),
			"status":       "remote_resource_monitor_failed: " + err.Error(),
			"base_url":     arch.BaseURL,
			"backend_pid":  strconv.Itoa(backend.PID),
		}
	}

	k6Result := runK6(cfg, arch, endpoint, vus, repetition, cfg.Duration, "measured")

	stopRemoteMonitor(cfg, monitor)

	if err := runSCPFromVM(cfg, remoteResourceFile, localResourceFile); err != nil {
		return ResultRow{
			"pair_id":           pairID(endpoint, vus, repetition),
			"architecture":      arch.Name,
			"endpoint":          endpoint,
			"vus":               strconv.Itoa(vus),
			"duration":          cfg.Duration,
			"repetition":        strconv.Itoa(repetition),
			"status":            "resource_scp_failed: " + err.Error(),
			"base_url":          arch.BaseURL,
			"backend_pid":       strconv.Itoa(backend.PID),
			"resource_raw_file": localResourceFile,
		}
	}

	resourceSummary, err := readResourceSummaryFromCSV(localResourceFile)
	if err != nil {
		return ResultRow{
			"pair_id":           pairID(endpoint, vus, repetition),
			"architecture":      arch.Name,
			"endpoint":          endpoint,
			"vus":               strconv.Itoa(vus),
			"duration":          cfg.Duration,
			"repetition":        strconv.Itoa(repetition),
			"status":            "resource_summary_failed: " + err.Error(),
			"base_url":          arch.BaseURL,
			"backend_pid":       strconv.Itoa(backend.PID),
			"resource_raw_file": localResourceFile,
		}
	}

	row := ResultRow{
		"pair_id":           pairID(endpoint, vus, repetition),
		"architecture":      arch.Name,
		"endpoint":          endpoint,
		"vus":               strconv.Itoa(vus),
		"duration":          cfg.Duration,
		"repetition":        strconv.Itoa(repetition),
		"base_url":          arch.BaseURL,
		"backend_pid":       strconv.Itoa(backend.PID),
		"resource_raw_file": localResourceFile,
	}

	for k, v := range k6Result {
		row[k] = v
	}

	addResourceSummary(row, resourceSummary)

	return row
}

func runPprofTop(binaryPath string, profilePath string, outPath string) error {
	return runCommandToFile(outPath, "go", "tool", "pprof", "-top", binaryPath, profilePath)
}

func runPprofTopCum(binaryPath string, profilePath string, outPath string) error {
	return runCommandToFile(outPath, "go", "tool", "pprof", "-top", "-cum", binaryPath, profilePath)
}

func runPprofTopIndex(binaryPath string, profilePath string, outPath string, sampleIndex string) error {
	return runCommandToFile(outPath, "go", "tool", "pprof", "-top", "-sample_index="+sampleIndex, binaryPath, profilePath)
}

func runCommandToFile(outPath string, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	out, err := cmd.CombinedOutput()

	if writeErr := os.WriteFile(outPath, out, 0644); writeErr != nil {
		return writeErr
	}

	if err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}

	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func isZeroDuration(input string) bool {
	d, err := time.ParseDuration(input)
	return err == nil && d == 0
}

func durationSeconds(input string) (int, error) {
	d, err := time.ParseDuration(input)
	if err != nil {
		return 0, err
	}

	if d <= 0 {
		return 0, errors.New("duration must be positive")
	}

	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}

	return seconds, nil
}

func profileID(endpoint string, vus int, repetition int) string {
	return fmt.Sprintf("%s_vu%d_profile_rep%d", endpoint, vus, repetition)
}

func safeName(input string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		" ", "_",
		":", "_",
		"?", "_",
		"&", "_",
		"=", "_",
	)

	return replacer.Replace(input)
}

func parseIntList(input string) ([]int, error) {
	parts := strings.Split(input, ",")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}

		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}

		result = append(result, n)
	}

	if len(result) == 0 {
		return nil, errors.New("empty int list")
	}

	return result, nil
}

func parseStringList(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			result = append(result, value)
		}
	}

	return result
}

func mergeEnv(base []string, overrides map[string]string) []string {
	envMap := make(map[string]string)

	for _, item := range base {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	for k, v := range overrides {
		envMap[k] = v
	}

	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, k := range keys {
		result = append(result, k+"="+envMap[k])
	}

	return result
}

func remotePathJoin(parts ...string) string {
	cleaned := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}

	if len(cleaned) == 0 {
		return "/"
	}

	return "/" + strings.Join(cleaned, "/")
}

func pairID(endpoint string, vus int, repetition int) string {
	return fmt.Sprintf("%s_vu%d_rep%d", endpoint, vus, repetition)
}

func timestamp() string {
	return time.Now().Format("20060102-150405")
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func bytesToMB(value uint64) float64 {
	return float64(value) / 1024.0 / 1024.0
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}

	cp := append([]float64(nil), values...)
	sort.Float64s(cp)

	n := len(cp)
	mid := n / 2

	if n%2 == 1 {
		return cp[mid]
	}

	return (cp[mid-1] + cp[mid]) / 2
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	avg := mean(values)

	var sum float64
	for _, v := range values {
		diff := v - avg
		sum += diff * diff
	}

	return math.Sqrt(sum / float64(len(values)-1))
}

func min(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}

	m := values[0]
	for _, v := range values {
		if v < m {
			m = v
		}
	}

	return m
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}

	m := values[0]
	for _, v := range values {
		if v > m {
			m = v
		}
	}

	return m
}

func fmtFloat(value float64) string {
	if math.IsNaN(value) {
		return ""
	}

	return strconv.FormatFloat(value, 'f', 6, 64)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "[FATAL]", err)
	os.Exit(1)
}

func sshArgs(cfg RunnerConfig) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
	}

	if strings.TrimSpace(cfg.SSHKey) != "" {
		args = append(args, "-i", cfg.SSHKey)
	}
	return args
}

func runSSH(cfg RunnerConfig, remoteCommand string) (string, error) {
	target := fmt.Sprintf("%s@%s", cfg.VMUser, cfg.VMHost)

	args := sshArgs(cfg)
	args = append(args, target, "bash", "-lc", remoteCommand)

	cmd := exec.Command("ssh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("ssh failed: %w\n%s", err, string(out))
	}

	return string(out), nil
}

func runSCPFromVM(cfg RunnerConfig, remotePath string, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	target := fmt.Sprintf("%s@%s:%s", cfg.VMUser, cfg.VMHost, remotePath)

	args := sshArgs(cfg)
	args = append(args, target, localPath)

	cmd := exec.Command("scp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %w\n%s", err, string(out))
	}
	return nil
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func filepathDirRemote(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}
