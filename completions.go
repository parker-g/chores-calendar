package main

import (
	"database/sql"
	"fmt"

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
	// Use URI with timeout and cache settings for better concurrent access
	dsn := fmt.Sprintf("file:%s?cache=shared&timeout=5000", path)
	conn, err := sql.Open("sqlite", dsn)
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
	// Configure connection pool for concurrent access
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
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
// Two clients toggling the same (choreID, date) at nearly the same time
// could both see "no row" and both attempt an insert; the second insert
// hits the (chore_id, date) primary key and fails with a constraint
// error. That failure is treated as "someone else already completed
// it" and reported as completed=true rather than surfaced as a 500.
func toggleCompletion(choreID, date string) (bool, error) {
	res, err := db.Exec(`DELETE FROM completions WHERE chore_id = ? AND date = ?`, choreID, date)
	if err != nil {
		return false, fmt.Errorf("deleting completion: %w", err)
	}
	rowsDeleted, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking delete result: %w", err)
	}
	if rowsDeleted > 0 {
		return false, nil
	}

	_, err = db.Exec(`INSERT INTO completions (chore_id, date) VALUES (?, ?)`, choreID, date)
	if err != nil {
		// A concurrent toggle may have inserted the same row between our
		// DELETE and this INSERT. Treat that race as "already completed"
		// rather than an error.
		if already, checkErr := isCompleted(choreID, date); checkErr == nil && already {
			return true, nil
		}
		return false, fmt.Errorf("inserting completion: %w", err)
	}
	return true, nil
}
