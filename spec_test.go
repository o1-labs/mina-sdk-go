package mina

// Conformance to the specification in spec/ (a copy of o1-labs/mina-sdk-spec
// at the tag in spec/VERSION):
//
//  1. the query strings in queries.go are exactly the documents of
//     spec/operations.graphql, up to white space, and every document has one;
//  2. likewise the ITN query strings and spec/itn-operations.graphql
//     (spec_itn_test.go).
//
// mina-sdk-spec's CI validates the documents against the daemon's schema.

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"
)

func tokenize(doc string) []string {
	var out []string
	rs := []rune(doc)
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' }
	for i := 0; i < len(rs); {
		switch r := rs[i]; {
		case r == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case isWord(r):
			start := i
			for i < len(rs) && isWord(rs[i]) {
				i++
			}
			out = append(out, string(rs[start:i]))
		default:
			if strings.ContainsRune("(){}:!,[]", r) {
				out = append(out, string(r))
			}
			i++
		}
	}
	return out
}

type specOperation struct {
	kind string
	toks []string
}

// operations returns the operations of a document, by name.
func operations(t *testing.T, doc string) map[string]specOperation {
	t.Helper()
	toks := tokenize(doc)
	ops := map[string]specOperation{}
	for i := 0; i < len(toks); {
		kind, name, start := toks[i], toks[i+1], i
		depth, seenBody := 0, false
		for {
			switch toks[i] {
			case "{":
				depth++
				seenBody = true
			case "}":
				depth--
			}
			i++
			if seenBody && depth == 0 {
				break
			}
		}
		if _, dup := ops[name]; dup {
			t.Fatalf("operation %s twice", name)
		}
		ops[name] = specOperation{kind: kind, toks: toks[start:i]}
	}
	return ops
}

// sdkDocuments is every query string of queries.go.
var sdkDocuments = []string{
	querySyncStatus,
	queryDaemonStatus,
	queryDaemonMetrics,
	queryNetworkID,
	queryGetAccount,
	queryBestChain,
	queryGenesisBlock,
	queryBlock,
	queryGetPeers,
	queryPooledUserCommands,
	queryPooledZkappCommands,
	queryTransactionStatus,
	queryGenesisConstants,
	queryTrackedAccounts,
	querySnarkPool,
	queryForkConfig,
	mutationSendPayment,
	mutationSendDelegation,
	mutationSendZkapp,
	mutationUnlockAccount,
	mutationSetSnarkWorker,
	mutationSetSnarkWorkFee,
}

func TestSDKQueriesAreTheSpecDocuments(t *testing.T) {
	AssertDocumentsAreTheSpec(t, "spec/operations.graphql", sdkDocuments)
}

// AssertDocumentsAreTheSpec checks that documents are exactly the operations
// of specFile. It is exported for spec_itn_test.go (package mina_test),
// which checks the ITN documents; package mina cannot import package itn.
func AssertDocumentsAreTheSpec(t *testing.T, specFile string, documents []string) {
	t.Helper()
	raw, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	spec := operations(t, string(raw))
	var covered []string
	for _, doc := range documents {
		ops := operations(t, doc)
		if len(ops) != 1 {
			t.Fatalf("one named operation per query string:\n%s", doc)
		}
		for name, op := range ops {
			expected, ok := spec[name]
			if !ok {
				t.Fatalf("%s is not in %s", name, specFile)
			}
			if !reflect.DeepEqual(op.toks, expected.toks) {
				t.Errorf("%s differs from %s", name, specFile)
			}
			covered = append(covered, name)
		}
	}
	all := make([]string, 0, len(spec))
	for name := range spec {
		all = append(all, name)
	}
	sort.Strings(covered)
	sort.Strings(all)
	if !reflect.DeepEqual(covered, all) {
		t.Errorf("every operation of %s has one query string: got %v, want %v", specFile, covered, all)
	}
}
