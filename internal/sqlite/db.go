package sqlite

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	com "go.spiff.io/gribble/internal/common"
	"go.spiff.io/gribble/internal/proc"
)

var ErrNoConnection = errors.New("no connection")

type DB struct {
	pool       *sqlitex.Pool
	autosaveID uint64 // atomic
}

func newFromPool(ctx context.Context, addr string, flags sqlite.OpenFlags, poolSize int) (*DB, error) {
	pool, err := sqlitex.Open(addr, flags, poolSize)
	if err != nil {
		return nil, err
	}
	db := &DB{
		pool: pool,
	}

	err = db.setupVersionTable(ctx)
	if err != nil {
		db.Close()
	}

	return db, nil
}

func NewMemoryDB(ctx context.Context, poolSize int) (*DB, error) {
	const flags = sqlite.SQLITE_OPEN_URI |
		sqlite.SQLITE_OPEN_SHAREDCACHE |
		sqlite.SQLITE_OPEN_NOMUTEX |
		sqlite.SQLITE_OPEN_READWRITE
	return newFromPool(ctx, "file::memory:?mode=memory", flags, poolSize)
}

func NewFileDB(ctx context.Context, path string, poolSize int) (*DB, error) {
	qpath := "file:" + url.PathEscape(path)
	return newFromPool(ctx, qpath, 0, poolSize)
}

func (db *DB) get(ctx context.Context) *sqlite.Conn {
	conn := db.pool.Get(ctx)
	if conn == nil {
		return nil
	}
	if err := sqlitex.Exec(conn, `PRAGMA foreign_keys = ON`, nil); err != nil {
		panic(err)
	}
	return conn
}

func (db *DB) put(conn *sqlite.Conn) {
	db.pool.Put(conn)
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
	c := db.get(ctx)
	if c == nil {
		return false, ErrNoConnection
	}
	defer db.put(c)

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
	conn := db.get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.put(conn)

	return sqlitex.ExecTransient(conn,
		`CREATE TABLE versions (
			component TEXT PRIMARY KEY,
			version INT
		)`, nil)
}

func (db *DB) Migrate(ctx context.Context) error {
	return db.migrate(ctx, systemPatches)
}

func (db *DB) migrate(ctx context.Context, patches PatchSet) error {
	conn := db.get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.put(conn)
	return patches.Apply(ctx, conn)
}

func (db *DB) CreateRunner(ctx context.Context, runner *com.Runner) error {
	if err := runner.CanCreate(); err != nil {
		return err
	}
	conn := db.get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.put(conn)

	return db.savepoint(ctx, conn, func() error {
		return createRunner(ctx, conn, runner)
	})
}

func (db *DB) SetRunnerUpdatedTime(ctx context.Context, runner *com.Runner, t time.Time) error {
	if runner.ID <= 0 {
		return com.ErrNoID
	}

	conn := db.get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.put(conn)

	set := conn.Prep(`UPDATE runners SET updated_time = $time WHERE id = $runner`)
	defer set.Reset()
	set.SetFloat("$time", ToSecs(t))
	set.SetInt64("$runner", runner.ID)
	_, err := set.Step()
	if err != nil {
		return err
	}

	runner.Updated = t
	return nil
}

func (db *DB) GetRunnerByToken(ctx context.Context, token string, getDeleted bool) (*com.Runner, error) {
	conn := db.get(ctx)
	if conn == nil {
		return nil, ErrNoConnection
	}
	defer db.put(conn)

	get := conn.Prep(
		`SELECT
			id, description, run_untagged, locked, active, max_timeout, deleted
		FROM runners
		WHERE token = $token
		LIMIT 1`)
	defer get.Reset()

	get.SetText("$token", token)
	haveRows, err := get.Step()

	if err != nil {
		return nil, err
	} else if !haveRows {
		return nil, com.ErrNotFound
	}

	deleted := itob(get.GetInt64("deleted"))
	if !getDeleted && deleted {
		return nil, com.ErrNotFound
	}

	r := &com.Runner{
		ID:          get.GetInt64("id"),
		Token:       token,
		Description: get.GetText("description"),
		RunUntagged: itob(get.GetInt64("run_untagged")),
		Locked:      itob(get.GetInt64("locked")),
		Active:      itob(get.GetInt64("active")),
		MaxTimeout:  itod(get.GetInt64("max_timeout")),
		Deleted:     deleted,
		Created:     FromSecs(get.GetFloat("created_time")),
		Updated:     FromSecs(get.GetFloat("updated_time")),
	}

	return r, nil
}

func (db *DB) GetRunnerTags(ctx context.Context, runner *com.Runner) error {
	if runner.ID <= 0 {
		return com.ErrNoID
	}

	conn := db.get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.put(conn)

	get := conn.Prep(`SELECT tags.tag FROM runner_tags INNER JOIN tags ON runner_tags.tag = tags.id WHERE runner = $runner`)
	defer get.Reset()

	get.SetInt64("$runner", runner.ID)

	var tags []string
	for {
		haveRows, err := get.Step()
		if err != nil {
			return err
		} else if !haveRows {
			break
		}
		tag := get.GetText("tag")
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	runner.Tags = tags

	return nil
}

func createRunner(ctx context.Context, conn *sqlite.Conn, runner *com.Runner) error {
	stmt := conn.Prep(`INSERT INTO
		runners(token, description, run_untagged, locked, max_timeout, active, created_time, updated_time)
		VALUES($token, $description, $run_untagged, $locked, $max_timeout, $active, $created_time, $updated_time)`)
	defer stmt.Reset()

	t := proc.Now(ctx)
	updated := *runner
	updated.Created = t
	updated.Updated = t

	stmt.SetText("$token", updated.Token)
	stmt.SetText("$description", updated.Description)
	stmt.SetInt64("$run_untagged", btoi(updated.RunUntagged))
	stmt.SetInt64("$locked", btoi(updated.Locked))
	stmt.SetInt64("$active", btoi(updated.Active))
	stmt.SetInt64("$max_timeout", dtoi(updated.MaxTimeout))
	stmt.SetFloat("$created_time", ToSecs(updated.Created))
	stmt.SetFloat("$updated_time", ToSecs(updated.Updated))
	_, err := stmt.Step()
	if err != nil {
		return err
	}

	id := conn.LastInsertRowID()
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

	conn := db.get(ctx)
	if conn == nil {
		return ErrNoConnection
	}
	defer db.put(conn)

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
	return db.pool.Close()
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
