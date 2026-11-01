package main

import (
	"time"

	"github.com/google/uuid"
)

type LedgerEntry struct {
	ID            uuid.UUID
	Amount        int64
	AccountId     uuid.UUID
	TransactionId uuid.UUID
	CreatedAt     time.Time
}
