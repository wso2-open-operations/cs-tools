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

package worklogreport

import "strings"

// Who a CS Worklogs report is sent to.
//
// *** THE CONFIGURED LISTS ARE AUTHORITATIVE, NOT THE DERIVED ONES. ***
//
// That is the opposite of what it looks like it should be, and the
// delivered mails are why. Their headers are not derivable from any team
// structure:
//
//   - The To/Cc roles are INVERTED between the two reports. The weekly
//     puts five people in To and two group lists in Cc. The daily puts
//     six people AND four group lists in To, and twenty-four people in
//     Cc. Groups are a Cc concept in one report and a To concept in the
//     other.
//   - mara@ and said@ appear in BOTH the To and the Cc of the
//     same daily mail. No rule derives a duplicate; a human maintaining
//     a list does.
//   - quinn@ and pia@ are To on the weekly and Cc on the daily --
//     the same people in a different field for the same week.
//   - rosa@, tomas@ and said@ are on the daily only, and appear
//     nowhere in either report's own contents.
//
// So these are two hand-maintained distribution lists. Reproducing them
// exactly -- which is the requirement -- means configuring them verbatim.
// Deriving an audience and hoping it matches would mail the wrong ~34
// people.
//
// The derived audience is still computed and still carried on the report,
// but only as a comparison: Drift reports how the two differ, so the day
// the configured list goes stale against the team registry is visible in
// a log line instead of in somebody's inbox.
type Audience struct {
	To []string
	Cc []string
}

// Source selects where the audience comes from.
type Source string

const (
	// SourceConfigured uses the lists in SUB_CRON_RECIPIENTS. The
	// default, and what reproduces the delivered mails exactly.
	SourceConfigured Source = "configured"
	// SourceDerived uses the audience entity-service builds from the
	// team registry. Switching to it is a deliberate decision that the
	// registry is now the truth and the hand-maintained lists are
	// stale -- it WILL change who receives the report. Drift() shows
	// the size of that change before anyone commits to it.
	SourceDerived Source = "derived"
)

// Resolve decides the real audience. Precedence, highest first:
//
//  1. override -- replaces everything, To and Cc both, so a test run
//     cannot reach the real lists.
//  2. the chosen source.
//
// Neither source falls back to the other FOR THE To LINE. An earlier
// version fell through from configured to derived when nothing was
// configured, which turned a blank entry -- {"to": [""]} -- into a mail
// to the registry-derived recipients: precisely what configuring the
// lists exists to prevent. Whichever source is selected and comes back
// empty, nothing is sent, which is the safe failure.
//
// Cc IS THE EXCEPTION, and only on the derived source: a configured cc
// is ADDED to the derived one rather than discarded. The registry
// derives a team's own group addresses and has no way to express "also
// copy this standing list", because carrying such an address would mean
// inventing a team row for a group that is not a team -- which
// misrepresents the org chart and, since a leadership-typed team also
// contributes its members to To, quietly promotes anyone later added to
// it. Precedent: query_hours_weekly_report already treats its
// configured to as an addition to the audience it derives.
//
// The To line keeps the old rule. Widening who a report is addressed to
// should stay a deliberate switch of source, not a side effect of
// adding a cc.
func Resolve(derived Recipients, configured Audience, source Source, override []string) Audience {
	if o := clean(override); len(o) > 0 {
		return Audience{To: o}
	}
	if source == SourceDerived {
		return Audience{
			To: dedupe(derived.To),
			Cc: dedupe(append(append([]string{}, derived.Cc...), configured.Cc...)),
		}
	}
	return Audience{To: dedupe(configured.To), Cc: dedupe(configured.Cc)}
}

// ParseSource reads the configured source, defaulting to configured for
// anything unrecognised -- including a typo, where quietly mailing a
// different set of people is the worse failure.
func ParseSource(raw string) (Source, bool) {
	switch Source(strings.ToLower(strings.TrimSpace(raw))) {
	case SourceDerived:
		return SourceDerived, true
	case SourceConfigured, "":
		return SourceConfigured, true
	default:
		return SourceConfigured, false
	}
}

// Drift reports addresses the team registry says should receive this
// report but the configured lists omit, and vice versa. Neither is an
// error -- the configured list is deliberately not the derived one -- but
// a lead who joins a team and never appears, or one who left months ago
// and still receives it, shows up here first.
func Drift(derived Recipients, configured Audience) (missing, extra []string) {
	cfg := map[string]bool{}
	for _, a := range append(dedupe(configured.To), dedupe(configured.Cc)...) {
		cfg[strings.ToLower(a)] = true
	}
	der := map[string]bool{}
	for _, a := range append(dedupe(derived.To), dedupe(derived.Cc)...) {
		der[strings.ToLower(a)] = true
	}
	for a := range der {
		if !cfg[a] {
			missing = append(missing, a)
		}
	}
	// Only individuals count as extra. A configured group address has no
	// derived counterpart by construction, so reporting every one of them
	// every run would bury the signal.
	for a := range cfg {
		if !der[a] && !looksLikeGroupAddress(a) {
			extra = append(extra, a)
		}
	}
	return sorted(missing), sorted(extra)
}

// looksLikeGroupAddress distinguishes a shared mailbox from a person, so
// Drift does not report every distribution list as an unexplained
// recipient -- they have no derived counterpart by construction, and
// listing them every run would bury the signal.
//
// The test is a hyphen in the local part. Distribution lists here are
// hyphenated and people are not. It is a heuristic, and a person whose
// address happens to be hyphenated is simply left out of the drift
// report; the cost is one missing log line, never a wrong recipient.
func looksLikeGroupAddress(addr string) bool {
	local := addr
	if i := strings.Index(addr, "@"); i >= 0 {
		local = addr[:i]
	}
	return strings.Contains(local, "-")
}

func clean(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// dedupe preserves first-seen order and compares case-insensitively.
// It does NOT deduplicate across To and Cc: the delivered daily mail has
// mara@ and said@ in both, and reproducing it exactly means
// keeping that.
func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range clean(in) {
		k := strings.ToLower(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

func sorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string{}, in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
