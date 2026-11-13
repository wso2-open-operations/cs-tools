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
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func cabDecision(state, comment string, daysAgo int, who string) repository.CABDecision {
	return repository.CABDecision{State: state, Comment: comment, DecidedOn: sgNow.AddDate(0, 0, -daysAgo), ApproverName: who}
}

// cabLookalike is a canceled change request the CAB rejected.
func cabLookalike(n int, subject, plan string, decisions ...repository.CABDecision) repository.CABChangeRequest {
	return repository.CABChangeRequest{
		ID: fmt.Sprintf("00000000-0000-4000-9000-%012d", n), Number: fmt.Sprintf("CHG%07d", n),
		Subject: subject, State: "canceled", ImplementationPlan: plan, Decisions: decisions,
	}
}

func TestCABPlainText(t *testing.T) {
	for in, want := range map[string]string{
		"":        "",
		"   ":     "",
		"<p></p>": "",
		"<p>Add the <b>verification</b> steps</p>":      "Add the verification steps",
		"Rollback plan &amp; owner missing\n\n  please": "Rollback plan & owner missing please",
	} {
		if got := cabPlainText(in); got != want {
			t.Errorf("cabPlainText(%q) = %q, want %q", in, got, want)
		}
	}
	long := cabPlainText(strings.Repeat("word ", 1000))
	if n := utf8.RuneCountInString(long); n > cabCommentMaxRunes || !strings.HasSuffix(long, "…") {
		t.Errorf("a long comment is %d characters (cap %d) and ends %q", n, cabCommentMaxRunes, long[len(long)-5:])
	}
}

func TestBuildCABHistory(t *testing.T) {
	member := repository.CABChangeRequest{ID: "m1", Number: "CHG0000001", Subject: "Deploy CApps", State: "closed",
		Decisions: []repository.CABDecision{
			cabDecision("APPROVED", "<p>Add the verification steps</p>", 2, "Carol"), // approved, with pushback
			cabDecision("REJECTED", "No rollback plan", 5, "Alice"),
			cabDecision("REJECTED", "", 9, "Bob"), // an older rejection of the same change request
		}}
	look := cabLookalike(2, "Deploy CApps", "", cabDecision("REJECTED", "<p></p>", 3, "Bob"))
	h := buildCABHistory([]cabSubject{{cr: member, member: true}, {cr: look}, {cr: member, member: true}})
	if h.CABRejectionCount != 2 || len(h.CABRejections) != 2 {
		t.Fatalf("rejections = %d %+v, want two change requests, each once", h.CABRejectionCount, h.CABRejections)
	}
	if r := h.CABRejections[0]; r.ChangeRequest.Number != "CHG0000002" || r.Member || r.RejectedBy != "Bob" || r.Comment != "" {
		t.Errorf("newest rejection = %+v, want the look-alike, with no comment (blank rich text)", r)
	}
	if r := h.CABRejections[1]; r.ChangeRequest.ID != "m1" || !r.Member || r.RejectedBy != "Alice" || r.Comment != "No rollback plan" ||
		r.ChangeRequest.State != "closed" || !r.RejectedOn.Equal(sgNow.AddDate(0, 0, -5)) {
		t.Errorf("the member's rejection = %+v, want its latest (Alice's)", r)
	}
	if h.CABCommentCount != 2 || len(h.CABComments) != 2 {
		t.Fatalf("comments = %d %+v, want the two with text", h.CABCommentCount, h.CABComments)
	}
	if c := h.CABComments[0]; c.State != "APPROVED" || c.Comment != "Add the verification steps" || c.ApproverName != "Carol" {
		t.Errorf("newest comment = %+v, want Carol's approval with its pushback as plain text", c)
	}

	t.Run("lists are capped, counts stay true", func(t *testing.T) {
		var subjects []cabSubject
		for i := 0; i < 30; i++ {
			subjects = append(subjects, cabSubject{cr: cabLookalike(100+i, "Deploy CApps", "", cabDecision("REJECTED", "no", i, "Alice"))})
		}
		h := buildCABHistory(subjects)
		if h.CABRejectionCount != 30 || len(h.CABRejections) != cabHistoryMaxListed || h.CABCommentCount != 30 || len(h.CABComments) != cabHistoryMaxListed {
			t.Errorf("counts %d/%d, listed %d/%d", h.CABRejectionCount, h.CABCommentCount, len(h.CABRejections), len(h.CABComments))
		}
		if h.CABRejections[0].ChangeRequest.Number != "CHG0000100" {
			t.Errorf("first listed = %s, want the newest", h.CABRejections[0].ChangeRequest.Number)
		}
	})
	t.Run("nothing to say is empty lists, never null", func(t *testing.T) {
		h := buildCABHistory(nil)
		if h.CABRejections == nil || h.CABComments == nil || h.CABRejectionCount != 0 {
			t.Errorf("history = %+v", h)
		}
	})
}

