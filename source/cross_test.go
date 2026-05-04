package source

import (
	"testing"
)

func TestCrossHarness_PathNormalization(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		projectRoot string
		want        string
	}{
		{"simple file with root", "/Users/hoang/burnwatch/src/main.go", "/Users/hoang/burnwatch", "src/main.go"},
		{"simple file no root", "src/main.go", "", "src/main.go"},
		{"dot prefix", "./src/main.go", "", "src/main.go"},
		{"dot prefix with root", "/Users/hoang/burnwatch/./src/main.go", "/Users/hoang/burnwatch", "src/main.go"},
		{"nested dir", "pkg/util/helper.go", "", "pkg/util/helper.go"},
		{"windows backslash no root", `src\app.js`, "", "src/app.js"},
		{"absolute with project root", "/Users/hoang/burnwatch/config/settings.json", "/Users/hoang/burnwatch", "config/settings.json"},
		{"relative no root", "config/settings.json", "", "config/settings.json"},
		{"claude format to opencode format", "/Users/hoang/proj/src/main.go", "/Users/hoang/proj", "src/main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePath(tt.path, tt.projectRoot)
			if got != tt.want {
				t.Errorf("NormalizePath(%q, %q) = %q, want %q", tt.path, tt.projectRoot, got, tt.want)
			}
		})
	}
}

func TestCrossHarness_ToolNameCanonicalization(t *testing.T) {
	tests := []struct {
		rawName string
		want    string
	}{
		{"Read", "read"},
		{"Write", "write"},
		{"Edit", "edit"},
		{"Glob", "glob"},
		{"Bash", "bash"},
		{"Skill", "skill"},
		{"read", "read"},
		{"write", "write"},
		{"edit", "edit"},
		{"glob", "glob"},
		{"READ_FILE", "read_file"},
		{"Execute_Command", "execute_command"},
	}

	for _, tt := range tests {
		t.Run(tt.rawName, func(t *testing.T) {
			got := canonicalizeToolName(tt.rawName)
			if got != tt.want {
				t.Errorf("canonicalizeToolName(%q) = %q, want %q", tt.rawName, got, tt.want)
			}
		})
	}
}

func TestCrossHarness_PathEquivalence(t *testing.T) {
	claudePath := "/Users/hoang/burnwatch/src/main.go"
	opencodePath := "./src/main.go"

	claudeNorm := NormalizePath(claudePath, "/Users/hoang/burnwatch")
	opencodeNorm := NormalizePath(opencodePath, "")

	if claudeNorm != opencodeNorm {
		t.Errorf("paths should normalize to same value: claude=%q opencode=%q", claudeNorm, opencodeNorm)
	}
	if claudeNorm != "src/main.go" {
		t.Errorf("expected src/main.go, got %q", claudeNorm)
	}
}
