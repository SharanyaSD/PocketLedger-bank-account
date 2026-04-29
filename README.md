# bank-account

Concurrent-safe bank account HTTP service in Go. Open/close accounts, deposit, withdraw, check balance, and list transactions — safely under concurrent goroutines via per-account mutexes.

## Design document

The architecture, concurrency model, edge cases, and API contract are specified in **[Bank Account.pdf](../Bank%20Account.pdf)**. Read it first — this README only covers what's needed to run and call the service.

Key decisions from the design doc:

- `sync.Mutex` per account — every balance op locks the entire check-then-update.
- `sync.RWMutex` on the store — concurrent balance reads don't block each other; only create/delete take the write lock.
- Money is `int64` paise/cents. Floats are converted at the HTTP boundary; the domain layer never sees a float.
- Layer separation: `domain/` (business logic) → `service/` (orchestration) → `handler/` (HTTP) → `router/` (wiring).

## Running

```bash
go run .
```

Defaults: listens on `:8080`, persists to `bank-data.json` in the working directory.

Overrides:

```bash
PORT=9090 DATA_FILE=/tmp/bank.json go run .
```

## Tests

```bash
go test -race ./...
```

The `-race` flag is mandatory — it's the only way to catch a missed lock in this codebase.

## API

Base path: `/api/v1`. All responses use the envelope `{ "data": ..., "error": ... }`.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/accounts` | Open a new account |
| GET | `/accounts/:id` | Get account details |
| DELETE | `/accounts/:id` | Close account |
| GET | `/accounts/:id/balance` | Get current balance |
| POST | `/accounts/:id/deposit` | Deposit funds |
| POST | `/accounts/:id/withdraw` | Withdraw funds |
| GET | `/accounts/:id/transactions` | List transactions |

Deposit and withdraw accept an optional `Idempotency-Key` header to make retries safe.

Status codes: `200` ok, `201` created, `400` bad input, `404` not found, `409` closed, `422` insufficient funds / daily limit exceeded, `500` server error. See [errors/errors.go](errors/errors.go) for the full set of sentinel errors.

## Layout

```
bank-account/
├── main.go            // server bootstrap
├── domain/            // Account struct, sync.Mutex, balance ops
├── store/             // registry + JSON snapshot persistence
├── service/           // orchestration between handler and store
├── handler/           // Gin handlers, JSON envelope, error mapping
├── router/            // route registration
├── dto/               // request/response shapes
├── errors/            // sentinel errors
└── middleware/        // RequestID, Logger, Recovery
```

The implementation extends the design doc with a service layer, JSON persistence, idempotency keys on mutating ops, and account holder / type / daily-limit metadata.
