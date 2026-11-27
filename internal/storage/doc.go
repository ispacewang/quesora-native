// Package storage owns Quesora's SQLite schema, transactions and snapshots.
//
// Store exposes question, practice and raw setting operations without returning a live SQL
// connection. Callers serialize access through their task queue and wait for it before Close.
// ReadState and RestoreFrom do not initialize UI or load native assets. RestoreFrom validates a
// private snapshot before creating a safety copy and atomically replacing the four business tables.
// JSON schemas for historical exams and learning preferences remain owned by their business layer.
package storage
