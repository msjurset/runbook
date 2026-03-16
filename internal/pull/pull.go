package pull

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitRepo clones or updates a git repository into the target directory.
// If the repo already exists, it pulls the latest changes.
func GitRepo(repoURL, targetDir string) (cloned bool, err error) {
	// Normalize URL: add https:// if missing
	if !strings.Contains(repoURL, "://") && !strings.HasPrefix(repoURL, "git@") {
		repoURL = "https://" + repoURL
	}

	repoName := repoBaseName(repoURL)
	dest := filepath.Join(targetDir, repoName)

	if isGitRepo(dest) {
		// Pull latest
		cmd := exec.Command("git", "-C", dest, "pull", "--ff-only")
		if out, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("git pull in %s: %s: %w", dest, strings.TrimSpace(string(out)), err)
		}
		return false, nil
	}

	// Clone
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return false, fmt.Errorf("creating target dir: %w", err)
	}

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git clone %s: %s: %w", repoURL, strings.TrimSpace(string(out)), err)
	}
	return true, nil
}

// SingleFile downloads a single YAML file from a URL into the target directory.
func SingleFile(fileURL, targetDir string) (string, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("creating target dir: %w", err)
	}

	// Normalize URL
	if !strings.Contains(fileURL, "://") {
		fileURL = "https://" + fileURL
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("downloading %s: HTTP %d", fileURL, resp.StatusCode)
	}

	filename := filepath.Base(fileURL)
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}

	dest := filepath.Join(targetDir, filename)
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(dest)
		return "", fmt.Errorf("writing %s: %w", dest, err)
	}

	return dest, nil
}

// ListRepos returns the names of git repos in the target directory.
func ListRepos(targetDir string) ([]string, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var repos []string
	for _, e := range entries {
		if e.IsDir() && isGitRepo(filepath.Join(targetDir, e.Name())) {
			repos = append(repos, e.Name())
		}
	}
	return repos, nil
}

// RemoveRepo removes a cloned repository.
func RemoveRepo(name, targetDir string) error {
	dest := filepath.Join(targetDir, name)
	if !isGitRepo(dest) {
		return fmt.Errorf("%q is not a pulled repository", name)
	}
	return os.RemoveAll(dest)
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func repoBaseName(url string) string {
	// Strip trailing .git
	url = strings.TrimSuffix(url, ".git")
	// Get last path component
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	if name == "" && len(parts) > 1 {
		name = parts[len(parts)-2]
	}
	return name
}
