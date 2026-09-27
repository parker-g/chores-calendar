package main

import (
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := initCompletionsDB(dbPath); err != nil {
		t.Fatalf("initCompletionsDB failed: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		db = nil
	})
}

func TestToggleCompletion_InsertsThenDeletes(t *testing.T) {
	setupTestDB(t)

	completed, err := toggleCompletion("kitchen_cleaner", "2026-09-26")
	if err != nil {
		t.Fatalf("first toggle failed: %v", err)
	}
	if !completed {
		t.Fatalf("expected first toggle to mark complete, got completed=%v", completed)
	}

	got, err := isCompleted("kitchen_cleaner", "2026-09-26")
	if err != nil {
		t.Fatalf("isCompleted failed: %v", err)
	}
	if !got {
		t.Fatalf("expected isCompleted true after first toggle")
	}

	completed, err = toggleCompletion("kitchen_cleaner", "2026-09-26")
	if err != nil {
		t.Fatalf("second toggle failed: %v", err)
	}
	if completed {
		t.Fatalf("expected second toggle to mark incomplete, got completed=%v", completed)
	}

	got, err = isCompleted("kitchen_cleaner", "2026-09-26")
	if err != nil {
		t.Fatalf("isCompleted failed: %v", err)
	}
	if got {
		t.Fatalf("expected isCompleted false after second toggle")
	}
}

func TestIsCompleted_FalseWhenNoRow(t *testing.T) {
	setupTestDB(t)

	got, err := isCompleted("kitchen_cleaner", "2026-09-26")
	if err != nil {
		t.Fatalf("isCompleted failed: %v", err)
	}
	if got {
		t.Fatalf("expected isCompleted false with no rows in the table")
	}
}

// TestToggleCompletion_ConcurrentTogglesDontError exercises the race where
// two clients toggle the same (chore_id, date) at nearly the same instant:
// both may see "no row" and both attempt an insert, so the second insert
// must be handled gracefully (see the comment in toggleCompletion) instead
// of surfacing a constraint-violation error.
func TestToggleCompletion_ConcurrentTogglesDontError(t *testing.T) {
	setupTestDB(t)

	const n = 10
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := toggleCompletion("kitchen_cleaner", "2026-09-26")
			errs <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent toggle returned error: %v", err)
		}
	}

	// The exact final state after N truly concurrent toggles isn't
	// well-defined (two calls can both observe "not completed" and both
	// try to insert), so this test's bar is just: no errors, and the DB
	// is left in a readable, valid state afterward — not that parity
	// with N is preserved.
	if _, err := isCompleted("kitchen_cleaner", "2026-09-26"); err != nil {
		t.Fatalf("isCompleted failed after concurrent toggles: %v", err)
	}
}
