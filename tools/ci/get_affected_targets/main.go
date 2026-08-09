package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
)

const (
	defaultBaseRef = "origin/main"
	configFileName = ".affected-targets-config.yaml"
)

// TargetTypeConfig holds filter configuration for a specific target type
type TargetTypeConfig struct {
	SkipFilePatterns   []string `yaml:"skip_file_patterns"`
	SkipExtensions     []string `yaml:"skip_extensions"`
	SkipTargetPatterns []string `yaml:"skip_target_patterns"`
}

// FilterConfig holds the configuration for all target types
// Global patterns are applied to all types, then type-specific patterns are added
type FilterConfig struct {
	Global TargetTypeConfig `yaml:"global"`
	Test   TargetTypeConfig `yaml:"test"`
	Build  TargetTypeConfig `yaml:"build"`
	Run    TargetTypeConfig `yaml:"run"`
}

var (
	log *logger.Logger
	ctx = context.Background()

	// Compiled patterns - populated from config based on requested target types
	skipPatterns       []*regexp.Regexp
	skipExtensions     []string
	skipTargetPatterns []*regexp.Regexp
)

type Config struct {
	BaseRef     string
	TargetTypes []string // "test", "binary", "run" or "all"
	Debug       bool
}

func main() {
	config := parseFlags()

	// Initialize logger with appropriate level
	var logLevel slog.Level
	if config.Debug {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}
	log = createStderrLogger("get_affected_targets", logLevel)

	if err := run(config); err != nil {
		log.Errorf(ctx, "Failed: %v", err)
		os.Exit(1)
	}
}

// createStderrLogger creates a logger that writes to stderr (instead of stdout)
// so that the target output on stdout remains clean for piping
func createStderrLogger(name string, level slog.Level) *logger.Logger {
	// Temporarily redirect stdout to stderr while creating the logger
	// This ensures the logger writes to stderr for this CLI tool
	oldStdout := os.Stdout
	os.Stdout = os.Stderr
	defer func() { os.Stdout = oldStdout }()

	return logger.NewLogger(name, level)
}

