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

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The standard change suggestions: one customer's completed change requests
// that look like the same change done again and again, which a standard change
// template could pre-authorise. This file is the pure part (normalising text,
// clustering, judging a cluster);
// standard_change_suggestion_service.go reads the data and serves it.
//
// A cluster is built in three steps, per customer (a template belongs to one
// customer, so a cluster never mixes two):
//  1. The members share a normalised subject (suggestionKey: lower case, tags,
//     ticket numbers, dates, times, versions, digits and filler words removed).
//  2. Inside such a group, the implementation plans confirm it: two members whose
//     plans are both present are the same change only when the plans are at least
//     PlanSimilarityPercent alike (word-set Jaccard; single linkage). A member
//     with no plan cannot contradict the subject, so it joins the group's largest
//     confirmed cluster rather than bridging two of them.
//  3. Clone lineage joins members directly: a member cloned from another joins
//     that member's cluster whatever its subject or plan says.
// No embeddings and no model are used: the rules are deterministic and
// explainable, and only self-hosted models would be allowed anyway.
//
// A cluster is deliberately NOT related to the customer's existing templates
// ("covered by STD…", "close to STD…"): deciding by word overlap whether two
// changes are the same can be wildly off, so it was left out (see
// entity-service CLAUDE.md, "Standard change suggestions"). Change requests
// raised from a template are not candidates at all, which is exact.

// suggestionMaxKey is the longest key stored (standard_change_suggestion_dismissal.suggestion_key).
const suggestionMaxKey = 512

// suggestionMaxListed caps the members listed per suggestion (memberCount stays
// the true count); suggestionMaxEvidence keeps the source plus the evidence
// within the 100 change requests a proposal may cite.
const (
	suggestionMaxListed    = 100
	suggestionMaxEvidence  = 99
	suggestionMinPlanWords = 3
)

var (
	suggestionHTMLTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	suggestionPlaceholder = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	suggestionTagged      = regexp.MustCompile(`\[[^\]]*\]`)
	// suggestionNumbered is every pattern that removes more than its digits, in
	// one pass (each alternative needs a digit, so text with none skips it):
	// ticket numbers (CHG0012345), dates, times (with am / pm), versions (v1.2.3,
	// with a suffix) and ordinals (3rd). Digits and every other non-letter are
	// word separators afterwards (suggestionWords), so they need no pass of their own.
	suggestionNumbered = regexp.MustCompile(
		`\b(?:chg|inc|cs|prb|scp|std|sr|ritm|req|task|ctask)[- ]?\d+\b` +
			`|\b\d{4}[-/.]\d{1,2}[-/.]\d{1,2}\b|\b\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4}\b` +
			`|\b\d{1,2}:\d{2}(?::\d{2})?\s*(?:am|pm)?\b` +
			`|\bv?\d+(?:\.\d+)+[a-z0-9-]*\b|\bv\d+\b` +
			`|\b\d+(?:st|nd|rd|th)\b`)
)

