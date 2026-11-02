package main

import (
	"os"
	"path/filepath"
	"testing"
)

func populatedStore(t *testing.T, filename string) *questionStore {
	t.Helper()
	s, err := openQuestionStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	result, err := parseQuestionFile(templateFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.importBanks(result); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.load()
	if err = s.saveResponse(qs[0].ID, response{Choice: 2, Correct: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO settings(key,value) VALUES('exam:fixture','saved exam')"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDataLocationInstalledPortableAndSource(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "custom-install", "QuesoraNativePreview.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0700); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(root, "source")
	os.MkdirAll(cwd, 0700)
	loc, err := resolveDataLocation(exe, cwd, filepath.Join(root, "user"))
	if err != nil || loc.Installed || loc.Dir != filepath.Join(filepath.Dir(exe), "data") {
		t.Fatal(loc, err)
	}
	os.WriteFile(filepath.Join(cwd, "mygo.json"), []byte("{}"), 0600)
	loc, err = resolveDataLocation(exe, cwd, filepath.Join(root, "user"))
	if err != nil || loc.Dir != filepath.Join(cwd, "data") {
		t.Fatal(loc, err)
	}
	os.WriteFile(filepath.Join(filepath.Dir(exe), installedMarker), []byte("installed"), 0600)
	loc, err = resolveDataLocation(exe, cwd, filepath.Join(root, "user"))
	if err != nil || !loc.Installed || loc.Dir != filepath.Join(root, "user", "QuesoraNative", "data") {
		t.Fatal(loc, err)
	}
	if _, err = resolveDataLocation(exe, cwd, ""); err == nil {
		t.Fatal("installed app silently used a relative data path")
	}
}

func TestBackupIncludesWALAndProtectsLiveDatabase(t *testing.T) {
	root := t.TempDir()
	s := populatedStore(t, filepath.Join(root, "live.db"))
	defer s.close()
	if _, err := s.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO settings(key,value) VALUES('wal-test','latest')"); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "备份 'quoted'", "backup.db")
	for i := 0; i < 2; i++ {
		if err := s.backupTo(backup); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := openQuestionStore(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer saved.close()
	qs, err := saved.load()
	if err != nil || len(qs) != 5 {
		t.Fatal(len(qs), err)
	}
	answers, _, err := saved.loadPractice()
	if err != nil || !answers[qs[0].ID].Correct {
		t.Fatal(answers, err)
	}
	for key, want := range map[string]string{"wal-test": "latest", "exam:fixture": "saved exam"} {
		var got string
		if err = saved.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&got); err != nil || got != want {
			t.Fatal(key, got, err)
		}
	}
	for _, p := range []string{s.path, s.path + "-wal", s.path + "-shm"} {
		if err = s.backupTo(p); err == nil {
			t.Fatal("live database path accepted", p)
		}
	}
}

func TestInstalledDataMigrationRetainsSourceAndExistingTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "old", "data", "questions.db")
	s := populatedStore(t, source)
	defer s.close()
	if _, err := s.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO settings(key,value) VALUES('migration','original')"); err != nil {
		t.Fatal(err)
	}
	loc := dataLocation{Dir: filepath.Join(root, "user", "data"), LegacyDB: source, Installed: true}
	if err := prepareInstalledData(loc); err != nil {
		t.Fatal(err)
	}
	target, err := openQuestionStore(filepath.Join(loc.Dir, "questions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()
	var value string
	if err = target.db.QueryRow("SELECT value FROM settings WHERE key='migration'").Scan(&value); err != nil || value != "original" {
		t.Fatal(value, err)
	}
	target.db.Exec("UPDATE settings SET value='newer data' WHERE key='migration'")
	if err = prepareInstalledData(loc); err != nil {
		t.Fatal(err)
	}
	target.db.QueryRow("SELECT value FROM settings WHERE key='migration'").Scan(&value)
	if value != "newer data" {
		t.Fatal("existing installed data overwritten")
	}
	s.db.QueryRow("SELECT value FROM settings WHERE key='migration'").Scan(&value)
	if value != "original" {
		t.Fatal("legacy source modified")
	}
}

func TestBackupFailurePreservesExistingBackup(t *testing.T) {
	root := t.TempDir()
	s := populatedStore(t, filepath.Join(root, "live.db"))
	backup := filepath.Join(root, "backup.db")
	original := []byte("existing backup must survive failed operation")
	if err := os.WriteFile(backup, original, 0600); err != nil {
		t.Fatal(err)
	}
	s.close()
	if err := s.backupTo(backup); err == nil {
		t.Fatal("closed database backup succeeded")
	}
	got, err := os.ReadFile(backup)
	if err != nil || string(got) != string(original) {
		t.Fatal("existing backup changed", err)
	}
	pending, err := filepath.Glob(filepath.Join(root, ".quesora-backup-*.db"))
	if err != nil || len(pending) != 0 {
		t.Fatal("incomplete backup left behind", pending, err)
	}
}
