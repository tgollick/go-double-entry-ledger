package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransferParams struct {
	IdempotencyKey string
	FromAccountID  uuid.UUID
	ToAccountID    uuid.UUID
	Amount         int64
}

type PostgresLedgerStore struct {
	pool *pgxpool.Pool
}

func (p *PostgresLedgerStore) CreateAccount(ctx context.Context, startingBalance int64, allowedOverdraft bool) (*Account, error) {
}

func (p *PostgresLedgerStore) FetchAccount(ctx context.Context, id uuid.UUID) (*Account, error) {
}

func (p *PostgresLedgerStore) ExecuteTransfer(ctx context.Context, t TransferParams) (*Transaction, error) {
}
