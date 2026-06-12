//go:build ignore

// check_schema_drift checks for drift between the SDK and a live Mina node
// in two layers:
//
//  1. Introspection diff: compare schema/graphql_schema.json to the live
//     __schema returned by the daemon.
//  2. Live query check: parse queries.go, send each operation with sentinel
//     variables, and classify GraphQL errors as either schema drift
//     (parse/validation) or runtime (auth, value-validation).
//
// Designed for a lightnet-style local daemon (see
// .github/workflows/schema-drift.yml for the CI setup); do not point at a
// public node by default.
//
// Usage:
//
//	go run scripts/check_schema_drift.go --endpoint http://127.0.0.1:8080/graphql --branch master --strict
package main

// The schema diff is implemented in-package (rather than via a JSON-diff
// dependency) so this `go:build ignore` tool stays dependency-free and the
// output can be tuned to GraphQL-specific drift categories. Output is
// human-readable by design; if a machine-readable format is needed later,
// emit JSON behind a flag.
import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

const (
	sentinelSender   = "B62qpRzFVjd56FiHnNfxokVbcHMQLT119My1FEdSq8ss7KomLiSZcan"
	sentinelReceiver = "B62qrPN5Y5yq8kGE3FbVKbGTdTAJNdtNtB5sNVpxyRwWGcDEhpMzc8g"
)

const introspectionQuery = `
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      kind name description
      fields(includeDeprecated: true) {
        name description
        args {
          name description
          type { kind name ofType { kind name ofType { kind name ofType { kind name } } } }
          defaultValue
        }
        type { kind name ofType { kind name ofType { kind name ofType { kind name } } } }
        isDeprecated deprecationReason
      }
      inputFields {
        name description
        type { kind name ofType { kind name ofType { kind name ofType { kind name } } } }
        defaultValue
      }
      interfaces { kind name ofType { kind name } }
      enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason }
      possibleTypes { kind name }
    }
  }
}`

// ─────────────────────────────────────────────────────────────────────────────
// Paths
// ─────────────────────────────────────────────────────────────────────────────

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(file))
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP
// ─────────────────────────────────────────────────────────────────────────────

func postGraphQL(endpoint string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(raw)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, resp.Status, snippet)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 1: introspection diff
// ─────────────────────────────────────────────────────────────────────────────

func sortByName(items []any) {
	nameOf := func(v any) string {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		s, _ := m["name"].(string)
		return s
	}
	sort.SliceStable(items, func(i, j int) bool {
		return nameOf(items[i]) < nameOf(items[j])
	})
}

func normalizeSchema(raw map[string]any) (map[string]any, error) {
	root := raw
	if d, ok := raw["data"].(map[string]any); ok {
		root = d
	}
	sc, ok := root["__schema"].(map[string]any)
	if !ok {
		// The envelope is an error/null payload — surface what we have
		// instead of silently treating it as an empty schema.
		if errs, ok := raw["errors"]; ok {
			return nil, fmt.Errorf("introspection returned errors envelope (no data.__schema): %v", errs)
		}
		return nil, fmt.Errorf("introspection response missing data.__schema")
	}
	s := sc
	types, ok := s["types"].([]any)
	if !ok {
		return nil, fmt.Errorf("introspection __schema.types is not a list (got %T)", s["types"])
	}
	sortByName(types)
	for _, t := range types {
		tm, _ := t.(map[string]any)
		if fields, ok := tm["fields"].([]any); ok {
			sortByName(fields)
			for _, f := range fields {
				if fm, ok := f.(map[string]any); ok {
					if args, ok := fm["args"].([]any); ok {
						sortByName(args)
					}
				}
			}
		}
		if v, ok := tm["inputFields"].([]any); ok {
			sortByName(v)
		}
		if v, ok := tm["enumValues"].([]any); ok {
			sortByName(v)
		}
		if v, ok := tm["interfaces"].([]any); ok {
			sortByName(v)
		}
		if v, ok := tm["possibleTypes"].([]any); ok {
			sortByName(v)
		}
	}
	return map[string]any{
		"queryType":        s["queryType"],
		"mutationType":     s["mutationType"],
		"subscriptionType": s["subscriptionType"],
		"types":            types,
	}, nil
}

