package ledger

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDBURL = "postgresql://ledger_admin:secretpassword@localhost:5432/ledger?sslmode=disable"

func SetupTestStore(t *testing.T) (*PostgresLedgerStore, error) {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed ping to test db: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return createNewLedgerStore(pool), nil
}

func TestExecuteTransfer_Success(t *testing.T) {
	store, _ := SetupTestStore(t)
	ctx := context.Background()

	accA, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  10000,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account a: %v", err)
	}

	accB, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  5000,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account b: %v", err)
	}

	_, err = store.ExecuteTransfer(ctx, TransferParams{
		IdempotencyKey: uuid.New().String(),
		FromAccountID:  accA.ID,
		ToAccountID:    accB.ID,
		Amount:         3000,
	})
	if err != nil {
		t.Fatalf("failed to execute transfer: %v", err)
	}

	updatedA, err := store.FetchAccount(ctx, accA.ID)
	if err != nil {
		t.Fatalf("failed to fetch account a: %v", err)
	}

	updatedB, err := store.FetchAccount(ctx, accB.ID)
	if err != nil {
		t.Fatalf("failed to fetch account b: %v", err)
	}

	if updatedA.Balance != 7000 {
		t.Errorf("expected amount 7000 from account a, got: %d", updatedA.Balance)
	}

	if updatedB.Balance != 8000 {
		t.Errorf("expected amount 8000 from account b, got: %d", updatedB.Balance)
	}
}

func TestExecuteTransfer_InsufficentFunds(t *testing.T) {
	store, _ := SetupTestStore(t)
	ctx := context.Background()

	accA, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  1000,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account a: %v", err)
	}

	accB, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  0,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account b: %v", err)
	}

	_, err = store.ExecuteTransfer(ctx, TransferParams{
		IdempotencyKey: uuid.New().String(),
		FromAccountID:  accA.ID,
		ToAccountID:    accB.ID,
		Amount:         5000,
	})
	if err == nil {
		t.Errorf("transaction should fail due to insufficent funds: %v", err)
	}

	updatedA, err := store.FetchAccount(ctx, accA.ID)
	if err != nil {
		t.Fatalf("failed to fetch account a: %v", err)
	}

	updatedB, err := store.FetchAccount(ctx, accB.ID)
	if err != nil {
		t.Fatalf("failed to fetch account b: %v", err)
	}

	if updatedA.Balance != 1000 {
		t.Errorf("expected amount 1000 from account a, got: %d", updatedA.Balance)
	}

	if updatedB.Balance != 0 {
		t.Errorf("expected amount 0 from account b, got: %d", updatedB.Balance)
	}
}

func TestExecuteTransfer_Idempotency(t *testing.T) {
	store, _ := SetupTestStore(t)
	ctx := context.Background()

	accA, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  15000,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account a: %v", err)
	}

	accB, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  0,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account b: %v", err)
	}

	idemKey := uuid.New().String()
	_, err = store.ExecuteTransfer(ctx, TransferParams{
		IdempotencyKey: idemKey,
		FromAccountID:  accA.ID,
		ToAccountID:    accB.ID,
		Amount:         5000,
	})
	if err != nil {
		t.Fatalf("failed to execute transfer: %v", err)
	}

	_, err = store.ExecuteTransfer(ctx, TransferParams{
		IdempotencyKey: idemKey,
		FromAccountID:  accA.ID,
		ToAccountID:    accB.ID,
		Amount:         5000,
	})
	if err == nil {
		t.Errorf("transaction should fail due to duplicate idempotency key: %v", err)
	}

	updatedA, err := store.FetchAccount(ctx, accA.ID)
	if err != nil {
		t.Fatalf("failed to fetch account a: %v", err)
	}

	updatedB, err := store.FetchAccount(ctx, accB.ID)
	if err != nil {
		t.Fatalf("failed to fetch account b: %v", err)
	}

	if updatedA.Balance != 10000 {
		t.Errorf("expected amount 10000 from account a, got: %d", updatedA.Balance)
	}

	if updatedB.Balance != 5000 {
		t.Errorf("expected amount 5000 from account b, got: %d", updatedB.Balance)
	}
}

func TestExecuteTransfer_ConcurrentDeadlock(t *testing.T) {
	store, _ := SetupTestStore(t)
	ctx := context.Background()

	accA, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  100000,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account a: %v", err)
	}

	accB, err := store.CreateAccount(ctx, CreateAccountParams{
		StartingBalance:  100000,
		AllowedOverdraft: false,
	})
	if err != nil {
		t.Fatalf("failed to create account b: %v", err)
	}

	var wg sync.WaitGroup
	transfersPerDirection := 10
	amount := int64(100)

	for i := 0; i < transfersPerDirection; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := store.ExecuteTransfer(ctx, TransferParams{
				IdempotencyKey: uuid.New().String(),
				FromAccountID:  accA.ID,
				ToAccountID:    accB.ID,
				Amount:         amount,
			})
			if err != nil {
				t.Errorf("A -> B transfer failed: %v", err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := store.ExecuteTransfer(ctx, TransferParams{
				IdempotencyKey: uuid.New().String(),
				FromAccountID:  accB.ID,
				ToAccountID:    accA.ID,
				Amount:         amount,
			})
			if err != nil {
				t.Errorf("B -> A transfer failed: %v", err)
			}
		}()
	}

	wg.Wait()

	updatedA, err := store.FetchAccount(ctx, accA.ID)
	if err != nil {
		t.Fatalf("failed to fetch account a: %v", err)
	}

	updatedB, err := store.FetchAccount(ctx, accB.ID)
	if err != nil {
		t.Fatalf("failed to fetch account b: %v", err)
	}

	if updatedA.Balance != 100000 {
		t.Errorf("expected amount 100000 from account a, got: %d", updatedA.Balance)
	}

	if updatedB.Balance != 100000 {
		t.Errorf("expected amount 100000 from account b, got: %d", updatedB.Balance)
	}
}
