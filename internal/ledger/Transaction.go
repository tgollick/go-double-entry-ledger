package main

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID             uuid.UUID
	IdempotencyKey string
	CreatedAt      time.Time
}
