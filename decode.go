package mina

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// scalar is a GraphQL scalar that the daemon sends as a string (UInt32,
// UInt64, Length, Slot, Balance, ...) or as a number (Int). It keeps the
// text, and null.
type scalar struct {
	text string
	null bool
}

func (s *scalar) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*s = scalar{null: true}
		return nil
	case len(b) > 0 && b[0] == '"':
		var text string
		if err := json.Unmarshal(b, &text); err != nil {
			return err
		}
		*s = scalar{text: text}
		return nil
	default:
		*s = scalar{text: string(b)}
		return nil
	}
}

// isNull is true for a null or absent value.
func (s scalar) isNull() bool { return s.null || s.text == "" }

func (s scalar) String() string { return s.text }

// Int returns the value, or 0 if it is null or not an integer.
func (s scalar) Int() int {
	n, _ := strconv.Atoi(s.text)
	return n
}

// OptInt returns nil for null.
func (s scalar) OptInt() *int {
	if s.isNull() {
		return nil
	}
	n, err := strconv.Atoi(s.text)
	if err != nil {
		return nil
	}
	return &n
}

// Currency parses the value; null is zero.
func (s scalar) Currency(field string) (Currency, error) {
	if s.isNull() {
		return Currency{}, nil
	}
	c, err := CurrencyFromGraphQL(s.text)
	if err != nil {
		return Currency{}, fmt.Errorf("parse %s: %w", field, err)
	}
	return c, nil
}

// OptCurrency parses the value; null is nil.
func (s scalar) OptCurrency(field string) (*Currency, error) {
	if s.isNull() {
		return nil, nil
	}
	c, err := s.Currency(field)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// strOrEmpty dereferences a nullable string.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// signatureVar is the value of the $signature variable: null when unset.
func signatureVar(s *SignatureInput) any {
	if s == nil {
		return nil
	}
	return s
}

// commandNode is the GraphQL shape of a payment or delegation in the
// SendPayment and SendDelegation results.
type commandNode struct {
	ID       string      `json:"id"`
	Hash     string      `json:"hash"`
	Kind     string      `json:"kind"`
	Nonce    scalar      `json:"nonce"`
	Source   publicKeyOf `json:"source"`
	Receiver publicKeyOf `json:"receiver"`
	Amount   scalar      `json:"amount"`
	Fee      scalar      `json:"fee"`
	Memo     string      `json:"memo"`
}

func (c commandNode) toSubmitted() (*SubmittedCommand, error) {
	amount, err := c.Amount.OptCurrency("amount")
	if err != nil {
		return nil, err
	}
	fee, err := c.Fee.OptCurrency("fee")
	if err != nil {
		return nil, err
	}
	return &SubmittedCommand{
		ID:       c.ID,
		Hash:     c.Hash,
		Nonce:    c.Nonce.Int(),
		Kind:     c.Kind,
		Source:   c.Source.PublicKey,
		Receiver: c.Receiver.PublicKey,
		Amount:   amount,
		Fee:      fee,
		Memo:     c.Memo,
	}, nil
}

// zkappNode is the GraphQL shape of a zkApp command in SendZkapp and
// PooledZkappCommands.
type zkappNode struct {
	ID           string `json:"id"`
	Hash         string `json:"hash"`
	ZkappCommand struct {
		Memo     string `json:"memo"`
		FeePayer struct {
			Body struct {
				PublicKey  string `json:"publicKey"`
				Fee        scalar `json:"fee"`
				Nonce      scalar `json:"nonce"`
				ValidUntil scalar `json:"validUntil"`
			} `json:"body"`
		} `json:"feePayer"`
	} `json:"zkappCommand"`
	FailureReason []struct {
		Index    scalar   `json:"index"`
		Failures []string `json:"failures"`
	} `json:"failureReason"`
}

func (z zkappNode) toResult() (*ZkappCommandResult, error) {
	body := z.ZkappCommand.FeePayer.Body
	fee, err := body.Fee.Currency("feePayer.fee")
	if err != nil {
		return nil, err
	}
	result := &ZkappCommandResult{
		ID:   z.ID,
		Hash: z.Hash,
		Memo: z.ZkappCommand.Memo,
		FeePayer: ZkappFeePayer{
			PublicKey:  body.PublicKey,
			Fee:        fee,
			Nonce:      body.Nonce.Int(),
			ValidUntil: body.ValidUntil.OptInt(),
		},
	}
	if z.FailureReason != nil {
		result.FailureReason = make([]ZkappFailure, len(z.FailureReason))
		for i, f := range z.FailureReason {
			result.FailureReason[i] = ZkappFailure{Index: f.Index.OptInt(), Failures: f.Failures}
		}
	}
	return result, nil
}
