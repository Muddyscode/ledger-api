-- Double-entry ledger schema.
-- Money is BIGINT kobo (NGN minor units). No MONEY, NUMERIC, or floating types.

CREATE TABLE operators (
    id              UUID PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id              UUID PRIMARY KEY,
    operator_id     UUID NOT NULL REFERENCES operators (id),
    code            TEXT NOT NULL,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL CHECK (type IN ('asset', 'liability', 'equity', 'income', 'expense')),
    currency        CHAR(3) NOT NULL DEFAULT 'NGN' CHECK (currency = 'NGN'),
    allow_negative  BOOLEAN NOT NULL DEFAULT false,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    balance_minor   BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (operator_id, code)
);

CREATE INDEX accounts_operator_idx ON accounts (operator_id);

CREATE TABLE journals (
    id                    UUID PRIMARY KEY,
    operator_id           UUID NOT NULL REFERENCES operators (id),
    status                TEXT NOT NULL DEFAULT 'posted' CHECK (status = 'posted'),
    description           TEXT NOT NULL DEFAULT '',
    idempotency_key       TEXT,
    reverses_journal_id   UUID REFERENCES journals (id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX journals_operator_created_idx ON journals (operator_id, created_at DESC);
CREATE UNIQUE INDEX journals_one_reversal_idx ON journals (reverses_journal_id)
    WHERE reverses_journal_id IS NOT NULL;
CREATE UNIQUE INDEX journals_idempotency_idx ON journals (operator_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE postings (
    id            UUID PRIMARY KEY,
    journal_id    UUID NOT NULL REFERENCES journals (id),
    account_id    UUID NOT NULL REFERENCES accounts (id),
    direction     TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount_minor  BIGINT NOT NULL CHECK (amount_minor > 0),
    currency      CHAR(3) NOT NULL DEFAULT 'NGN' CHECK (currency = 'NGN')
);

CREATE UNIQUE INDEX postings_journal_account_idx ON postings (journal_id, account_id);
CREATE INDEX postings_account_idx ON postings (account_id);

CREATE TABLE idempotency_keys (
    operator_id    UUID NOT NULL REFERENCES operators (id),
    key            TEXT NOT NULL,
    request_hash   BYTEA NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('processing', 'completed')),
    http_status    INT,
    response_body  JSONB,
    journal_id     UUID REFERENCES journals (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (operator_id, key)
);

-- Posted history is append-only. Corrections are reversing journals, never UPDATEs.
CREATE FUNCTION forbid_posted_mutation() RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'posted journal rows are immutable';
END;
$$;

CREATE TRIGGER journals_immutable
    BEFORE UPDATE OR DELETE ON journals
    FOR EACH ROW
    EXECUTE FUNCTION forbid_posted_mutation();

CREATE TRIGGER postings_immutable
    BEFORE UPDATE OR DELETE ON postings
    FOR EACH ROW
    EXECUTE FUNCTION forbid_posted_mutation();

-- Natural-balance recompute from the journal (source of truth).
CREATE VIEW account_balances_from_journal AS
SELECT
    a.id,
    a.operator_id,
    COALESCE(SUM(
        CASE
            WHEN a.type IN ('asset', 'expense') THEN
                CASE WHEN p.direction = 'debit' THEN p.amount_minor ELSE -p.amount_minor END
            ELSE
                CASE WHEN p.direction = 'credit' THEN p.amount_minor ELSE -p.amount_minor END
        END
    ), 0) AS balance_minor
FROM accounts a
LEFT JOIN postings p ON p.account_id = a.id
GROUP BY a.id, a.operator_id, a.type;