func indexByName(items []any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		n, _ := m["name"].(string)
		out[n] = m
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func computeSchemaDiff(local, remote map[string]any) []string {
	var diffs []string
	localTypes, _ := local["types"].([]any)
	remoteTypes, _ := remote["types"].([]any)
	lt := indexByName(localTypes)
	rt := indexByName(remoteTypes)

	for _, n := range sortedKeys(lt) {
		if _, ok := rt[n]; !ok {
			diffs = append(diffs, fmt.Sprintf("REMOVED type: %s", n))
		}
	}
	for _, n := range sortedKeys(rt) {
		if _, ok := lt[n]; !ok {
			diffs = append(diffs, fmt.Sprintf("ADDED type: %s", n))
		}
	}

	for _, n := range sortedKeys(lt) {
		r, ok := rt[n]
		if !ok {
			continue
		}
		l := lt[n]
		// Only compare kind when both sides set it — otherwise a partial dump
		// produces a spurious "<nil> -> OBJECT" line for every type.
		if l["kind"] != nil && r["kind"] != nil && l["kind"] != r["kind"] {
			diffs = append(diffs, fmt.Sprintf("CHANGED %s: kind %v -> %v", n, l["kind"], r["kind"]))
		}

		lf := indexByName(asSlice(l["fields"]))
		rf := indexByName(asSlice(r["fields"]))
		for _, fn := range sortedKeys(lf) {
			if _, ok := rf[fn]; !ok {
				diffs = append(diffs, fmt.Sprintf("REMOVED field: %s.%s", n, fn))
			}
		}
		for _, fn := range sortedKeys(rf) {
			if _, ok := lf[fn]; !ok {
				diffs = append(diffs, fmt.Sprintf("ADDED field: %s.%s", n, fn))
			}
		}
		for _, fn := range sortedKeys(lf) {
			rField, ok := rf[fn]
			if !ok {
				continue
			}
			lField := lf[fn]
			if !reflect.DeepEqual(lField["type"], rField["type"]) {
				diffs = append(diffs, fmt.Sprintf("CHANGED field type: %s.%s", n, fn))
			}
			la := indexByName(asSlice(lField["args"]))
			ra := indexByName(asSlice(rField["args"]))
			for _, an := range sortedKeys(la) {
				if _, ok := ra[an]; !ok {
					diffs = append(diffs, fmt.Sprintf("REMOVED arg: %s.%s(%s)", n, fn, an))
				}
			}
			for _, an := range sortedKeys(ra) {
				if _, ok := la[an]; !ok {
					diffs = append(diffs, fmt.Sprintf("ADDED arg: %s.%s(%s)", n, fn, an))
				}
			}
			// Compare type of args that exist on both sides — without this,
			// a scalar swap like account(token: UInt64 -> TokenId) is missed.
			for _, an := range sortedKeys(la) {
				rArg, ok := ra[an]
				if !ok {
					continue
				}
				if !reflect.DeepEqual(la[an]["type"], rArg["type"]) {
					diffs = append(diffs, fmt.Sprintf("CHANGED arg type: %s.%s(%s)", n, fn, an))
				}
			}
		}

		li := indexByName(asSlice(l["inputFields"]))
		ri := indexByName(asSlice(r["inputFields"]))
		for _, fn := range sortedKeys(li) {
			if _, ok := ri[fn]; !ok {
				diffs = append(diffs, fmt.Sprintf("REMOVED inputField: %s.%s", n, fn))
			}
		}
		for _, fn := range sortedKeys(ri) {
			if _, ok := li[fn]; !ok {
				diffs = append(diffs, fmt.Sprintf("ADDED inputField: %s.%s", n, fn))
			}
		}
		// Compare type of inputFields that exist on both sides.
		for _, fn := range sortedKeys(li) {
			rField, ok := ri[fn]
			if !ok {
				continue
			}
			if !reflect.DeepEqual(li[fn]["type"], rField["type"]) {
				diffs = append(diffs, fmt.Sprintf("CHANGED inputField type: %s.%s", n, fn))
			}
		}

		le := enumNames(asSlice(l["enumValues"]))
		re := enumNames(asSlice(r["enumValues"]))
		for _, en := range sortedSetDiff(le, re) {
			diffs = append(diffs, fmt.Sprintf("REMOVED enumValue: %s.%s", n, en))
		}
		for _, en := range sortedSetDiff(re, le) {
			diffs = append(diffs, fmt.Sprintf("ADDED enumValue: %s.%s", n, en))
		}
	}

	return diffs
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	// Mismatched JSON shape — surface so we don't silently report every entry
	// as ADDED.
	fmt.Fprintf(os.Stderr, "warning: expected list, got %T\n", v)
	return nil
}

func enumNames(items []any) map[string]bool {
	out := map[string]bool{}
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			if n, ok := m["name"].(string); ok {
				out[n] = true
			}
		}
	}
	return out
}

func sortedSetDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 2: live query check
// ─────────────────────────────────────────────────────────────────────────────

// opSingleRe matches a stand-alone "const NAME = `...`" declaration.
var opSingleRe = regexp.MustCompile("(?s)(?m)^const\\s+(\\w+)\\s*=\\s*`(.*?)`")

// opGroupRe captures the body of every "const ( ... )" block; we then scan
// each block for "NAME = `...`" lines (which lack the const keyword).
var opGroupRe = regexp.MustCompile(`(?s)const\s*\(\s*(.*?)\s*\)`)
var opGroupedLineRe = regexp.MustCompile("(?s)(?m)^[ \\t]*(\\w+)\\s*=\\s*`(.*?)`")

// varsRe captures the variable-declaration list of a GraphQL operation
// header. Anchored to the start of the body to avoid matching inner
// field-argument parens like `bestChain(maxLength: 1)`.
var varsRe = regexp.MustCompile(`(?s)^\s*(?:query|mutation|subscription)(?:\s+\w+)?\s*\(([^)]*)\)`)

// declRe matches a single GraphQL variable declaration `$name: Type[!]`.
// The type capture allows list nesting (`[Foo!]!`) and stops before a
// default value (`= …`) — which we ignore for sentinel selection.
var declRe = regexp.MustCompile(`^\$(\w+)\s*:\s*([\w!\[\]]+)`)
var operationStartRe = regexp.MustCompile(`(?i)^\s*(query|mutation|subscription)\b`)

type op struct {
	name string
	body string
}

func parseQueries(src string) []op {
	seen := map[string]bool{}
	var out []op
	add := func(name, body string) {
		// Filter out non-operation backtick consts (e.g. example snippets in
		// the same file). A real GraphQL op starts with one of the operation
		// keywords.
		if !operationStartRe.MatchString(body) {
			return
		}
		if seen[name] {
			return
		}
		seen[name] = true
		out = append(out, op{name: name, body: body})
	}
	for _, m := range opSingleRe.FindAllStringSubmatch(src, -1) {
		add(m[1], m[2])
	}
	for _, gm := range opGroupRe.FindAllStringSubmatch(src, -1) {
		for _, lm := range opGroupedLineRe.FindAllStringSubmatch(gm[1], -1) {
			add(lm[1], lm[2])
		}
	}
	return out
}

type varDecl struct {
	name string
	typ  string
}

func parseVariableDecls(body string) []varDecl {
	m := varsRe.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	var out []varDecl
	for _, raw := range strings.Split(m[1], ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		dm := declRe.FindStringSubmatch(raw)
		if dm == nil {
			continue
		}
		out = append(out, varDecl{name: dm[1], typ: dm[2]})
	}
	return out
}

func sentinelForType(typ string) any {
	base := strings.NewReplacer("[", "", "]", "", "!", "").Replace(typ)
	switch base {
	case "PublicKey":
		return sentinelSender
	case "UInt32", "UInt64", "Fee", "Balance":
		return "1000000000"
	case "Int":
		return 1
	case "String", "TokenId":
		return "1"
	case "Boolean":
		return true
	case "SendPaymentInput":
		return map[string]any{"from": sentinelSender, "to": sentinelReceiver, "amount": "1000000000", "fee": "1000000000"}
	case "SendDelegationInput":
		return map[string]any{"from": sentinelSender, "to": sentinelReceiver, "fee": "1000000000"}
	case "SetSnarkWorkerInput":
		return map[string]any{"publicKey": sentinelSender}
	case "SetSnarkWorkFee":
		return map[string]any{"fee": "1000000000"}
	default:
		return nil
	}
}

