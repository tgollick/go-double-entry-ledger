package main

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID               uuid.UUID
	Balance          int64
	AllowedOverdraft bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
