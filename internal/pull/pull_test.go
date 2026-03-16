package pull

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoBaseName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/runbooks", "runbooks"},
		{"https://github.com/user/runbooks.git", "runbooks"},
		{"github.com/user/my-books", "my-books"},
		{"git@github.com:user/ops.git", "ops"},
	}
	for _, tt := range tests {
		if got := repoBaseName(tt.url); got != tt.want {
			t.Errorf("repoBaseName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestIsGitRepo(t *testing.T) {
	dir := t.TempDir()

	if isGitRepo(dir) {
		t.Error("empty dir should not be a git repo")
	}

	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	if !isGitRepo(dir) {
		t.Error("dir with .git should be a git repo")
	}
}

func TestSingleFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("name: test-remote\nsteps:\n  - name: echo\n    type: shell\n    shell:\n      command: echo hi\n"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest, err := SingleFile(srv.URL+"/test-book.yaml", dir)
	if err != nil {
		t.Fatalf("SingleFile() error: %v", err)
	}

	if filepath.Base(dest) != "test-book.yaml" {
		t.Errorf("filename = %q, want test-book.yaml", filepath.Base(dest))
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got == "" {
		t.Error("downloaded file is empty")
	}
}

func TestSingleFile404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := SingleFile(srv.URL+"/missing.yaml", dir)
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestListReposEmpty(t *testing.T) {
	dir := t.TempDir()
	repos, err := ListRepos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Errorf("ListRepos() = %v, want empty", repos)
	}
}

func TestListRepos(t *testing.T) {
	dir := t.TempDir()

	// Create fake repo dirs
	os.MkdirAll(filepath.Join(dir, "repo1", ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, "repo2", ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, "not-a-repo"), 0o755)

	repos, err := ListRepos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Errorf("ListRepos() = %d repos, want 2", len(repos))
	}
}

func TestRemoveRepoNotARepo(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "plain"), 0o755)

	err := RemoveRepo("plain", dir)
	if err == nil {
		t.Error("expected error for non-repo dir")
	}
}

func TestRemoveRepo(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "myrepo", ".git"), 0o755)

	err := RemoveRepo("myrepo", dir)
	if err != nil {
		t.Fatalf("RemoveRepo() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "myrepo")); !os.IsNotExist(err) {
		t.Error("repo dir should have been removed")
	}
}
