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

package service

import "strings"

// translateSLACondition converts one sla_policy condition column's raw text,
// exactly as the external sync job that populates sla_policy/sla writes it
// (sys_ids, "u_"-prefixed field names, numeric state/priority codes, the
// "EQ" end-of-query terminator, "^NQ"/"^OR"/"^" grouping), into the neutral
// predicate text internal/slacondition parses -- field=value joined by "^"/
// "^OR"/"^NQ", using this engine's own CaseFacts vocabulary (state/priority
// labels, case_type/product/project_type names) rather than raw codes.
//
// This is the one place in this codebase that knows both vocabularies at
// once; internal/slacondition itself stays generic, with no idea any of this
// translation ever happened (see that package's own doc comment). Only this
// adapter -- not slacondition, not the resolver, not the published contract
// -- names the previous system or its field/value conventions.
//
// Translation runs at evaluation time, on every read, rather than being a
// one-time data migration: sla_policy is written on a recurring schedule by
// the external sync job, which maps its own start/stop/pause/resume/cancel
// condition text verbatim on every delta sync and full backfill -- a
// one-off rewrite of the stored text would be silently reverted the next
// time that job runs. Evaluating the raw text fresh, every time, is what
// survives that.
//
// idempotent: a value this function doesn't recognize (not a known sys_id,
// not a known numeric code) is left exactly as it was, so running this twice
// never over-translates; a row that happens to already carry
// human-readable text (e.g. hand-authored, or touched by one of the earlier,
// now purely cosmetic, one-time cleanup migrations) translates to the exact
// same neutral text unchanged, since every value this function would
// otherwise rewrite is matched by its *raw* (sys_id/numeric) form only.
func translateSLACondition(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	tokens := strings.Split(raw, "^")
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		prefix := ""
		rest := tok
		switch {
		case strings.HasPrefix(tok, "NQ"):
			prefix, rest = "NQ", tok[2:]
		case strings.HasPrefix(tok, "OR"):
			prefix, rest = "OR", tok[2:]
		}
		rest = strings.TrimSpace(rest)
		if rest == "" || rest == "EQ" {
			// The empty string is a stray separator; "EQ" is the external
			// sync job's own end-of-query terminator, confirmed to appear
			// exactly once, at the very end of every condition string it
			// writes -- a no-op token this grammar has no use for.
			continue
		}
		// translateSLAClauseToken never needs to omit a token entirely any
		// more -- a clause that becomes vacuously true after translation is
		// represented as the literal "TRUE" (internal/slacondition's own
		// OpTrue), never dropped, so this token's own "^OR"/"^NQ" prefix is
		// always preserved exactly as parsed. Omitting a token here instead
		// would be WRONG outside a plain AND run: inside an OR run it can
		// turn an always-true disjunct into a missing one that changes the
		// result, and as a whole "^NQ" group it loses an always-true
		// alternative entirely -- see OpTrue's own doc comment.
		out = append(out, prefix+translateSLAClauseToken(rest))
	}
	return strings.Join(out, "^")
}

// slaConditionFieldAlias names one raw field spelling this translator
// recognizes and the neutral field name internal/slacondition's own
// knownFields expects it to become. Matched longest-prefix-first (same
// convention as slacondition's own matchField), so "project.u_project_type"
// is never shadowed by a shorter, unrelated prefix. A field already spelled
// in its neutral form translates to itself -- see translateSLACondition's
// own doc comment on idempotency.
type slaConditionFieldAlias struct {
	raw     string
	neutral string
}

var slaConditionFieldAliases = []slaConditionFieldAlias{
	{"project.u_project_type", "project_type"},
	{"project_type", "project_type"},
	{"u_case_type", "case_type"},
	{"case_type", "case_type"},
	{"u_wso2_response_comment", "cs_engineer_response_comment"},
	{"cs_engineer_response_comment", "cs_engineer_response_comment"},
	{"u_fix_eta_shared", "fix_eta_shared"},
	{"fix_eta_shared", "fix_eta_shared"},
	{"u_workaround_provided", "u_workaround_provided"},
	{"sys_created_on", "sys_created_on"},
	{"opened_at", "opened_at"},
	{"priority", "priority"},
	{"state", "state"},
	{"product", "product"},
	{"account", "account"},
}