func buildVariables(decls []varDecl) (map[string]any, bool) {
	out := map[string]any{}
	for _, d := range decls {
		v := sentinelForType(d.typ)
		if v == nil {
			return nil, false
		}
		out[d.name] = v
	}
	return out, true
}

// driftPatterns are case-insensitive substrings that uniquely identify
// schema-level errors emitted by Mina's GraphQL surface (graphql-ppx / OCaml).
// Each phrase has been chosen so it does NOT overlap with value-coercion
// runtime errors. In particular we avoid bare "expected type" — that phrase
// appears in both "Expected type Foo, found Bar" (value coercion → runtime)
// and "Variable $x of type Foo used in position expecting type Bar" (drift).
var driftPatterns = []string{
	"cannot query field",
	"unknown argument",
	"unknown type",
	"is not defined",
	"is not a subtype",
	"is required",
	"but not provided",
	"used in position expecting type",
	"must have a sub selection",
	"did you mean",
	"unknown directive",
}

// valueCoercionRe matches Mina's "Argument X of type Y expected on field Z,
// found <value>" message, which is value-validation (runtime) — not drift.
var valueCoercionRe = regexp.MustCompile(`(?i)expected on field .* found `)

// classifyError decides whether a GraphQL error reflects a schema mismatch
// ("drift") or an expected runtime failure ("runtime"). The order matters:
// we check structural drift signatures BEFORE the `path` short-circuit, since
// Mina attaches `path` to many validation errors as well.
func classifyError(err map[string]any) string {
	msg, _ := err["message"].(string)
	lc := strings.ToLower(msg)
	for _, p := range driftPatterns {
		if strings.Contains(lc, p) {
			return "drift"
		}
	}
	if valueCoercionRe.MatchString(msg) {
		return "runtime"
	}
	if p, ok := err["path"].([]any); ok && len(p) > 0 {
		return "runtime"
	}
	// Unknown error without a path — surface as drift so silent breakage is
	// visible at least in --strict mode.
	return "drift"
}

type queryStats struct {
	ok       int
	runtime  int
	skipped  []string // ops we couldn't probe — sentinel table is stale
	drift    []string // real schema drift
	failures []string // infra errors (HTTP, JSON parse, network)
}

func runQueryLayer(endpoint string) queryStats {
	stats := queryStats{}
	src, err := os.ReadFile(filepath.Join(repoRoot(), "queries.go"))
	if err != nil {
		fmt.Printf("FAIL: cannot read queries.go: %v\n", err)
		stats.failures = append(stats.failures, fmt.Sprintf("read queries.go: %v", err))
		return stats
	}
	ops := parseQueries(string(src))
	if len(ops) == 0 {
		fmt.Println("WARN: no operations parsed from queries.go")
		stats.failures = append(stats.failures, "no operations parsed from queries.go")
		return stats
	}

	for _, o := range ops {
		decls := parseVariableDecls(o.body)
		vars, ok := buildVariables(decls)
		if !ok {
			missing := missingSentinelTypes(decls)
			fmt.Printf("SKIP  %s (no sentinel for: %s)\n", o.name, strings.Join(missing, ", "))
			stats.skipped = append(stats.skipped, fmt.Sprintf("%s: missing sentinel for %s", o.name, strings.Join(missing, ", ")))
			continue
		}
		payload := map[string]any{"query": o.body, "variables": vars}
		result, err := postGraphQL(endpoint, payload)
		if err != nil {
			fmt.Printf("FAIL  %s: %v\n", o.name, err)
			stats.failures = append(stats.failures, fmt.Sprintf("%s: %v", o.name, err))
			continue
		}
		errs, _ := result["errors"].([]any)
		if len(errs) == 0 {
			fmt.Printf("OK    %s\n", o.name)
			stats.ok++
			continue
		}

		var driftMsgs, runtimeMsgs []string
		for _, e := range errs {
			em, _ := e.(map[string]any)
			msg, _ := em["message"].(string)
			if classifyError(em) == "drift" {
				driftMsgs = append(driftMsgs, msg)
			} else {
				runtimeMsgs = append(runtimeMsgs, msg)
			}
		}
		if len(driftMsgs) > 0 {
			joined := strings.Join(driftMsgs, "; ")
			fmt.Printf("DRIFT %s: %s\n", o.name, joined)
			stats.drift = append(stats.drift, fmt.Sprintf("%s: %s", o.name, joined))
		} else {
			fmt.Printf("RUNTIME %s: %s\n", o.name, strings.Join(runtimeMsgs, "; "))
			stats.runtime++
		}
	}
	return stats
}

