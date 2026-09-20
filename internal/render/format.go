// Package render writes reports as aligned tables, JSON, or CSV.
package render

import (
	"fmt"
	"math"
	"strconv"
)

// Tokens abbreviates a count for table display: 41153 becomes 41.2K.
//
// Tables abbreviate, machine-readable output does not. A JSON consumer needs
// the exact figure, and a reader scanning a column needs a number short enough
// to compare at a glance.
func Tokens(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := abbreviate(float64(n))
	if neg {
		return "-" + s
	}
	return s
}

func abbreviate(v float64) string {
	switch {
	case v >= 1e12:
		return oneDecimal(v/1e12) + "T"
	case v >= 1e9:
		return oneDecimal(v/1e9) + "B"
	case v >= 1e6:
		return oneDecimal(v/1e6) + "M"
	case v >= 1e3:
		return oneDecimal(v/1e3) + "K"
	}
	return strconv.FormatInt(int64(v), 10)
}

// oneDecimal always keeps the decimal place, including a trailing zero. In a
// column, 19.0M beside 603.8M reads as the same kind of number; 19M does not.
func oneDecimal(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// USD formats a cost for display. Sub-cent amounts keep enough precision to
// show that something was spent rather than rounding to $0.00.
func USD(v float64) string {
	switch {
	case v == 0:
		return "$0.00"
	case v < 0.01 && v > -0.01:
		return fmt.Sprintf("$%.4f", v)
	}
	// Thousands separators: totals run into four figures on a real corpus, and
	// an unbroken run of digits is hard to read at a glance.
	whole, frac := math.Modf(math.Abs(v))
	sign := ""
	if v < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s$%s.%02d", sign, Count(int64(whole)), int(math.Round(frac*100)))
}

// Count renders an exact integer with thousands separators, for the small
// counts in headers where abbreviating would lose meaning.
func Count(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := false
	if s[0] == '-' {
		neg, s = true, s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// Plural picks a noun form without the awkward "1 sessions".
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%s %s", Count(int64(n)), plural)
}
