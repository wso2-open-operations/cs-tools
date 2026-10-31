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

// Package slacondition parses and evaluates a small, vendor-neutral
// predicate language over a case's current field values -- the format
// internal/service's adapter layer translates sla_policy's
// start_condition/stop_condition/pause_condition/resume_condition/
// cancel_condition columns into before handing them to this package (see
// service.translateSLACondition). This package itself knows nothing about
// where that text originally came from or how it got translated; it only
// parses and evaluates the grammar below, against the fixed vocabulary
// CaseFacts/Facts can answer questions about.
//
// Grammar:
//
//   - Clauses are joined by "^" (AND).
//   - "^OR" ORs a clause with the clause immediately before it, in the same
//     AND-group -- a run of "^OR"-joined clauses forms one OR'd term that is
//     then AND'd with the rest of the group, e.g.
//     "product=A^ORproduct=B^state=OPEN" is (A OR B) AND state=OPEN.
//   - "^NQ" ("new query") starts a brand new AND-group that is OR'd against
//     every group before it, e.g.
//     "state=OPEN^priority=S1^NQstate=OPEN^case_type=hosting" is
//     (state=OPEN AND priority=S1) OR (state=OPEN AND case_type=hosting).
//
// So the overall shape is: OR of AND-groups (split on "^NQ"), each AND-group
// itself a list of terms ANDed together, each term either a single clause or
// an OR'd run of clauses (split on "^OR" within the group). "^OR"/"^NQ" are
// glued directly onto the next field name with no separator.
//
// A single clause is one of:
//
//   - field=value / field!=value           -- equality/inequality
//   - fieldINv1,v2,v3                      -- membership
//   - fieldNOT INv1,v2                     -- negated membership (note the
//     literal space in "NOT IN" -- parsed defensively here even though no
//     case policy this engine evaluates currently uses it, so a malformed or
//     future input never panics).
//   - fieldISNOTEMPTY / fieldISEMPTY       -- presence check, no value, no
//     separator before the suffix at all -- checked before ever trying to
//     split the clause on "=".
//   - field>=value                         -- a date threshold, e.g.
//     opened_at>=YYYY-MM-DD.
//   - the bare literal TRUE                -- a tautology clause (OpTrue),
//     always true; a translating adapter's own stand-in for a clause whose
//     original meaning became unconditionally true, without having to omit
//     the token (and lose its "^OR"/"^NQ" grouping) entirely.
//
// Parsing never errors and never panics: a clause whose field isn't in this
// package's own known vocabulary (knownFields below -- the same vocabulary
// the SLA engine's own CaseFacts establishes) is kept as an "unknown" clause
// that always evaluates false. That is deliberate -- an unrecognized or
// untranslated field must never be silently treated as a match. It must also
// not be silently invisible: callers that evaluate a translated condition
// should call Condition.UnknownClauses first and log a warning for any that
// come back, since a clause that fails closed with no visible trace is
// exactly how a translation gap (an un-mapped raw value, a new field the
// adapter doesn't yet translate) goes unnoticed in production -- see
// slaPolicyResolver.resolve's own call to it.
package slacondition

import (
	"strings"
	"time"
)

// Op identifies a single clause's comparison.
type Op int

const (
	// OpUnknown marks a clause this package could not confidently parse --
	// an unrecognized field, or a field whose suffix didn't match any known
	// operator shape. Always evaluates to false; see this package's own doc
	// comment for why that's the safe default rather than an error.
	OpUnknown Op = iota
	OpEqual
	OpNotEqual
	OpIn
	OpNotIn
	OpIsEmpty
	OpIsNotEmpty
	// OpGTE is ">=", used only for the two date fields (opened_at/
	// sys_created_on) in this vocabulary.
	OpGTE
	// OpTrue is a tautology clause -- always evaluates true, carries no
	// field or value. An adapter translating some other grammar into this
	// one uses it to stand in for a clause that is always true after
	// translation (e.g. a negated comparison against a value with no
	// counterpart in this vocabulary, which can never be unequal to
	// anything real) WITHOUT omitting the token entirely: inside an
	// AndGroup, a missing token and an explicit OpTrue both act as a no-op,
	// but inside an OrTerm or as a whole AndGroup under "^NQ", omitting the
	// token changes the result (a disjunction loses a disjunct that should
	// always have carried it; a whole always-true alternative group
	// disappears) while OpTrue preserves it correctly. See Parse's own
	// recognition of the literal "TRUE" token.
	OpTrue
)

