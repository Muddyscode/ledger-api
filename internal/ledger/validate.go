package ledger

import (
	"math"

	"github.com/google/uuid"
)

const (
	MinLines = 2
	MaxLines = 20
)

func ValidateLines(lines []Line) error {
	if len(lines) < MinLines || len(lines) > MaxLines {
		return Err(CodeInvalidJournal, "journal must have between 2 and 20 postings")
	}
	seen := make(map[uuid.UUID]struct{}, len(lines))
	var debits, credits int64
	for _, l := range lines {
		if l.AccountID == uuid.Nil {
			return Err(CodeInvalidJournal, "account_id is required")
		}
		if !l.Direction.Valid() {
			return Err(CodeInvalidJournal, "direction must be debit or credit")
		}
		if l.Amount <= 0 {
			return Err(CodeInvalidAmount, "amount_minor must be > 0")
		}
		if _, ok := seen[l.AccountID]; ok {
			return Err(CodeDuplicateAccount, "each account may appear at most once in a journal")
		}
		seen[l.AccountID] = struct{}{}
		if l.Direction == Debit {
			if debits > math.MaxInt64-l.Amount {
				return Err(CodeInvalidAmount, "debit total overflows int64")
			}
			debits += l.Amount
		} else {
			if credits > math.MaxInt64-l.Amount {
				return Err(CodeInvalidAmount, "credit total overflows int64")
			}
			credits += l.Amount
		}
	}
	if debits != credits {
		return Unbalanced(debits, credits)
	}
	return nil
}
