package seed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Muddyscode/ledger-api/internal/auth"
	"github.com/Muddyscode/ledger-api/internal/ledger"
	"github.com/Muddyscode/ledger-api/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const OpeningKobo int64 = 1_000_000

type Spec struct {
	Email    string
	Password string
}

func Run(ctx context.Context, st *store.Store, spec Spec, log *slog.Logger) error {
	hash, err := auth.HashPassword(spec.Password)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	op, err := st.CreateOperator(ctx, st.Pool, id, spec.Email, hash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errorsAsPg(err, &pgErr) && pgErr.Code == "23505" {
			op, err = st.GetOperatorByEmail(ctx, st.Pool, spec.Email)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	type specAcct struct {
		code, name string
		typ        ledger.AccountType
	}
	chart := []specAcct{
		{"1000", "Cash", ledger.Asset},
		{"1010", "Settlement", ledger.Asset},
		{"2000", "Wallet · Alice", ledger.Liability},
		{"2010", "Wallet · Bob", ledger.Liability},
		{"3000", "Opening equity", ledger.Equity},
		{"4000", "Fee income", ledger.Income},
		{"5000", "Operating expense", ledger.Expense},
	}
	for _, c := range chart {
		aid, err := uuid.NewV7()
		if err != nil {
			return err
		}
		_, err = st.CreateAccount(ctx, st.Pool, ledger.Account{
			ID:         aid,
			OperatorID: op.ID,
			Code:       c.code,
			Name:       c.name,
			Type:       c.typ,
			Currency:   "NGN",
			Status:     ledger.AccountOpen,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errorsAsPg(err, &pgErr) && pgErr.Code == "23505" {
				continue
			}
			return err
		}
	}

	n, err := st.CountPostings(ctx, st.Pool, op.ID)
	if err != nil {
		return err
	}
	if n > 0 {
		log.Info("seed skipped opening journal; postings already exist", "operator", op.Email)
		return nil
	}

	cash, err := st.GetAccountByCode(ctx, st.Pool, op.ID, "1000")
	if err != nil {
		return err
	}
	eq, err := st.GetAccountByCode(ctx, st.Pool, op.ID, "3000")
	if err != nil {
		return err
	}

	poster := ledger.StorePoster{
		LockAccounts:  st.LockAccounts,
		InsertJournal: st.InsertJournal,
		InsertPosting: st.InsertPosting,
		UpdateBalance: st.UpdateBalance,
	}
	err = st.WithTx(ctx, func(tx pgx.Tx) error {
		_, err := ledger.Post(ctx, tx, poster, ledger.PostRequest{
			OperatorID:  op.ID,
			Description: "opening balances",
			Lines: []ledger.Line{
				{AccountID: cash.ID, Direction: ledger.Debit, Amount: OpeningKobo},
				{AccountID: eq.ID, Direction: ledger.Credit, Amount: OpeningKobo},
			},
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("seed opening journal: %w", err)
	}
	log.Info("seeded demo book", "email", spec.Email, "opening_kobo", OpeningKobo)
	return nil
}

func errorsAsPg(err error, target **pgconn.PgError) bool {
	return store.IsUnique(err, target)
}
