// Package entrepo implements the repository interfaces with Ent.
// It is the only package (besides internal/database) that imports ent.
package entrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/internal/apperror"
)

type txKey struct{}

// base gives each repository access to the transactional client if present.
type base struct {
	client *ent.Client
}

func (b base) db(ctx context.Context) *ent.Client {
	if tx, ok := ctx.Value(txKey{}).(*ent.Tx); ok {
		return tx.Client()
	}
	return b.client
}

func inTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(*ent.Tx)
	return ok
}

// TxManager implements repository.TxManager.
type TxManager struct {
	client *ent.Client
}

func NewTxManager(client *ent.Client) *TxManager {
	return &TxManager{client: client}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if inTx(ctx) {
		return fn(ctx)
	}

	tx, err := m.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// mapErr translates Ent errors into application errors.
func mapErr(err error, entity, conflictMsg string) error {
	switch {
	case err == nil:
		return nil
	case ent.IsNotFound(err):
		return apperror.NotFound(entity)
	case ent.IsConstraintError(err):
		if conflictMsg == "" {
			conflictMsg = entity + " conflicts with an existing record"
		}
		e := apperror.Conflict(conflictMsg)
		e.Err = err
		return e
	case ent.IsValidationError(err):
		e := apperror.Validation(err.Error())
		e.Err = err
		return e
	default:
		return fmt.Errorf("%s repository: %w", entity, err)
	}
}

// order builds an ORDER BY on a column, with id as a stable tie-breaker.
// field must be a trusted column name (services validate sort keys).
func order[O ~func(*entsql.Selector)](field string, desc bool) []O {
	opt := entsql.OrderAsc()
	if desc {
		opt = entsql.OrderDesc()
	}
	return []O{
		O(entsql.OrderByField(field, opt).ToFunc()),
		O(entsql.OrderByField("id", opt).ToFunc()),
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
