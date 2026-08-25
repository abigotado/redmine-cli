// Command renderformula renders a local source-building Homebrew Formula.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	shaPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func main() {
	var version string
	var sourceURL string
	var sha256 string
	var output string
	flag.StringVar(&version, "version", "", "release version without v prefix")
	flag.StringVar(&sourceURL, "source-url", "", "immutable GitHub tag archive URL")
	flag.StringVar(&sha256, "sha256", "", "lowercase SHA-256 of the tag archive")
	flag.StringVar(&output, "output", "", "new local Formula path")
	flag.Parse()
	if err := run(version, sourceURL, sha256, output); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "renderformula: %v\n", err)
		os.Exit(1)
	}
}

func run(version, sourceURL, sha256, output string) error {
	if !versionPattern.MatchString(version) {
		return errors.New("--version must be a semantic version without a v prefix")
	}
	expectedURL := "https://github.com/abigotado/redmine-cli/releases/download/v" + version + "/redmine-cli-" + version + ".tar.gz"
	if sourceURL != expectedURL {
		return fmt.Errorf("--source-url must be the checksum-pinned release asset URL %s", expectedURL)
	}
	if !shaPattern.MatchString(sha256) {
		return errors.New("--sha256 must be exactly 64 lowercase hexadecimal characters")
	}
	if output == "" || !filepath.IsAbs(output) {
		return errors.New("--output must be an absolute local path")
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(root, "packaging", "homebrew", "redmine-agent-cli.rb.tmpl"))
	if err != nil {
		return fmt.Errorf("read Formula template: %w", err)
	}
	rendered := strings.NewReplacer(
		"{{VERSION}}", version,
		"{{SOURCE_URL}}", sourceURL,
		"{{SHA256}}", sha256,
	).Replace(string(raw))
	if strings.Contains(rendered, "{{") {
		return errors.New("Formula template contains an unresolved placeholder")
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	if _, err := file.WriteString(rendered); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("write output: %w", err), closeErr)
	}
	if err := file.Sync(); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("sync output: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	return nil
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found above the working directory")
		}
		dir = parent
	}
}
