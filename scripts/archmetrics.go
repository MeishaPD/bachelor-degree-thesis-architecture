// Semi-automatic metrics tool for Go backend architecture research.

// Features:
// 1. Structural complexity measurement:
//    - production LOC
//    - source files
//    - packages
//    - interfaces/ports
//    - internal dependency edges
//
// 2. Modifiability measurement from Git diff:
//    - changed files
//    - changed packages
//    - affected architectural components
//    - added LOC
//    - deleted LOC
//    - code churn
//    - relative churn
//    - contract/interface changed files
//
// Usage examples:
//
// go run archmetrics.go structure --arch layered --root ./layered --out results/structure_layered.csv --json results/structure_layered.json
//
// go run archmetrics.go structure --arch hexagonal --root ./hexagonal --out results/structure_hexagonal.csv --json results/structure_hexagonal.json
//
// go run archmetrics.go modifiability --scenario M1 --arch layered --root ./layered --base baseline --target HEAD --out results/mod_m1_layered.csv --json results/mod_m1_layered.json
//
// go run archmetrics.go modifiability --scenario M1 --arch hexagonal --root ./hexagonal --base baseline --target HEAD --out results/mod_m1_hexagonal.csv --json results/mod_m1_hexagonal.json

package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type StringSet map[string]struct{}

func NewStringSet() StringSet {
	return map[string]struct{}{}
}

func (s StringSet) Add(value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		s[value] = struct{}{}
	}
}

func (s StringSet) Len() int {
	return len(s)
}

func (s StringSet) Values() []string {
	values := make([]string, 0, len(s))
	for value := range s {
		values = append(values, value)
	}

	sortStrings(values)
	return values
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		j := i
		for j > 0 && values[j-1] > values[j] {
			values[j-1], values[j] = values[j], values[j-1]
			j--
		}
	}
}

type StructureMetrics struct {
	Timestamp               string   `json:"timestamp"`
	Architecture            string   `json:"architecture"`
	Root                    string   `json:"root"`
	LOCMethod               string   `json:"loc_method"`
	ProductionLOC           int      `json:"production_loc"`
	SourceFiles             int      `json:"source_files"`
	Packages                int      `json:"packages"`
	PackageNames            []string `json:"package_names"`
	InterfacePorts          int      `json:"interface_ports"`
	InternalDependencyEdges int      `json:"internal_dependency_edges"`
	InternalDependencies    []string `json:"internal_dependencies"`
	ExcludedRules           []string `json:"excluded_rules"`
}

type ModifiabilityMetrics struct {
	Timestamp                     string   `json:"timestamp"`
	Scenario                      string   `json:"scenario"`
	Architecture                  string   `json:"architecture"`
	Root                          string   `json:"root"`
	BaseRef                       string   `json:"base_ref"`
	TargetRef                     string   `json:"target_ref"`
	ChangedFiles                  int      `json:"changed_files"`
	ChangedFilePaths              []string `json:"changed_file_paths"`
	ChangedPackages               int      `json:"changed_packages"`
	ChangedPackageNames           []string `json:"changed_package_names"`
	AffectedComponents            int      `json:"affected_components"`
	AffectedComponentNames        []string `json:"affected_component_names"`
	AddedLOC                      int      `json:"added_loc"`
	DeletedLOC                    int      `json:"deleted_loc"`
	CodeChurn                     int      `json:"code_churn"`
	BaselineProductionLOC         int      `json:"baseline_production_loc"`
	RelativeChurnPct              float64  `json:"relative_churn_pct"`
	ContractInterfaceChangedFiles int      `json:"contract_interface_changed_files"`
	InterfaceDeclsInChangedFiles  int      `json:"interface_decls_in_changed_files"`
	Note                          string   `json:"note"`
}

