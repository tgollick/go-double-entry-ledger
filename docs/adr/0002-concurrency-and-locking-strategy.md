# Concurrency & Locking Strategy

> What if we have a scenario like so? Two requests are sent at the exact same MS:

* **Alice Account Balance:** $30
* **Bob Account Balance:** $0

1. **Alice sends Bob $10**
2. **Alice sends Bob $30**

---

### The Problem: `READ COMMITTED` Race Conditions

With the way the `READ COMMITTED` rules work inside of SQL, whilst the first transaction takes place, the second transaction would be able to read the value of Alice's account before the updates to her account balance are made, causing the second request to update her balance to `-$10`.

### The Solution: Explicit Pessimistic Locking

The solution is to use explicit pessimistic locking through the use of `SELECT ... FOR UPDATE` inside of a `BEGIN`/`COMMIT` block. Therefore, when the second transaction begins at the exact same MS, the row for Alice's account is blocked until the `COMMIT` takes place.

---

### Preventing Deadlocks

There is one issue: **Deadlocks**. If we fire both transactions at the same time, transaction 1 locks Alice's account and transaction 2 locks Bob's account. This causes a deadlock and neither transaction can complete.

To solve this and ensure it's mathematically impossible to deadlock, we can sort the IDs of the accounts we are handling so that the smallest account ID is locked first. Therefore, in the case of two transactions handling the same two accounts, whichever request gets there first locks both accounts and the other transaction has to wait for the `COMMIT` before continuing.

---

### Mitigating 'Hot Accounts'

One issue we could face with this strategy is a **'hot account'** — an account that is involved in 1000s of transactions. This could cause transactions to take ages to complete due to the way we lock the rows of involved accounts. Therefore, we can apply two approaches:

1. **Account Sharding**
2. **Asynchronous Balance Rollups**

We can use account sharding to split hot accounts, such as commission accounts, into 64 different accounts and choose a random account out of the 64 for each transaction, speeding up these transactions by 64 times. In the case of a commission account where we will only be incrementing the amount, we can also add the Ledger Entry to the transaction and add the updates to the account balance into a queue which can be processed in due time through an asynchronous rollup.

> **Invariant Check:** This means we also don't break **Invariant 1 (Zero-Sum Conservation)**, as the Ledger Entry is still contained inside of the transaction, which then still sums to `0`.
