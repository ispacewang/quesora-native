package desktop

import (
	"database/sql"
	"testing"
)

// fixtureDB is exclusively for SQL-level fixtures and fault injection.
// Production code never accesses a repository connection or creates extra connections.
func fixtureDB(t *testing.T, s *questionStore) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", s.path())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