// matchSLAConditionField returns the longest raw alias matching the front of
// token, the neutral field name it maps to, and the unconsumed remainder
// (the operator+value suffix) -- or ok=false when nothing in
// slaConditionFieldAliases matches at all (a field outside this engine's own
// case-policy vocabulary, e.g. an incident/escalation-only field -- left
// completely unmatched here, exactly as it would be against slacondition's
// own knownFields, so it still fails closed as OpUnknown downstream).
func matchSLAConditionField(token string) (neutral, rest string, ok bool) {
	bestRaw := ""
	bestNeutral := ""
	for _, alias := range slaConditionFieldAliases {
		if strings.HasPrefix(token, alias.raw) && len(alias.raw) > len(bestRaw) {
			bestRaw = alias.raw
			bestNeutral = alias.neutral
		}
	}
	if bestRaw == "" {
		return "", token, false
	}
	return bestNeutral, token[len(bestRaw):], true
}

// translateSLAClauseToken translates one clause (already stripped of its
// "^OR"/"^NQ" prefix) from raw to neutral field/value spelling. Never omits
// the clause: a clause that becomes vacuously true after translation is
// returned as the literal "TRUE" (internal/slacondition's OpTrue) rather
// than dropped, so translateSLACondition's caller never has to reason about
// a missing token's own "^OR"/"^NQ" prefix (see that function's own doc
// comment on why dropping would be wrong outside a plain AND run).
func translateSLAClauseToken(token string) string {
	field, rest, ok := matchSLAConditionField(token)
	if !ok {
		// Not a field this engine's case-policy vocabulary knows at all --
		// left completely unmatched, same as it would be against
		// slacondition's own knownFields. Covers incident/escalation-only
		// fields (business_service, category, active_escalation.*,
		// escalation_severity, source_table) that the resolver has already
		// decided this row is not a case policy for (see isCasePolicy) --
		// never actually evaluated, but harmless to pass through unchanged
		// regardless.
		return token
	}

	switch {
	case strings.HasPrefix(rest, "ISNOTEMPTY"):
		return field + "ISNOTEMPTY"
	case strings.HasPrefix(rest, "ISEMPTY"):
		return field + "ISEMPTY"
	case strings.HasPrefix(rest, "!="):
		value := strings.TrimSpace(rest[len("!="):])
		if field == "case_type" {
			if swapped, ok := translateIncidentQueryCaseType(value, true); ok {
				return swapped
			}
		}
		v, drop := translateSLAConditionValue(field, value)
		if drop {
			// A negated comparison against a value with nothing left to
			// compare against is vacuously true (see
			// translateSLAConditionValue's own doc comment for the one
			// remaining drop case: a malformed, empty value).
			return "TRUE"
		}
		return field + "!=" + v
	case strings.HasPrefix(rest, ">="):
		return field + ">=" + translateSLADateValue(rest[len(">="):])
	case strings.HasPrefix(rest, "NOT IN"):
		v, drop := translateSLAConditionValueList(field, rest[len("NOT IN"):])
		if drop {
			// NOT IN an empty set is vacuously true, same reasoning as "!="
			// above.
			return "TRUE"
		}
		return field + "NOT IN" + v
	case strings.HasPrefix(rest, "IN"):
		// A positive IN clause left with no real members after translation
		// (every value was known-dead) is NOT vacuously true -- membership in
		// an empty set is always false. Falling back to the untranslated
		// token keeps the clause present as a comparison against a raw
		// sys_id/code that a decoded fact value will never equal, which
		// evaluates false exactly as the real condition does.
		v, drop := translateSLAConditionValueList(field, rest[len("IN"):])
		if drop {
			return token
		}
		return field + "IN" + v
	case strings.HasPrefix(rest, "="):
		value := strings.TrimSpace(rest[len("="):])
		if field == "case_type" {
			if swapped, ok := translateIncidentQueryCaseType(value, false); ok {
				return swapped
			}
		}
		// Same reasoning as IN above: a positive equality clause whose value
		// has no translation is false, not vacuously true.
		v, drop := translateSLAConditionValue(field, value)
		if drop {
			return token
		}
		return field + "=" + v
	default:
		return token
	}
}

// slaIncidentCaseTypeSysID/slaQueryCaseTypeSysID are the two case_type
// sys_ids that both map to Postgres work_item.type = 'CASE' -- the vendor's
// own "Incident" and "Query" case-type categories, respectively. Identified
// by cross-referencing the real, already-decoded rows in
// entity-service/migrations/0250_sla_policy_case_condition_enums.sql: every
// P0-P3 response/resolution definition combines
// slaIncidentCaseTypeSysID with an explicit severity (priority=S0..S3), and
// the one "none of the other known types" catch-all definition (id
// 85fe07fa-1b76-4510-d64e-64a2604bcb4b) excludes every other known
// case_type sys_id, slaIncidentCaseTypeSysID included, but never names
// slaQueryCaseTypeSysID at all -- by elimination, that's the one case type
// it exists to select.
const (
	slaIncidentCaseTypeSysID = "8d4b87bd1b18f010cb6898aebd4bcb59"
	slaQueryCaseTypeSysID    = "0d5b8fbd1b18f010cb6898aebd4bcba5"
)