type DiffRow struct {
	Added   int
	Deleted int
	Path    string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "structure":
		err = runStructure(os.Args[2:])
	case "modifiability":
		err = runModifiability(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		printUsage()
		err = fmt.Errorf("unknown subcommand: %s", os.Args[1])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`archmetrics - semi-automatic architecture metrics for Go backends

Subcommands:
  structure       Measure baseline structural complexity
  modifiability   Measure change impact from Git diff

Examples:
  go run archmetrics.go structure --arch layered --root ./layered --out results/structure_layered.csv --json results/structure_layered.json
  go run archmetrics.go structure --arch hexagonal --root ./hexagonal --out results/structure_hexagonal.csv --json results/structure_hexagonal.json

  go run archmetrics.go modifiability --scenario M1 --arch layered --root ./layered --base baseline --target HEAD --out results/mod_m1_layered.csv --json results/mod_m1_layered.json
  go run archmetrics.go modifiability --scenario M1 --arch hexagonal --root ./hexagonal --base baseline --target HEAD --out results/mod_m1_hexagonal.csv --json results/mod_m1_hexagonal.json

Notes:
  - Production files exclude *_test.go, generated files, vendor, .git, results, build, dist, tmp, bin, and node_modules.
  - Structural LOC uses tokei if installed. If tokei is not installed, this tool uses an internal LOC fallback.
  - AST is used to count packages, interface declarations, and internal dependency edges.
  - Git diff is used for modifiability metrics.
`)
}

func runStructure(args []string) error {
	flags := flag.NewFlagSet("structure", flag.ExitOnError)

	arch := flags.String("arch", "", "architecture name: layered or hexagonal")
	root := flags.String("root", ".", "path to architecture root")
	outCSV := flags.String("out", "", "output CSV path")
	outJSON := flags.String("json", "", "output JSON path")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *arch == "" {
		return errors.New("--arch is required")
	}

	metrics, err := MeasureStructure(*arch, *root)
	if err != nil {
		return err
	}

	if *outCSV != "" {
		if err := writeStructureCSV(*outCSV, metrics); err != nil {
			return err
		}
	}

	if *outJSON != "" {
		if err := writeJSON(*outJSON, metrics); err != nil {
			return err
		}
	}

	printJSON(metrics)
	return nil
}

func runModifiability(args []string) error {
	flags := flag.NewFlagSet("modifiability", flag.ExitOnError)

	scenario := flags.String("scenario", "", "scenario label, for example: M1, M2, M3")
	arch := flags.String("arch", "", "architecture name: layered or hexagonal")
	root := flags.String("root", ".", "path to architecture root")
	base := flags.String("base", "", "baseline git ref, tag, branch, or commit")
	target := flags.String("target", "HEAD", "target git ref")
	outCSV := flags.String("out", "", "output CSV path")
	outJSON := flags.String("json", "", "output JSON path")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *scenario == "" {
		return errors.New("--scenario is required")
	}

	if *arch == "" {
		return errors.New("--arch is required")
	}

	if *base == "" {
		return errors.New("--base is required")
	}

	metrics, err := MeasureModifiability(*scenario, *arch, *root, *base, *target)
	if err != nil {
		return err
	}

	if *outCSV != "" {
		if err := writeModifiabilityCSV(*outCSV, metrics); err != nil {
			return err
		}
	}

	if *outJSON != "" {
		if err := writeJSON(*outJSON, metrics); err != nil {
			return err
		}
	}

	printJSON(metrics)
	return nil
}

