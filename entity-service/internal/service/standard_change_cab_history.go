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
	"html"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The CAB's pushback on a standard change: its rejections and its comments, on
// a suggestion's members and look-alikes (computed in the daily refresh) and on
// a proposal's source and evidence and their look-alikes (read live, for the
// approvers). It is shown, like a deviation from the plan, and decides nothing:
// no criterion, count, key or cluster depends on it. Which approval stage is
// the CAB's is the repository's (isCABStageInHistory); this is the pure part.

// cabHistoryMaxListed caps each list (the counts stay true); cabCommentMaxRunes
// caps one comment's text.
const (
	cabHistoryMaxListed = 20
	cabCommentMaxRunes  = 2000
)

// cabPlainText is a CAB comment as plain text: a comment migrated from the previous system
// can be rich text, so tags are removed, entities decoded and white space
// collapsed, then it is cut to cabCommentMaxRunes.
func cabPlainText(s string) string {
	if strings.IndexByte(s, '<') >= 0 {
		s = suggestionHTMLTag.ReplaceAllString(s, " ")
	}
	if strings.IndexByte(s, '&') >= 0 {
		s = html.UnescapeString(s)
	}
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > cabCommentMaxRunes {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:cabCommentMaxRunes-1])) + "…"
	}
	return s
}

// cabCommentCount is how many of a change request's CAB decisions carry a comment.
func cabCommentCount(c repository.CABChangeRequest) int {
	n := 0
	for _, d := range c.Decisions {
		if cabPlainText(d.Comment) != "" {
			n++
		}
	}
	return n
}

// cabSubject is one change request a CAB history is built from: member says
// it is one of the suggestion's members (a proposal's source or evidence)
// rather than a look-alike.
type cabSubject struct {
	cr     repository.CABChangeRequest
	member bool
}

// buildCABHistory lists the CAB's rejections (one per change request: its
// latest) and comments over the change requests given, each counted once,
// newest first, lists capped at cabHistoryMaxListed.
func buildCABHistory(subjects []cabSubject) domain.StandardChangeCABHistory {
	h := domain.StandardChangeCABHistory{
		CABRejections: []domain.StandardChangeCABRejection{},
		CABComments:   []domain.StandardChangeCABComment{},
	}
	seen := map[string]bool{}
	for _, s := range subjects {
		id := strings.ToLower(s.cr.ID)
		if seen[id] {
			continue
		}
		seen[id] = true
		ref := domain.StandardChangeCABChangeRequest{ID: s.cr.ID, Number: s.cr.Number, Subject: s.cr.Subject, State: s.cr.State}
		rejected := false
		for _, d := range s.cr.Decisions { // newest first
			text := cabPlainText(d.Comment)
			if d.State == "REJECTED" && !rejected {
				rejected = true
				h.CABRejections = append(h.CABRejections, domain.StandardChangeCABRejection{
					ChangeRequest: ref, Member: s.member, RejectedOn: d.DecidedOn, RejectedBy: d.ApproverName, Comment: text,
				})
			}
			if text != "" {
				h.CABComments = append(h.CABComments, domain.StandardChangeCABComment{
					ChangeRequest: ref, Member: s.member, ApproverName: d.ApproverName, State: d.State, Comment: text, DecidedOn: d.DecidedOn,
				})
			}
		}
	}
	sort.SliceStable(h.CABRejections, func(a, b int) bool {
		x, y := h.CABRejections[a], h.CABRejections[b]
		if !x.RejectedOn.Equal(y.RejectedOn) {
			return x.RejectedOn.After(y.RejectedOn)
		}
		return x.ChangeRequest.Number > y.ChangeRequest.Number
	})
	sort.SliceStable(h.CABComments, func(a, b int) bool {
		x, y := h.CABComments[a], h.CABComments[b]
		if !x.DecidedOn.Equal(y.DecidedOn) {
			return x.DecidedOn.After(y.DecidedOn)
		}
		return x.ChangeRequest.Number > y.ChangeRequest.Number
	})
	h.CABRejectionCount, h.CABCommentCount = len(h.CABRejections), len(h.CABComments)
	if len(h.CABRejections) > cabHistoryMaxListed {
		h.CABRejections = h.CABRejections[:cabHistoryMaxListed]
	}
	if len(h.CABComments) > cabHistoryMaxListed {
		h.CABComments = h.CABComments[:cabHistoryMaxListed]
	}
	return h
}

// attachCABLookalikes gives each of ONE customer's clusters the look-alikes
// that belong to it (see the file comment of standard_change_suggestion.go):
// a look-alike whose suggestionKey(subject) is a cluster's core key goes to
// that cluster; when several clusters share the key, to the one whose core
// members' plans are most like the look-alike's (the best word-set Jaccard of
// any core member, as step 2 compares plans), else (no plan to compare, or none
// alike at all) to the largest, then the one with the smaller key. A look-alike
// whose key no cluster has is dropped. The result is indexed like clusters.
func attachCABLookalikes(clusters []suggestionCluster, lookalikes []repository.CABChangeRequest) [][]repository.CABChangeRequest {
	out := make([][]repository.CABChangeRequest, len(clusters))
	if len(lookalikes) == 0 {
		return out
	}
	byKey := map[string][]int{}
	for i, cl := range clusters {
		byKey[cl.key] = append(byKey[cl.key], i)
	}
	corePlans := map[int][]wordSet{} // computed when a key is shared
	plansOf := func(ci int) []wordSet {
		if p, ok := corePlans[ci]; ok {
			return p
		}
		var p []wordSet
		for i, n := range clusters[ci].nodes {
			if !clusters[ci].core[i] {
				continue
			}
			words := n.plan
			if words == nil {
				words = suggestionWordSet(n.c.ImplementationPlan)
			}
			if len(words) >= suggestionMinPlanWords {
				p = append(p, words)
			}
		}
		corePlans[ci] = p
		return p
	}
	idOf := func(ci int) string {
		if clusters[ci].id != "" {
			return clusters[ci].id
		}
		return clusters[ci].key
	}
	for _, la := range lookalikes {
		key := suggestionKey(la.Subject)
		if key == "" {
			continue
		}
		cands := byKey[key]
		if len(cands) == 0 {
			continue
		}
		pick := cands[0]
		if len(cands) > 1 {
			plan := suggestionWordSet(la.ImplementationPlan)
			best := 0.0
			if len(plan) >= suggestionMinPlanWords {
				for _, ci := range cands {
					for _, p := range plansOf(ci) {
						if j := jaccard(plan, p); j > best || (j == best && j > 0 && better(clusters, idOf, ci, pick)) {
							best, pick = j, ci
						}
					}
				}
			}
			if best == 0 {
				pick = cands[0]
				for _, ci := range cands[1:] {
					if better(clusters, idOf, ci, pick) {
						pick = ci
					}
				}
			}
		}
		out[pick] = append(out[pick], la)
	}
	return out
}

// better: cluster a is preferred to b on a tie (more members, then the smaller key).
func better(clusters []suggestionCluster, idOf func(int) string, a, b int) bool {
	if len(clusters[a].nodes) != len(clusters[b].nodes) {
		return len(clusters[a].nodes) > len(clusters[b].nodes)
	}
	return idOf(a) < idOf(b)
}
