package sqlite

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	com "go.spiff.io/gribble/internal/common"
)

var ErrNoConnection = errors.New("no connection")

type DB struct {
	*sqlitex.Pool
	autosaveID uint64 // atomic
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

func (db *DB) savepoint(ctx context.Context, conn *sqlite.Conn, transaction func() error) error {
	id := atomic.AddUint64(&db.autosaveID, 1)
	nsec := time.Now().UnixNano()
	name := "gribble/autosave/" + strconv.FormatUint(id, 32) + "/" + strconv.FormatInt(nsec, 32)
	return InSavepoint(ctx, conn, name, transaction)
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

func (db *DB) CreateRunner(ctx context.Context, runner *com.Runner) error {
	if err := runner.CanCreate(); err != nil {
		return err
	}
	conn := db.Get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.Put(conn)

	return db.savepoint(ctx, conn, func() error {
		return createRunner(conn, runner)
	})
}

func createRunner(conn *sqlite.Conn, runner *com.Runner) error {
	stmt := conn.Prep(`INSERT INTO
		runners(token, description, run_untagged, locked, max_timeout, active)
		VALUES($token, $description, $run_untagged, $locked, $max_timeout, $active)`)
	defer stmt.Reset()
	defer stmt.ClearBindings()
	stmt.SetText("$token", runner.Token)
	stmt.SetText("$description", runner.Description)
	stmt.SetInt64("$run_untagged", btoi(runner.RunUntagged))
	stmt.SetInt64("$locked", btoi(runner.Locked))
	stmt.SetInt64("$active", btoi(runner.Active))
	stmt.SetInt64("$max_timeout", dtoi(runner.MaxTimeout))
	_, err := stmt.Step()
	if err != nil {
		return err
	}

	id := conn.LastInsertRowID()
	updated := *runner
	updated.ID = id

	if len(runner.Tags) == 0 {
		// nop
	} else if err = tagRunner(conn, &updated, runner.Tags); err != nil {
		return err
	}

	*runner = updated
	return nil
}

// TagRunner associates the given tags to the runner and vice-versa.
func (db *DB) TagRunner(ctx context.Context, runner *com.Runner, tags []string) (err error) {
	if runner.ID <= 0 {
		return com.ErrNoID
	}

	conn := db.Get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.Put(conn)

	return db.savepoint(ctx, conn, func() error {
		return tagRunner(conn, runner, tags)
	})
}

func tagRunner(conn *sqlite.Conn, runner *com.Runner, tags []string) error {
	if len(tags) == 0 {
		return removeRunnerTags(conn, runner)
	}

	tagIDs := make([]int64, len(tags))
	for i, tag := range tags {
		id, err := ensureTag(conn, tag)
		if err != nil {
			return err
		}
		tagIDs[i] = id
	}

	unlink, _, err := conn.PrepareTransient(`DELETE FROM runner_tags
		WHERE runner = $runner AND tag NOT IN (` + idList(tagIDs...) + `)`)
	if err != nil {
		return err
	}
	defer unlink.Finalize()

	unlink.SetInt64("$runner", runner.ID)
	if _, err = unlink.Step(); err != nil {
		return err
	}

	link := conn.Prep(`INSERT INTO runner_tags(tag, runner) VALUES ($tag, $runner)`)
	defer link.Reset()
	for _, tag := range tagIDs {
		link.SetInt64("$runner", runner.ID)
		link.SetInt64("$tag", tag)
		if _, err = link.Step(); err != nil {
			return err
		}
		link.Reset()
	}

	return nil
}

func removeRunnerTags(conn *sqlite.Conn, runner *com.Runner) error {
	unlink := conn.Prep(`DELETE FROM runnerTags WHERE runner = $runner`)
	defer unlink.Reset()
	unlink.SetInt64("$runner", runner.ID)
	_, err := unlink.Step()
	return err
}

func ensureTag(conn *sqlite.Conn, tag string) (int64, error) {
	get := conn.Prep(`SELECT id FROM tags WHERE tag = $tag`)
	defer get.Reset()
	get.SetText("$tag", tag)
	found, err := get.Step()
	if err != nil {
		return 0, err
	} else if found {
		return get.ColumnInt64(0), nil
	}

	insert := conn.Prep(`INSERT INTO tags(tag) VALUES($tag)`)
	defer insert.Reset()
	insert.SetText("$tag", tag)
	_, err = insert.Step()
	if err != nil {
		return 0, err
	}

	return conn.LastInsertRowID(), nil
}

func (db *DB) Close() error {
	return db.Pool.Close()
}

func btoi(b bool) int64 {
	if !b {
		return 0
	}
	return 1
}

func itob(i int64) bool {
	return i != 0
}

func dtoi(d time.Duration) int64 {
	return int64(d)
}

func itod(i int64) time.Duration {
	return time.Duration(i)
}

func eachRow(ctx context.Context, stmt *sqlite.Stmt, fn func() error) (err error) {
	defer func() {
		rerr := stmt.Reset()
		if err == nil {
			err = rerr
		}
	}()
	var haveRows bool
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		if haveRows, err = stmt.Step(); err != nil || !haveRows {
			return err
		}
		if err = fn(); err != nil {
			return err
		}
	}
}

func idList(ids ...int64) string {
	var buf strings.Builder
	for i, id := range ids {
		if i > 0 {
			_ = buf.WriteByte(',')
		}
		_, _ = buf.WriteString(strconv.FormatInt(id, 10))
	}
	return buf.String()
}
