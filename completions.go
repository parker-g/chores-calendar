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
// All writes are serialized through a single connection (SetMaxOpenConns(1)),
// which eliminates SQLITE_BUSY errors by construction. The insert-conflict
// fallback below is a backstop in case the constraint is encountered, but
// the primary defense against concurrent races is serialization itself.
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
