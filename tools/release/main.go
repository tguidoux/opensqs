package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ─── ANSI Colors ────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorBold   = "\033[1m"
)

func info(format string, args ...any) {
	fmt.Printf("%sℹ️  %s%s\n", colorBlue, fmt.Sprintf(format, args...), colorReset)
}

func ok(format string, args ...any) {
	fmt.Printf("%s✅ %s%s\n", colorGreen, fmt.Sprintf(format, args...), colorReset)
}

func warn(format string, args ...any) {
	fmt.Printf("%s⚠️  %s%s\n", colorYellow, fmt.Sprintf(format, args...), colorReset)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s❌ %s%s\n", colorRed, fmt.Sprintf(format, args...), colorReset)
	os.Exit(1)
}

// ─── Configuration ───────────────────────────────────────────────────────────

const (
	ghcrRepo        = "ghcr.io/tguidoux/opensqs/opensqs-server"
	remoteName      = "origin"
	imagePushTarget = "//apps/go/server:opensqs_server_image_push"
	chartFilePath   = "deploy/helm/Chart.yaml"
	versionRegex    = `^v[0-9]+\.[0-9]+\.[0-9]+$`
)

// ─── Flags ───────────────────────────────────────────────────────────────────

type flags struct {
	version     string
	skipImage   bool
	skipTag     bool
	skipRelease bool
	dryRun      bool
}

func parseFlags(args []string) flags {
	f := flags{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--skip-image":
			f.skipImage = true
		case "--skip-tag":
			f.skipTag = true
		case "--skip-release":
			f.skipRelease = true
		case "--dry-run":
			f.dryRun = true
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			if strings.HasPrefix(arg, "v") && regexp.MustCompile(versionRegex).MatchString(arg) {
				f.version = arg
			} else {
				fail("Unknown argument: %s", arg)
			}
		}
	}
	return f
}

func printUsage() {
	fmt.Print(`Usage: release [VERSION] [OPTIONS]

Arguments:
  VERSION                 Semantic version (e.g., v1.0.0). If omitted, auto-increments patch.

Options:
  --skip-image            Skip Bazel image build and push
  --skip-tag              Skip git tag creation and push
  --skip-release          Skip GitHub Release creation
  --dry-run               Show what would happen without making changes
  -h, --help              Show this help message

Examples:
  release                        # Auto-increment: v0.1.0 → v0.1.1
  release v1.0.0                 # Release v1.0.0
  release v2.0.0 --skip-image    # Tag v2.0.0, skip image push
  release --dry-run              # Preview what would happen
`)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func checkCommand(name string) {
	if _, err := exec.LookPath(name); err != nil {
		fail("Required command not found: %s", name)
	}
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	return strings.TrimRight(string(output), "\n"), err
}

func runCommandInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getRepoRoot() string {
	// Bazel sets BUILD_WORKSPACE_DIRECTORY when running via `bazel run`
	if wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); wsDir != "" {
		return wsDir
	}
	// Fallback: use git to find the repo root
	output, err := runCommand("git", "rev-parse", "--show-toplevel")
	if err != nil {
		fail("Failed to find git repo root: %v", err)
	}
	return output
}

func getLatestTag() string {
	// Use --sort=-v:refname to get the highest semver tag, regardless of commit position
	output, err := runCommand("git", "tag", "--sort=-v:refname")
	if err == nil && output != "" {
		lines := strings.Split(output, "\n")
		return lines[0]
	}

	// No git tags — fall back to Helm chart version
	chartVer := getChartVersion()
	if chartVer != "" {
		return "v" + chartVer
	}
	return "v0.1.0"
}

func bumpPatchVersion(version string) string {
	stripped := strings.TrimPrefix(version, "v")
	parts := strings.Split(stripped, ".")
	if len(parts) != 3 {
		fail("Invalid version format: %s", version)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		fail("Invalid patch version in: %s", version)
	}
	patch++
	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch)
}

func getChartVersion() string {
	repoRoot := getRepoRoot()
	chartPath := filepath.Join(repoRoot, chartFilePath)

	file, err := os.Open(chartPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"`)
				return val
			}
		}
	}
	return ""
}