func parseFlags() *Config {
	config := &Config{}

	baseRef := flag.String("base", defaultBaseRef, "Git reference to compare against")
	targetTypes := flag.String("types", "test,binary,run", "Target types to find (comma-separated: test,binary,run,all)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	help := flag.Bool("help", false, "Show help")

	flag.Parse()

	if *help {
		showUsage()
		os.Exit(0)
	}

	config.BaseRef = *baseRef
	config.TargetTypes = strings.Split(*targetTypes, ",")
	config.Debug = *debug || os.Getenv("DEBUG") == "true"

	return config
}

func showUsage() {
	fmt.Fprintf(os.Stderr, `Usage: get_affected_targets [OPTIONS]

Find Bazel targets (run, build, test) affected by git diff changes.

Options:
  -base string
        Git reference to compare against (default: %s)
  -types string
        Target types to find: test,binary,run,all (default: test,binary,run)
  -debug
        Enable debug logging
  -help
        Show this help message

Environment Variables:
  DEBUG    Set to 'true' to enable debug logging

Examples:
  get_affected_targets -types test  # Find all affected test targets
  get_affected_targets -types binary,run  # Find all affected binary and run targets
  get_affected_targets -base origin/develop  # Compare against specific branch
  get_affected_targets -types all  # Find all types of targets
  bazel test $(get_affected_targets -types test)  # Run affected tests

Output:
  Space-separated list of Bazel targets to stdout
  Logging messages to stderr

Exit Codes:
  0: Success (with or without targets found)
  1: Error in git operations or bazel query
`, defaultBaseRef)
}

func run(config *Config) error {
	log.Info(ctx, "Analyzing changed files to determine affected Bazel targets")
	log.Infof(ctx, "Comparing against: %s", config.BaseRef)

	// Change to git repository root
	if err := changeToGitRoot(); err != nil {
		return fmt.Errorf("failed to change to git root: %w", err)
	}

	// Load filter configuration based on requested target types
	if err := loadFilterConfig(config.TargetTypes, config.Debug); err != nil {
		return fmt.Errorf("failed to load filter config: %w", err)
	}

	// Verify git reference exists
	if err := verifyGitRef(config.BaseRef); err != nil {
		return fmt.Errorf("invalid git reference %s: %w", config.BaseRef, err)
	}

	// Get changed files
	changedFiles, err := getChangedFiles(config.BaseRef)
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	if config.Debug {
		log.Debugf(ctx, "Changed files (%d):", len(changedFiles))
		for _, f := range changedFiles {
			log.Debugf(ctx, "  - %s", f)
		}
	}

	// Filter files
	sourceFiles := filterFiles(changedFiles, config.Debug)
	log.Infof(ctx, "Found %d source files that may affect targets", len(sourceFiles))

	if len(sourceFiles) == 0 {
		log.Info(ctx, "No source files changed that affect targets")
		// Output empty line to stdout so CI can detect no targets
		fmt.Println()
		return nil
	}

	// Convert files to Bazel packages
	packages := filesToPackages(sourceFiles, config.Debug)
	log.Infof(ctx, "Found %d unique Bazel packages", len(packages))

	if len(packages) == 0 {
		log.Info(ctx, "No Bazel packages found for changed files")
		// Output empty line to stdout so CI can detect no targets
		fmt.Println()
		return nil
	}

	// Determine query scope based on packages
	scope := determineQueryScope(packages)
	log.Infof(ctx, "Query scope: %s", scope)

	// Query affected targets - use scope as both search area and dependency source
	targets, err := queryAffectedTargets(packages, scope, config.TargetTypes, config.Debug)
	if err != nil {
		return fmt.Errorf("failed to query affected targets: %w", err)
	}

	if len(targets) == 0 {
		log.Info(ctx, "No affected targets found")
		// Output empty line to stdout so CI can detect no targets
		fmt.Println()
		return nil
	}

	log.Infof(ctx, "Found %d affected targets", len(targets))

	// Output targets to stdout (space-separated, single line)
	fmt.Println(strings.Join(targets, " "))

	return nil
}

// loadFilterConfig loads the filter configuration from the config file.
// Patterns are merged from all requested target types.
// If the config file doesn't exist or a target type has no config, no patterns are applied for that type.
func loadFilterConfig(targetTypes []string, debug bool) error {
	var filterConfig FilterConfig

	// Try to read config file from current directory (git root)
	configData, err := os.ReadFile(configFileName)
	if err == nil {
		log.Infof(ctx, "Loading filter config from %s", configFileName)
		if err := yaml.Unmarshal(configData, &filterConfig); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config file: %w", err)
	} else {
		log.Info(ctx, "No config file found, no patterns will be skipped")
		// No config file - don't skip anything
		skipPatterns = nil
		skipExtensions = nil
		skipTargetPatterns = nil
		return nil
	}

	// Merge configs from all requested target types
	mergedConfig := mergeTargetTypeConfigs(filterConfig, targetTypes, debug)

	// Compile file patterns
	skipPatterns = make([]*regexp.Regexp, 0, len(mergedConfig.SkipFilePatterns))
	for _, pattern := range mergedConfig.SkipFilePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid skip_file_pattern %q: %w", pattern, err)
		}
		skipPatterns = append(skipPatterns, re)
	}

	// Set extensions
	skipExtensions = mergedConfig.SkipExtensions

	// Compile target patterns
	skipTargetPatterns = make([]*regexp.Regexp, 0, len(mergedConfig.SkipTargetPatterns))
	for _, pattern := range mergedConfig.SkipTargetPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid skip_target_pattern %q: %w", pattern, err)
		}
		skipTargetPatterns = append(skipTargetPatterns, re)
	}

	if debug {
		log.Debugf(ctx, "Loaded %d skip file patterns", len(skipPatterns))
		log.Debugf(ctx, "Loaded %d skip extensions", len(skipExtensions))
		log.Debugf(ctx, "Loaded %d skip target patterns", len(skipTargetPatterns))
	}

	return nil
}

