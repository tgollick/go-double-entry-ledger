package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	createAccountQuery = `INSERT INTO accounts (id, balance, allowed_overdraft) VALUES ($1, $2, $3) RETURNING id, balance, allowed_overdraft, created_at, updated_at;`
	fetchAccountQuery  = `SELECT id, balance, allowed_overdraft, created_at, updated_at FROM accounts WHERE id = $1`
)

type TransferParams struct {
	IdempotencyKey string
	FromAccountID  uuid.UUID
	ToAccountID    uuid.UUID
	Amount         int64
}

type CreateAccountParams struct {
	StartingBalance  int64
	AllowedOverdraft bool
}

type PostgresLedgerStore struct {
	pool *pgxpool.Pool
}

func (p *PostgresLedgerStore) CreateAccount(ctx context.Context, a CreateAccountParams) (*Account, error) {
	/*
	* Take parsing arguments -> Perform some kind of validation if not done before call is made ->
	* Execute SQL -> Tranform Return object into Type -> Return the Data
	 */

	// Check to see if a negative balance is passed without an overdraft present on the account
	if a.StartingBalance < 0 && !a.AllowedOverdraft {
		return nil, fmt.Errorf("Cannot have a negative starting balance if an overdraft is not active.")
	}

	// Creating UUID to pass into Query Row function and create struct for return obj
	accountID := uuid.New()
	newAccount := Account{}

	// Run query and scan to new obj
	err := p.pool.QueryRow(ctx, createAccountQuery, accountID, a.StartingBalance, a.AllowedOverdraft).Scan(&newAccount.ID, &newAccount.Balance, &newAccount.AllowedOverdraft, &newAccount.CreatedAt, &newAccount.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Return pointer to obj
	return &newAccount, nil
}

func (p *PostgresLedgerStore) FetchAccount(ctx context.Context, id uuid.UUID) (*Account, error) {
	returnAccount := Account{}

	err := p.pool.QueryRow(ctx, fetchAccountQuery, id).Scan(&returnAccount.ID, &returnAccount.Balance, &returnAccount.AllowedOverdraft, &returnAccount.CreatedAt, &returnAccount.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("account not found")
		}

		return nil, err
	}

	return &returnAccount, nil
}

func (p *PostgresLedgerStore) ExecuteTransfer(ctx context.Context, t TransferParams) (*Transaction, error) {
}
