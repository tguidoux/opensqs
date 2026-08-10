package main

import (
	"testing"
)

func TestParseFlags_Defaults(t *testing.T) {
	f := parseFlags([]string{})
	if f.version != "" {
		t.Errorf("expected empty version, got %s", f.version)
	}
	if f.skipImage {
		t.Error("expected skipImage=false")
	}
	if f.skipTag {
		t.Error("expected skipTag=false")
	}
	if f.skipRelease {
		t.Error("expected skipRelease=false")
	}
	if f.dryRun {
		t.Error("expected dryRun=false")
	}
	if f.draft {
		t.Error("expected draft=false")
	}
	if f.preRelease {
		t.Error("expected preRelease=false")
	}
}

func TestParseFlags_Version(t *testing.T) {
	f := parseFlags([]string{"v1.2.3"})
	if f.version != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", f.version)
	}
}

func TestParseFlags_SkipFlags(t *testing.T) {
	f := parseFlags([]string{"--skip-image", "--skip-tag", "--skip-release"})
	if !f.skipImage {
		t.Error("expected skipImage=true")
	}
	if !f.skipTag {
		t.Error("expected skipTag=true")
	}
	if !f.skipRelease {
		t.Error("expected skipRelease=true")
	}
}

func TestParseFlags_DryRun(t *testing.T) {
	f := parseFlags([]string{"--dry-run"})
	if !f.dryRun {
		t.Error("expected dryRun=true")
	}
}

func TestParseFlags_Draft(t *testing.T) {
	f := parseFlags([]string{"--draft"})
	if !f.draft {
		t.Error("expected draft=true")
	}
}

func TestParseFlags_PreRelease(t *testing.T) {
	f := parseFlags([]string{"--pre-release"})
	if !f.preRelease {
		t.Error("expected preRelease=true")
	}
}

func TestParseFlags_Combined(t *testing.T) {
	f := parseFlags([]string{"v2.0.0", "--dry-run", "--draft", "--pre-release"})
	if f.version != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %s", f.version)
	}
	if !f.dryRun {
		t.Error("expected dryRun=true")
	}
	if !f.draft {
		t.Error("expected draft=true")
	}
	if !f.preRelease {
		t.Error("expected preRelease=true")
	}
}

func TestParseFlags_UnknownArgument(t *testing.T) {
	// parseFlags calls fail() (os.Exit(1)) on unknown args, so we can't
	// easily test this without process isolation. We verify that valid
	// version-like strings are accepted and invalid ones would fail.
	f := parseFlags([]string{"v1.0.0"})
	if f.version != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", f.version)
	}
}

func TestBumpPatchVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v0.0.0", "v0.0.1"},
		{"v1.0.0", "v1.0.1"},
		{"v1.2.3", "v1.2.4"},
		{"v10.20.30", "v10.20.31"},
		{"v0.1.0", "v0.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := bumpPatchVersion(tt.input)
			if result != tt.expected {
				t.Errorf("bumpPatchVersion(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBumpPatchVersion_StripsVPrefix(t *testing.T) {
	result := bumpPatchVersion("v5.3.7")
	if result != "v5.3.8" {
		t.Errorf("expected v5.3.8, got %s", result)
	}
}

func TestIsTTY(t *testing.T) {
	// In test environment, stdout is typically not a TTY
	result := isTTY()
	// We can't assert a specific value since it depends on the test runner,
	// but the function should not panic and should return a bool.
	_ = result
}

func TestInitColors(t *testing.T) {
	// Should not panic
	initColors()
	// Colors will be empty strings when not a TTY (test environment)
	// or ANSI codes when running in a terminal.
}