func MeasureStructure(arch string, root string) (StructureMetrics, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return StructureMetrics{}, err
	}

	files, err := collectProductionGoFiles(absRoot)
	if err != nil {
		return StructureMetrics{}, err
	}

	productionLOC, locMethod := productionLOCWithTokeiOrFallback(absRoot, files)

	packages := NewStringSet()
	internalDependencies := NewStringSet()
	interfaceCount := 0

	modulePath := readGoModulePath(absRoot)

	for _, filePath := range files {
		relativePath := mustRel(absRoot, filePath)
		packageDirectory := filepath.ToSlash(filepath.Dir(relativePath))

		if packageDirectory == "." {
			packageDirectory = "root"
		}

		fileSet := token.NewFileSet()
		parsedFile, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		packageKey := packageDirectory + ":" + parsedFile.Name.Name
		packages.Add(packageKey)

		ast.Inspect(parsedFile, func(node ast.Node) bool {
			typeSpec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}

			if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				interfaceCount++
			}

			return true
		})

		fromPackage := packageDirectory

		for _, imported := range parsedFile.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")

			if isInternalProjectImport(importPath, modulePath) {
				toPackage := internalImportToPackage(importPath, modulePath)
				internalDependencies.Add(fromPackage + " -> " + toPackage)
			}
		}
	}

	return StructureMetrics{
		Timestamp:               time.Now().Format(time.RFC3339),
		Architecture:            arch,
		Root:                    absRoot,
		LOCMethod:               locMethod,
		ProductionLOC:           productionLOC,
		SourceFiles:             len(files),
		Packages:                packages.Len(),
		PackageNames:            packages.Values(),
		InterfacePorts:          interfaceCount,
		InternalDependencyEdges: internalDependencies.Len(),
		InternalDependencies:    internalDependencies.Values(),
		ExcludedRules: []string{
			"*_test.go",
			"generated files",
			".git",
			"vendor",
			"node_modules",
			"results",
			"benchmark-results",
			"build",
			"dist",
			"tmp",
			"bin",
		},
	}, nil
}

func MeasureModifiability(scenario string, arch string, root string, baseRef string, targetRef string) (ModifiabilityMetrics, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ModifiabilityMetrics{}, err
	}

	repoRoot, err := gitRepoRoot(absRoot)
	if err != nil {
		return ModifiabilityMetrics{}, err
	}

	rootRel := filepath.ToSlash(mustRel(repoRoot, absRoot))

	diffRows, err := gitDiffNumstat(repoRoot, baseRef, targetRef, rootRel)
	if err != nil {
		return ModifiabilityMetrics{}, err
	}

	changedFiles := NewStringSet()
	changedPackages := NewStringSet()
	affectedComponents := NewStringSet()

	addedLOC := 0
	deletedLOC := 0
	contractInterfaceChangedFiles := 0
	interfaceDeclsInChangedFiles := 0

	for _, row := range diffRows {
		path := filepath.ToSlash(row.Path)

		if !isProductionGoPath(path) {
			continue
		}

		if rootRel != "." && rootRel != "" {
			prefix := strings.TrimSuffix(rootRel, "/") + "/"
			if !strings.HasPrefix(path, prefix) && path != rootRel {
				continue
			}
		}

		relToArch := path
		if rootRel != "." && rootRel != "" {
			relToArch = strings.TrimSuffix(rootRel, "/") + "/"
		}

		changedFiles.Add(relToArch)
		changedPackages.Add(packageFromPath(relToArch))
		affectedComponents.Add(componentForPath(arch, relToArch))

		addedLOC += row.Added
		deletedLOC += row.Deleted

		if isContractPath(relToArch) {
			contractInterfaceChangedFiles++
		} else {
			hasInterfaceBefore := hasInterfaceInGitFile(repoRoot, baseRef, path)
			hasInterfaceAfter := hasInterfaceInGitFile(repoRoot, targetRef, path)

			if hasInterfaceBefore || hasInterfaceAfter {
				contractInterfaceChangedFiles++
			}
		}

		interfaceDeclsInChangedFiles += countInterfacesInGitFile(repoRoot, targetRef, path)
	}

	baselineProductionLOC, err := baselineProductionLOCFromGit(repoRoot, rootRel, baseRef)
	if err != nil {
		return ModifiabilityMetrics{}, err
	}

	codeChurn := addedLOC + deletedLOC

	relativeChurnPct := 0.0
	if baselineProductionLOC > 0 {
		relativeChurnPct = float64(codeChurn) / float64(baselineProductionLOC) * 100.0
	}

	note := "Contract/interface changes are detected semi-automatically from port/interface paths or changed files containing interface declarations. Validate manually for final thesis interpretation."

	return ModifiabilityMetrics{
		Timestamp:                     time.Now().Format(time.RFC3339),
		Scenario:                      scenario,
		Architecture:                  arch,
		Root:                          absRoot,
		BaseRef:                       baseRef,
		TargetRef:                     targetRef,
		ChangedFiles:                  changedFiles.Len(),
		ChangedFilePaths:              changedFiles.Values(),
		ChangedPackages:               changedPackages.Len(),
		ChangedPackageNames:           changedPackages.Values(),
		AffectedComponents:            affectedComponents.Len(),
		AffectedComponentNames:        affectedComponents.Values(),
		AddedLOC:                      addedLOC,
		DeletedLOC:                    deletedLOC,
		CodeChurn:                     codeChurn,
		BaselineProductionLOC:         baselineProductionLOC,
		RelativeChurnPct:              round2(relativeChurnPct),
		ContractInterfaceChangedFiles: contractInterfaceChangedFiles,
		InterfaceDeclsInChangedFiles:  interfaceDeclsInChangedFiles,
		Note:                          note,
	}, nil
}