// Clause is one field/operator/value(s) term.
type Clause struct {
	Field  string
	Op     Op
	Value  string   // OpEqual/OpNotEqual/OpGTE
	Values []string // OpIn/OpNotIn
	// Raw is the original, unparsed clause text -- kept for logging/
	// diagnostics when Op is OpUnknown, since Field/Value carry no useful
	// information in that case.
	Raw string
}

// OrTerm is one or more clauses OR'd together (a "^OR"-joined run, or a
// single clause with no run at all).
type OrTerm []Clause

// AndGroup is one or more OrTerms AND'd together -- everything between two
// "^NQ" boundaries.
type AndGroup []OrTerm

// Condition is one or more AndGroups OR'd together ("^NQ"-separated). A nil/
// empty Condition (an absent or blank sla_policy condition column) never
// matches anything -- callers that need "no condition configured" to mean
// something else (e.g. "this transition is simply not used by this policy")
// should check the raw string themselves before calling Parse, the same way
// every other Postgres-only-gap in this codebase checks for nil before
// delegating.
type Condition []AndGroup

// knownFields is the vocabulary this engine's CaseFacts can answer
// questions about -- see the package doc comment. Longest-prefix matching
// (matchField below) means order here doesn't matter; kept alphabetical for
// readability.
var knownFields = []string{
	"account",
	"case_type",
	"cs_engineer_response_comment",
	"fix_eta_shared",
	"opened_at",
	"priority",
	"product",
	"project_type",
	"state",
	"sys_created_on",
	"u_workaround_provided",
}

// Parse parses raw sla_policy condition text into a Condition. Never
// returns an error -- see the package doc comment on why malformed/
// unrecognized input degrades to an always-false clause instead.
func Parse(raw string) Condition {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	groups := strings.Split(raw, "^NQ")
	cond := make(Condition, 0, len(groups))
	for _, g := range groups {
		cond = append(cond, parseGroup(g))
	}
	return cond
}

func parseGroup(group string) AndGroup {
	tokens := strings.Split(group, "^")
	var andGroup AndGroup
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.HasPrefix(tok, "OR") {
			clause := parseClauseToken(tok[len("OR"):])
			if len(andGroup) == 0 {
				// Malformed (a group can't start with "^OR"), but handled
				// defensively rather than dropped or panicked on.
				andGroup = append(andGroup, OrTerm{clause})
			} else {
				andGroup[len(andGroup)-1] = append(andGroup[len(andGroup)-1], clause)
			}
			continue
		}
		andGroup = append(andGroup, OrTerm{parseClauseToken(tok)})
	}
	return andGroup
}

func parseClauseToken(token string) Clause {
	token = strings.TrimSpace(token)
	if token == "" {
		return Clause{Op: OpUnknown}
	}
	if token == "TRUE" {
		// Reserved literal -- see OpTrue's own doc comment. "TRUE" can never
		// collide with a real field+operator+value clause: every real field
		// name in knownFields is lowercase, and matchField below would never
		// match this all-caps token as a field prefix anyway, so checking it
		// first (rather than falling through to OpUnknown) costs nothing.
		return Clause{Op: OpTrue, Raw: token}
	}
	field, rest, ok := matchField(token)
	if !ok {
		return Clause{Op: OpUnknown, Raw: token}
	}
	switch {
	case rest == "ISNOTEMPTY":
		return Clause{Field: field, Op: OpIsNotEmpty, Raw: token}
	case rest == "ISEMPTY":
		return Clause{Field: field, Op: OpIsEmpty, Raw: token}
	case strings.HasPrefix(rest, "!="):
		return Clause{Field: field, Op: OpNotEqual, Value: rest[len("!="):], Raw: token}
	case strings.HasPrefix(rest, ">="):
		return Clause{Field: field, Op: OpGTE, Value: rest[len(">="):], Raw: token}
	case strings.HasPrefix(rest, "NOT IN"):
		return Clause{Field: field, Op: OpNotIn, Values: splitValues(rest[len("NOT IN"):]), Raw: token}
	case strings.HasPrefix(rest, "IN"):
		return Clause{Field: field, Op: OpIn, Values: splitValues(rest[len("IN"):]), Raw: token}
	case strings.HasPrefix(rest, "="):
		return Clause{Field: field, Op: OpEqual, Value: rest[len("="):], Raw: token}
	default:
		return Clause{Field: field, Op: OpUnknown, Raw: token}
	}
}