func TestAttachCABLookalikes(t *testing.T) {
	const otherPlan = "<p>Rotate the keystore password, update the secure vault, restart every node one by one</p>"
	// Two clusters share the subject (dissimilar plans), a third has another.
	cands := []repository.SuggestionCandidate{
		sgCand(1, "Deploy CApps", "CLOSED", sgPlan), sgCand(2, "Deploy CApps", "CLOSED", sgPlan), sgCand(3, "Deploy CApps", "CLOSED", sgPlan),
		sgCand(4, "Deploy CApps", "CLOSED", otherPlan), sgCand(5, "Deploy CApps", "CLOSED", otherPlan),
		sgCand(6, "Restart gateway", "CLOSED", ""), sgCand(7, "Restart gateway", "CLOSED", ""),
	}
	clusters := mustCluster(t, cands, 0.5)
	assignSuggestionIdentities(clusters, nil)
	if len(clusters) != 3 {
		t.Fatalf("clusters = %v", clusterKeys(clusters))
	}
	index := func(member string) int {
		for i, cl := range clusters {
			for _, n := range cl.nodes {
				if n.c.Number == member {
					return i
				}
			}
		}
		t.Fatalf("no cluster holds %s", member)
		return -1
	}
	big, small, restart := index("CHG0000001"), index("CHG0000004"), index("CHG0000006")
	got := attachCABLookalikes(clusters, []repository.CABChangeRequest{
		cabLookalike(21, "[ACME] Deploy CApps 2026-10-01", otherPlan+" and check"), // shared key: the plan picks the smaller cluster
		cabLookalike(22, "Deploy capps", "short"),                                  // shared key, no plan to compare: the largest
		cabLookalike(23, "Restart gateway", sgPlan),                                // one cluster with the key: that one
		cabLookalike(24, "Upgrade the database", sgPlan),                           // no cluster has the key: dropped
		cabLookalike(25, "12345", sgPlan),                                          // no key at all: dropped
	})
	numbers := func(list []repository.CABChangeRequest) []string {
		var out []string
		for _, c := range list {
			out = append(out, c.Number)
		}
		return out
	}
	want := map[int][]string{small: {"CHG0000021"}, big: {"CHG0000022"}, restart: {"CHG0000023"}}
	for i := range clusters {
		if !reflect.DeepEqual(numbers(got[i]), want[i]) {
			t.Errorf("cluster %q got %v, want %v", clusters[i].id, numbers(got[i]), want[i])
		}
	}
	// Attaching never changes the clusters. Compared as sets of members: two
	// clusters share the key "deploy capps", and clustering orders clusters by
	// key only, so clusterKeys would merge them in either order.
	memberSets := func(cls []suggestionCluster) []string {
		var out []string
		for _, cl := range cls {
			var ns []string
			for _, n := range cl.nodes {
				ns = append(ns, n.c.Number)
			}
			sort.Strings(ns)
			out = append(out, cl.key+": "+strings.Join(ns, ","))
		}
		sort.Strings(out)
		return out
	}
	if again := mustCluster(t, cands, 0.5); !reflect.DeepEqual(memberSets(again), memberSets(clusters)) {
		t.Errorf("clusters moved: %v, want %v", memberSets(again), memberSets(clusters))
	}
}

