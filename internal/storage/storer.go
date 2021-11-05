package storage

import (
	"context"
	"time"

	"currency-rates/internal/datastructs"
)

type Storer interface {
	UpdateCurencyRates(ctx context.Context, rates map[string]*datastructs.CurrencyRates) (bool, error)
	LastDateUpdate(ctx context.Context) (time.Time, error)
	InvokeBulkLoadUpdate(ctx context.Context) error
}
