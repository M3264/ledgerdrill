package store

import (
	"path/filepath"
	"testing"
)

func TestJournalSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "runs.db")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	original := map[string]any{"status": "running", "hash": "abc"}
	if e = s.Put("run", "one", original); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var read map[string]any
	if e = s.Get("run", "one", &read); e != nil {
		t.Fatal(e)
	}
	if read["hash"] != "abc" {
		t.Fatal("journal lost hash")
	}
	if e = s.Get("run", "missing", &read); e != ErrNotFound {
		t.Fatalf("not-found mismatch: %v", e)
	}
}
