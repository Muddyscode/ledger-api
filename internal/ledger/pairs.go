package ledger

// TransferPairAllowed is the v1 posting policy for two-legged transfers.
// General N-legged journals skip this matrix; reversals skip it too.
func TransferPairAllowed(debit, credit AccountType) bool {
	return transferPairs[[2]AccountType{debit, credit}]
}

var transferPairs = map[[2]AccountType]bool{
	{Asset, Asset}:         true, // bank sweep / reclass
	{Asset, Liability}:     true, // customer deposit
	{Asset, Equity}:        true, // capital in
	{Asset, Income}:        true, // cash revenue
	{Asset, Expense}:       false,
	{Liability, Asset}:     true, // withdrawal
	{Liability, Liability}: true, // p2p
	{Liability, Equity}:    true, // debt to equity
	{Liability, Income}:    true, // fee from wallet
	{Liability, Expense}:   false,
	{Equity, Asset}:        true, // draw
	{Equity, Liability}:    true,
	{Equity, Equity}:       true, // reclass
	{Equity, Income}:       false,
	{Equity, Expense}:      false,
	{Income, Asset}:        false,
	{Income, Liability}:    false,
	{Income, Equity}:       false,
	{Income, Income}:       false,
	{Income, Expense}:      false,
	{Expense, Asset}:       true, // pay cost
	{Expense, Liability}:   true, // accrue
	{Expense, Equity}:      false,
	{Expense, Income}:      false,
	{Expense, Expense}:     true, // reclass
}
