package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestList(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"deploy-2026-04-29T100000.yaml",
		"deploy-2026-04-28T100000.yaml",
		"check-my-pi-2026-04-29T120000.yaml",
		"runbook-2026-04-29T130000.tar.gz",
		"unrelated.txt",
		"deploy.yaml", // no timestamp — must be skipped
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("want 4 entries, got %d", len(entries))
	}
	// Newest-first ordering: 2026-04-29T130000 (snapshot), 2026-04-29T120000, 2026-04-29T100000, 2026-04-28T100000
	if entries[0].RunbookName != "runbook" || entries[0].Kind != KindSnapshot {
		t.Errorf("first should be snapshot, got %+v", entries[0])
	}
	if entries[3].RunbookName != "deploy" || !entries[3].Timestamp.Equal(mustTime("2026-04-28T100000")) {
		t.Errorf("last should be deploy@2026-04-28T100000, got %+v", entries[3])
	}

	// Filter by name
	deploys, err := List(dir, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if len(deploys) != 2 {
		t.Fatalf("want 2 deploys, got %d", len(deploys))
	}
	for _, e := range deploys {
		if e.RunbookName != "deploy" {
			t.Errorf("non-deploy entry leaked through filter: %+v", e)
		}
	}
}

func TestListMissingDir(t *testing.T) {
	entries, err := List(filepath.Join(t.TempDir(), "nope"), "")
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing dir should give zero entries, got %d", len(entries))
	}
}

func TestFind(t *testing.T) {
	entries := []Entry{
		{RunbookName: "deploy", Timestamp: mustTime("2026-04-29T120000")},
		{RunbookName: "deploy", Timestamp: mustTime("2026-04-29T080000")},
		{RunbookName: "deploy", Timestamp: mustTime("2026-04-28T100000")},
		{RunbookName: "other", Timestamp: mustTime("2026-04-29T120000")},
	}

	// Newest by name
	got := Find(entries, "deploy", "")
	if got == nil || !got.Timestamp.Equal(mustTime("2026-04-29T120000")) {
		t.Errorf("newest deploy: got %+v", got)
	}
	// By prefix — the most recent matching the prefix wins
	got = Find(entries, "deploy", "2026-04-29T08")
	if got == nil || !got.Timestamp.Equal(mustTime("2026-04-29T080000")) {
		t.Errorf("prefix match: got %+v", got)
	}
	// No match
	got = Find(entries, "deploy", "2026-01")
	if got != nil {
		t.Errorf("no match expected, got %+v", got)
	}
}

func TestPrune_KeepPerName(t *testing.T) {
	dir := t.TempDir()
	for _, ts := range []string{
		"2026-04-29T100000",
		"2026-04-28T100000",
		"2026-04-27T100000",
		"2026-04-26T100000",
		"2026-04-25T100000",
	} {
		if err := os.WriteFile(filepath.Join(dir, "deploy-"+ts+".yaml"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := Prune(dir, 2, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 3 {
		t.Fatalf("want 3 deleted, got %d", len(deleted))
	}
	// The 3 oldest should be deleted, the 2 newest kept
	remaining, _ := List(dir, "deploy")
	if len(remaining) != 2 {
		t.Fatalf("want 2 remaining, got %d", len(remaining))
	}
	if !remaining[0].Timestamp.Equal(mustTime("2026-04-29T100000")) ||
		!remaining[1].Timestamp.Equal(mustTime("2026-04-28T100000")) {
		t.Errorf("wrong files remained: %+v", remaining)
	}
}

func TestPrune_OlderThan(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration) {
		ts := now.Add(-age).Format(timestampLayout)
		f := filepath.Join(dir, name+"-"+ts+".yaml")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("deploy", 5*time.Hour)
	mk("deploy", 30*24*time.Hour)
	mk("deploy", 60*24*time.Hour)

	deleted, err := Prune(dir, 0, 7*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 {
		t.Fatalf("want 2 deleted (older than 7d), got %d", len(deleted))
	}
}

func TestPrune_DryRun(t *testing.T) {
	dir := t.TempDir()
	for _, ts := range []string{"2026-04-29T100000", "2026-04-28T100000", "2026-04-27T100000"} {
		if err := os.WriteFile(filepath.Join(dir, "deploy-"+ts+".yaml"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := Prune(dir, 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(deleted))
	}
	// Files still on disk
	remaining, _ := List(dir, "deploy")
	if len(remaining) != 3 {
		t.Fatalf("dry-run should not delete; got %d remaining", len(remaining))
	}
}

func TestSnapshot(t *testing.T) {
	home := t.TempDir()
	booksDir := filepath.Join(home, "books")
	historyDir := filepath.Join(home, "history")
	for _, d := range []string{booksDir, historyDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(booksDir, "deploy.yaml"), []byte("name: deploy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(historyDir, "rec.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "pinned.json"), []byte(`["deploy"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	// highlights.yaml intentionally missing — should be skipped silently

	outDir := t.TempDir()
	tarPath, err := Snapshot(home, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(tarPath, ".tar.gz") {
		t.Errorf("snapshot path should be .tar.gz, got %s", tarPath)
	}

	// Verify contents
	names := readTarballEntries(t, tarPath)
	want := map[string]bool{
		"books":             false,
		"books/deploy.yaml": false,
		"history":           false,
		"history/rec.json":  false,
		"pinned.json":       false,
	}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("missing %s in snapshot tarball", n)
		}
	}
	// Anti-check: highlights.yaml shouldn't be there
	for _, n := range names {
		if n == "highlights.yaml" {
			t.Errorf("highlights.yaml shouldn't be in tarball when source is missing")
		}
	}
}

func readTarballEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, strings.TrimRight(hdr.Name, "/"))
	}
	sort.Strings(names)
	return names
}

func mustTime(s string) time.Time {
	t, err := time.ParseInLocation(timestampLayout, s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}
