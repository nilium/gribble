package sqlite

import (
	"context"
	"fmt"

	"crawshaw.io/sqlite"
)

type SimplePatch struct {
	name      string
	component string
	version   int
	apply     func(ctx context.Context, conn *sqlite.Conn) error
}

func NewSimplePatch(name, component string, version int, apply func(context.Context, *sqlite.Conn) error) *SimplePatch {
	return &SimplePatch{
		name:      name,
		component: component,
		version:   version,
		apply:     apply,
	}
}

func (p *SimplePatch) Name() string      { return p.name }
func (p *SimplePatch) Component() string { return p.component }
func (p *SimplePatch) Version() int      { return p.version }
func (p *SimplePatch) Apply(ctx context.Context, conn *sqlite.Conn) error {
	return p.apply(ctx, conn)
}

func StatementPatch(name, component string, version int, sqlStatements ...string) *SimplePatch {
	apply := func(ctx context.Context, conn *sqlite.Conn) error {
		return applyStatements(ctx, conn, sqlStatements...)
	}
	return NewSimplePatch(name, component, version, apply)
}

func applyStatements(ctx context.Context, conn *sqlite.Conn, sqlStatements ...string) error {
	for _, sql := range sqlStatements {
		if err := applyStatement(ctx, conn, sql); err != nil {
			return err
		}
	}
	return nil
}

func applyStatement(ctx context.Context, conn *sqlite.Conn, sql string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stmt, _, err := conn.PrepareTransient(sql)
	if err != nil {
		return err
	}
	defer stmt.Finalize()
	for {
		haveRows, err := stmt.Step()
		if err != nil {
			return nil
		} else if !haveRows {
			return nil
		}
	}
}

type Patch interface {
	Name() string
	Component() string
	Version() int
	Apply(ctx context.Context, conn *sqlite.Conn) error
}

type PatchSet []Patch

func (ps PatchSet) Apply(ctx context.Context, conn *sqlite.Conn) error {
	for i, p := range ps {
		savepoint := fmt.Sprintf("%06d_%s", i+1, p.Name())
		if err := applyPatch(ctx, conn, savepoint, p); err != nil {
			return err
		}
	}
	return nil
}

func applyPatch(ctx context.Context, conn *sqlite.Conn, savepoint string, patch Patch) error {
	getVersion := conn.Prep(`SELECT COUNT(*) AS found FROM versions WHERE component = $component AND version >= $version LIMIT 1`)
	defer getVersion.Reset()
	getVersion.SetText("$component", patch.Component())
	getVersion.SetInt64("$version", int64(patch.Version()))
	ok, err := getVersion.Step()
	if !ok {
		return fmt.Errorf("cannot query for version (%d) of component: %q", patch.Version(), patch.Component())
	} else if err != nil {
		return err
	}

	found := getVersion.GetInt64("found")
	if found > 0 {
		return nil
	}

	updateVersion := conn.Prep(`INSERT INTO versions (component, version) VALUES ($component, $version)
	ON CONFLICT (component) DO UPDATE SET version = excluded.version`)
	defer updateVersion.Reset()
	apply := func() error {
		err := patch.Apply(ctx, conn)
		if err == nil {
			updateVersion.SetText("$component", patch.Component())
			updateVersion.SetInt64("$version", int64(patch.Version()))
			_, err = updateVersion.Step()
		}
		return err
	}
	return InSavepoint(ctx, conn, savepoint, apply)
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
