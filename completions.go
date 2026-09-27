package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

// db is the shared handle to the completions SQLite database. It's a
// package-level var, consistent with the existing housemates/chores
// package vars — this is a small single-process household app, not a
// service that needs a dependency-injected DB handle threaded everywhere.
var db *sql.DB

// initCompletionsDB opens (creating if necessary) the SQLite database at
// path and ensures the completions table exists.
func initCompletionsDB(path string) error {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("opening completions db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return fmt.Errorf("pinging completions db: %w", err)
	}
	// Enable WAL mode for better concurrent access handling
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return fmt.Errorf("enabling WAL mode: %w", err)
	}
	// Serialize all database writes through a single connection to avoid
	// SQLITE_BUSY errors. This is safe for a small single-process household app.
	conn.SetMaxOpenConns(1)
	const schema = `
		CREATE TABLE IF NOT EXISTS completions (
			chore_id TEXT NOT NULL,
			date     TEXT NOT NULL,
			PRIMARY KEY (chore_id, date)
		);
	`
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return fmt.Errorf("creating completions table: %w", err)
	}
	db = conn
	return nil
}

// isCompleted reports whether the given chore was marked complete on the
// given date (YYYY-MM-DD).
func isCompleted(choreID, date string) (bool, error) {
	var exists int
	err := db.QueryRow(
		`SELECT 1 FROM completions WHERE chore_id = ? AND date = ?`,
		choreID, date,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking completion: %w", err)
	}
	return true, nil
}

// toggleCompletion flips the completion state of (choreID, date) and
// returns the new state. A row's presence means "completed"; toggling
// off deletes the row rather than storing a completed=false row.
//
// The DELETE and INSERT are wrapped in a single transaction to ensure
// atomicity. Combined with SetMaxOpenConns(1), a transaction holds the
// one pooled connection for its full duration, ensuring true serialization
// of the complete delete-then-insert sequence. A second goroutine's Begin()
// will block until the first transaction's Commit()/Rollback() releases the
// connection, making concurrent race conditions impossible.
func toggleCompletion(choreID, date string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`DELETE FROM completions WHERE chore_id = ? AND date = ?`, choreID, date)
	if err != nil {
		return false, fmt.Errorf("deleting completion: %w", err)
	}
	rowsDeleted, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking delete result: %w", err)
	}
	if rowsDeleted > 0 {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("committing transaction: %w", err)
		}
		return false, nil
	}

	_, err = tx.Exec(`INSERT INTO completions (chore_id, date) VALUES (?, ?)`, choreID, date)
	if err != nil {
		return false, fmt.Errorf("inserting completion: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("committing transaction: %w", err)
	}
	return true, nil
}

// isValidChoreID reports whether choreID matches one of the configured
// chores. Guards against typos or a stale client referencing a chore
// that's been renamed or removed.
func isValidChoreID(choreID string) bool {
	for _, c := range chores {
		if c.ID == choreID {
			return true
		}
	}
	return false
}

// todayAndYesterday returns now's calendar date and the day before it,
// both as YYYY-MM-DD strings in now's own location.
func todayAndYesterday(now time.Time) (today, yesterday string) {
	const layout = "2006-01-02"
	today = now.Format(layout)
	yesterday = now.AddDate(0, 0, -1).Format(layout)
	return
}

// isValidCompletionDate reports whether date is exactly today or
// yesterday (relative to now) AND is a well-formed YYYY-MM-DD string.
// A string that happens to parse but isn't in that exact format (extra
// time component, different separators) is rejected rather than
// normalized, so stored dates are always predictable.
func isValidCompletionDate(date string, now time.Time) bool {
	const layout = "2006-01-02"
	parsed, err := time.Parse(layout, date)
	if err != nil {
		return false
	}
	if parsed.Format(layout) != date {
		return false
	}
	today, yesterday := todayAndYesterday(now)
	return date == today || date == yesterday
}

// CompletionToggleRequest is the body of POST /completions/toggle.
type CompletionToggleRequest struct {
	ChoreID string `json:"chore_id"`
	Date    string `json:"date"`
}

// handleToggleCompletion toggles completion of one chore on one date.
// Only today or yesterday (server local time) may be toggled.
func handleToggleCompletion(c *gin.Context) {
	handleOriginHeader(c)

	var req CompletionToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ChoreID == "" || req.Date == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "chore_id and date are required"})
		return
	}
	if !isValidChoreID(req.ChoreID) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "unknown chore_id"})
		return
	}
	if !isValidCompletionDate(req.Date, time.Now()) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "date must be today or yesterday"})
		return
	}

	completed, err := toggleCompletion(req.ChoreID, req.Date)
	if err != nil {
		log.Printf("toggling completion for chore_id=%s date=%s: %v", req.ChoreID, req.Date, err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "could not record completion"})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"completed": completed})
}

// handleCompletionsPreflight handles the CORS preflight (OPTIONS) request
// the browser sends ahead of POST /completions/toggle, mirroring
// handleWeekPreflight in main.go.
func handleCompletionsPreflight(c *gin.Context) {
	handleOriginHeader(c)
	c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Status(http.StatusNoContent)
}
