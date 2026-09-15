// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package snquery translates a ServiceNow encoded query into a spec.Condition.
//
// ┌─────────────────────────────────────────────────────────────────────────┐
// │ THIS PACKAGE HAS A PLANNED END OF LIFE. DELETE IT AFTER CUTOVER.        │
// └─────────────────────────────────────────────────────────────────────────┘
//
// It lives under internal/porting/ rather than beside spec and eval because it
// is NOT part of the running service. Nothing in the engine calls it. At
// runtime the consumer reads an already-translated condition out of
// flow_versions; this code has been gone from that path since the moment the
// flow was published.
//
// # WHY IT EXISTS AT ALL, GIVEN SERVICENOW IS BEING DECOMMISSIONED
//
// It is a bulk-migration tool. Roughly 55 flows need their conditions moved,
// and the translation is mechanical but easy to get wrong by hand. Consider
// one real trigger:
//
//	priority=10^u_case_type=8d4b87bd^u_wso2_product!=252a8a6e^ORu_wso2_product!=da90e058
//
// ^OR binds to the IMMEDIATELY PRECEDING term, so this is
// A AND B AND (C OR D) — not (A AND B AND C) OR D. A human transcribing 55 of
// these will get some wrong, and a wrong condition does not fail loudly: it
// fires on the wrong records, quietly, forever.
//
// The second reason is a coverage signal. "31 of 31 real conditions translate
// and validate" is evidence the port is mechanical. Had it been 20 of 31, the
// other 11 would be a known list of flows needing hand work rather than a
// surprise discovered mid-migration.
//
// DELETION CRITERION: when every flow in docs/port-backlog.md is ported and
// ServiceNow is off, delete this package and its tests. Nothing else imports
// it. If it is still here after cutover, that is a leftover, not a dependency.
//
// It is deliberately conservative: anything it cannot translate faithfully
// becomes an error rather than a guess.
//
// Every test case in parse_test.go is a REAL condition taken from the WSO2
// instance — see docs/servicenow-discovery/RESULTS.md §26.
//
// ── ServiceNow encoded-query grammar, as far as we need it ────────────────
//
//	term       field + operator + value, e.g. priority=10
//	^          AND between terms
//	^OR        OR with the IMMEDIATELY PRECEDING term, not the whole query:
//	             a^b^ORc  ==  a AND (b OR c)
//	^NQ        "new query" — OR across WHOLE groups:
//	             a^b^NQc^d  ==  (a AND b) OR (c AND d)
//	^EQ        end-of-query marker, carries no meaning
//
// There are no parentheses in the syntax, which is also why building these
// queries by string concatenation is unsafe — a lesson learned the hard way
// while writing the discovery scripts.
package snquery

import (
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/spec"
)

// opMapping maps a ServiceNow operator token to ours.
//
// ORDER MATTERS: the list is scanned in sequence and longest-first, so
// CHANGESTO is tried before CHANGES, and "NOT LIKE" before "LIKE". Getting
// this order wrong silently mis-parses — `stateCHANGESTO3` would become field
// "state" op CHANGES value "TO3".
var opMapping = []struct {
	token string
	op    spec.Operator
	unary bool
}{
	{"ISNOTEMPTY", spec.OpExists, true},
	{"ISEMPTY", spec.OpNotExists, true},
	{"ANYTHING", spec.OpExists, true},

	{"CHANGESFROM", spec.OpChangedFrom, false},
	{"CHANGESTO", spec.OpChangedTo, false},
	{"VALCHANGES", spec.OpChanged, true},
	{"CHANGES", spec.OpChanged, true},

	{"STARTSWITH", spec.OpStartsWith, false},
	{"ENDSWITH", spec.OpEndsWith, false},
	{"NOT LIKE", spec.OpNotContains, false},
	{"LIKE", spec.OpContains, false},

	{"NOT IN", spec.OpNotIn, false},
	{"IN", spec.OpIn, false},

	{"!=", spec.OpNeq, false},
	{">=", spec.OpGte, false},
	{"<=", spec.OpLte, false},
	{"=", spec.OpEq, false},
	{">", spec.OpGt, false},
	{"<", spec.OpLt, false},
}

// Unsupported reports a construct the translator will not guess at.
type Unsupported struct {
	Term   string
	Reason string
}

func (e *Unsupported) Error() string {
	return fmt.Sprintf("snquery: cannot translate %q: %s", e.Term, e.Reason)
}

