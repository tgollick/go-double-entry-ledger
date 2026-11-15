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
	createAccountQuery         = `INSERT INTO accounts (id, balance, allowed_overdraft) VALUES ($1, $2, $3) RETURNING id, balance, allowed_overdraft, created_at, updated_at;`
	fetchAccountQuery          = `SELECT id, balance, allowed_overdraft, created_at, updated_at FROM accounts WHERE id = $1`
	fetchAccountForUpdateQuery = `SELECT id, balance, allowed_overdraft, created_at, updated_at FROM accounts WHERE id = $1 FOR UPDATE`
	createTransactionQuery     = `INSERT INTO transactions (id, idempotency_key) VALUES ($1, $2) RETURNING id, idempotency_key, created_at`
	createLedgerEntryQuery     = `INSERT INTO ledger_entries (id, amount, account_id, transaction_id) VALUES ($1, $2, $3, $4) RETURNING id, amount, account_id, transaction_id, created_at`
	updateAccountBalance       = `UPDATE accounts SET balance = balance + $1, updated_at = NOW() WHERE id = $2 RETURNING id, balance, allowed_overdraft, created_at, updated_at`
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
	// 1. Check that the sender and reciver aren't the same ID and that the amount is positive, also check a valid idempotency key has been passed in
	if t.Amount <= 0 {
		return nil, fmt.Errorf("cannot transfer negative or zero amounts")
	}

	if t.FromAccountID == t.ToAccountID {
		return nil, fmt.Errorf("cannot transfer to and from the same account")
	}

	if t.IdempotencyKey == "" {
		return nil, fmt.Errorf("please provide a valid idempotency key to execute a transaction")
	}

	// 2. Create the transaction to execute all SQL queries
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	// Defer roll back if any errors occur or the commit fails
	defer tx.Rollback(ctx)

	// 3. Get the from and to account and ensure the from balance has enough to make the transfer
	fromAccount := Account{}
	toAccount := Account{}

	if t.FromAccountID.String() < t.ToAccountID.String() {
		err = tx.QueryRow(ctx, fetchAccountForUpdateQuery, t.FromAccountID).Scan(&fromAccount.ID, &fromAccount.Balance, &fromAccount.AllowedOverdraft, &fromAccount.CreatedAt, &fromAccount.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("account not found")
			}
			return nil, err
		}

		err = tx.QueryRow(ctx, fetchAccountForUpdateQuery, t.ToAccountID).Scan(&toAccount.ID, &toAccount.Balance, &toAccount.AllowedOverdraft, &toAccount.CreatedAt, &toAccount.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("account not found")
			}
			return nil, err
		}
	} else {
		err = tx.QueryRow(ctx, fetchAccountForUpdateQuery, t.ToAccountID).Scan(&toAccount.ID, &toAccount.Balance, &toAccount.AllowedOverdraft, &toAccount.CreatedAt, &toAccount.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("account not found")
			}
			return nil, err
		}

		err = tx.QueryRow(ctx, fetchAccountForUpdateQuery, t.FromAccountID).Scan(&fromAccount.ID, &fromAccount.Balance, &fromAccount.AllowedOverdraft, &fromAccount.CreatedAt, &fromAccount.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("account not found")
			}
			return nil, err
		}
	}

	if fromAccount.Balance < t.Amount && !fromAccount.AllowedOverdraft {
		return nil, fmt.Errorf("insufficent funds to complete transaction")
	}

	// 4. Create the trasaction row
	transaction := Transaction{
		ID:             uuid.New(),
		IdempotencyKey: t.IdempotencyKey,
	}
	err = tx.QueryRow(ctx, createTransactionQuery, transaction.ID, transaction.IdempotencyKey).Scan(&transaction.ID, &transaction.IdempotencyKey, &transaction.CreatedAt)
	if err != nil {
		return nil, err
	}

	// 5. Create the ledger entry for both accounts and create relationship to the transaction
	fromLedgerEntry := LedgerEntry{
		ID:            uuid.New(),
		Amount:        -t.Amount,
		AccountId:     fromAccount.ID,
		TransactionId: transaction.ID,
	}

	toLedgerEntry := LedgerEntry{
		ID:            uuid.New(),
		Amount:        t.Amount,
		AccountId:     toAccount.ID,
		TransactionId: transaction.ID,
	}

	err = tx.QueryRow(ctx, createLedgerEntryQuery, fromLedgerEntry.ID, fromLedgerEntry.Amount, fromLedgerEntry.AccountId, fromLedgerEntry.TransactionId).Scan(&fromLedgerEntry.ID, &fromLedgerEntry.Amount, &fromLedgerEntry.AccountId, &fromLedgerEntry.TransactionId, &fromLedgerEntry.CreatedAt)
	if err != nil {
		return nil, err
	}

	err = tx.QueryRow(ctx, createLedgerEntryQuery, toLedgerEntry.ID, toLedgerEntry.Amount, toLedgerEntry.AccountId, toLedgerEntry.TransactionId).Scan(&toLedgerEntry.ID, &toLedgerEntry.Amount, &toLedgerEntry.AccountId, &toLedgerEntry.TransactionId, &toLedgerEntry.CreatedAt)
	if err != nil {
		return nil, err
	}

	// 6. Update both accounts to reflect the new change
	fromAccountUpdatedBalance := fromAccount.Balance + fromLedgerEntry.Amount
	toAccountUpdatedBalance := toAccount.Balance + toLedgerEntry.Amount

	err = tx.QueryRow(ctx, updateAccountBalance, fromLedgerEntry.Amount, fromAccount.ID).Scan(&fromAccount.ID, &fromAccount.Balance, &fromAccount.AllowedOverdraft, &fromAccount.CreatedAt, &fromAccount.UpdatedAt)
	if err != nil {
		return nil, err
	}

	err = tx.QueryRow(ctx, updateAccountBalance, toLedgerEntry.Amount, toAccount.ID).Scan(&toAccount.ID, &toAccount.Balance, &toAccount.AllowedOverdraft, &toAccount.CreatedAt, &toAccount.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// 7. Final check to see if the balances are updated and correct before returning
	if toAccount.Balance != toAccountUpdatedBalance || fromAccount.Balance != fromAccountUpdatedBalance {
		return nil, fmt.Errorf("new account balance doesn't match updated amount after transaction")
	}

	// 8. Finally we commit all the queries inside of the transaction, if any fail, we roll back
	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return &transaction, nil
}
