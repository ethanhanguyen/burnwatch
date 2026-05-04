package source

import (
	"testing"
)

func TestNormalizePath_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		projectRoot string
		want        string
	}{
		{"empty path", "", "", ""},
		{"empty path with root", "", "/Users/proj", ""},
		{"root only", "/", "", ""},
		{"dot only", ".", "", ""},
		{"dot then clean", "./.", "", ""},
		{"dot dot segment", "../src/main.go", "", "../src/main.go"},
		{"dot dot within root", "/Users/proj/../other/main.go", "/Users/proj", "../other/main.go"},
		{"trailing slash", "src/main.go/", "", "src/main.go"},
		{"double dot dot", "src/../../etc/passwd", "", "../etc/passwd"},
		{"windows double backslash", `src\\app.js`, "", "src/app.js"},
		{"mixed slashes", `./src\lib/util.go`, "", "src/lib/util.go"},
		{"absolute to relative with root", "/Users/proj/src/main.go", "/Users/proj", "src/main.go"},
		{"absolute no match", "/other/path/file.go", "/Users/proj", "other/path/file.go"},
		{"root with trailing slash in project", "/Users/proj/src/main.go", "/Users/proj/", "src/main.go"},
		{"relative stays relative", "pkg/util.go", "", "pkg/util.go"},
		{"dot prefix with absolute", "/Users/proj/./src/main.go", "/Users/proj", "src/main.go"},
		{"config file path", "/home/user/.config/burnwatch/config.toml", "", "home/user/.config/burnwatch/config.toml"},
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
