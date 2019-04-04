package sqlite

import (
	"context"
	"fmt"

	"crawshaw.io/sqlite"
)

type ReleaseError struct {
	Savepoint string
	Err       error
}

func (e *ReleaseError) Error() string {
	return fmt.Sprintf("error releasing savepoint %q: %v", e.Savepoint, e.Err)
}

type RollbackError struct {
	Savepoint      string
	Err            error
	TransactionErr error
}

func (e *RollbackError) Error() string {
	return fmt.Sprintf("error rolling back to savepoint %q: %v (caused by: %v)", e.Savepoint, e.Err, e.TransactionErr)
}

func InSavepoint(ctx context.Context, conn *sqlite.Conn, name string, transaction func() error) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}

	qname := QuoteIdentifier(name)

	// Can't use bound names in these, so this is just kind of grody initialization
	release, _, err := conn.PrepareTransient(`RELEASE SAVEPOINT ` + qname)
	if err != nil {
		return err
	}
	defer release.Finalize()

	rollback, _, err := conn.PrepareTransient(`ROLLBACK TRANSACTION TO SAVEPOINT ` + qname)
	if err != nil {
		return err
	}
	defer rollback.Finalize()

	savepoint, _, err := conn.PrepareTransient(`SAVEPOINT ` + qname)
	if err != nil {
		return err
	}
	defer savepoint.Finalize()

	// Create savepoint
	if _, err = savepoint.Step(); err != nil {
		return err
	}

	// Clean up savepoint -- rollback in case of an error, and release the savepoint either way.
	defer func() {
		if rc := recover(); rc != nil {
			perr, ok := rc.(error)
			if !ok || perr == nil {
				perr = fmt.Errorf("transaction panic: %T: %v", rc, rc)
			}
			if err == nil {
				err = perr
			}
		}

		if err == nil {
			err = ctx.Err()
		}

		if err != nil {
			if _, ferr := rollback.Step(); ferr != nil {
				err = &RollbackError{Savepoint: name, Err: ferr, TransactionErr: err}
			}
		}

		if _, ferr := release.Step(); ferr != nil {
			err = &ReleaseError{Savepoint: name, Err: ferr}
		}
	}()

	return transaction()
}
