package sqlite

import (
	"context"
	"errors"
	"net/url"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

var ErrNoConnection = errors.New("no connection")

type DB struct {
	*sqlitex.Pool
}

func newFromPool(ctx context.Context, addr string, flags sqlite.OpenFlags, poolSize int) (*DB, error) {
	pool, err := sqlitex.Open(addr, flags, poolSize)
	if err != nil {
		return nil, err
	}
	db := &DB{
		Pool: pool,
	}

	err = db.setupVersionTable(ctx)
	if err != nil {
		db.Close()
	}

	return db, nil
}

func NewMemoryDB(ctx context.Context, poolSize int) (*DB, error) {
	return newFromPool(ctx, "file:memory:?mode=memory", 0, poolSize)
}

func NewFileDB(ctx context.Context, path string, poolSize int) (*DB, error) {
	const flags = sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE
	qpath := "file:" + url.PathEscape(path)
	return newFromPool(ctx, qpath, flags, poolSize)
}

func (db *DB) setupVersionTable(ctx context.Context) error {
	// Check for versions table
	haveVersions, err := db.haveTable(ctx, "versions")
	if err == nil && !haveVersions {
		err = db.createVersionTable(context.Background())
	}
	return err
}

func (db *DB) haveTable(ctx context.Context, table string) (ok bool, err error) {
	c := db.Get(ctx)
	if c == nil {
		return false, ErrNoConnection
	}
	defer db.Put(c)

	stmt := c.Prep(`SELECT COUNT(*) AS found FROM sqlite_master WHERE name = $name LIMIT 1`)
	defer stmt.Reset()

	stmt.SetText("$name", table)
	if ok, err := stmt.Step(); err != nil {
		return false, err
	} else if !ok {
		return false, nil
	}
	ok = stmt.GetInt64("found") > 0
	return ok, nil
}

func (db *DB) createVersionTable(ctx context.Context) error {
	conn := db.Get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.Put(conn)

	stmt, _, err := conn.PrepareTransient(`CREATE TABLE versions (
	component TEXT PRIMARY KEY,
	version INT
)`)
	if err != nil {
		return err
	}
	defer stmt.Finalize()

	_, err = stmt.Step()
	return err
}

func (db *DB) Migrate(ctx context.Context) error {
	conn := db.Get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.Put(conn)
	return systemPatches.Apply(ctx, conn)
}

func (db *DB) Close() error {
	return db.Pool.Close()
}
