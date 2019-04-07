package sqlite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"crawshaw.io/sqlite"
)

func TestVersions(t *testing.T) {
	ctx := context.Background()
	db, err := NewMemoryDB(ctx, 1)
	if err != nil {
		panic(fmt.Errorf("Error opening DB pool: %v", err))
	}
	defer db.Close()

	// Double-create version table using internal method
	if err := db.setupVersionTable(ctx); err != nil {
		t.Fatalf("setupVersionTable() = %#v; want nil", err)
	}

	conn := db.get(ctx)
	if conn == nil {
		t.Fatal("No connection available")
	}
	defer db.put(conn)

	stmt := conn.Prep(`SELECT COUNT(*) AS n FROM versions`)
	defer stmt.Finalize()

	ok, err := stmt.Step()
	if err != nil {
		t.Fatalf("Unable to query empty version table: %v", err)
	} else if !ok {
		t.Fatalf("No results from empty version table")
	}

	n := stmt.GetInt64("n")
	if n != 0 {
		t.Fatalf("Invalid count from empty version table: %d", n)
	}
}

func TestStatementPatch(t *testing.T) {
	ctx := context.Background()
	db, err := NewMemoryDB(ctx, 1)
	if err != nil {
		panic(fmt.Errorf("Error opening DB pool: %v", err))
	}
	defer db.Close()

	conn := db.get(ctx)
	if conn == nil {
		t.Fatal("No connection available")
	}
	defer db.put(conn)

	// Test normal patch application (and that it does not repeat)
	patch := StatementPatch("test-patch", "test-component", 1,
		`CREATE TABLE runners (identity TEXT PRIMARY KEY, tags TEXT)`,
	)
	normal := PatchSet{
		patch,
	}
	if err := normal.Apply(ctx, conn); err != nil {
		t.Fatalf("normal.Apply() err = %#v; want nil", err)
	}

	// Test that errors are returned
	errPatch := errors.New("failed")
	patchErr := NewSimplePatch("error-patch", "error-component", 1, func(ctx context.Context, conn *sqlite.Conn) error {
		return errPatch
	})
	withErr := PatchSet{
		patch,
		patchErr,
	}
	if err := withErr.Apply(ctx, conn); err != errPatch {
		t.Fatalf("normal.Apply() err = %#v; want %#v", err, errPatch)
	}

	// Test that panic errors are returned
	func(t *testing.T) {
		defer func() {
			if rc := recover(); rc == nil {
				t.Error("Apply did not panic")
			}
		}()
		errPanic := errors.New("panic")
		patchPanic := NewSimplePatch("error-patch", "error-component", 2, func(ctx context.Context, conn *sqlite.Conn) error {
			panic(errPanic)
		})
		withPanic := PatchSet{
			patch,
			patchPanic,
		}
		if err := withPanic.Apply(ctx, conn); err != errPanic {
			t.Fatalf("normal.Apply() err = %#v; want %#v", err, errPanic)
		}
	}(t)

	want := map[string]int{
		"test-component": 1,
	}

	stmt := conn.Prep(`SELECT component, version AS n FROM versions`)
	defer stmt.Finalize()

	err = eachRow(ctx, stmt, func() error {
		component := stmt.GetText("component")
		version := int(stmt.GetInt64("version"))
		if v, ok := want[component]; ok && v == version {
			t.Logf("Recorded version %q => %d", component, version)
			delete(want, component)
		}
		return nil
	})

	if err != nil {
		t.Errorf("eachRow() = %v; want nil", err)
	}
}

func TestMigrations(t *testing.T) {
	ctx := context.Background()
	db, err := NewMemoryDB(ctx, 1)
	if err != nil {
		panic(fmt.Errorf("Error opening DB pool: %v", err))
	}
	defer db.Close()

	for i, p := range systemPatches {
		ps := PatchSet{p}
		name := fmt.Sprintf("%d_%s_%s_v%d", i, p.Name(), p.Component(), p.Version())
		ok := t.Run(name, func(t *testing.T) {
			if err := db.migrate(ctx, ps); err != nil {
				t.Fatalf("Migration failed: (%T) %v", err, err)
			}
		})
		if !ok {
			t.FailNow()
		}
	}
}
