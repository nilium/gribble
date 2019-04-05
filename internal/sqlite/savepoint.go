package sqlite

import (
	"context"
	"fmt"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
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

	// Can't use bound names in these, so this is just kind of grody initialization
	qname := QuoteIdentifier(name)
	release := `RELEASE SAVEPOINT ` + qname
	rollback := `ROLLBACK TRANSACTION TO SAVEPOINT ` + qname
	savepoint := `SAVEPOINT ` + qname

	// Create savepoint
	if err = sqlitex.ExecTransient(conn, savepoint, nil); err != nil {
		return err
	}

	// Clean up savepoint -- rollback in case of an error, and release the savepoint either way.
	defer func() {
		rc := recover()
		if rc != nil {
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
			if ferr := sqlitex.ExecTransient(conn, rollback, nil); ferr != nil {
				ferr = &RollbackError{Savepoint: name, Err: ferr, TransactionErr: err}
				panic(ferr)
			}
		}

		if ferr := sqlitex.ExecTransient(conn, release, nil); ferr != nil {
			err = &ReleaseError{Savepoint: name, Err: ferr}
			panic(err)
		}

		if rc != nil {
			// Continue panicking
			panic(rc)
		}
	}()

	return transaction()
}
