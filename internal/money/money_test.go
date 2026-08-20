package money

import (
	"encoding/json"
	"testing"
)

func TestParseJSONAmountAcceptsIntegerKobo(t *testing.T) {
	n, err := ParseJSONAmount(json.RawMessage("5000"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5000 {
		t.Fatalf("got %d", n)
	}
}

func TestParseJSONAmountRejectsNonIntegers(t *testing.T) {
	cases := []string{
		`10.5`,
		`"1000"`,
		`1e2`,
		`1E2`,
		`null`,
		``,
		`0`,
		`-1`,
		`true`,
		`{}`,
		`9223372036854775808`,
	}
	for _, c := range cases {
		if _, err := ParseJSONAmount(json.RawMessage(c)); err == nil {
			t.Fatalf("expected error for %s", c)
		}
	}
}

func TestParseJSONAmountRejectsFloatThatWouldPassAtoi(t *testing.T) {
	// A broken parser using json.Number.Int64() can coerce 1e2 → 100.
	if _, err := ParseJSONAmount(json.RawMessage(`1e2`)); err == nil {
		t.Fatal("scientific notation must not be accepted as money")
	}
}

func TestFormatNGN(t *testing.T) {
	cases := map[int64]string{
		0:       "₦0.00",
		99:      "₦0.99",
		100:     "₦1.00",
		1234567: "₦12,345.67",
		-5000:   "-₦50.00",
	}
	for in, want := range cases {
		if got := FormatNGN(in); got != want {
			t.Fatalf("FormatNGN(%d)=%q want %q", in, got, want)
		}
	}
}
