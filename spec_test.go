package mina

// The common API specification (spec/operations.graphql, see spec/SPEC.md)
// against this SDK:
//
//  1. every document of the specification is valid against
//     schema/graphql_schema.json (fields, arguments, nested selections);
//  2. the query strings in queries.go are exactly the specification's
//     documents, up to white space, and every document has one.

import (
	"encoding/json"
	"fmt"
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

type schemaTypeRef struct {
	Name   *string        `json:"name"`
	OfType *schemaTypeRef `json:"ofType"`
}

func (r *schemaTypeRef) base() string {
	if r.Name != nil {
		return *r.Name
	}
	return r.OfType.base()
}

type schemaField struct {
	Name string        `json:"name"`
	Type schemaTypeRef `json:"type"`
	Args []struct {
		Name string `json:"name"`
	} `json:"args"`
}

type schemaType struct {
	Kind   string        `json:"kind"`
	Name   string        `json:"name"`
	Fields []schemaField `json:"fields"`
}

type specSchema struct {
	query, mutation string
	types           map[string]schemaType
}

func loadSchema(t *testing.T) specSchema {
	t.Helper()
	raw, err := os.ReadFile("schema/graphql_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Data struct {
			Schema struct {
				QueryType    struct{ Name string } `json:"queryType"`
				MutationType struct{ Name string } `json:"mutationType"`
				Types        []schemaType          `json:"types"`
			} `json:"__schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	s := specSchema{
		query:    v.Data.Schema.QueryType.Name,
		mutation: v.Data.Schema.MutationType.Name,
		types:    map[string]schemaType{},
	}
	for _, ty := range v.Data.Schema.Types {
		s.types[ty.Name] = ty
	}
	return s
}

// selection checks the selection set at toks[i] against type ty.
func (s specSchema) selection(ty string, toks []string, i int, problems *[]string) int {
	i++
	for toks[i] != "}" {
		name := toks[i]
		var field *schemaField
		for k := range s.types[ty].Fields {
			if s.types[ty].Fields[k].Name == name {
				field = &s.types[ty].Fields[k]
			}
		}
		if field == nil {
			*problems = append(*problems, fmt.Sprintf("%s.%s is not in the schema", ty, name))
			return len(toks) - 1
		}
		i++
		if toks[i] == "(" {
			args := map[string]bool{}
			for _, a := range field.Args {
				args[a.Name] = true
			}
			depth := 1
			for i++; depth > 0; i++ {
				switch arg := toks[i]; {
				case arg == "(" || arg == "{":
					depth++
				case arg == ")" || arg == "}":
					depth--
				case depth == 1 && toks[i+1] == ":" && !strings.HasPrefix(arg, "$") && !args[arg]:
					*problems = append(*problems, fmt.Sprintf("%s.%s has no argument %s", ty, name, arg))
				}
			}
		}
		fieldType := field.Type.base()
		if toks[i] == "{" {
			i = s.selection(fieldType, toks, i, problems)
		} else if s.types[fieldType].Kind == "OBJECT" {
			*problems = append(*problems, fmt.Sprintf("%s.%s is an object and needs a selection", ty, name))
		}
	}
	return i + 1
}

func (s specSchema) check(op specOperation) []string {
	root := s.query
	if op.kind == "mutation" {
		root = s.mutation
	}
	depth, start := 0, -1
	for i, tok := range op.toks {
		switch tok {
		case "(":
			depth++
		case ")":
			depth--
		}
		if depth == 0 && tok == "{" {
			start = i
			break
		}
	}
	var problems []string
	s.selection(root, op.toks, start, &problems)
	return problems
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

func readSpec(t *testing.T) map[string]specOperation {
	t.Helper()
	raw, err := os.ReadFile("spec/operations.graphql")
	if err != nil {
		t.Fatal(err)
	}
	return operations(t, string(raw))
}

func TestSpecDocumentsAreValidAgainstTheSchema(t *testing.T) {
	schema := loadSchema(t)
	spec := readSpec(t)
	if len(spec) != 22 {
		t.Fatalf("the specification has %d operations, want 22", len(spec))
	}
	for name, op := range spec {
		if problems := schema.check(op); len(problems) > 0 {
			t.Errorf("%s: %v", name, problems)
		}
	}
}

func TestSDKQueriesAreTheSpecDocuments(t *testing.T) {
	spec := readSpec(t)
	var covered []string
	for _, doc := range sdkDocuments {
		ops := operations(t, doc)
		if len(ops) != 1 {
			t.Fatalf("one named operation per query string:\n%s", doc)
		}
		for name, op := range ops {
			expected, ok := spec[name]
			if !ok {
				t.Fatalf("%s is not in the specification", name)
			}
			if !reflect.DeepEqual(op.toks, expected.toks) {
				t.Errorf("%s differs from spec/operations.graphql", name)
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
		t.Errorf("every specification operation has one query string: got %v, want %v", covered, all)
	}
}

func TestSpecCheckerCatchesDrift(t *testing.T) {
	schema := loadSchema(t)
	check := func(doc string) []string {
		for _, op := range operations(t, doc) {
			return schema.check(op)
		}
		return nil
	}
	cases := []struct{ doc, want string }{
		{"query A { snarkPool { workIdz } }", "not in the schema"},
		{"query A($p: ID) { transactionStatus(paymentX: $p) }", "no argument"},
		{"query A { genesisBlock }", "needs a selection"},
	}
	for _, c := range cases {
		if p := check(c.doc); len(p) == 0 || !strings.Contains(p[0], c.want) {
			t.Errorf("%q: got %v, want a problem with %q", c.doc, p, c.want)
		}
	}
	if p := check("mutation A($f: UInt64!) { setSnarkWorkFee(input: {fee: $f}) { lastFee } }"); len(p) > 0 {
		t.Errorf("valid document reported: %v", p)
	}
}