// suggestionFillerWords carry no meaning for "is this the same change": articles,
// prepositions, conjunctions, and month and day names (a date written in words).
var suggestionFillerWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an the and or of to for on in at by from with into onto via as is be
		please pls request change cr
		jan january feb february mar march apr april may jun june jul july aug august sep sept september
		oct october nov november dec december mon monday tue tues tuesday wed wednesday thu thur thurs
		thursday fri friday sat saturday sun sunday am pm utc ist gmt`) {
		suggestionFillerWords[w] = true
	}
}

// suggestionWords reduces text (plain or HTML) to its meaningful words, in order:
// markup, entities, {{placeholders}} and [tags] removed, lower case, ticket
// numbers, dates, times, versions and ordinals removed, then the runs of letters
// a-z, less one-letter and filler words. Each pass runs only when the text can
// contain what it removes (a plan is read for every candidate, so this is hot).
func suggestionWords(text string) []string {
	s := text
	if strings.IndexByte(s, '<') >= 0 {
		s = suggestionHTMLTag.ReplaceAllString(s, " ")
	}
	if strings.IndexByte(s, '&') >= 0 {
		s = html.UnescapeString(s)
	}
	if strings.Contains(s, "{{") {
		s = suggestionPlaceholder.ReplaceAllString(s, " ")
	}
	s = strings.ToLower(s)
	if strings.IndexByte(s, '[') >= 0 {
		s = suggestionTagged.ReplaceAllString(s, " ")
	}
	if strings.IndexAny(s, "0123456789") >= 0 {
		s = suggestionNumbered.ReplaceAllString(s, " ")
	}
	var out []string
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return r < 'a' || r > 'z' }) {
		if len(w) < 2 || suggestionFillerWords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// suggestionKey is a subject's normalised form: the cluster's name, and what a
// dismissal is recorded under. "" when nothing meaningful is left.
func suggestionKey(subject string) string {
	k := strings.Join(suggestionWords(subject), " ")
	for len(k) > suggestionMaxKey {
		_, size := utf8.DecodeLastRuneInString(k)
		k = k[:len(k)-size]
	}
	return strings.TrimSpace(k)
}

type wordSet map[string]struct{}

func suggestionWordSet(text string) wordSet {
	set := wordSet{}
	for _, w := range suggestionWords(text) {
		set[w] = struct{}{}
	}
	return set
}

// jaccard is |a ∩ b| / |a ∪ b|, 0 when both are empty.
func jaccard(a, b wordSet) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if _, ok := b[w]; ok {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// suggestionNode is a candidate in the clustering.
type suggestionNode struct {
	c    repository.SuggestionCandidate
	key  string
	plan wordSet // nil when the plan has too few words to judge by
}

func (n suggestionNode) hasPlan() bool { return n.plan != nil }

// unionFind over node indexes.
type unionFind []int

func newUnionFind(n int) unionFind {
	u := make(unionFind, n)
	for i := range u {
		u[i] = i
	}
	return u
}

func (u unionFind) find(i int) int {
	for u[i] != i {
		u[i] = u[u[i]]
		i = u[i]
	}
	return i
}

func (u unionFind) union(a, b int) { u[u.find(a)] = u.find(b) }

// newer orders nodes newest completion first, then by number (stable output).
func newer(a, b repository.SuggestionCandidate) bool {
	if !a.CompletedOn.Equal(b.CompletedOn) {
		return a.CompletedOn.After(b.CompletedOn)
	}
	return a.Number > b.Number
}

// suggestionCluster is one cluster before it is judged: its nodes (newest first),
// and which of them are in the core (the subject-and-plan cluster the key names)
// rather than joined by clone lineage. compared / total are set when the key's
// subject group had more than suggestionMaxCompared members: the cluster was
// built from the newest compared of them.
type suggestionCluster struct {
	key             string
	nodes           []suggestionNode
	core            map[int]bool // index into nodes
	compared, total int
}

// suggestionMaxCompared caps a subject group: only its newest this many change
// requests are compared and clustered. The plan comparison is pairwise (about
// 4 microseconds a pair), so 500 is at most 124,750 pairs (well under a second)
// where 3,000 was 4.5 million (17 seconds) and 6,000 over a minute.
const suggestionMaxCompared = 500

// suggestionCtxCheckEvery: the pairwise comparison checks the context this often
// (in pairs), so a customer's deadline stops it promptly.
const suggestionCtxCheckEvery = 4096

// similarAtLeast reports jaccard(a, b) >= threshold without always counting the
// whole intersection: it loops over the smaller set and stops as soon as the
// answer is known. J = I / (A + B - I) >= t  <=>  I * (1 + t) >= t * (A + B).
func similarAtLeast(a, b wordSet, threshold float64) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b) == 0 {
		return 0 >= threshold // jaccard of two empty sets is 0
	}
	need := threshold * float64(len(a)+len(b))
	reached := func(inter int) bool { return float64(inter)*(1+threshold) >= need }
	// The intersection is at most the smaller set.
	if !reached(len(a)) {
		return false
	}
	inter, left := 0, len(a)
	for w := range a {
		left--
		if _, ok := b[w]; ok {
			inter++
			if reached(inter) {
				return true
			}
		}
		if !reached(inter + left) {
			return false
		}
	}
	return reached(inter)
}

// clusterSuggestionCandidates builds the clusters of ONE customer's candidates
// (see the file comment). Clusters of one member are left out. A subject group
// larger than suggestionMaxCompared is cut to its newest members first (the
// older ones are in no cluster; the cluster records how many there were), and
// only members of a subject group of two or more have their plan read. It
// returns ctx's error when ctx ends during the comparison.
func clusterSuggestionCandidates(ctx context.Context, cands []repository.SuggestionCandidate, planThreshold float64) ([]suggestionCluster, error) {
	nodes := make([]suggestionNode, len(cands))
	byID := make(map[string]int, len(cands))
	for i, c := range cands {
		nodes[i] = suggestionNode{c: c, key: suggestionKey(c.Subject)}
		byID[c.ID] = i
	}

	// Steps 1 and 2: per subject group, plans confirm.
	core := newUnionFind(len(nodes))
	excluded := make([]bool, len(nodes)) // older than its subject group's cap
	groups := map[string][]int{}
	for i, n := range nodes {
		if n.key != "" {
			groups[n.key] = append(groups[n.key], i)
		}
	}
	groupTotal := map[string]int{} // keys whose group was cut, to its full size
	pairs := 0
	for key, members := range groups {
		if len(members) < 2 {
			continue // no plan to compare: a lone member joins only by clone lineage
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sort.Slice(members, func(a, b int) bool { return newer(nodes[members[a]].c, nodes[members[b]].c) })
		if len(members) > suggestionMaxCompared {
			groupTotal[key] = len(members)
			for _, i := range members[suggestionMaxCompared:] {
				excluded[i] = true
			}
			members = members[:suggestionMaxCompared]
		}
		var planned, unplanned []int
		for _, i := range members {
			if words := suggestionWordSet(nodes[i].c.ImplementationPlan); len(words) >= suggestionMinPlanWords {
				nodes[i].plan = words
				planned = append(planned, i)
			} else {
				unplanned = append(unplanned, i)
			}
		}
		for a := 0; a < len(planned); a++ {
			for b := a + 1; b < len(planned); b++ {
				if pairs++; pairs%suggestionCtxCheckEvery == 0 {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
				}
				if similarAtLeast(nodes[planned[a]].plan, nodes[planned[b]].plan, planThreshold) {
					core.union(planned[a], planned[b])
				}
			}
		}
		// Members with no plan join the largest confirmed cluster (on a tie, the
		// one with the newest member), or one of their own.
		anchor := -1
		if len(planned) > 0 {
			size := map[int]int{}
			newest := map[int]int{}
			for _, i := range planned {
				r := core.find(i)
				size[r]++
				if j, ok := newest[r]; !ok || newer(nodes[i].c, nodes[j].c) {
					newest[r] = i
				}
			}
			for r := range size {
				if anchor == -1 || size[r] > size[anchor] ||
					(size[r] == size[anchor] && newer(nodes[newest[r]].c, nodes[newest[anchor]].c)) {
					anchor = r
				}
			}
		}
		for _, i := range unplanned {
			if anchor == -1 {
				anchor = i
				continue
			}
			core.union(i, anchor)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Step 3: clone lineage joins directly (never to or from a member left out
	// by its group's cap).
	all := newUnionFind(len(nodes))
	for i := range nodes {
		all.union(i, core.find(i))
	}
	for i, n := range nodes {
		if n.c.ClonedFrom == nil || excluded[i] {
			continue
		}
		if j, ok := byID[strings.ToLower(*n.c.ClonedFrom)]; ok && !excluded[j] {
			all.union(i, j)
		}
	}

	byRoot := map[int][]int{}
	for i := range nodes {
		if !excluded[i] {
			byRoot[all.find(i)] = append(byRoot[all.find(i)], i)
		}
	}
	var out []suggestionCluster
	for _, idx := range byRoot {
		if len(idx) < 2 {
			continue
		}
		sort.Slice(idx, func(a, b int) bool { return newer(nodes[idx[a]].c, nodes[idx[b]].c) })
		// The core is the largest subject-and-plan cluster inside (on a tie, the
		// one with the newest member); its key names the cluster.
		coreSize := map[int]int{}
		var coreRoot = -1
		for _, i := range idx {
			if nodes[i].key == "" {
				continue
			}
			r := core.find(i)
			coreSize[r]++
			if coreRoot == -1 || coreSize[r] > coreSize[coreRoot] {
				coreRoot = r
			}
		}
		if coreRoot == -1 {
			continue // nothing meaningful to call it by
		}
		cl := suggestionCluster{core: map[int]bool{}}
		for _, i := range idx {
			if core.find(i) == coreRoot {
				cl.core[len(cl.nodes)] = true
				if cl.key == "" {
					cl.key = nodes[i].key
				}
			}
			cl.nodes = append(cl.nodes, nodes[i])
		}
		if total, cut := groupTotal[cl.key]; cut {
			cl.compared, cl.total = suggestionMaxCompared, total
		}
		out = append(out, cl)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].key < out[b].key })
	return out, nil
}

// countLevel adds a risk or impact level to a breakdown. MEDIUM (impact's
// middle) counts as moderate.
func countLevel(b *domain.StandardChangeSuggestionBreakdown, level *string) {
	switch strings.ToUpper(strings.TrimSpace(stringOrEmpty(level))) {
	case "LOW":
		b.Low++
	case "MODERATE", "MEDIUM":
		b.Moderate++
	case "HIGH":
		b.High++
	default:
		b.Unknown++
	}
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// judgeSuggestionCluster turns a cluster into a suggestion: its counts, the
// criteria it misses, what a proposal would start from and cite. incidents and
// openProposals are keyed by change request id. The dismissal is left to the
// caller.
func judgeSuggestionCluster(cl suggestionCluster, crit domain.StandardChangeSuggestionCriteria,
	incidents, openProposals map[string][]domain.EntityRef) domain.StandardChangeSuggestion {
	s := domain.StandardChangeSuggestion{
		Key:           cl.key,
		Customer:      domain.EntityRef{ID: cl.nodes[0].c.AccountID, Name: cl.nodes[0].c.AccountName},
		Subject:       cl.nodes[0].c.Subject,
		MemberCount:   len(cl.nodes),
		Incidents:     []domain.EntityRef{},
		Evidence:      []domain.EntityRef{},
		OpenProposals: []domain.EntityRef{},
		Members:       []domain.StandardChangeSuggestionMember{},
		Unmet:         []string{},
	}
	seenIncident, seenProposal := map[string]bool{}, map[string]bool{}
	if cl.total > 0 {
		s.Sample = &domain.StandardChangeSuggestionSample{Compared: cl.compared, Total: cl.total}
	}
	s.LastCompletedOn = cl.nodes[0].c.CompletedOn
	s.FirstCompletedOn = cl.nodes[len(cl.nodes)-1].c.CompletedOn
	for i, n := range cl.nodes {
		c := n.c
		if c.State == "CLOSED" {
			s.ClosedCount++
		} else {
			s.RollbackCount++
		}
		countLevel(&s.Risk, c.Risk)
		countLevel(&s.Impact, c.Impact)
		for _, inc := range incidents[c.ID] {
			if !seenIncident[inc.ID] {
				seenIncident[inc.ID] = true
				s.Incidents = append(s.Incidents, inc)
			}
		}
		for _, p := range openProposals[c.ID] {
			if !seenProposal[p.ID] {
				seenProposal[p.ID] = true
				s.OpenProposals = append(s.OpenProposals, p)
			}
		}
		ref := domain.EntityRef{ID: c.ID, Name: c.Number}
		if c.State == "CLOSED" {
			if s.ProposeFrom == nil && !c.HasDeviation {
				s.ProposeFrom = &ref
			} else if len(s.Evidence) < suggestionMaxEvidence {
				s.Evidence = append(s.Evidence, ref)
			}
		}
		if len(s.Members) < suggestionMaxListed {
			joined := "subject"
			if !cl.core[i] {
				joined = "clone"
			}
			s.Members = append(s.Members, domain.StandardChangeSuggestionMember{
				ID: c.ID, Number: c.Number, Subject: c.Subject, State: strings.ToLower(c.State),
				CompletedOn: c.CompletedOn, Risk: lowerPtr(c.Risk), Impact: lowerPtr(c.Impact),
				ClonedFrom: c.ClonedFrom, HasDeviation: c.HasDeviation, JoinedBy: joined,
			})
		}
	}
	// ProposeFrom stays nil when no closed member is clean (each deviated from its
	// plan, or none closed): there is nothing to start a proposal from, which the
	// page says; the closed ones stay evidence. Not a criterion of its own.
	if done := s.ClosedCount + s.RollbackCount; done > 0 {
		s.SuccessRate = float64(s.ClosedCount) / float64(done)
	}

	if s.ClosedCount < crit.MinClosed {
		s.Unmet = append(s.Unmet, fmt.Sprintf("closed %d %s in %d months (needs %d)",
			s.ClosedCount, plural(s.ClosedCount, "time", "times"), crit.WindowMonths, crit.MinClosed))
	}
	if pct := s.SuccessRate * 100; pct+1e-9 < float64(crit.MinSuccessPercent) {
		s.Unmet = append(s.Unmet, fmt.Sprintf("success rate %.0f%% (needs %d%%)", pct, crit.MinSuccessPercent))
	}
	if n := len(s.Incidents); n > 0 {
		s.Unmet = append(s.Unmet, fmt.Sprintf("%d %s linked to a member", n, plural(n, "incident", "incidents")))
	}
	if s.Risk.High > 0 {
		s.Unmet = append(s.Unmet, fmt.Sprintf("%d %s high risk", s.Risk.High, plural(s.Risk.High, "member has", "members have")))
	}
	if s.Impact.High > 0 {
		s.Unmet = append(s.Unmet, fmt.Sprintf("%d %s high impact", s.Impact.High, plural(s.Impact.High, "member has", "members have")))
	}
	s.Eligible = len(s.Unmet) == 0
	return s
}

// suggestionWindowStart is the earliest completion the window counts.
func suggestionWindowStart(now time.Time, months int) time.Time {
	return now.AddDate(0, -months, 0)
}
