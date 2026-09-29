package itn

import "fmt"

// UnauthorizedError is returned when the ITN server rejects the request
// signature (HTTP 401): the signature is wrong, or the key's public half is
// not in the daemon's --itn-keys.
type UnauthorizedError struct {
	QueryName string
}

func (e *UnauthorizedError) Error() string {
	return fmt.Sprintf("ITN server rejected the signature of %s (HTTP 401); is the public key in --itn-keys?", e.QueryName)
}

// SequencingError is returned when the ITN server still rejects the
// sequence information (HTTP 412) after a new auth handshake.
type SequencingError struct {
	QueryName string
}

func (e *SequencingError) Error() string {
	return fmt.Sprintf("ITN server rejected the sequence number of %s again after a new auth (HTTP 412)", e.QueryName)
}

// InvalidKeyError is returned when an ITN key cannot be decoded.
type InvalidKeyError struct {
	Reason string
}

func (e *InvalidKeyError) Error() string {
	return "invalid ITN key: " + e.Reason
}

// HTTPError is the LastError of a mina.ConnectionError when the daemon
// answered with an HTTP error status other than 401 and 412. Use errors.As
// to read the status, for example to tell a node that is unwell (5xx) from
// a request that no node would accept.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}
