// Package backup manages files in ~/.runbook/backups/. Two kinds coexist there:
//
//   - Per-YAML backups written by the Mac app before each save/delete.
//     Filename pattern: <runbook-name>-<YYYY-MM-DDTHHmmss>.yaml
//
//   - Full-state snapshots created by `runbook backup snapshot`.
//     Filename pattern: runbook-<YYYY-MM-DDTHHmmss>.tar.gz
//     Snapshots contain books/, history/, pinned.json, highlights.yaml.
//
// The two coexist because they serve different purposes (per-edit safety net
// vs. machine-migration archive) and the filename patterns disambiguate them
// reliably. List, Show, Restore, Diff, and Prune all understand both kinds;
// Snapshot creates a new tarball; Restore expands per-YAML backups (tarballs
// are intentionally left to the user with `tar -xzf`).
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Kind values returned in Entry.Kind.
const (
	KindYAML     = "yaml"
	KindSnapshot = "snapshot"
)

// SnapshotPrefix is the runbook-name slot used for snapshot tarballs so
// they're distinguishable from per-YAML backups by name alone.
const SnapshotPrefix = "runbook"

// timestampLayout matches the Mac app's filename timestamps: ISO 8601-ish
// with capital T and no separators in the time portion.
const timestampLayout = "2006-01-02T150405"

// fileRegex captures (1) name, (2) timestamp, (3) extension.
// Names may contain dashes; the regex anchors the timestamp pattern from the
// right so the LAST `-YYYY-MM-DDTHHmmss` is the boundary.
var fileRegex = regexp.MustCompile(`^(.+)-(\d{4}-\d{2}-\d{2}T\d{6})\.(yaml|yml|tar\.gz|tgz)$`)

// Entry describes one backup file.
type Entry struct {
	Kind        string // KindYAML or KindSnapshot
	RunbookName string // for yaml: the runbook's name; for snapshots: SnapshotPrefix
	Timestamp   time.Time
	Path        string
	Size        int64
}

// DefaultDir returns ~/.runbook/backups/.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runbook", "backups")
}

// HomeDir returns ~/.runbook/.
func HomeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runbook")
}

// List returns backup entries newest-first. If name != "" filters to that name.
// Missing directory is treated as empty (not an error).
func List(dir, name string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []Entry
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		m := fileRegex.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		rbName, tsStr, ext := m[1], m[2], m[3]
		ts, err := time.ParseInLocation(timestampLayout, tsStr, time.Local)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		kind := KindYAML
		if ext == "tar.gz" || ext == "tgz" {
			kind = KindSnapshot
		}
		if name != "" && rbName != name {
			continue
		}
		results = append(results, Entry{
			Kind:        kind,
			RunbookName: rbName,
			Timestamp:   ts,
			Path:        filepath.Join(dir, e.Name()),
			Size:        info.Size(),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	return results, nil
}

// Find returns the newest entry matching name (and optionally a timestamp prefix).
// Returns nil if no entry matches.
func Find(entries []Entry, name, tsPrefix string) *Entry {
	for i := range entries {
		if entries[i].RunbookName != name {
			continue
		}
		if tsPrefix == "" {
			return &entries[i]
		}
		if strings.HasPrefix(entries[i].Timestamp.Format(timestampLayout), tsPrefix) {
			return &entries[i]
		}
	}
	return nil
}

// Restore copies backupPath over targetPath. Before overwriting the target
// (if it exists), writes a fresh per-YAML backup of the current state into
// backupsDir so the restore is itself reversible.
//
// Tarball backups are intentionally not auto-restored — they contain a
// directory tree spanning books/, history/, etc., and rewriting all of those
// silently is too destructive. Tarball callers get an error pointing at
// `tar -xzf` instead.
func Restore(backupPath, targetPath, backupsDir string) error {
	if isTarball(backupPath) {
		return fmt.Errorf("snapshot tarballs cannot be restored automatically; expand with: tar -xzf %s -C %s", backupPath, HomeDir())
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("backup %q is a directory", backupPath)
	}

	if _, err := os.Stat(targetPath); err == nil {
		base := filepath.Base(targetPath)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		ts := time.Now().Format(timestampLayout)
		savedBackup := filepath.Join(backupsDir, fmt.Sprintf("%s-%s.yaml", name, ts))
		if err := copyFile(targetPath, savedBackup); err != nil {
			return fmt.Errorf("backing up current state: %w", err)
		}
	}
	return copyFile(backupPath, targetPath)
}

// Show returns the contents of a backup. For tarballs returns a `tar -tzf`
// listing instead of binary contents.
func Show(backupPath string) ([]byte, error) {
	if isTarball(backupPath) {
		out, err := exec.Command("tar", "-tzf", backupPath).Output()
		if err != nil {
			return nil, fmt.Errorf("listing tarball: %w", err)
		}
		return out, nil
	}
	return os.ReadFile(backupPath)
}

// Diff returns a unified diff between backupPath and currentPath (YAML only).
// `diff` exits with code 1 when files differ — that's not an error here.
func Diff(backupPath, currentPath string) ([]byte, error) {
	if isTarball(backupPath) {
		return nil, fmt.Errorf("snapshots cannot be diffed; list contents with: runbook backup show <name>")
	}
	out, err := exec.Command("diff", "-u", backupPath, currentPath).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return out, nil
		}
		return out, fmt.Errorf("diff: %w", err)
	}
	return out, nil
}