// Parse converts an encoded query into a Condition.
//
// An empty query (or a bare "^EQ") returns nil, meaning "no condition" — a
// flow that fires on every matching record. Several real flows are configured
// exactly that way.
func Parse(query string) (*spec.Condition, error) {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, "^EQ")
	q = strings.TrimSpace(q)
	if q == "" || q == "^EQ" {
		return nil, nil
	}

	// ^NQ splits whole groups, OR'd together at the top level.
	groups := strings.Split(q, "^NQ")
	var anyOf []spec.Condition
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		c, err := parseGroup(g)
		if err != nil {
			return nil, err
		}
		if c != nil {
			anyOf = append(anyOf, *c)
		}
	}

	switch len(anyOf) {
	case 0:
		return nil, nil
	case 1:
		return &anyOf[0], nil
	default:
		return &spec.Condition{Any: anyOf}, nil
	}
}

// parseGroup handles one ^NQ group: terms joined by ^, where ^OR ORs with the
// immediately preceding term.
func parseGroup(group string) (*spec.Condition, error) {
	// Split on ^ but keep the OR marker attached to its term.
	raw := strings.Split(group, "^")

	// andParts accumulates the AND chain. Each element may itself be an Any
	// group, built up as ^OR terms attach to the previous one.
	var andParts []spec.Condition

	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" || part == "EQ" {
			continue
		}

		isOr := strings.HasPrefix(part, "OR")
		termStr := part
		if isOr {
			termStr = strings.TrimPrefix(part, "OR")
		}

		term, err := parseTerm(termStr)
		if err != nil {
			return nil, err
		}

		if isOr {
			if len(andParts) == 0 {
				return nil, &Unsupported{
					Term:   part,
					Reason: "^OR with no preceding term to attach to",
				}
			}
			// Fold into the previous element, turning it into an Any group.
			prev := &andParts[len(andParts)-1]
			if len(prev.Any) > 0 {
				prev.Any = append(prev.Any, *term)
			} else {
				*prev = spec.Condition{Any: []spec.Condition{*prev, *term}}
			}
			continue
		}

		andParts = append(andParts, *term)
	}

	switch len(andParts) {
	case 0:
		return nil, nil
	case 1:
		return &andParts[0], nil
	default:
		return &spec.Condition{All: andParts}, nil
	}
}

// parseTerm splits one "fieldOPvalue" term.
func parseTerm(term string) (*spec.Condition, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, &Unsupported{Term: term, Reason: "empty term"}
	}

	// DYNAMIC is a ServiceNow reference qualifier resolved against a stored
	// query — e.g. caller_idDYNAMIC90d1921e5f510100a9ad2572f2b477fe. It has no
	// standalone meaning outside ServiceNow, so refuse rather than guess.
	if i := strings.Index(term, "DYNAMIC"); i > 0 {
		return nil, &Unsupported{
			Term:   term,
			Reason: "DYNAMIC references a stored ServiceNow query and has no equivalent — resolve it by hand",
		}
	}
	// NSAMEAS/SAMEAS compare two FIELDS, not a field and a value. Our
	// condition leaf has no field-to-field form.
	if strings.Contains(term, "NSAMEAS") || strings.Contains(term, "SAMEAS") {
		return nil, &Unsupported{
			Term:   term,
			Reason: "field-to-field comparison (SAMEAS) is not expressible as a condition leaf",
		}
	}

	for _, m := range opMapping {
		i := strings.Index(term, m.token)
		if i <= 0 {
			// i == 0 would mean an empty field name; i < 0 means no match.
			continue
		}
		field := strings.TrimSpace(term[:i])
		value := strings.TrimSpace(term[i+len(m.token):])

		if field == "" {
			return nil, &Unsupported{Term: term, Reason: "empty field name"}
		}

		// A bare CHANGESFROM/CHANGESTO with no value means "changed at all"
		// in practice — `assigned_toCHANGESFROM` appears exactly this way in
		// the Twilio P1 assign flow. Downgrade to `changed`, which is the
		// honest reading, rather than inventing an empty-string comparison.
		if value == "" && (m.op == spec.OpChangedFrom || m.op == spec.OpChangedTo) {
			return &spec.Condition{Field: field, Op: spec.OpChanged}, nil
		}

		if m.unary {
			if value != "" {
				return nil, &Unsupported{
					Term:   term,
					Reason: fmt.Sprintf("%s takes no value but got %q", m.token, value),
				}
			}
			return &spec.Condition{Field: field, Op: m.op}, nil
		}

		if value == "" {
			return nil, &Unsupported{
				Term:   term,
				Reason: fmt.Sprintf("%s requires a value", m.token),
			}
		}
		return &spec.Condition{Field: field, Op: m.op, Value: value}, nil
	}

	return nil, &Unsupported{Term: term, Reason: "no recognised operator"}
}
