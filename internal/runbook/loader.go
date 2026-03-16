package runbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultDir returns the default runbook directory (~/.runbook/books/).
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runbook", "books")
}

// Load reads and parses a runbook from a YAML file.
func Load(path string) (*Runbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading runbook: %w", err)
	}

	var rb Runbook
	if err := yaml.Unmarshal(data, &rb); err != nil {
		return nil, fmt.Errorf("parsing runbook %s: %w", path, err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	rb.FilePath = abs

	return &rb, nil
}

// Discover finds all .yaml and .yml files in the given directory
// and one level of subdirectories (for pulled repos).
func Discover(dir string) ([]*Runbook, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var books []*Runbook
	for _, e := range entries {
		if e.IsDir() {
			// Scan subdirectories (pulled repos)
			subBooks, _ := discoverFlat(filepath.Join(dir, e.Name()))
			books = append(books, subBooks...)
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		rb, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		books = append(books, rb)
	}
	return books, nil
}

// discoverFlat finds YAML runbook files in a single directory (no recursion).
func discoverFlat(dir string) ([]*Runbook, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var books []*Runbook
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		rb, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		books = append(books, rb)
	}
	return books, nil
}

// Validate checks a runbook for common errors.
func Validate(rb *Runbook) []error {
	var errs []error

	if rb.Name == "" {
		errs = append(errs, fmt.Errorf("runbook has no name"))
	}
	if len(rb.Steps) == 0 {
		errs = append(errs, fmt.Errorf("runbook %q has no steps", rb.Name))
	}

	for i, s := range rb.Steps {
		if s.Name == "" {
			errs = append(errs, fmt.Errorf("step %d has no name", i+1))
		}
		switch s.Type {
		case StepShell:
			if s.Shell == nil || s.Shell.Command == "" {
				errs = append(errs, fmt.Errorf("step %d (%s): shell step requires a command", i+1, s.Name))
			}
		case StepSSH:
			if s.SSH == nil {
				errs = append(errs, fmt.Errorf("step %d (%s): ssh step requires ssh config", i+1, s.Name))
			} else {
				if s.SSH.Host == "" {
					errs = append(errs, fmt.Errorf("step %d (%s): ssh step requires a host", i+1, s.Name))
				}
				if s.SSH.Command == "" {
					errs = append(errs, fmt.Errorf("step %d (%s): ssh step requires a command", i+1, s.Name))
				}
			}
		case StepHTTP:
			if s.HTTP == nil {
				errs = append(errs, fmt.Errorf("step %d (%s): http step requires http config", i+1, s.Name))
			} else if s.HTTP.URL == "" {
				errs = append(errs, fmt.Errorf("step %d (%s): http step requires a url", i+1, s.Name))
			}
		case "":
			// Steps without a type are allowed if they only have a confirm field
			if s.Confirm == "" {
				errs = append(errs, fmt.Errorf("step %d (%s): missing type (shell, ssh, or http)", i+1, s.Name))
			}
		default:
			errs = append(errs, fmt.Errorf("step %d (%s): unknown type %q", i+1, s.Name, s.Type))
		}

		if s.OnError == PolicyRetry && s.Retries <= 0 {
			errs = append(errs, fmt.Errorf("step %d (%s): on_error is retry but retries is %d", i+1, s.Name, s.Retries))
		}
	}

	return errs
}

// FindRunbook looks up a runbook by name or file path.
func FindRunbook(nameOrPath string, dirs ...string) (*Runbook, error) {
	// Try as a direct file path first
	if _, err := os.Stat(nameOrPath); err == nil {
		return Load(nameOrPath)
	}

	// Try with yaml extension
	for _, ext := range []string{".yaml", ".yml"} {
		if _, err := os.Stat(nameOrPath + ext); err == nil {
			return Load(nameOrPath + ext)
		}
	}

	// Search in directories by name
	for _, dir := range dirs {
		books, err := Discover(dir)
		if err != nil {
			continue
		}
		for _, b := range books {
			if b.Name == nameOrPath {
				return b, nil
			}
		}
		// Also try filename match
		for _, ext := range []string{".yaml", ".yml"} {
			path := filepath.Join(dir, nameOrPath+ext)
			if _, err := os.Stat(path); err == nil {
				return Load(path)
			}
		}
	}

	return nil, fmt.Errorf("runbook %q not found", nameOrPath)
}