// Prune deletes backups exceeding keepPerName per (kind, name) group AND/OR
// older than olderThan. Pass keepPerName=0 to disable the count rule and
// olderThan=0 to disable the age rule. Returns the paths that were (or
// would be in dryRun) deleted.
func Prune(dir string, keepPerName int, olderThan time.Duration, dryRun bool) ([]string, error) {
	entries, err := List(dir, "")
	if err != nil {
		return nil, err
	}

	type key struct{ kind, name string }
	grouped := map[key][]Entry{}
	for _, e := range entries {
		k := key{kind: e.Kind, name: e.RunbookName}
		grouped[k] = append(grouped[k], e)
	}

	var toDelete []string
	cutoff := time.Now().Add(-olderThan)
	for _, group := range grouped {
		for i, e := range group {
			tooOld := olderThan > 0 && e.Timestamp.Before(cutoff)
			beyondKeep := keepPerName > 0 && i >= keepPerName
			if tooOld || beyondKeep {
				toDelete = append(toDelete, e.Path)
			}
		}
	}

	if !dryRun {
		for _, p := range toDelete {
			if err := os.Remove(p); err != nil {
				return toDelete, err
			}
		}
	}
	return toDelete, nil
}

// Snapshot creates a tarball at <outputDir>/runbook-<timestamp>.tar.gz containing
// a curated subset of the user's runbook home: books/, history/, pinned.json,
// highlights.yaml. Logs and the backups directory itself are excluded
// (ephemeral / recursive). Missing entries are silently skipped so the snapshot
// works on a fresh install with no history yet.
func Snapshot(runbookHome, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	ts := time.Now().Format(timestampLayout)
	outPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.tar.gz", SnapshotPrefix, ts))

	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, item := range []string{"books", "history", "pinned.json", "highlights.yaml"} {
		full := filepath.Join(runbookHome, item)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		if err := addToTar(tw, runbookHome, item); err != nil {
			return outPath, fmt.Errorf("adding %s: %w", item, err)
		}
	}
	return outPath, nil
}

// addToTar walks `relPath` (relative to baseDir) and writes entries to tw.
func addToTar(tw *tar.Writer, baseDir, relPath string) error {
	fullPath := filepath.Join(baseDir, relPath)
	return filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func isTarball(path string) bool {
	return strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz")
}
