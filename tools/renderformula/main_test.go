package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRendersDeterministicMacOSSourceFormula(t *testing.T) {
	output := filepath.Join(t.TempDir(), "redmine-agent-cli.rb")
	version := "0.1.0"
	sourceURL := "https://github.com/abigotado/redmine-cli/releases/download/v0.1.0/redmine-cli-0.1.0.tar.gz"
	sha := strings.Repeat("a", 64)
	if err := run(version, sourceURL, sha, output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	formula := string(raw)
	for _, expected := range []string{
		`class RedmineAgentCli < Formula`,
		`depends_on :macos`,
		`ENV["CGO_ENABLED"] = "1"`,
		`ENV["GOPROXY"] = "off"`,
		`ENV["GOSUMDB"] = "off"`,
		`ENV["GOFLAGS"] = "-mod=vendor -trimpath"`,
		sourceURL,
		`version "0.1.0"`,
		`sha256 "` + sha + `"`,
		`releaseVersion=v0.1.0`,
		`Security.framework`,
	} {
		if !strings.Contains(formula, expected) {
			t.Fatalf("Formula missing %q:\n%s", expected, formula)
		}
	}
	if strings.Contains(formula, "{{") {
		t.Fatalf("Formula has unresolved placeholder:\n%s", formula)
	}
	resources, err := os.ReadFile(filepath.Join("..", "..", "packaging", "homebrew", "resources.tsv"))
	if err != nil {
		t.Fatalf("read Homebrew resources: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(resources)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("invalid Homebrew resource line %q", line)
		}
		module, resourceVersion, digest := fields[0], fields[1], fields[2]
		for _, expected := range []string{
			`resource "` + module + `"`,
			`url "https://proxy.golang.org/` + module + `/@v/` + resourceVersion + `.zip"`,
			`sha256 "` + digest + `"`,
			`resource("` + module + `").stage buildpath/"vendor/` + module + `"`,
		} {
			if !strings.Contains(formula, expected) {
				t.Fatalf("Formula missing resource contract %q", expected)
			}
		}
	}
	if err := run(version, sourceURL, sha, output); err == nil {
		t.Fatal("run() overwrote an existing Formula")
	}
}

func TestRunRejectsMutableOrMismatchedInputs(t *testing.T) {
	output := filepath.Join(t.TempDir(), "formula.rb")
	tests := []struct {
		name    string
		version string
		url     string
		sha     string
	}{
		{name: "branch URL", version: "0.1.0", url: "https://github.com/abigotado/redmine-cli/archive/refs/heads/main.tar.gz", sha: strings.Repeat("a", 64)},
		{name: "mismatched tag", version: "0.1.0", url: "https://github.com/abigotado/redmine-cli/releases/download/v0.2.0/redmine-cli-0.2.0.tar.gz", sha: strings.Repeat("a", 64)},
		{name: "alternate owner", version: "0.1.0", url: "https://github.com/example/redmine-cli/releases/download/v0.1.0/redmine-cli-0.1.0.tar.gz", sha: strings.Repeat("a", 64)},
		{name: "query", version: "0.1.0", url: "https://github.com/abigotado/redmine-cli/releases/download/v0.1.0/redmine-cli-0.1.0.tar.gz?download=1", sha: strings.Repeat("a", 64)},
		{name: "fragment", version: "0.1.0", url: "https://github.com/abigotado/redmine-cli/releases/download/v0.1.0/redmine-cli-0.1.0.tar.gz#asset", sha: strings.Repeat("a", 64)},
		{name: "uppercase checksum", version: "0.1.0", url: "https://github.com/abigotado/redmine-cli/archive/refs/tags/v0.1.0.tar.gz", sha: strings.Repeat("A", 64)},
		{name: "prerelease", version: "0.1.0-rc.1", url: "", sha: strings.Repeat("a", 64)},
		{name: "placeholder version", version: "VERSION", url: "", sha: strings.Repeat("a", 64)},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if err := run(testCase.version, testCase.url, testCase.sha, output); err == nil {
				t.Fatal("run() error = nil")
			}
		})
	}
}