// translateIncidentQueryCaseType handles the one case_type distinction
// Postgres cannot represent at all: Incident vs. Query. Both sys_ids decode
// to the identical work_item.type ('CASE'), so CaseFacts.CaseType can never
// tell them apart -- but severity can. Confirmed directly: Incident is case
// severity S0-S3, Query is S4, and case_severity_enum has exactly five
// values (migration 0023), so "not S4" and "S4" are complete,
// non-overlapping characterizations of the two. This swaps the clause's
// FIELD itself from case_type to priority, rather than translating the
// VALUE the way every other case_type sys_id does, because case_type alone
// has no value that could carry this distinction.
//
//	case_type=Incident-sysid  -> priorityINS0,S1,S2,S3
//	case_type!=Incident-sysid -> priorityNOT INS0,S1,S2,S3
//	case_type=Query-sysid     -> priority=S4
//	case_type!=Query-sysid    -> priority!=S4
//
// Incident's own pair uses membership (IN/NOT IN), not "!=S4"/"=S4", even
// though the two are equivalent for a case that actually HAS a severity:
// internal/slacondition's OpNotEqual treats a missing field as "not equal to
// anything" (true) -- correct for a genuinely absent value, but wrong here,
// since a non-case-type work item (engagement, service_request, ...) always
// has an empty severity and would then satisfy a positive "priority!=S4"
// Incident condition it has no business matching, registering a bogus clock
// after e.g. a case-to-engagement type transfer (ReviseCaseClocks). OpIn/
// OpNotIn both treat a missing field as "not a member"/"not excluded"
// respectively -- false and true -- which is what every caller here actually
// needs. Query's own pair keeps "=S4"/"!=S4": OpEqual already requires
// presence (false when absent, correctly never matching a severity-less
// item), and Query's negated form needing "true on absent" is itself
// correct (a severity-less item is certainly not a Query case).
//
// negated is whether the original clause was "!=" (true) or "=" (false).
// ok=false for any other value, so the caller falls back to the generic
// case_type value translation (translateSLAConditionValue).
//
// Not applied inside an IN/NOT IN value list: confirmed absent from every
// one of the 26 real, active case-policy rows (case_type there is only ever
// a chain of "="/"!=" clauses -- see 0250_sla_policy_case_condition_enums.sql),
// so decomposing a list member into a field-swapped predicate is left
// unimplemented rather than guessed at.
func translateIncidentQueryCaseType(value string, negated bool) (translated string, ok bool) {
	switch value {
	case slaIncidentCaseTypeSysID:
		if negated {
			return "priorityNOT INS0,S1,S2,S3", true
		}
		return "priorityINS0,S1,S2,S3", true
	case slaQueryCaseTypeSysID:
		if negated {
			return "priority!=S4", true
		}
		return "priority=S4", true
	default:
		return "", false
	}
}

// translateSLAConditionValue translates one clause's value according to
// field's own vocabulary. drop=true only for a malformed, empty value -- a
// confirmed, pre-existing artifact of the raw external data (migration
// 0258's own history), not something a real policy edit would ever produce
// going forward.
func translateSLAConditionValue(field, value string) (translated string, drop bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	switch field {
	case "state":
		if label, ok := slaCaseStateCodes[value]; ok {
			return label, false
		}
	case "priority":
		if label, ok := slaCaseSeverityCodes[value]; ok {
			return label, false
		}
	case "case_type":
		if name, ok := slaCaseTypeSysIDs[value]; ok {
			return name, false
		}
	case "product":
		if name, ok := slaProductSysIDs[value]; ok {
			return name, false
		}
	case "project_type":
		if name, ok := slaProjectTypeSysIDs[value]; ok {
			return name, false
		}
	}
	// No translation known for this value -- left exactly as it was (a
	// value already in its neutral spelling translates to itself; a value
	// in neither vocabulary is passed through unchanged and will simply
	// fail to match any real case, the same fail-closed outcome an
	// unrecognized field already has). Covers Hosting/Hosting Query/Hosting
	// Task (deliberately not in slaCaseTypeSysIDs at all -- see that map's
	// own doc comment): a positive clause against one of their raw sys_ids
	// can never equal a real, decoded CaseType value (always false), and a
	// negated one is vacuously true -- exactly "this policy branch is
	// permanently inert," which is the intended, product-decided outcome.
	return value, false
}

