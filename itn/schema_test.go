package itn

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
)

// Offline check of the ITN documents in queries.go against
// ../schema/itn_graphql_schema.json: every selected field must exist on its
// type, and every argument must exist on its field. The documents are simple
// (no fragments or aliases), so a small parser is enough.

type schemaType struct {
	Name   string        `json:"name"`
	Fields []schemaField `json:"fields"`
}

type schemaField struct {
	Name string `json:"name"`
	Args []struct {
		Name string `json:"name"`
	} `json:"args"`
	Type typeRef `json:"type"`
}

type typeRef struct {
	Name   *string  `json:"name"`
	OfType *typeRef `json:"ofType"`
}

func (t typeRef) base() string {
	if t.Name != nil {
		return *t.Name
	}
	return t.OfType.base()
}

type itnSchema struct {
	query, mutation string
	types           map[string]schemaType
}

func loadSchema(t *testing.T) itnSchema {
	t.Helper()
	raw, err := os.ReadFile("../schema/itn_graphql_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Schema struct {
				QueryType    struct{ Name string } `json:"queryType"`
				MutationType struct{ Name string } `json:"mutationType"`
				Types        []schemaType          `json:"types"`
			} `json:"__schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	s := itnSchema{
		query:    doc.Data.Schema.QueryType.Name,
		mutation: doc.Data.Schema.MutationType.Name,
		types:    map[string]schemaType{},
	}
	for _, ty := range doc.Data.Schema.Types {
		s.types[ty.Name] = ty
	}
	return s
}

func tokenize(doc string) []string {
	var toks []string
	rs := []rune(doc)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_' || rs[j] == '$') {
				j++
			}
			toks = append(toks, string(rs[i:j]))
			i = j
		case strings.ContainsRune("(){}:!,[]", r):
			toks = append(toks, string(r))
			i++
		default:
			i++
		}
	}
	return toks
}

// checkSelection checks the selection set at toks[i] ("{") against type ty
// and returns the index after its closing "}".
func (s itnSchema) checkSelection(ty string, toks []string, i int) (next int, problem string) {
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
			return i, ty + "." + name + " is not in the ITN schema"
		}
		i++
		if toks[i] == "(" {
			for i++; toks[i] != ")"; i++ {
				if toks[i+1] != ":" || strings.HasPrefix(toks[i], "$") {
					continue
				}
				found := false
				for _, a := range field.Args {
					found = found || a.Name == toks[i]
				}
				if !found {
					return i, ty + "." + name + " has no argument " + toks[i]
				}
			}
			i++
		}
		if toks[i] == "{" {
			if i, problem = s.checkSelection(field.Type.base(), toks, i); problem != "" {
				return i, problem
			}
		}
	}
	return i + 1, ""
}

func (s itnSchema) check(doc string) string {
	toks := tokenize(doc)
	root := s.query
	if toks[0] == "mutation" {
		root = s.mutation
	}
	depth := 0
	for i, tok := range toks {
		switch tok {
		case "(":
			depth++
		case ")":
			depth--
		case "{":
			if depth == 0 {
				_, problem := s.checkSelection(root, toks, i)
				return problem
			}
		}
	}
	return "no selection set"
}

func TestQueriesMatchVendoredSchema(t *testing.T) {
	s := loadSchema(t)
	for _, doc := range []string{
		QueryAuth, QuerySlotsWon, QueryInternalLogs, MutationFlushInternalLogs,
		MutationSchedulePayments, MutationScheduleZkappCommands,
		MutationStopScheduledTransactions, MutationUpdateGating,
		MutationStopDaemon, MutationZkappCommandLimit,
	} {
		if problem := s.check(doc); problem != "" {
			t.Errorf("%s:\n%s", problem, doc)
		}
	}
}

// The checker must catch drift: an unknown field, an unknown argument, and
// stopPayments, a name hand-written clients used that the daemon lacks.
func TestSchemaCheckerRejectsDrift(t *testing.T) {
	s := loadSchema(t)
	for doc, want := range map[string]string{
		"query { auth { serverUuid noSuchField } }":                "is not in the ITN schema",
		"mutation ($x: Int!) { flushInternalLogs(noSuchArg: $x) }": "has no argument",
		"mutation ($h: String!) { stopPayments(handle: $h) }":      "is not in the ITN schema",
	} {
		if problem := s.check(doc); !strings.Contains(problem, want) {
			t.Errorf("%q: got %q, want %q", doc, problem, want)
		}
	}
}
