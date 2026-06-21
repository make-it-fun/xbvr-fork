package models

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

// newTestActorDB opens an isolated temp SQLite DB with the actors table (and its
// actors.name unique index) so tests can exercise the real Actor.Save path.
func newTestActorDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open("sqlite3", path+"?_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.AutoMigrate(&Actor{}).Error; err != nil {
		t.Fatalf("migrate actors: %v", err)
	}
	return db
}

// withCommonConn points the package-global common connection (used by
// Actor.Save) at db for the duration of a test, then restores it.
func withCommonConn(t *testing.T, db *gorm.DB) {
	t.Helper()
	prev := commonConnection
	commonConnection = db
	t.Cleanup(func() { commonConnection = prev })
}

// TestActorSaveRecoversFromNameRace is the deterministic reproduction of Cause A:
// a second scrape goroutine saving an actor whose name already exists used to hit
// the actors.name unique index and crash the server. It must now adopt the
// existing row and return nil.
func TestActorSaveRecoversFromNameRace(t *testing.T) {
	db := newTestActorDB(t)
	withCommonConn(t, db)

	winner := &Actor{Name: "Ava Adams"}
	if err := winner.Save(); err != nil {
		t.Fatalf("seeding the winning actor failed: %v", err)
	}
	if winner.ID == 0 {
		t.Fatal("seeded actor has no id")
	}

	// Fresh struct, same name (ID 0 -> gorm attempts an INSERT -> UNIQUE).
	loser := &Actor{Name: "Ava Adams", StarRating: 5}
	if err := loser.Save(); err != nil {
		t.Fatalf("losing the create race should recover, got: %v", err)
	}
	if loser.ID != winner.ID {
		t.Fatalf("loser should adopt the winner's id %d, got %d", winner.ID, loser.ID)
	}

	var n int
	db.Model(&Actor{}).Where("name = ?", "Ava Adams").Count(&n)
	if n != 1 {
		t.Fatalf("expected exactly 1 actor row, got %d", n)
	}
}

// TestActorSaveConcurrentCreateNoCrash drives the actual race: many goroutines
// creating the same new actor at once. Pre-fix this exited the process (and this
// test binary); now every Save must return nil and exactly one row must exist.
func TestActorSaveConcurrentCreateNoCrash(t *testing.T) {
	db := newTestActorDB(t)
	withCommonConn(t, db)

	const n = 12
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a := &Actor{Name: "Riley Race"}
			errs[idx] = a.Save()
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: Save returned %v (want nil)", i, e)
		}
	}
	var cnt int
	db.Model(&Actor{}).Where("name = ?", "Riley Race").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("expected exactly 1 actor after the race, got %d", cnt)
	}
}

// uniqueWidget has a unique index on Name so a duplicate insert reproduces the
// "UNIQUE constraint failed" error that used to crash the server.
type uniqueWidget struct {
	ID   uint   `gorm:"primary_key"`
	Name string `gorm:"unique_index"`
}

// TestSaveWithRetryReturnsErrorInsteadOfExiting is the regression test for the
// fatal-crash fix: a permanent save failure must be RETURNED to the caller, not
// passed to log.Fatal (which would os.Exit and take the whole process — and this
// test binary — down).
func TestSaveWithRetryReturnsErrorInsteadOfExiting(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer db.Close()
	db.AutoMigrate(&uniqueWidget{})

	if err := SaveWithRetry(db, &uniqueWidget{Name: "ava"}); err != nil {
		t.Fatalf("first save should succeed, got: %v", err)
	}

	// Second row with the same unique name -> UNIQUE constraint violation.
	// Pre-fix this called log.Fatal; now it must return the error so the caller
	// can skip the row and keep running.
	err = SaveWithRetry(db, &uniqueWidget{Name: "ava"})
	if err == nil {
		t.Fatal("expected a unique-constraint error to be returned, got nil")
	}
}

func TestIsTransient(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"database is locked", true}, // Cause B
		{"database is busy", true},   //
		{"cannot start a transaction within a transaction: database table is locked", true},
		{"UNIQUE constraint failed: actors.name", false}, // Cause A is NOT transient
		{"", false},
	}
	for _, c := range cases {
		var err error
		if c.msg != "" {
			err = &stringErr{c.msg}
		}
		if got := isTransient(err); got != c.want {
			t.Errorf("isTransient(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}

func TestIsDuplicateName(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"UNIQUE constraint failed: actors.name", true}, // the race we recover from
		{"UNIQUE constraint failed: scenes.scene_id", false},
		{"database is locked", false},
		{"", false},
	}
	for _, c := range cases {
		var err error
		if c.msg != "" {
			err = &stringErr{c.msg}
		}
		if got := isDuplicateName(err); got != c.want {
			t.Errorf("isDuplicateName(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}

type stringErr struct{ s string }

func (e *stringErr) Error() string { return e.s }