func gitRepoRoot(directory string) (string, error) {
	output, err := exec.Command("git", "-C", directory, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("directory is not inside a git repository: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func gitDiffNumstat(repoRoot string, baseRef string, targetRef string, rootRel string) ([]DiffRow, error) {
	args := []string{
		"-C",
		repoRoot,
		"diff",
		"--numstat",
		baseRef,
		targetRef,
		"--",
	}

	if rootRel != "" && rootRel != "." {
		args = append(args, rootRel)
	}

	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	rows := []DiffRow{}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 3)

		if len(parts) != 3 {
			continue
		}

		added := atoiDash(parts[0])
		deleted := atoiDash(parts[1])
		path := normalizeGitPath(parts[2])

		rows = append(rows, DiffRow{
			Added:   added,
			Deleted: deleted,
			Path:    path,
		})
	}

	return rows, scanner.Err()
}

func normalizeGitPath(path string) string {
	path = strings.TrimSpace(path)

	if strings.Contains(path, " => ") {
		path = strings.ReplaceAll(path, "{", "")
		path = strings.ReplaceAll(path, "}", "")

		parts := strings.Split(path, " => ")
		path = parts[len(parts)-1]
	}

	return filepath.ToSlash(path)
}

func atoiDash(value string) int {
	if value == "-" {
		return 0
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return number
}

func baselineProductionLOCFromGit(repoRoot string, rootRel string, ref string) (int, error) {
	args := []string{
		"-C",
		repoRoot,
		"ls-tree",
		"-r",
		"--name-only",
		ref,
	}

	if rootRel != "" && rootRel != "." {
		args = append(args, rootRel)
	}

	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return 0, fmt.Errorf("git ls-tree failed: %w", err)
	}

	totalLOC := 0

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		path := filepath.ToSlash(scanner.Text())

		if !isProductionGoPath(path) {
			continue
		}

		content, err := gitShowFile(repoRoot, ref, path)
		if err != nil {
			continue
		}

		if isGeneratedContent(content) {
			continue
		}

		totalLOC += countCodeLOC(content)
	}

	return totalLOC, scanner.Err()
}

func gitShowFile(repoRoot string, ref string, path string) ([]byte, error) {
	output, err := exec.Command("git", "-C", repoRoot, "show", ref+":"+path).Output()
	if err != nil {
		return nil, err
	}

	return output, nil
}

func hasInterfaceInGitFile(repoRoot string, ref string, path string) bool {
	return countInterfacesInGitFile(repoRoot, ref, path) > 0
}

func countInterfacesInGitFile(repoRoot string, ref string, path string) int {
	content, err := gitShowFile(repoRoot, ref, path)
	if err != nil {
		return 0
	}

	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fileSet, path, content, parser.ParseComments)
	if err != nil {
		return 0
	}

	count := 0

	ast.Inspect(parsedFile, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}

		if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
			count++
		}

		return true
	})

	return count
}

func collectProductionGoFiles(root string) ([]string, error) {
	files := []string{}

	err := filepath.WalkDir(root, func(path string, directoryEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := directoryEntry.Name()

		if directoryEntry.IsDir() {
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}

			return nil
		}

		if !isProductionGoPath(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if isGeneratedContent(content) {
			return nil
		}

		files = append(files, path)
		return nil
	})

	sortStrings(files)
	return files, err
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git",
		"vendor",
		"node_modules",
		"results",
		"benchmark-results",
		"build",
		"dist",
		"tmp",
		"bin",
		".idea",
		".vscode":
		return true
	default:
		return false
	}
}

