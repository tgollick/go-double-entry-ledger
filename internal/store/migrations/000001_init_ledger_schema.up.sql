CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0,
    allowed_overdraft BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    constraint CHK_OVERDRAFT CHECK (balance >= 0 OR allowed_overdraft = TRUE)
);

CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    idempotency_key VARCHAR NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (idempotency_key)
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY,
    amount BIGINT NOT NULL,
    account_id UUID NOT NULL,
    transaction_id UUID NOT NULL,
    FOREIGN KEY (account_id) REFERENCES accounts (id),
    FOREIGN KEY (transaction_id) REFERENCES transactions (id),
    constraint CHK_VALID CHECK (amount <> 0)
);

CREATE UNIQUE INDEX idx_ledger_foreign_keys
ON ledger_entries (account_id, transaction_id);
