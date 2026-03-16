package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Record represents a single runbook execution.
type Record struct {
	RunbookName string       `json:"runbook_name"`
	FilePath    string       `json:"file_path"`
	StartedAt   time.Time    `json:"started_at"`
	Duration    string       `json:"duration"`
	Success     bool         `json:"success"`
	StepCount   int          `json:"step_count"`
	Steps       []StepRecord `json:"steps"`
}

// StepRecord represents the outcome of a single step.
type StepRecord struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
}

// Store handles reading and writing run history.
type Store struct {
	Dir string
}

// NewStore creates a store at the given directory.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Save writes a record to disk as a JSON file.
func (s *Store) Save(rec Record) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("creating history dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json",
		rec.StartedAt.Format("2006-01-02_15-04-05"),
		sanitize(rec.RunbookName),
	)
	path := filepath.Join(s.Dir, filename)

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling history: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	return nil
}

// List returns recent history records, newest first.
// If name is non-empty, only records matching that runbook name are returned.
func (s *Store) List(limit int, name string) ([]Record, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading history dir: %w", err)
	}

	var records []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var rec Record
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if name != "" && rec.RunbookName != name {
			continue
		}
		records = append(records, rec)
	}

	// Sort newest first
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

func sanitize(s string) string {
	r := strings.NewReplacer("/", "_", " ", "_", "\\", "_")
	return r.Replace(s)
}
