package runbook

import (
	"os"
	"path/filepath"
	"testing"
)

var sampleYAML = `name: %s
steps:
  - name: s1
    type: shell
    shell:
      command: "echo ok"
`

func writeRunbook(t *testing.T, dir, name string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	content := []byte("name: " + name + "\nsteps:\n  - name: s1\n    type: shell\n    shell:\n      command: echo ok\n")
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverTopLevel(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "alpha")
	writeRunbook(t, dir, "beta")

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 2 {
		t.Fatalf("got %d runbooks, want 2", len(books))
	}
}

func TestDiscoverSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "local")
	writeRunbook(t, filepath.Join(dir, "repo", "system"), "remote")

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 2 {
		t.Fatalf("got %d runbooks, want 2", len(books))
	}
	names := map[string]bool{}
	for _, b := range books {
		names[b.Name] = true
	}
	if !names["local"] || !names["remote"] {
		t.Errorf("expected local and remote, got %v", names)
	}
}

func TestDiscoverDeduplicatePreferTopLevel(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "deploy")
	writeRunbook(t, filepath.Join(dir, "repo"), "deploy")

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("got %d runbooks, want 1 (deduplicated)", len(books))
	}
	// Top-level should win (shorter path)
	if !filepath.IsAbs(books[0].FilePath) {
		t.Errorf("expected absolute path, got %s", books[0].FilePath)
	}
}

func TestDiscoverSkipsTemplatesDir(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "real")
	writeRunbook(t, filepath.Join(dir, "repo", "templates"), "tmpl")

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("got %d runbooks, want 1 (templates excluded)", len(books))
	}
	if books[0].Name != "real" {
		t.Errorf("got %q, want %q", books[0].Name, "real")
	}
}

func TestDiscoverTemplates(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "real")
	writeRunbook(t, filepath.Join(dir, "repo", "templates"), "tmpl-a")
	writeRunbook(t, filepath.Join(dir, "repo", "templates"), "tmpl-b")

	templates, err := DiscoverTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 2 {
		t.Fatalf("got %d templates, want 2", len(templates))
	}
	names := map[string]bool{}
	for _, b := range templates {
		names[b.Name] = true
	}
	if !names["tmpl-a"] || !names["tmpl-b"] {
		t.Errorf("expected tmpl-a and tmpl-b, got %v", names)
	}
}

func TestDiscoverTemplatesEmpty(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "real")

	templates, err := DiscoverTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 0 {
		t.Fatalf("got %d templates, want 0", len(templates))
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	dir := t.TempDir()

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 0 {
		t.Fatalf("got %d runbooks, want 0", len(books))
	}
}

func TestDiscoverMissingDir(t *testing.T) {
	books, err := Discover("/nonexistent/dir")
	if err != nil {
		t.Fatal("expected nil error for missing dir")
	}
	if len(books) != 0 {
		t.Fatalf("got %d runbooks, want 0", len(books))
	}
}

func TestDiscoverSkipsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "good")
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("{{invalid"), 0o644)

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("got %d runbooks, want 1 (bad file skipped)", len(books))
	}
}
