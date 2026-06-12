package mina

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// NanominaPerMina is the number of nanomina in one MINA.
const NanominaPerMina = 1_000_000_000

// Currency represents a Mina currency value with nanomina precision.
// Internally it stores the value as nanomina (the atomic unit).
type Currency struct {
	nanomina uint64
}

// NewCurrency creates a Currency from whole MINA (e.g. 10 means 10 MINA).
func NewCurrency(wholeMina uint64) Currency {
	return Currency{nanomina: wholeMina * NanominaPerMina}
}

// CurrencyFromNanomina creates a Currency from a nanomina value.
func CurrencyFromNanomina(nanomina uint64) Currency {
	return Currency{nanomina: nanomina}
}

// CurrencyFromString parses a decimal MINA string like "1.5" or "100".
func CurrencyFromString(s string) (Currency, error) {
	nanomina, err := parseDecimal(s)
	if err != nil {
		return Currency{}, err
	}
	return Currency{nanomina: nanomina}, nil
}

// MustCurrencyFromString is like CurrencyFromString but panics on error.
func MustCurrencyFromString(s string) Currency {
	c, err := CurrencyFromString(s)
	if err != nil {
		panic(err)
	}
	return c
}

// CurrencyFromGraphQL parses a nanomina string as returned by the GraphQL API.
func CurrencyFromGraphQL(value string) (Currency, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return Currency{}, fmt.Errorf("invalid nanomina value %q: %w", value, err)
	}
	return Currency{nanomina: n}, nil
}

// RandomCurrency returns a random Currency between lower and upper (inclusive).
func RandomCurrency(lower, upper Currency) (Currency, error) {
	if upper.nanomina < lower.nanomina {
		return Currency{}, fmt.Errorf("upper bound must be >= lower bound")
	}
	if lower.nanomina == upper.nanomina {
		return lower, nil
	}
	delta := rand.Uint64() % (upper.nanomina - lower.nanomina + 1)
	return Currency{nanomina: lower.nanomina + delta}, nil
}

// Nanomina returns the value in nanomina.
func (c Currency) Nanomina() uint64 {
	return c.nanomina
}

// Mina returns the decimal string representation in whole MINA (e.g. "1.500000000").
// The fractional part is always rendered with the full 9 nanomina digits so the
// output round-trips exactly through CurrencyFromString.
func (c Currency) Mina() string {
	s := strconv.FormatUint(c.nanomina, 10)
	if len(s) > 9 {
		return s[:len(s)-9] + "." + s[len(s)-9:]
	}
	return "0." + strings.Repeat("0", 9-len(s)) + s
}

// NanominaString returns the nanomina value as a string (for GraphQL API).
func (c Currency) NanominaString() string {
	return strconv.FormatUint(c.nanomina, 10)
}

// String implements fmt.Stringer.
func (c Currency) String() string {
	return c.Mina()
}

// IsZero returns true if the currency value is zero.
func (c Currency) IsZero() bool {
	return c.nanomina == 0
}

// Equal returns true if two currency values are equal.
func (c Currency) Equal(other Currency) bool {
	return c.nanomina == other.nanomina
}

// Less returns true if c < other.
func (c Currency) Less(other Currency) bool {
	return c.nanomina < other.nanomina
}

// LessOrEqual returns true if c <= other.
func (c Currency) LessOrEqual(other Currency) bool {
	return c.nanomina <= other.nanomina
}

// Greater returns true if c > other.
func (c Currency) Greater(other Currency) bool {
	return c.nanomina > other.nanomina
}

// GreaterOrEqual returns true if c >= other.
func (c Currency) GreaterOrEqual(other Currency) bool {
	return c.nanomina >= other.nanomina
}

// Add returns the sum of two currency values.
func (c Currency) Add(other Currency) Currency {
	return Currency{nanomina: c.nanomina + other.nanomina}
}

// Sub returns the difference of two currency values.
// Returns an error if the result would be negative.
func (c Currency) Sub(other Currency) (Currency, error) {
	if c.nanomina < other.nanomina {
		return Currency{}, &CurrencyUnderflowError{
			A: c,
			B: other,
		}
	}
	return Currency{nanomina: c.nanomina - other.nanomina}, nil
}

// Mul returns the currency multiplied by an integer.
func (c Currency) Mul(n uint64) Currency {
	return Currency{nanomina: c.nanomina * n}
}

// CurrencyUnderflowError is returned when a subtraction would result in a negative value.
type CurrencyUnderflowError struct {
	A, B Currency
}

func (e *CurrencyUnderflowError) Error() string {
	return fmt.Sprintf("subtraction would result in negative: %s - %s", e.A, e.B)
}

// parseDecimal converts a decimal MINA string (e.g. "1.5") into nanomina.
// It is implemented by hand rather than via a float/decimal library to keep
// the package dependency-free and to avoid binary-floating-point rounding of
// nanomina amounts.
func parseDecimal(s string) (uint64, error) {
	segments := strings.SplitN(s, ".", 3)
	switch len(segments) {
	case 1:
		whole, err := strconv.ParseUint(segments[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid mina currency format: %s", s)
		}
		return whole * NanominaPerMina, nil
	case 2:
		left, right := segments[0], segments[1]
		if len(right) > 9 {
			return 0, fmt.Errorf("invalid mina currency format: %s (more than 9 decimal places)", s)
		}
		combined := left + right + strings.Repeat("0", 9-len(right))
		n, err := strconv.ParseUint(combined, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid mina currency format: %s", s)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("invalid mina currency format: %s", s)
	}
}
