package ledger

import "testing"

func TestTransferPairMatrix(t *testing.T) {
	types := []AccountType{Asset, Liability, Equity, Income, Expense}
	want := map[[2]AccountType]bool{
		{Asset, Asset}: true, {Asset, Liability}: true, {Asset, Equity}: true, {Asset, Income}: true, {Asset, Expense}: false,
		{Liability, Asset}: true, {Liability, Liability}: true, {Liability, Equity}: true, {Liability, Income}: true, {Liability, Expense}: false,
		{Equity, Asset}: true, {Equity, Liability}: true, {Equity, Equity}: true, {Equity, Income}: false, {Equity, Expense}: false,
		{Income, Asset}: false, {Income, Liability}: false, {Income, Equity}: false, {Income, Income}: false, {Income, Expense}: false,
		{Expense, Asset}: true, {Expense, Liability}: true, {Expense, Equity}: false, {Expense, Income}: false, {Expense, Expense}: true,
	}
	for _, d := range types {
		for _, c := range types {
			got := TransferPairAllowed(d, c)
			if got != want[[2]AccountType{d, c}] {
				t.Errorf("TransferPairAllowed(%s,%s)=%v", d, c, got)
			}
		}
	}
}

func TestIncomeIsNeverDebitedOnTransfers(t *testing.T) {
	for _, c := range []AccountType{Asset, Liability, Equity, Income, Expense} {
		if TransferPairAllowed(Income, c) {
			t.Fatalf("income must not be the debit side of a transfer (credit=%s)", c)
		}
	}
}