// translateSLAConditionValueList is translateSLAConditionValue applied to
// every comma-separated member of an IN/NOT IN clause's value list. A
// member this drops (a blank entry) is simply omitted from the translated
// list rather than dropping the whole clause -- a membership list with the
// other, real values intact is still meaningful, unlike a single-value
// equality clause with nothing left at all.
func translateSLAConditionValueList(field, raw string) (translated string, drop bool) {
	parts := strings.Split(raw, ",")
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		v, omit := translateSLAConditionValue(field, p)
		if omit {
			continue
		}
		kept = append(kept, v)
	}
	if len(kept) == 0 {
		return "", true
	}
	return strings.Join(kept, ","), false
}

// translateSLADateValue strips the external sync job's own
// "javascript:gs.dateGenerate('YYYY-MM-DD','HH:MM:SS')" literal-date wrapper
// down to the plain YYYY-MM-DD date internal/slacondition's OpGTE clause
// parses -- both real occurrences in the live data use a midnight time, so
// nothing is lost dropping the wrapper. A value with no such wrapper (already
// a plain date) passes through unchanged.
func translateSLADateValue(value string) string {
	value = strings.TrimSpace(value)
	const prefix = "javascript:gs.dateGenerate("
	if !strings.HasPrefix(value, prefix) {
		return value
	}
	inner := strings.TrimSuffix(value[len(prefix):], ")")
	first := strings.SplitN(inner, ",", 2)[0]
	return strings.Trim(strings.TrimSpace(first), "'\"")
}

// slaCaseStateCodes maps the external sync job's own numeric case-state
// choice-list values to this database's case_state_enum labels -- the
// mapping is specific to a policy that targets the case work item type; the
// same numeric codes mean something else entirely on an
// incident/account-escalation policy's conditions, which this engine never
// evaluates (see isCasePolicy).
var slaCaseStateCodes = map[string]string{
	"1":    "OPEN",
	"3":    "CLOSED",
	"6":    "SOLUTION_PROPOSED",
	"10":   "WORK_IN_PROGRESS",
	"18":   "AWAITING_INFO",
	"1003": "WAITING_ON_WSO2",
	"1006": "REOPENED",
}

// slaCaseSeverityCodes maps the external sync job's own numeric priority
// choice-list values to this database's case_severity_enum labels, for a
// case-targeting policy specifically (see slaCaseStateCodes' own doc
// comment on why these numeric vocabularies don't carry across entity
// types).
var slaCaseSeverityCodes = map[string]string{
	"14": "S0",
	"10": "S1",
	"11": "S2",
	"12": "S3",
	"13": "S4",
}

// slaCaseTypeSysIDs maps the external sync job's own case_type sys_ids to
// this engine's own CaseFacts.CaseType vocabulary. Hand-maintained, like
// every other sys_id-keyed lookup this codebase keeps as a Go map rather
// than a database table (see internal/service/sn_case_service.go's own
// snCaseTypeSysidMap, the source this was built from) -- a case type added
// upstream with no entry here simply never matches any real case (fails
// closed), the same limitation that map already has.
//
// Deliberately NOT here: the Incident/Query sys_ids
// (slaIncidentCaseTypeSysID/slaQueryCaseTypeSysID) -- both decode to the
// identical "case" work_item.type and so cannot be told apart through this
// map at all; they're handled separately by translateIncidentQueryCaseType,
// which swaps the clause's field to priority instead. Also deliberately NOT
// here, by explicit product decision: Hosting/Hosting Query/Hosting Task
// (won't be maintained going forward), and slaAdminTaskCaseTypeSysID (see
// its own doc comment) -- all four fall through to
// translateSLAConditionValue's generic "no translation known" path, which
// passes them through as raw, untranslated text. A real CaseType value can
// never equal a raw sys_id, so every policy gated on one of these types is
// permanently inert (a positive clause always false, a negated one
// vacuously true) -- the intended outcome for each, achieved with no
// special-case code.
var slaCaseTypeSysIDs = map[string]string{
	"5aeff1201b74c210264c997a234bcb54": "service_request",
	"ab36479047ccf510a0a29cd3846d43ee": "security_report_analysis",
	"3b8b43311b58f010cb6898aebd4bcb8f": "announcement",
	"8f8fc2c41b0bd550d64e64a2604bcb38": "engagement",
}

