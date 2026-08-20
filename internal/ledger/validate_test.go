package ledger

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateLinesRejectsUnbalanced(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	err := ValidateLines([]Line{
		{AccountID: a, Direction: Debit, Amount: 5000},
		{AccountID: b, Direction: Credit, Amount: 4000},
	})
	le := AsError(err)
	if le == nil || le.Code != CodeUnbalanced {
		t.Fatalf("got %v", err)
	}
}

func TestValidateLinesAcceptsBalanced(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	if err := ValidateLines([]Line{
		{AccountID: a, Direction: Debit, Amount: 5000},
		{AccountID: b, Direction: Credit, Amount: 5000},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLinesRejectsDuplicateAccount(t *testing.T) {
	a := uuid.New()
	err := ValidateLines([]Line{
		{AccountID: a, Direction: Debit, Amount: 5000},
		{AccountID: a, Direction: Credit, Amount: 5000},
	})
	le := AsError(err)
	if le == nil || le.Code != CodeDuplicateAccount {
		t.Fatalf("got %v", err)
	}
}

func TestValidateLinesRejectsNonPositive(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	err := ValidateLines([]Line{
		{AccountID: a, Direction: Debit, Amount: 0},
		{AccountID: b, Direction: Credit, Amount: 0},
	})
	le := AsError(err)
	if le == nil || le.Code != CodeInvalidAmount {
		t.Fatalf("got %v", err)
	}
}

func TestValidateLinesRejectsOneLine(t *testing.T) {
	err := ValidateLines([]Line{{AccountID: uuid.New(), Direction: Debit, Amount: 1}})
	le := AsError(err)
	if le == nil || le.Code != CodeInvalidJournal {
		t.Fatalf("got %v", err)
	}
}

func TestNaturalDelta(t *testing.T) {
	if NaturalDelta(Asset, Debit, 10) != 10 {
		t.Fatal("asset debit should increase")
	}
	if NaturalDelta(Asset, Credit, 10) != -10 {
		t.Fatal("asset credit should decrease")
	}
	if NaturalDelta(Liability, Credit, 10) != 10 {
		t.Fatal("liability credit should increase")
	}
	if NaturalDelta(Liability, Debit, 10) != -10 {
		t.Fatal("liability debit should decrease")
	}
}
