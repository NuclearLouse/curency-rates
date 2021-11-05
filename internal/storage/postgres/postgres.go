package postgres

import (
	"context"
	"currency-rates/internal/datastructs"
	"currency-rates/internal/storage"
	"fmt"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Postgres struct {
	*pgxpool.Pool
}

func New(pool *pgxpool.Pool) storage.Storer {
	return &Postgres{pool}
}

func (db *Postgres) LastDateUpdate(ctx context.Context) (time.Time, error) {
	lastUpdate := time.Time{}
	var date pgtype.Date
	if err := db.QueryRow(ctx,
		"SELECT date_rate FROM web_backend__settings.t_currency_rate LIMIT 1",
	).Scan(
		&date,
	); err != nil {
		if err == pgx.ErrNoRows {
			return lastUpdate, nil
		}
		return lastUpdate, err
	}

	if date.Status != pgtype.Null {
		lastUpdate = date.Time
	}
	return lastUpdate, nil
}

func (db *Postgres) UpdateCurencyRates(ctx context.Context, rates map[string]*datastructs.CurrencyRates) (bool, error) {

	tx, err := db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin a transaction: %s", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "TRUNCATE TABLE web_backend__settings.t_currency_rate"); err != nil {
		return false, fmt.Errorf("truncate currency rates table: %s", err)
	}

	if _, err := tx.Prepare(ctx,
		"insert",
		`INSERT INTO web_backend__settings.t_currency_rate VALUES ($1,$2,$3,$4)`,
	); err != nil {
		return false, fmt.Errorf("prepare insert statement: %s", err)
	}

	batch := &pgx.Batch{}
	for _, c := range rates {
		batch.Queue(
			"insert",
			c.Name,
			c.Code,
			c.Proportion,
			c.Date,
		)
	}
	br := tx.SendBatch(ctx, batch)

	if err := br.Close(); err != nil {
		return false, fmt.Errorf("close insert statement: %s", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit a transaction: %s", err)
	}

	return true, nil
}

func (db *Postgres) InvokeBulkLoadUpdate(ctx context.Context) error {
	_, err := db.Exec(ctx,
		"SELECT * FROM web_backend__settings.f_currency_bulk_load_upd(11,10)")
	return err
}
