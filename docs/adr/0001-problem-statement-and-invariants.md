# System Invariants & Guarantees

> This document outlines the fundamental invariants the ledger engine must uphold at all times.

---

### Core Principles

1. **Invariant 1 — Zero-Sum Conservation**  
   Every committed entry must have greater than or equal to 2 postings whose amount values sum to 0.

2. **Invariant 2 — Immutability**  
   Rows inside of grouped entries and postings must never have `UPDATED` or `DELETED`. Any corrections to amounts require their own compensating reversal entry to balance the sum.

3. **Invariant 3 — Idempotent Execution**  
   If a user submits $N$ identical requests with the same idempotency key, we always produce at most 1 ledger state transition (never charge twice).

---

### Implementation Strategy

These will be ensured through the use of both:

* **Idempotency Keys**
* An **Immutable Double Entry model**  
* *Additional functionality where required*

> **Note:** Requirements will naturally evolve over the course of development.
