package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

func TestIsValidChoreID(t *testing.T) {
	if !isValidChoreID("kitchen_cleaner") {
		t.Errorf("expected kitchen_cleaner to be a valid chore id")
	}
	if isValidChoreID("not_a_real_chore") {
		t.Errorf("expected not_a_real_chore to be invalid")
	}
}

func TestTodayAndYesterday(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC) // a Saturday
	today, yesterday := todayAndYesterday(now)
	if today != "2026-09-26" {
		t.Errorf("expected today 2026-09-26, got %s", today)
	}
	if yesterday != "2026-09-25" {
		t.Errorf("expected yesterday 2026-09-25, got %s", yesterday)
	}
}

func TestIsValidCompletionDate(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC)

	cases := []struct {
		date string
		want bool
	}{
		{"2026-09-26", true},            // today
		{"2026-09-25", true},            // yesterday
		{"2026-09-24", false},           // two days ago
		{"2026-09-27", false},           // tomorrow
		{"09/26/2026", false},           // wrong format
		{"2026-09-26T00:00:00Z", false}, // timestamp, not a date
		{"", false},
	}
	for _, c := range cases {
		if got := isValidCompletionDate(c.date, now); got != c.want {
			t.Errorf("isValidCompletionDate(%q) = %v, want %v", c.date, got, c.want)
		}
	}
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/completions/toggle", handleToggleCompletion)
	return r
}

func postToggle(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/completions/toggle", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandleToggleCompletion_TogglesOnAndOff(t *testing.T) {
	setupTestDB(t)
	router := newTestRouter()
	today, _ := todayAndYesterday(time.Now())

	body := fmt.Sprintf(`{"chore_id":"kitchen_cleaner","date":%q}`, today)

	rec := postToggle(t, router, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var first struct {
		Completed bool `json:"completed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if !first.Completed {
		t.Fatalf("expected completed=true on first toggle")
	}

	rec = postToggle(t, router, body)
	var second struct {
		Completed bool `json:"completed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if second.Completed {
		t.Fatalf("expected completed=false on second toggle")
	}
}

func TestHandleToggleCompletion_RejectsUnknownChore(t *testing.T) {
	setupTestDB(t)
	router := newTestRouter()
	today, _ := todayAndYesterday(time.Now())

	body := fmt.Sprintf(`{"chore_id":"not_a_real_chore","date":%q}`, today)
	rec := postToggle(t, router, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleToggleCompletion_RejectsOutOfRangeDate(t *testing.T) {
	setupTestDB(t)
	router := newTestRouter()

	body := `{"chore_id":"kitchen_cleaner","date":"2020-01-01"}`
	rec := postToggle(t, router, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleToggleCompletion_RejectsMalformedBody(t *testing.T) {
	setupTestDB(t)
	router := newTestRouter()

	rec := postToggle(t, router, `not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = postToggle(t, router, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d: %s", rec.Code, rec.Body.String())
	}
}
