package desktop

import (
	"encoding/json"
	"github.com/ispacewang/quesora-native/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRestoreIncludesCommittedWALAndPreservesSafetyCopy(t *testing.T) {
	dir := t.TempDir()
	live := restorableStore(t, filepath.Join(dir, "fixtureDB(t, live)"))
	defer live.Close()
	source := restorableStore(t, filepath.Join(dir, "备份 'source'.db"))
	defer source.Close()
	fixtureDB(t, source).Exec("PRAGMA journal_mode=WAL")
	fixtureDB(t, source).Exec("UPDATE banks SET name='恢复题库'")
	fixtureDB(t, source).Exec("INSERT INTO settings(key,value) VALUES('restore-fixture','committed WAL')")
	safety, err := live.restoreFrom(source.path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(safety, "before-restore-") {
		t.Fatal(safety)
	}
	qs, err := live.LoadQuestions()
	if err != nil || len(qs) != 5 || qs[0].Bank != "恢复题库" {
		t.Fatal(qs, err)
	}
	var value string
	if err = fixtureDB(t, live).QueryRow("SELECT value FROM settings WHERE key='restore-fixture'").Scan(&value); err != nil || value != "committed WAL" {
		t.Fatal(value, err)
	}
	before, err := openQuestionStore(safety)
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	old, err := before.LoadQuestions()
	if err != nil || len(old) != 5 || old[0].Bank == "恢复题库" {
		t.Fatal(old, err)
	}
	if _, err = os.Stat(source.path()); err != nil {
		t.Fatal("source lost", err)
	}
}

func TestRestoreRejectsWrongCorruptAndLiveSources(t *testing.T) {
	dir := t.TempDir()
	live := restorableStore(t, filepath.Join(dir, "fixtureDB(t, live)"))
	defer live.Close()
	corrupt := filepath.Join(dir, "corrupt.db")
	os.WriteFile(corrupt, []byte("not a database"), 0600)
	wrong := filepath.Join(dir, "wrong.db")
	db, err := storage.OpenReadOnly(live.path())
	if err != nil {
		t.Fatal(err)
	}
	if err = storage.Snapshot(db, wrong, false); err != nil {
		t.Fatal(err)
	}
	db.Close()
	other, err := openQuestionStore(wrong)
	if err != nil {
		t.Fatal(err)
	}
	fixtureDB(t, other).Exec("DROP TABLE practice")
	other.Close()
	for _, name := range []string{corrupt, wrong, live.path(), live.path() + "-wal"} {
		if _, err = live.restoreFrom(name); err == nil {
			t.Fatal("invalid source accepted", name)
		}
		qs, e := live.LoadQuestions()
		if e != nil || len(qs) != 5 {
			t.Fatal("live data changed", e)
		}
	}
}

func TestRestoreRollsBackAllTablesOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	live := restorableStore(t, filepath.Join(dir, "fixtureDB(t, live)"))
	defer live.Close()
	source := restorableStore(t, filepath.Join(dir, "fixtureDB(t, source)"))
	defer source.Close()
	fixtureDB(t, source).Exec("UPDATE banks SET name='new bank'")
	if _, err := fixtureDB(t, live).Exec("CREATE TRIGGER reject_restore BEFORE INSERT ON banks BEGIN SELECT RAISE(ABORT,'fixture failure'); END"); err != nil {
		t.Fatal(err)
	}
	safety, err := live.restoreFrom(source.path())
	if err == nil {
		t.Fatal("forced failure accepted")
	}
	if safety == "" {
		t.Fatal("pre-restore backup missing")
	}
	qs, e := live.LoadQuestions()
	if e != nil || len(qs) != 5 || qs[0].Bank == "new bank" {
		t.Fatal("transaction did not preserve old questions", e)
	}
	answers, _, e := live.LoadPractice()
	if e != nil || len(answers) != 1 {
		t.Fatal("practice lost", answers, e)
	}
	if _, e = os.Stat(safety); e != nil {
		t.Fatal(e)
	}
}

// restorableStore uses a genuine exam payload: the backup helper's text fixture is intentionally not a valid exam.
func restorableStore(t *testing.T, filename string) *questionStore {
	t.Helper()
	s := populatedStore(t, filename)
	qs, err := s.LoadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	e, err := newExamSession(qs, examSettings{Bank: qs[0].Bank, Minutes: 60, Counts: [5]float64{1, 1, 1, 1, 1}}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixtureDB(t, s).Exec("UPDATE settings SET value=? WHERE key='exam:fixture'", string(raw)); err != nil {
		t.Fatal(err)
	}
	return s
}