func isProductionGoPath(path string) bool {
	path = filepath.ToSlash(path)
	baseName := filepath.Base(path)

	if !strings.HasSuffix(baseName, ".go") {
		return false
	}

	if strings.HasSuffix(baseName, "_test.go") {
		return false
	}

	parts := strings.Split(path, "/")
	for _, part := range parts {
		if shouldSkipDir(part) {
			return false
		}
	}

	return true
}

func isGeneratedContent(content []byte) bool {
	firstPart := string(content)
	if len(firstPart) > 2048 {
		firstPart = firstPart[:2048]
	}

	return strings.Contains(firstPart, "Code generated") && strings.Contains(firstPart, "DO NOT EDIT")
}

func productionLOCWithTokeiOrFallback(root string, files []string) (int, string) {
	_, err := exec.LookPath("tokei")

	if err == nil && len(files) > 0 && len(files) < 500 {
		args := []string{"--output", "json"}
		args = append(args, files...)

		output, err := exec.Command("tokei", args...).Output()
		if err == nil {
			var raw map[string]struct {
				Code int `json:"code"`
			}

			if json.Unmarshal(output, &raw) == nil {
				if goStats, ok := raw["Go"]; ok {
					return goStats.Code, "tokei_filtered_files"
				}
			}
		}
	}

	totalLOC := 0

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		totalLOC += countCodeLOC(content)
	}

	return totalLOC, "internal_nonblank_noncomment_fallback"
}

func countCodeLOC(content []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(content))

	count := 0
	inBlockComment := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false

				afterComment := strings.TrimSpace(line[strings.Index(line, "*/")+2:])
				if afterComment != "" && !strings.HasPrefix(afterComment, "//") {
					count++
				}
			}

			continue
		}

		if strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "/*") {
			if !strings.Contains(line, "*/") {
				inBlockComment = true
			}

			afterComment := ""
			if index := strings.Index(line, "*/"); index >= 0 {
				afterComment = strings.TrimSpace(line[index+2:])
			}

			if afterComment != "" && !strings.HasPrefix(afterComment, "//") {
				count++
			}

			continue
		}

		count++
	}

	return count
}

func readGoModulePath(root string) string {
	directory := root

	for {
		content, err := os.ReadFile(filepath.Join(directory, "go.mod"))
		if err == nil {
			scanner := bufio.NewScanner(bytes.NewReader(content))

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())

				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
			}
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}

		directory = parent
	}

	return ""
}

func isInternalProjectImport(importPath string, modulePath string) bool {
	if modulePath != "" && strings.HasPrefix(importPath, modulePath+"/") {
		return true
	}

	return strings.Contains(importPath, "/internal/")
}

func internalImportToPackage(importPath string, modulePath string) string {
	if modulePath != "" {
		importPath = strings.TrimPrefix(importPath, modulePath+"/")
	}

	return importPath
}

func packageFromPath(path string) string {
	directory := filepath.ToSlash(filepath.Dir(path))

	if directory == "." {
		return "root"
	}

	return directory
}

func componentForPath(arch string, path string) string {
	path = filepath.ToSlash(strings.ToLower(path))
	arch = strings.ToLower(arch)

	if strings.Contains(arch, "hex") {
		switch {
		case strings.Contains(path, "internal/adapter/inbound"):
			return "inbound_adapter"
		case strings.Contains(path, "internal/adapter/outbound"):
			return "outbound_adapter"
		case strings.Contains(path, "internal/core/port/input"):
			return "input_port"
		case strings.Contains(path, "internal/core/port/output"):
			return "output_port"
		case strings.Contains(path, "internal/core/port"):
			return "port"
		case strings.Contains(path, "internal/core/usecase") || strings.Contains(path, "internal/usecase"):
			return "usecase"
		case strings.Contains(path, "internal/core/domain") || strings.Contains(path, "internal/domain"):
			return "domain"
		case strings.Contains(path, "internal/router"):
			return "router"
		case strings.Contains(path, "internal/db") || strings.Contains(path, "internal/database"):
			return "infrastructure_db"
		case strings.Contains(path, "cmd/"):
			return "app_entrypoint"
		case strings.Contains(path, "config"):
			return "config"
		default:
			return "other"
		}
	}

	switch {
	case strings.Contains(path, "internal/router"):
		return "router"
	case strings.Contains(path, "internal/handler"):
		return "handler"
	case strings.Contains(path, "internal/service"):
		return "service"
	case strings.Contains(path, "internal/repository"):
		return "repository"
	case strings.Contains(path, "internal/model") || strings.Contains(path, "internal/entity"):
		return "model"
	case strings.Contains(path, "internal/db") || strings.Contains(path, "internal/database"):
		return "infrastructure_db"
	case strings.Contains(path, "cmd/"):
		return "app_entrypoint"
	case strings.Contains(path, "config"):
		return "config"
	default:
		return "other"
	}
}

