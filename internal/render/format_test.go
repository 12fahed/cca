package render

import "testing"

func TestTokens(t *testing.T) {
	tests := map[int64]string{
		0: "0", 1: "1", 999: "999",
		1000: "1.0K", 1500: "1.5K", 41153: "41.2K", 999999: "1000.0K",
		1_000_000: "1.0M", 2_840_000: "2.8M", 298_400_000: "298.4M",
		19_007_930:    "19.0M", // a trailing zero is kept so columns line up
		1_000_000_000: "1.0B", 1_318_089_194: "1.3B",
		1_500_000_000_000: "1.5T",
		-41153:            "-41.2K",
	}
	for in, want := range tests {
		if got := Tokens(in); got != want {
			t.Errorf("Tokens(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestUSD(t *testing.T) {
	tests := map[float64]string{
		0: "$0.00", 211.46: "$211.46", 1259.129: "$1,259.13",
		1_262_370.5: "$1,262,370.50", -1234.5: "-$1,234.50",
		0.42: "$0.42",
		// A tiny but non-zero spend must not read as nothing.
		0.0004: "$0.0004",
		0.009:  "$0.0090",
	}
	for in, want := range tests {
		if got := USD(in); got != want {
			t.Errorf("USD(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	tests := map[int64]string{
		0: "0", 42: "42", 999: "999", 1000: "1,000",
		5261: "5,261", 1_318_089_194: "1,318,089,194", -1000: "-1,000",
	}
	for in, want := range tests {
		if got := Count(in); got != want {
			t.Errorf("Count(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 sessions"}, {1, "1 session"}, {2, "2 sessions"}, {5261, "5,261 sessions"},
	}
	for _, tt := range tests {
		if got := Plural(tt.n, "session", "sessions"); got != tt.want {
			t.Errorf("Plural(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
