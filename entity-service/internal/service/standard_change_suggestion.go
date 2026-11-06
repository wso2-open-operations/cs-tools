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
// clustering, judging a cluster, relating it to the customer's templates);
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
	suggestionTicket      = regexp.MustCompile(`\b(chg|inc|cs|prb|scp|std|sr|ritm|req|task|ctask)[- ]?\d+\b`)
	suggestionDate        = regexp.MustCompile(`\b\d{4}[-/.]\d{1,2}[-/.]\d{1,2}\b|\b\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4}\b`)
	suggestionTime        = regexp.MustCompile(`\b\d{1,2}:\d{2}(:\d{2})?\s*(am|pm)?\b`)
	suggestionVersion     = regexp.MustCompile(`\bv?\d+(\.\d+)+[a-z0-9-]*\b|\bv\d+\b`)
	suggestionOrdinal     = regexp.MustCompile(`\b\d+(st|nd|rd|th)\b`)
	suggestionDigits      = regexp.MustCompile(`\d+`)
	suggestionNonWord     = regexp.MustCompile(`[^a-z]+`)
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

// suggestionWords reduces text (plain or HTML) to its meaningful words, in order.
func suggestionWords(text string) []string {
	s := suggestionHTMLTag.ReplaceAllString(text, " ")
	s = html.UnescapeString(s)
	s = suggestionPlaceholder.ReplaceAllString(s, " ")
	s = strings.ToLower(s)
	s = suggestionTagged.ReplaceAllString(s, " ")
	for _, re := range []*regexp.Regexp{suggestionTicket, suggestionDate, suggestionTime, suggestionVersion, suggestionOrdinal, suggestionDigits} {
		s = re.ReplaceAllString(s, " ")
	}
	s = suggestionNonWord.ReplaceAllString(s, " ")
	var out []string
	for _, w := range strings.Fields(s) {
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
// rather than joined by clone lineage.
type suggestionCluster struct {
	key   string
	nodes []suggestionNode
	core  map[int]bool // index into nodes
}

// clusterSuggestionCandidates builds the clusters of ONE customer's candidates
// (see the file comment). Clusters of one member are left out.
func clusterSuggestionCandidates(cands []repository.SuggestionCandidate, planThreshold float64) []suggestionCluster {
	nodes := make([]suggestionNode, len(cands))
	byID := make(map[string]int, len(cands))
	for i, c := range cands {
		nodes[i] = suggestionNode{c: c, key: suggestionKey(c.Subject)}
		if words := suggestionWordSet(c.ImplementationPlan); len(words) >= suggestionMinPlanWords {
			nodes[i].plan = words
		}
		byID[c.ID] = i
	}

	// Steps 1 and 2: per subject group, plans confirm.
	core := newUnionFind(len(nodes))
	groups := map[string][]int{}
	for i, n := range nodes {
		if n.key != "" {
			groups[n.key] = append(groups[n.key], i)
		}
	}
	for _, members := range groups {
		var planned, unplanned []int
		for _, i := range members {
			if nodes[i].hasPlan() {
				planned = append(planned, i)
			} else {
				unplanned = append(unplanned, i)
			}
		}
		for a := 0; a < len(planned); a++ {
			for b := a + 1; b < len(planned); b++ {
				if jaccard(nodes[planned[a]].plan, nodes[planned[b]].plan) >= planThreshold {
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

	// Step 3: clone lineage joins directly.
	all := newUnionFind(len(nodes))
	for i := range nodes {
		all.union(i, core.find(i))
	}
	for i, n := range nodes {
		if n.c.ClonedFrom == nil {
			continue
		}
		if j, ok := byID[strings.ToLower(*n.c.ClonedFrom)]; ok {
			all.union(i, j)
		}
	}

	byRoot := map[int][]int{}
	for i := range nodes {
		byRoot[all.find(i)] = append(byRoot[all.find(i)], i)
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
		out = append(out, cl)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].key < out[b].key })
	return out
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
// openProposals are keyed by change request id. Match and Dismissal are left to
// the caller.
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
		Match:         domain.StandardChangeSuggestionNew,
	}
	seenIncident, seenProposal := map[string]bool{}, map[string]bool{}
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

// overlap is |a ∩ b| / min(|a|, |b|), 0 when either is empty: how much of the
// smaller set the larger one contains.
func overlap(a, b wordSet) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if _, ok := b[w]; ok {
			inter++
		}
	}
	return float64(inter) / float64(min(len(a), len(b)))
}

// matchSuggestionTemplate relates a cluster to the customer's live templates,
// against each template's current version with its {{placeholders}} removed.
// The subject counts by overlap (a template's subject is the cluster's with the
// varying words made placeholders, so it is a subset, which Jaccard would
// penalise), the implementation plan by Jaccard, each half (the subject alone
// when either side has no plan). The nearest template at or above
// CoveredPercent covers the cluster; at or above ClosePercent it is close.
func matchSuggestionTemplate(cl suggestionCluster, templates []repository.SuggestionTemplate, crit domain.StandardChangeSuggestionCriteria) (domain.StandardChangeSuggestionMatch, *domain.StandardChangeSuggestionTemplateRef) {
	rep := cl.nodes[0]
	for i, n := range cl.nodes {
		if cl.core[i] && n.hasPlan() {
			rep = n
			break
		}
	}
	subject := suggestionWordSet(cl.key)
	var best *domain.StandardChangeSuggestionTemplateRef
	for _, t := range templates {
		score := overlap(subject, suggestionWordSet(t.Subject))
		if tplPlan := suggestionWordSet(t.ImplementationPlan); rep.hasPlan() && len(tplPlan) >= suggestionMinPlanWords {
			score = (score + jaccard(rep.plan, tplPlan)) / 2
		}
		if best == nil || score > best.Score {
			best = &domain.StandardChangeSuggestionTemplateRef{
				ID: t.ID, Number: t.Number, Name: t.Name, State: strings.ToLower(t.State), Score: score,
			}
		}
	}
	switch {
	case best == nil || best.Score*100+1e-9 < float64(crit.ClosePercent):
		return domain.StandardChangeSuggestionNew, nil
	case best.Score*100+1e-9 >= float64(crit.CoveredPercent):
		return domain.StandardChangeSuggestionCovered, best
	default:
		return domain.StandardChangeSuggestionClose, best
	}
}

// suggestionWindowStart is the earliest completion the window counts.
func suggestionWindowStart(now time.Time, months int) time.Time {
	return now.AddDate(0, -months, 0)
}