func updateChartVersion(version string, dryRun bool) {
	repoRoot := getRepoRoot()
	chartPath := filepath.Join(repoRoot, chartFilePath)
	stripped := strings.TrimPrefix(version, "v")

	if dryRun {
		info("[dry-run] Would update Chart.yaml: version=%s, appVersion=%s", stripped, version)
		return
	}

	content, err := os.ReadFile(chartPath)
	if err != nil {
		fail("Failed to read Chart.yaml: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "version:") {
			lines[i] = fmt.Sprintf(`version: "%s"`, stripped)
		} else if strings.HasPrefix(line, "appVersion:") {
			lines[i] = fmt.Sprintf(`appVersion: "%s"`, version)
		}
	}

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(chartPath, []byte(output), 0644); err != nil {
		fail("Failed to write Chart.yaml: %v", err)
	}

	ok("Updated Chart.yaml: version=%s, appVersion=%s", stripped, version)
}

func tagExists(tag string) bool {
	cmd := exec.Command("git", "rev-parse", tag)
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run() == nil
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	f := parseFlags(os.Args[1:])

	repoRoot := getRepoRoot()
	if err := os.Chdir(repoRoot); err != nil {
		fail("Failed to change to repo root: %v", err)
	}

	// ─── Preflight Checks ───────────────────────────────────────────────────
	info("Running preflight checks...")

	checkCommand("git")
	checkCommand("bazel")
	if !f.skipRelease {
		checkCommand("gh")
	}

	// Check working tree is clean
	status, _ := runCommand("git", "status", "--porcelain")
	if status != "" {
		fail("Working tree is not clean. Please commit or stash changes before releasing.\n%s", status)
	}

	// Check we're on a branch
	currentBranch, _ := runCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if currentBranch == "HEAD" {
		fail("Detached HEAD state. Please checkout a branch first.")
	}
	ok("On branch: %s", currentBranch)

	// ─── Determine Version ──────────────────────────────────────────────────
	// Capture the previous tag BEFORE creating the new one, so release notes
	// cover the correct commit range.
	previousTag := getLatestTag()

	if f.version == "" {
		f.version = bumpPatchVersion(previousTag)
		info("No version specified. Auto-incrementing: %s → %s", previousTag, f.version)
	} else {
		info("Using specified version: %s", f.version)
	}

	// Validate version format
	if !regexp.MustCompile(versionRegex).MatchString(f.version) {
		fail("Invalid version format. Expected: v1.2.3 (semver with 'v' prefix)")
	}

	// Check if tag already exists
	if tagExists(f.version) {
		fail("Tag %s already exists!", f.version)
	}

	strippedVersion := strings.TrimPrefix(f.version, "v")
	info("Release version: %s%s%s", colorBold, f.version, colorReset)

	// ─── Step 1: Update Helm Chart ──────────────────────────────────────────
	fmt.Println()
	info("%sStep 1: Update Helm Chart%s", colorBold, colorReset)

	currentChartVersion := getChartVersion()
	info("Current Chart.yaml: version=%s", currentChartVersion)

	updateChartVersion(f.version, f.dryRun)

	if !f.dryRun {
		if _, err := runCommand("git", "add", chartFilePath); err != nil {
			fail("Failed to stage Chart.yaml: %v", err)
		}
	}

	// ─── Step 2: Commit Version Bump ────────────────────────────────────────
	fmt.Println()
	info("%sStep 2: Commit version bump%s", colorBold, colorReset)

	commitMsg := fmt.Sprintf("release(%s): bump Helm chart to %s", f.version, strippedVersion)
	if f.dryRun {
		info("[dry-run] Would commit: %s", commitMsg)
	} else {
		if _, err := runCommand("git", "commit", "-m", commitMsg); err != nil {
			fail("Failed to commit version bump: %v", err)
		}
		ok("Committed version bump")
	}

	// ─── Step 3: Create and Push Git Tag ────────────────────────────────────
	fmt.Println()
	info("%sStep 3: Create and push git tag%s", colorBold, colorReset)

	if f.skipTag {
		warn("Skipping git tag creation (--skip-tag)")
	} else {
		if f.dryRun {
			info("[dry-run] Would create annotated tag: %s", f.version)
			info("[dry-run] Would push tag %s to %s", f.version, remoteName)
		} else {
			if _, err := runCommand("git", "tag", "-a", f.version, "-m", "Release "+f.version); err != nil {
				fail("Failed to create tag: %v", err)
			}
			ok("Created annotated tag: %s", f.version)

			if err := runCommandInteractive("git", "push", remoteName, f.version); err != nil {
				fail("Failed to push tag: %v", err)
			}
			ok("Pushed tag %s to %s", f.version, remoteName)
		}
	}

	// Push the version bump commit
	if !f.dryRun {
		if err := runCommandInteractive("git", "push", remoteName, currentBranch); err != nil {
			fail("Failed to push branch: %v", err)
		}
		ok("Pushed commits to %s/%s", remoteName, currentBranch)
	}

	// ─── Step 4: Build and Push OCI Image ────────────────────────────────────
	fmt.Println()
	info("%sStep 4: Build and push OCI image%s", colorBold, colorReset)

	if f.skipImage {
		warn("Skipping image build and push (--skip-image)")
	} else {
		if f.dryRun {
			info("[dry-run] Would run: bazel run %s --stamp --embed_label=%s", imagePushTarget, f.version)
		} else {
			info("Building and pushing OCI image to GHCR: %s:%s", ghcrRepo, f.version)
			if err := runCommandInteractive("bazel", "run", imagePushTarget, "--stamp", "--embed_label="+f.version); err != nil {
				fail("Failed to push image with version tag: %v", err)
			}
			ok("Image pushed: %s:%s", ghcrRepo, f.version)

			info("Pushing OCI image to GHCR: %s:latest", ghcrRepo)
			if err := runCommandInteractive("bazel", "run", imagePushTarget, "--stamp", "--embed_label=latest"); err != nil {
				fail("Failed to push image with latest tag: %v", err)
			}
			ok("Image pushed: %s:latest", ghcrRepo)
		}
	}

	// ─── Step 5: Create GitHub Release ──────────────────────────────────────
	fmt.Println()
	info("%sStep 5: Create GitHub Release%s", colorBold, colorReset)

	if f.skipRelease {
		warn("Skipping GitHub Release creation (--skip-release)")
	} else {
		if f.dryRun {
			info("[dry-run] Would create GitHub Release: %s", f.version)
		} else {
			// Generate release notes from commits since the previous tag.
			// previousTag was captured before the new tag was created.
			var releaseNotes string
			if previousTag == "v0.0.0" {
				releaseNotes, _ = runCommand("git", "log", "--pretty=format:- %s", "HEAD")
			} else {
				releaseNotes, _ = runCommand("git", "log", "--pretty=format:- %s", previousTag+"..HEAD")
			}

			notes := fmt.Sprintf("## Changes\n\n%s\n\n## Docker Image\n\n```\ndocker pull %s:%s\n```\n\n## Helm\n\n```bash\nhelm install opensqs deploy/helm \\\n  --set image.tag=%s\n```\n",
				releaseNotes, ghcrRepo, f.version, f.version)

			if err := runCommandInteractive("gh", "release", "create", f.version,
				"--title", "Release "+f.version,
				"--notes", notes,
				"--verify-tag",
			); err != nil {
				fail("Failed to create GitHub Release: %v", err)
			}
			ok("GitHub Release created: %s", f.version)
		}
	}

	// ─── Summary ─────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Printf("%s%s════════════════════════════════════════════════════%s\n", colorGreen, colorBold, colorReset)
	fmt.Printf("%s%s  🎉 Release %s completed successfully!%s\n", colorGreen, colorBold, f.version, colorReset)
	fmt.Printf("%s%s════════════════════════════════════════════════════%s\n", colorGreen, colorBold, colorReset)
	fmt.Println()
	fmt.Printf("  %sGit Tag:%s        %s\n", colorBold, colorReset, f.version)
	fmt.Printf("  %sImage:%s          %s:%s\n", colorBold, colorReset, ghcrRepo, f.version)
	fmt.Printf("  %sImage (latest):%s %s:latest\n", colorBold, colorReset, ghcrRepo)
	fmt.Printf("  %sHelm:%s           helm install opensqs deploy/helm --set image.tag=%s\n", colorBold, colorReset, f.version)
	fmt.Println()
}
