# Ledger

A double-entry money core: accounts, postings, transfers, statements, and a small operator console.

This is not a wallet CRUD app. Posted history is immutable. Money is integer **kobo** (NGN). Every successful post keeps the chart identity at zero:

```
assets + expenses = liabilities + equity + income
```

or, as the code and `/v1/invariants` compute it:

```
SUM(asset,expense balances) − SUM(liability,equity,income balances) = 0
```

No bank rails. No cards. No floats.

## Run (hiring-manager path)

Needs Docker.

```bash
docker compose up --build
```

Open [http://localhost:8080](http://localhost:8080)

| | |
|---|---|
| Email | `operator@ledger.local` |
| Password | `change-me-now-12` |

You should see seven accounts, Cash at **1,000,000 kobo** (₦10,000.00), and an identity badge of **0**.

1. **Move** → recipe **P2P** → `5000` kobo → Post. Alice decreases, Bob increases, identity stays 0.
2. **Journals** → Reverse that post. Balances restore. The original journal is still there; the reversal is a new journal.
3. `GET /v1/health` returns `{"status":"ok"}`.

Those credentials and the JWT secret in Compose are **dev only**. Do not use them anywhere public.

## Tests

Postgres must be reachable (Compose starts `ledger_test` next to the demo database):

```bash
docker compose up -d postgres
go test ./... -count=1
```

CI runs the same suite against Postgres 16, plus `go vet`, `govulncheck`, and a Docker build.

The tests that matter if someone “refactors” the ledger:

- unbalanced journals are rejected
- JSON floats / scientific notation are rejected as money
- the same `Idempotency-Key` replays the same journal; a different body with that key is `409`
- 50 concurrent transfers on one pair of accounts: no lost updates, identity still 0
- 20 concurrent requests with the same key create one journal
- operator B cannot read operator A’s accounts (`404`)
- reversals append; they do not `UPDATE` posted rows

## Money model

- Currency: **NGN**. Minor unit: **kobo**. `100 kobo = ₦1`.
- Wire and database: `int64` / `BIGINT` only. `"1000"`, `10.5`, and `1e2` are `400`.
- Accounts are typed: `asset`, `liability`, `equity`, `income`, `expense`.
- A **journal** is N balanced postings (2–20). Each account at most once.
- A **transfer** is a 2-legged journal (one debit, one credit) with an extra type-pair policy (income is never the debit side except via reversal).
- Cached `accounts.balance_minor` is updated in the same Postgres transaction as the insert. The journal is the source of truth; `/v1/invariants` also checks cache drift against `account_balances_from_journal`.

Natural balance: debit-normal accounts (`asset`, `expense`) increase on debit; credit-normal accounts increase on credit.

## API (v1)

Auth: `Authorization: Bearer <jwt>` from `POST /v1/auth/register` or `/login`. Password hashes are argon2id.

`Idempotency-Key` is **required** on `POST /v1/transfers`, `POST /v1/journals`, and `POST /v1/journals/{id}/reversal`.

```http
POST /v1/transfers
Idempotency-Key: 0193abc-demo-001
Authorization: Bearer …

{
  "debit_account_id": "<alice-wallet>",
  "credit_account_id": "<bob-wallet>",
  "amount_minor": 5000,
  "description": "p2p Alice to Bob"
}
```

OpenAPI: [`api/openapi.yaml`](api/openapi.yaml).

Demo curl after Compose is up:

```bash
TOKEN=$(curl -s http://localhost:8080/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"email":"operator@ledger.local","password":"change-me-now-12"}' \
  | python -c "import sys,json; print(json.load(sys.stdin)['token'])")

curl -s http://localhost:8080/v1/invariants -H "Authorization: Bearer $TOKEN"
curl -s http://localhost:8080/v1/accounts -H "Authorization: Bearer $TOKEN"
```

## Layout

```
cmd/ledgerd/          process
internal/ledger/      posting engine (the sacred unit)
internal/money/       kobo parse/format; reject floats
internal/store/       SQL
internal/httpserver/  JSON API
internal/console/     operator UI
migrations/           schema + immutability triggers
```

## v1 limits (documented, not accidental)

- Single currency (NGN). The `currency` column exists; anything else is rejected.
- No holds/pending entries. Posted means posted.
- A crash that leaves an idempotency row in `processing` is not swept; that window does not exist in the single-transaction path used here.
- Demo JWT secret and seed password are in Compose labels as non-production.

## License

MIT