// matchField returns the longest entry of knownFields that is a prefix of
// token, so that e.g. "state" never shadows "sys_created_on" and vice
// versa, regardless of which order knownFields happens to list them in.
func matchField(token string) (field, rest string, ok bool) {
	best := ""
	for _, f := range knownFields {
		if strings.HasPrefix(token, f) && len(f) > len(best) {
			best = f
		}
	}
	if best == "" {
		return "", token, false
	}
	return best, token[len(best):], true
}

func splitValues(s string) []string {
	parts := strings.Split(s, ",")
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		values = append(values, strings.TrimSpace(p))
	}
	return values
}

// Facts answers the questions a Clause needs to evaluate itself -- one
// adapter per case-fact source. service.caseConditionFacts is the only
// implementation today, over service.CaseFacts.
type Facts interface {
	// Field returns the field's current string value (a state label like
	// "OPEN", a severity label like "S1", a case type like "case", a
	// resolved project-type/product display name, or "true"/"false" for a
	// boolean field) and whether that field is known/applicable at all --
	// false for e.g. a nil severity on a case with no severity set.
	Field(name string) (value string, ok bool)
	// Present reports whether a presence-only field (account,
	// u_workaround_provided, fix_eta_shared, case_type) currently has a
	// value.
	Present(name string) bool
	// Date returns the instant backing a date field (opened_at/
	// sys_created_on both mean the case's own creation date).
	Date(name string) (time.Time, bool)
}

// Evaluate reports whether this Condition matches f -- true when any
// AndGroup matches (the "^NQ" disjunction), false (not an error) for a nil/
// empty Condition.
func (c Condition) Evaluate(f Facts) bool {
	for _, group := range c {
		if group.evaluate(f) {
			return true
		}
	}
	return false
}

// UnknownClauses returns the raw text of every clause in c that parsed as
// OpUnknown -- an unrecognized field, or a suffix matching no known operator
// shape. Every such clause always evaluates false (see this package's own
// doc comment), so a condition built from un-translated or mismapped input
// fails closed with no visible symptom other than "never matches" -- callers
// should log the result of this call as a warning whenever it's non-empty,
// so that failure mode is never silent in production.
func (c Condition) UnknownClauses() []string {
	var unknown []string
	for _, group := range c {
		for _, term := range group {
			for _, cl := range term {
				if cl.Op == OpUnknown && cl.Raw != "" {
					unknown = append(unknown, cl.Raw)
				}
			}
		}
	}
	return unknown
}

func (g AndGroup) evaluate(f Facts) bool {
	if len(g) == 0 {
		return false
	}
	for _, term := range g {
		if !term.evaluate(f) {
			return false
		}
	}
	return true
}

func (t OrTerm) evaluate(f Facts) bool {
	for _, cl := range t {
		if cl.evaluate(f) {
			return true
		}
	}
	return false
}

func (c Clause) evaluate(f Facts) bool {
	switch c.Op {
	case OpEqual:
		v, ok := f.Field(c.Field)
		return ok && strings.EqualFold(v, c.Value)
	case OpNotEqual:
		v, ok := f.Field(c.Field)
		if !ok {
			// The field has no known value at all (e.g. a nil severity) --
			// treated as "not equal to anything", the same way a SQL NULL
			// would behave under this engine's own CASE-based field
			// resolution elsewhere in this codebase (caseLikeStateColumn
			// etc. all COALESCE to a usable value or leave it absent, never
			// treated as a guaranteed mismatch).
			return true
		}
		return !strings.EqualFold(v, c.Value)
	case OpIn:
		v, ok := f.Field(c.Field)
		if !ok {
			return false
		}
		for _, val := range c.Values {
			if strings.EqualFold(v, val) {
				return true
			}
		}
		return false
	case OpNotIn:
		v, ok := f.Field(c.Field)
		if !ok {
			return true
		}
		for _, val := range c.Values {
			if strings.EqualFold(v, val) {
				return false
			}
		}
		return true
	case OpIsEmpty:
		return !f.Present(c.Field)
	case OpIsNotEmpty:
		return f.Present(c.Field)
	case OpGTE:
		d, ok := f.Date(c.Field)
		if !ok {
			return false
		}
		threshold, err := time.Parse("2006-01-02", strings.TrimSpace(c.Value))
		if err != nil {
			return false
		}
		return !d.Before(threshold)
	case OpTrue:
		return true
	default: // OpUnknown
		return false
	}
}