// unique returns a new slice with duplicate strings removed.
func unique(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// combineWithGlobal merges global patterns with type-specific patterns
func combineWithGlobal(global, typeSpecific TargetTypeConfig) TargetTypeConfig {
	return TargetTypeConfig{
		SkipFilePatterns:   unique(append(global.SkipFilePatterns, typeSpecific.SkipFilePatterns...)),
		SkipExtensions:     unique(append(global.SkipExtensions, typeSpecific.SkipExtensions...)),
		SkipTargetPatterns: unique(append(global.SkipTargetPatterns, typeSpecific.SkipTargetPatterns...)),
	}
}

// mergeTargetTypeConfigs merges configurations from multiple target types.
// Global patterns are first combined with each type-specific config.
// When multiple types are requested (e.g., test,binary), only patterns common
// to ALL types are used (intersection), ensuring nothing is incorrectly skipped.
func mergeTargetTypeConfigs(config FilterConfig, targetTypes []string, debug bool) TargetTypeConfig {
	if len(targetTypes) == 0 {
		return config.Global
	}

	// Get configs for each requested target type (combined with global)
	var configs []TargetTypeConfig
	for _, t := range targetTypes {
		t = strings.TrimSpace(strings.ToLower(t))
		var cfg TargetTypeConfig
		switch t {
		case "test":
			cfg = combineWithGlobal(config.Global, config.Test)
		case "build", "binary":
			cfg = combineWithGlobal(config.Global, config.Build)
		case "run":
			cfg = combineWithGlobal(config.Global, config.Run)
		case "all":
			// For "all", use intersection of all defined configs (each combined with global)
			configs = append(configs,
				combineWithGlobal(config.Global, config.Test),
				combineWithGlobal(config.Global, config.Build),
				combineWithGlobal(config.Global, config.Run),
			)
			continue
		default:
			if debug {
				log.Debugf(ctx, "Unknown target type %q, no patterns loaded", t)
			}
			continue
		}
		configs = append(configs, cfg)
	}

	if len(configs) == 0 {
		return config.Global
	}

	// If only one config, use it directly
	if len(configs) == 1 {
		if debug {
			log.Debugf(ctx, "Using config for single target type")
		}
		return configs[0]
	}

	// For multiple configs, use intersection (patterns common to ALL types)
	// This ensures we don't skip files that might be relevant to any requested type
	if debug {
		log.Debugf(ctx, "Using intersection of %d target type configs", len(configs))
	}

	return TargetTypeConfig{
		SkipFilePatterns:   intersectStrings(extractField(configs, "SkipFilePatterns")),
		SkipExtensions:     intersectStrings(extractField(configs, "SkipExtensions")),
		SkipTargetPatterns: intersectStrings(extractField(configs, "SkipTargetPatterns")),
	}
}

// extractField extracts a specific field from all configs
func extractField(configs []TargetTypeConfig, field string) [][]string {
	result := make([][]string, len(configs))
	for i, cfg := range configs {
		switch field {
		case "SkipFilePatterns":
			result[i] = cfg.SkipFilePatterns
		case "SkipExtensions":
			result[i] = cfg.SkipExtensions
		case "SkipTargetPatterns":
			result[i] = cfg.SkipTargetPatterns
		}
	}
	return result
}

// intersectStrings returns elements present in ALL input slices
func intersectStrings(slices [][]string) []string {
	if len(slices) == 0 {
		return nil
	}

	// Start with the first slice
	result := make(map[string]int)
	for _, s := range slices[0] {
		result[s] = 1
	}

	// Count occurrences across all slices
	for _, slice := range slices[1:] {
		seen := make(map[string]bool)
		for _, s := range slice {
			if _, exists := result[s]; exists && !seen[s] {
				result[s]++
				seen[s] = true
			}
		}
	}

	// Keep only elements present in all slices
	var intersection []string
	for s, count := range result {
		if count == len(slices) {
			intersection = append(intersection, s)
		}
	}

	sort.Strings(intersection)
	return intersection
}

func changeToGitRoot() error {
	// When running via `bazel run`, use BUILD_WORKING_DIRECTORY if available
	// This is the directory where the user ran bazel run from
	if buildWorkingDir := os.Getenv("BUILD_WORKING_DIRECTORY"); buildWorkingDir != "" {
		if err := os.Chdir(buildWorkingDir); err != nil {
			return err
		}
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	gitRoot := strings.TrimSpace(string(output))

	// Check if we're already in the git root
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// If already in git root, no need to change
	if cwd == gitRoot {
		return nil
	}

	return os.Chdir(gitRoot)
}

func verifyGitRef(ref string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func getChangedFiles(baseRef string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=ACMRTUXB", baseRef, "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var files []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}

	return files, scanner.Err()
}

func filterFiles(files []string, debug bool) []string {
	var filtered []string
	for _, file := range files {
		if shouldSkipFile(file, debug) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func shouldSkipFile(file string, debug bool) bool {
	// Check skip patterns
	for _, pattern := range skipPatterns {
		if pattern.MatchString(file) {
			if debug {
				log.Debugf(ctx, "Skipping (pattern): %s", file)
			}
			return true
		}
	}

	// Check skip extensions
	ext := filepath.Ext(file)
	for _, skipExt := range skipExtensions {
		if ext == skipExt {
			if debug {
				log.Debugf(ctx, "Skipping (extension): %s", file)
			}
			return true
		}
	}

	return false
}

func shouldSkipTarget(target string, debug bool) bool {
	for _, pattern := range skipTargetPatterns {
		if pattern.MatchString(target) {
			if debug {
				log.Debugf(ctx, "Skipping target (pattern match): %s", target)
			}
			return true
		}
	}
	return false
}

func filesToPackages(files []string, debug bool) []string {
	packageSet := make(map[string]bool)
	for _, file := range files {
		pkg := findPackage(file)
		if pkg != "" {
			packageSet[pkg] = true
		} else if debug {
			log.Debugf(ctx, "No BUILD file found for: %s", file)
		}
	}

	// Convert to sorted slice
	packages := make([]string, 0, len(packageSet))
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	return packages
}

func findPackage(file string) string {
	dir := filepath.Dir(file)

	// Walk up directories looking for BUILD.bazel or BUILD
	for {
		buildBazel := filepath.Join(dir, "BUILD.bazel")
		build := filepath.Join(dir, "BUILD")

		// Check if BUILD.bazel or BUILD exists in this directory
		if _, err := os.Stat(buildBazel); err == nil {
			if dir == "." {
				return "//:all"
			}
			return "//" + dir + ":all"
		}
		if _, err := os.Stat(build); err == nil {
			if dir == "." {
				return "//:all"
			}
			return "//" + dir + ":all"
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir || dir == "." {
			break
		}
		dir = parent
	}

	return ""
}

func determineQueryScope(packages []string) string {
	// Extract directory paths from packages
	paths := make([]string, 0, len(packages))
	hasRoot := false

	for _, pkg := range packages {
		pkg = strings.TrimPrefix(pkg, "//")
		if idx := strings.Index(pkg, ":"); idx != -1 {
			pkg = pkg[:idx]
		}

		if pkg == "" {
			hasRoot = true
		} else {
			paths = append(paths, pkg)
		}
	}

	// If ONLY root is affected and nothing else, query entire workspace
	if hasRoot && len(paths) == 0 {
		return "//..."
	}

	// If no paths at all, return entire workspace
	if len(paths) == 0 {
		return "//..."
	}

	// Sort paths to enable efficient parent-child deduplication
	sort.Strings(paths)

	// Remove redundant child paths - keep only parent directories
	// This minimizes the scope while ensuring we catch all dependencies
	uniquePaths := []string{paths[0]}
	for i := 1; i < len(paths); i++ {
		isChild := false
		for _, parent := range uniquePaths {
			// Check if current path is under an existing parent directory
			if strings.HasPrefix(paths[i], parent+"/") {
				isChild = true
				break
			}
		}
		if !isChild {
			uniquePaths = append(uniquePaths, paths[i])
		}
	}

	// Convert to Bazel scope format
	// Note: We don't include //:all in the scope even if root changed
	// because it forces Bazel to traverse the entire workspace in rdeps.
	// Instead, //:all is included in the package set for dependency analysis.
	scopes := make([]string, 0, len(uniquePaths))

	for _, path := range uniquePaths {
		scopes = append(scopes, "//"+path+"/...")
	}

	// If we have no scopes (only root changed), use focused top-level directories
	if len(scopes) == 0 {
		// Query only the main directories where tests typically exist
		return "//apps/... + //pkgs/..."
	}

	return strings.Join(scopes, " + ")
}

func queryAffectedTargets(packages []string, scope string, targetTypes []string, debug bool) ([]string, error) {
	// Build the kind filter based on target types
	kindFilter := buildKindFilter(targetTypes)

	// Create the set of packages for the query
	packageSet := strings.Join(packages, " ")

	// Use rdeps with the calculated scope to limit search area
	// This is much faster than rdeps(//..., ...) when scope is narrow
	query := fmt.Sprintf("kind(%s, rdeps(%s, set(%s)))", kindFilter, scope, packageSet)

	if debug {
		log.Debugf(ctx, "Bazel query: %s", query)
	}

	cmd := exec.Command("bazel", "query", query, "--output", "label", "--keep_going")
	output, err := cmd.CombinedOutput()

	// Check if error is because no targets match (this is ok)
	if err != nil {
		// Exit code 3 with --keep_going means there were errors but we got results
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			// Continue processing output despite errors
			if debug {
				log.Debugf(ctx, "Query completed with warnings (exit code 3)")
			}
		} else if strings.Contains(string(output), "empty query results") ||
			strings.Contains(string(output), "Couldn't find target") {
			return nil, nil
		} else {
			return nil, fmt.Errorf("bazel query failed: %w\nOutput: %s", err, string(output))
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var targets []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && strings.HasPrefix(line, "//") {
			if shouldSkipTarget(line, debug) {
				continue
			}
			targets = append(targets, line)
		}
	}

	sort.Strings(targets)

	return targets, scanner.Err()
}

func buildKindFilter(targetTypes []string) string {
	filterSet := make(map[string]bool)
	for _, t := range targetTypes {
		t = strings.TrimSpace(strings.ToLower(t))
		switch t {
		case "test":
			filterSet["test"] = true
		case "build", "binary":
			// Match any rule ending in _binary
			filterSet[".*_binary"] = true
		case "run":
			// Match runnable targets (binaries and rules with run)
			filterSet[".*_binary"] = true
			filterSet[".*_run"] = true
		case "all":
			// Match everything
			return "rule"
		}
	}

	if len(filterSet) == 0 {
		return "test"
	}

	// Convert set to sorted slice
	var filters []string
	for f := range filterSet {
		filters = append(filters, f)
	}
	sort.Strings(filters)

	// Join with | for regex OR and quote for Bazel query
	filter := strings.Join(filters, "|")
	// Bazel query requires regex patterns with special chars to be quoted
	if strings.ContainsAny(filter, ".*|") {
		filter = "\"" + filter + "\""
	}
	return filter
}