// slaAdminTaskCaseTypeSysID is the "Admin Task" case type upstream --
// confirmed present in the real case-type picklist, just never covered by
// snCaseTypeSysidMap (sn_case_service.go) from the very start, and
// deliberately left unmapped here too: confirmed directly with the team
// that Admin Task cases are no longer created at all, so there is nothing
// left to map it to. Referenced only for this comment's own documentation
// value -- no code branches on it; it falls through
// translateSLAConditionValue's generic "no translation known" path exactly
// like an sys_id this map has simply never heard of.
const slaAdminTaskCaseTypeSysID = "3f5b47bd1b18f010cb6898aebd4bcbc2"

// slaProductSysIDs maps every product sys_id this database's own "product"
// table carries (product.id uses the same dashed-sys_id-as-primary-key
// convention as project_type.id below) to its name. Mirrored here, rather
// than resolved with a database read at evaluation time, because this
// translator runs per clause, per policy, per case-lifecycle event -- a
// per-call database round trip for a handful of stable reference rows (this
// table changes about as often as a new product line ships) is not worth
// the latency. A sys_id added to the real table with no entry here fails
// closed (never matches), the same accepted limitation as slaCaseTypeSysIDs.
var slaProductSysIDs = map[string]string{
	"720bf91d1bf92010cb6898aebd4bcb10": "Asgardeo",
	"7bf5811d1b8ae210a002c9d3604bcbc8": "Bijira",
	"9c2f321a1b700350a002c9d3604bcb2d": "Devant",
	"6436091d1b8ae210a002c9d3604bcb45": "Devant",
	"c81ffe3a1bb6f610a002c9d3604bcb5c": "WSO2 API Manager",
	"487a6454c3e33210af2f404599013138": "WSO2 API Manager",
	"f699281c1b2b7654182c0dc5604bcb36": "WSO2 API Manager Analytics",
	"8399e05c1b2b7654182c0dc5604bcb23": "WSO2 API Platform for Kubernetes",
	"691b4f861b746010cb6898aebd4bcbc0": "WSO2 Developer Platform",
	"7e99281c1b2b7654182c0dc5604bcb38": "WSO2 Enterprise Integrator",
	"4c8f3efa1bb6f610a002c9d3604bcbe7": "WSO2 Identity Server",
	"4799e05c1b2b7654182c0dc5604bcb24": "WSO2 Identity Server Analytics",
	"4b99281c1b2b7654182c0dc5604bcb3d": "WSO2 Integrator: BI",
	"c399281c1b2b7654182c0dc5604bcb3b": "WSO2 Integrator: MI",
	"0f99281c1b2b7654182c0dc5604bcb3e": "WSO2 MI Monitoring Dashboard",
	"487a6454c3e33210af2f404599013137": "WSO2 MI Monitoring Dashboard",
	"0b99e05c1b2b7654182c0dc5604bcb25": "WSO2 Open Banking AM",
	"487a6454c3e33210af2f404599013136": "WSO2 Open Banking AM",
	"3299281c1b2b7654182c0dc5604bcb3a": "WSO2 Open Banking BI",
	"487a6454c3e33210af2f404599013129": "WSO2 Open Banking BI",
	"487a6454c3e33210af2f404599013127": "WSO2 Open Banking IAM",
	"ba99281c1b2b7654182c0dc5604bcb37": "WSO2 Open Banking IAM",
}

// slaProjectTypeSysIDs maps every project_type sys_id this database's own
// "project_type" table carries to its name -- see slaProductSysIDs' own doc
// comment for why this is a hand-maintained map rather than a live join.
var slaProjectTypeSysIDs = map[string]string{
	"2406c2724709d190a0a29cd3846d434e": "Platformer Subscription",
	"368a07bd1b18f010cb6898aebd4bcb27": "Development Support",
	"50b8332a1bfdf0106a67caa1604bcb8e": "Professional Services",
	"87a8676c1b347410cb6898aebd4bcb76": "Managed Cloud Subscription",
	"b567f5f21bec1d109871eac5624bcb62": "Cloud Support - Platformer",
	"bf1d3c401bac7610a002c9d3604bcb69": "Regular",
	"cb0b88af1bf4f81000ae86acdd4bcbba": "Evaluation Subscription",
	"cb4a87bd1b18f010cb6898aebd4bcb4f": "Subscription",
	"ccd8dc14477a0910a0a29cd3846d434c": "Cloud Evaluation Support",
	"e411f0a61b0205109871eac5624bcb4f": "Cloud Support",
	"f50ba53d1b4cdd10d64e64a2604bcb1f": "Internal",
}