func missingSentinelTypes(decls []varDecl) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range decls {
		if sentinelForType(d.typ) == nil && !seen[d.typ] {
			seen[d.typ] = true
			out = append(out, d.typ)
		}
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:8080/graphql", "GraphQL endpoint")
	branch := flag.String("branch", "unknown", "Mina branch being tested")
	strict := flag.Bool("strict", false, "Fail on any drift")
	skipSchema := flag.Bool("skip-schema", false, "Skip introspection diff layer")
	skipQueries := flag.Bool("skip-queries", false, "Skip live query check layer")
	flag.Parse()

	var schemaDiffs []string

	if !*skipSchema {
		fmt.Printf("\n── Layer 1: schema introspection (%s) ──\n", *branch)
		localBytes, err := os.ReadFile(filepath.Join(repoRoot(), "schema", "graphql_schema.json"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot load local schema: %v\n", err)
			os.Exit(2)
		}
		var localRaw map[string]any
		if err := json.Unmarshal(localBytes, &localRaw); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot parse local schema: %v\n", err)
			os.Exit(2)
		}

		fmt.Printf("Fetching introspection from %s...\n", *endpoint)
		remoteRaw, err := postGraphQL(*endpoint, map[string]any{"query": introspectionQuery})
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot fetch remote schema: %v\n", err)
			os.Exit(2)
		}

		local, err := normalizeSchema(localRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: local schema malformed: %v\n", err)
			os.Exit(2)
		}
		remote, err := normalizeSchema(remoteRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: remote introspection malformed: %v\n", err)
			os.Exit(2)
		}
		schemaDiffs = computeSchemaDiff(local, remote)
		if len(schemaDiffs) == 0 {
			fmt.Println("OK: local schema matches node schema")
		} else {
			fmt.Printf("Schema drift: %d difference(s)\n", len(schemaDiffs))
			for _, d := range schemaDiffs {
				fmt.Printf("  %s\n", d)
			}
		}
	}

	var qstats queryStats
	if !*skipQueries {
		fmt.Printf("\n── Layer 2: live query check (%s) ──\n", *branch)
		qstats = runQueryLayer(*endpoint)
		fmt.Printf("\nResults: %d ok, %d drift, %d runtime, %d skipped, %d infra-failures\n",
			qstats.ok, len(qstats.drift), qstats.runtime, len(qstats.skipped), len(qstats.failures))
	}

	fmt.Printf("\n── Summary (%s) ──\n", *branch)
	if *skipSchema {
		fmt.Println("Schema diffs: SKIPPED")
	} else {
		fmt.Printf("Schema diffs:    %d\n", len(schemaDiffs))
	}
	if *skipQueries {
		fmt.Println("Query drift:     SKIPPED")
	} else {
		fmt.Printf("Query drift:     %d\n", len(qstats.drift))
		fmt.Printf("Skipped (cov):   %d\n", len(qstats.skipped))
		fmt.Printf("Infra failures:  %d\n", len(qstats.failures))
	}

	// Infra failures (HTTP / parse / network) always fail — we can't trust
	// the result if we couldn't talk to the daemon.
	if len(qstats.failures) > 0 {
		fmt.Println("FAIL: infrastructure errors prevented a clean check")
		os.Exit(1)
	}

	totalDrift := len(schemaDiffs) + len(qstats.drift)
	if totalDrift == 0 && len(qstats.skipped) == 0 {
		fmt.Println("OK: no drift detected")
		return
	}
	if *strict {
		// In strict mode, skipped ops are also a failure: we can't claim
		// the SDK is in sync if we couldn't probe parts of it.
		fmt.Println("FAIL: drift or coverage gap detected in --strict mode")
		os.Exit(1)
	}
	fmt.Printf("WARN: drift or coverage gap differs from %s (non-blocking).\n", *branch)
}