func isContractPath(path string) bool {
	path = filepath.ToSlash(strings.ToLower(path))

	return strings.Contains(path, "/port/") ||
		strings.Contains(path, "/ports/") ||
		strings.Contains(path, "interface")
}

func writeStructureCSV(path string, metrics StructureMetrics) error {
	if err := ensureParent(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{
		"timestamp",
		"architecture",
		"root",
		"loc_method",
		"production_loc",
		"source_files",
		"packages",
		"interfaces_ports",
		"internal_dependency_edges",
	})
	if err != nil {
		return err
	}

	err = writer.Write([]string{
		metrics.Timestamp,
		metrics.Architecture,
		metrics.Root,
		metrics.LOCMethod,
		strconv.Itoa(metrics.ProductionLOC),
		strconv.Itoa(metrics.SourceFiles),
		strconv.Itoa(metrics.Packages),
		strconv.Itoa(metrics.InterfacePorts),
		strconv.Itoa(metrics.InternalDependencyEdges),
	})
	if err != nil {
		return err
	}

	return writer.Error()
}

func writeModifiabilityCSV(path string, metrics ModifiabilityMetrics) error {
	if err := ensureParent(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{
		"timestamp",
		"scenario",
		"architecture",
		"root",
		"base_ref",
		"target_ref",
		"changed_files",
		"changed_packages",
		"affected_components",
		"affected_component_names",
		"added_loc",
		"deleted_loc",
		"code_churn",
		"baseline_production_loc",
		"relative_churn_pct",
		"contract_interface_changed_files",
		"interface_decls_in_changed_files",
	})

	if err != nil {
		return err
	}

	err = writer.Write([]string{
		metrics.Timestamp,
		metrics.Scenario,
		metrics.Architecture,
		metrics.Root,
		metrics.BaseRef,
		metrics.TargetRef,
		strconv.Itoa(metrics.ChangedFiles),
		strconv.Itoa(metrics.ChangedPackages),
		strconv.Itoa(metrics.AffectedComponents),
		strings.Join(metrics.AffectedComponentNames, ";"),
		strconv.Itoa(metrics.AddedLOC),
		strconv.Itoa(metrics.DeletedLOC),
		strconv.Itoa(metrics.CodeChurn),
		strconv.Itoa(metrics.BaselineProductionLOC),
		fmt.Sprintf("%.2f", metrics.RelativeChurnPct),
		strconv.Itoa(metrics.ContractInterfaceChangedFiles),
		strconv.Itoa(metrics.InterfaceDeclsInChangedFiles),
	})

	if err != nil {
		return err
	}

	return writer.Error()
}

func writeJSON(path string, value any) error {
	if err := ensureParent(path); err != nil {
		return err
	}

	content, err := json.MarshalIndent(value, "", " ")
	if err != nil {
		return err
	}

	content = append(content, '\n')
	return os.WriteFile(path, content, 0644)
}

func printJSON(value any) {
	content, _ := json.MarshalIndent(value, "", " ")
	fmt.Println(string(content))
}

func ensureParent(path string) error {
	directory := filepath.Dir(path)

	if directory == "." || directory == "" {
		return nil
	}

	return os.MkdirAll(directory, 0755)
}

func mustRel(base string, target string) string {
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}

	return filepath.ToSlash(relative)
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
