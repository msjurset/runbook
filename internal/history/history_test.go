package history

import (
	"testing"
	"time"
)

func TestSaveAndList(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	rec := Record{
		RunbookName: "test-book",
		FilePath:    "/tmp/test.yaml",
		StartedAt:   time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
		Duration:    "1.5s",
		Success:     true,
		StepCount:   3,
		Steps: []StepRecord{
			{Name: "step1", Type: "shell", Status: "success", Duration: "0.5s"},
			{Name: "step2", Type: "http", Status: "success", Duration: "0.8s"},
			{Name: "step3", Type: "shell", Status: "success", Duration: "0.2s"},
		},
	}

	if err := store.Save(rec); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	records, err := store.List(0, "")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List() returned %d records, want 1", len(records))
	}
	if records[0].RunbookName != "test-book" {
		t.Errorf("RunbookName = %q, want %q", records[0].RunbookName, "test-book")
	}
	if !records[0].Success {
		t.Error("Success = false, want true")
	}
	if len(records[0].Steps) != 3 {
		t.Errorf("Steps = %d, want 3", len(records[0].Steps))
	}
}

func TestListFilterByName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	base := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	for i, name := range []string{"deploy", "healthcheck", "deploy"} {
		rec := Record{
			RunbookName: name,
			StartedAt:   base.Add(time.Duration(i) * time.Second),
			Success:     true,
		}
		if err := store.Save(rec); err != nil {
			t.Fatal(err)
		}
	}

	records, err := store.List(0, "deploy")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("List(deploy) = %d records, want 2", len(records))
	}
}

func TestListLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	base := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		rec := Record{
			RunbookName: "book",
			StartedAt:   base.Add(time.Duration(i) * time.Second),
			Success:     true,
		}
		store.Save(rec)
	}

	records, err := store.List(3, "")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("List(limit=3) = %d records, want 3", len(records))
	}
}

func TestListNewestFirst(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	store.Save(Record{RunbookName: "old", StartedAt: t1, Success: true})
	store.Save(Record{RunbookName: "new", StartedAt: t2, Success: true})

	records, err := store.List(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 {
		t.Fatal("expected at least 2 records")
	}
	if records[0].RunbookName != "new" {
		t.Errorf("first record = %q, want 'new' (newest first)", records[0].RunbookName)
	}
}

func TestListNonexistentDir(t *testing.T) {
	store := NewStore("/nonexistent/history/dir")
	records, err := store.List(0, "")
	if err != nil {
		t.Errorf("List() unexpected error: %v", err)
	}
	if records != nil {
		t.Errorf("List() = %v, want nil", records)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"deploy web", "deploy_web"},
		{"a/b/c", "a_b_c"},
	}
	for _, tt := range tests {
		if got := sanitize(tt.input); got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