func TestJudgeSuggestionCluster_CAB(t *testing.T) {
	mk := func(cands ...repository.SuggestionCandidate) suggestionCluster {
		cls := mustCluster(t, cands, 0.5)
		if len(cls) != 1 {
			t.Fatalf("want one cluster, got %v", clusterKeys(cls))
		}
		return cls[0]
	}
	var cands []repository.SuggestionCandidate
	for i := 1; i <= 5; i++ {
		cands = append(cands, sgCand(i, "Deploy CApps", "CLOSED", sgPlan))
	}
	cands[1].HasDeviation = true // CHG0000002 deviated
	cl := mk(cands...)
	cab := map[string]repository.CABChangeRequest{
		// The newest was rejected once, then resubmitted and closed.
		cands[0].ID: {ID: cands[0].ID, Number: cands[0].Number, Subject: cands[0].Subject, State: "closed",
			Decisions: []repository.CABDecision{cabDecision("APPROVED", "Fine this time", 20, "Bob"), cabDecision("REJECTED", "Plan unclear", 25, "Alice")}},
		cands[3].ID: {ID: cands[3].ID, Number: cands[3].Number, Subject: cands[3].Subject, State: "closed",
			Decisions: []repository.CABDecision{cabDecision("APPROVED", "Add the verification steps", 60, "Carol")}},
	}
	look := []repository.CABChangeRequest{cabLookalike(30, "Deploy CApps", "", cabDecision("REJECTED", "Not in a freeze", 10, "Alice"))}

	plain := judgeSuggestionCluster(cl, sgCriteria, nil, nil, nil, nil)
	s := judgeSuggestionCluster(cl, sgCriteria, nil, nil, cab, look)
	if s.ProposeFrom == nil || s.ProposeFrom.Name != "CHG0000003" {
		t.Fatalf("proposeFrom = %+v, want CHG0000003 (the newest neither deviated nor rejected)", s.ProposeFrom)
	}
	var ev []string
	for _, e := range s.Evidence {
		ev = append(ev, e.Name)
	}
	if strings.Join(ev, ",") != "CHG0000001,CHG0000002,CHG0000004,CHG0000005" {
		t.Errorf("evidence = %v, want every other closed member", ev)
	}
	if s.Eligible != plain.Eligible || !reflect.DeepEqual(s.Unmet, plain.Unmet) || s.MemberCount != plain.MemberCount || s.Key != plain.Key {
		t.Errorf("the CAB's pushback changed the judgement: %+v / %+v", s.Unmet, plain.Unmet)
	}
	if s.DeviationCount != 1 || s.CABRejectionCount != 2 || s.CABCommentCount != 4 {
		t.Errorf("deviations %d, rejections %d, comments %d; want 1, 2 (the member and the look-alike), 4", s.DeviationCount, s.CABRejectionCount, s.CABCommentCount)
	}
	if m := s.Members[0]; !m.CABRejected || m.CABCommentCount != 2 {
		t.Errorf("member %s = rejected %v, comments %d", m.Number, m.CABRejected, m.CABCommentCount)
	}
	if m := s.Members[3]; m.CABRejected || m.CABCommentCount != 1 {
		t.Errorf("member %s = rejected %v, comments %d", m.Number, m.CABRejected, m.CABCommentCount)
	}
	if r := s.CABRejections[0]; r.ChangeRequest.Number != "CHG0000030" || r.Member {
		t.Errorf("newest rejection = %+v, want the look-alike", r)
	}

	t.Run("every clean member rejected: the newest clean one still", func(t *testing.T) {
		all := map[string]repository.CABChangeRequest{}
		for _, c := range cands {
			all[c.ID] = repository.CABChangeRequest{ID: c.ID, Decisions: []repository.CABDecision{cabDecision("REJECTED", "", 40, "Alice")}}
		}
		if s := judgeSuggestionCluster(cl, sgCriteria, nil, nil, all, nil); s.ProposeFrom == nil || s.ProposeFrom.Name != "CHG0000001" {
			t.Errorf("proposeFrom = %+v, want CHG0000001", s.ProposeFrom)
		}
	})
}

func TestStandardChangeSuggestionService_RefreshRecordsTheCABsPushback(t *testing.T) {
	repo := sgFixture()
	first := repo.cands[0] // CHG0000001, Acme's "Deploy CApps"
	repo.cab = map[string]repository.CABChangeRequest{
		first.ID: {ID: first.ID, Number: first.Number, Subject: first.Subject, State: "closed",
			Decisions: []repository.CABDecision{cabDecision("REJECTED", "Plan unclear", 30, "Alice")}},
	}
	repo.lookalikes = map[string][]repository.CABChangeRequest{
		sgAccountA: {cabLookalike(40, "Deploy CApps", "", cabDecision("REJECTED", "Freeze", 3, "Bob"))},
	}
	svc := sgService(repo, true)
	refreshAll(t, svc, 10)
	resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, sg := range resp.Suggestions {
		if sg.CABRejections == nil || sg.CABComments == nil {
			t.Errorf("%s: null lists", sg.Key)
		}
		switch {
		case sg.Customer.ID == sgAccountA && sg.SubjectKey == "deploy capps":
			if sg.CABRejectionCount != 2 || sg.CABCommentCount != 2 || !sg.Members[0].CABRejected {
				t.Errorf("Acme's deploy: rejections %d, comments %d, %+v", sg.CABRejectionCount, sg.CABCommentCount, sg.Members[0])
			}
			if sg.ProposeFrom == nil || sg.ProposeFrom.Name != "CHG0000002" {
				t.Errorf("proposeFrom = %+v, want CHG0000002", sg.ProposeFrom)
			}
			if !sg.Eligible {
				t.Error("a CAB rejection made the suggestion ineligible")
			}
		default:
			// Beta's "Deploy CApps" shares the subject but not the customer.
			if sg.CABRejectionCount != 0 || sg.CABCommentCount != 0 {
				t.Errorf("%s of %s: %d rejections, %d comments, want none", sg.Key, sg.Customer.Name, sg.CABRejectionCount, sg.CABCommentCount)
			}
		}
	}
}

// A snapshot stored before the CAB's pushback existed decodes with zero counts
// and empty lists.
func TestStandardChangeSuggestionService_OlderSnapshotHasNoCABFields(t *testing.T) {
	repo := sgFixture()
	svc := sgService(repo, true)
	refreshAll(t, svc, 10)
	for runID, rows := range repo.rows {
		for i := range rows {
			rows[i].Suggestion.StandardChangeCABHistory = domain.StandardChangeCABHistory{}
		}
		repo.rows[runID] = rows
	}
	resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
	if err != nil || len(resp.Suggestions) == 0 {
		t.Fatal(resp, err)
	}
	for _, sg := range resp.Suggestions {
		if sg.CABRejections == nil || sg.CABComments == nil || sg.CABRejectionCount != 0 {
			t.Errorf("%s: %+v", sg.Key, sg.StandardChangeCABHistory)
		}
	}
}
