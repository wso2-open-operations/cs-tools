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
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// What keeps one large customer from taking the refresh down: the subject
// group cap, the early-exit similarity, the context checks, and the skip.

func sgGroup(n int, subject string) []repository.SuggestionCandidate {
	out := make([]repository.SuggestionCandidate, n)
	for i := range out {
		out[i] = sgCand(i+1, subject, "CLOSED", sgPlan)
	}
	return out
}

func TestClusterSuggestionCandidates_CapsASubjectGroup(t *testing.T) {
	cls := mustCluster(t, sgGroup(suggestionMaxCompared+100, "Monthly OS patching"), 0.5)
	if len(cls) != 1 || len(cls[0].nodes) != suggestionMaxCompared {
		t.Fatalf("clusters = %d, nodes = %d; want one of %d", len(cls), len(cls[0].nodes), suggestionMaxCompared)
	}
	// The newest are kept: CHG0000001 is a day old, CHG0000600 600 days.
	if first, last := cls[0].nodes[0].c.Number, cls[0].nodes[len(cls[0].nodes)-1].c.Number; first != "CHG0000001" || last != "CHG0000500" {
		t.Errorf("kept %s..%s, want the newest CHG0000001..CHG0000500", first, last)
	}
	s := judgeSuggestionCluster(cls[0], sgCriteria, nil, nil)
	if s.Sample == nil || *s.Sample != (domain.StandardChangeSuggestionSample{Compared: suggestionMaxCompared, Total: suggestionMaxCompared + 100}) {
		t.Fatalf("sample = %+v, want 500 of 600", s.Sample)
	}
	if s.MemberCount != suggestionMaxCompared {
		t.Errorf("memberCount = %d, want the %d compared", s.MemberCount, suggestionMaxCompared)
	}

	t.Run("a group within the cap has no sample", func(t *testing.T) {
		cls := mustCluster(t, sgGroup(suggestionMaxCompared, "Monthly OS patching"), 0.5)
		if s := judgeSuggestionCluster(cls[0], sgCriteria, nil, nil); s.Sample != nil || s.MemberCount != suggestionMaxCompared {
			t.Errorf("sample = %+v, members = %d", s.Sample, s.MemberCount)
		}
	})
	t.Run("a clone of a member left out of the cap is not pulled in by it", func(t *testing.T) {
		cands := sgGroup(suggestionMaxCompared+1, "Monthly OS patching")
		clone := sgCand(9000, "Promote patching to DR", "CLOSED", sgPlan)
		clone.CompletedOn = sgNow
		clone.ClonedFrom = sp(cands[suggestionMaxCompared].ID) // the oldest: cut
		cls := mustCluster(t, append(cands, clone), 0.5)
		for _, cl := range cls {
			for _, n := range cl.nodes {
				if n.c.Number == clone.Number || n.c.Number == cands[suggestionMaxCompared].Number {
					t.Fatalf("%s is in cluster %q; the cut member and its clone belong to none", n.c.Number, cl.key)
				}
			}
		}
	})
}

func TestSimilarAtLeast_SameAsJaccard(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	set := func(n, vocab int) wordSet {
		out := wordSet{}
		for len(out) < n {
			out[fmt.Sprintf("w%d", rng.Intn(vocab))] = struct{}{}
		}
		return out
	}
	cases := [][2]wordSet{{{}, {}}, {set(1, 5), {}}, {set(1, 3), set(400, 500)}, {set(400, 500), set(1, 3)}}
	for i := 0; i < 2000; i++ {
		vocab := 5 + rng.Intn(80)
		cases = append(cases, [2]wordSet{set(1+rng.Intn(min(vocab, 40)), vocab), set(1+rng.Intn(min(vocab, 40)), vocab)})
	}
	for _, c := range cases {
		for _, th := range []float64{0, 0.01, 0.25, 0.5, 0.51, 0.9, 1} {
			if got, want := similarAtLeast(c[0], c[1], th), jaccard(c[0], c[1]) >= th; got != want {
				t.Fatalf("similarAtLeast(%d words, %d words, %v) = %v, jaccard %v", len(c[0]), len(c[1]), th, got, jaccard(c[0], c[1]))
			}
		}
	}
}

func BenchmarkSimilarAtLeast(b *testing.B) {
	small, large := wordSet{}, wordSet{}
	for i := 0; i < 60; i++ {
		small[fmt.Sprintf("w%d", i)] = struct{}{}
	}
	for i := 30; i < 330; i++ {
		large[fmt.Sprintf("w%d", i)] = struct{}{}
	}
	b.Run("jaccard", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = jaccard(large, small) >= 0.5
		}
	})
	b.Run("similarAtLeast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = similarAtLeast(large, small, 0.5)
		}
	})
}

func TestClusterSuggestionCandidates_StopsWhenTheContextEnds(t *testing.T) {
	// 100 planned members are 4,950 pairs: past the first check.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := clusterSuggestionCandidates(ctx, sgGroup(100, "Monthly OS patching"), 0.5); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := clusterSuggestionCandidates(ctx, sgGroup(2, "Restart gateway"), 0.5); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a small group past its deadline: err = %v, want context.DeadlineExceeded", err)
	}
}

func TestStandardChangeSuggestionService_Refresh_SkipsACustomerThatOverruns(t *testing.T) {
	old := suggestionCustomerTimeout
	suggestionCustomerTimeout = 20 * time.Millisecond
	t.Cleanup(func() { suggestionCustomerTimeout = old })

	repo := sgFixture()
	repo.slow = map[string]bool{sgAccountA: true}
	svc := sgService(repo, true)
	pages, last := refreshAll(t, svc, 1)
	if pages != 2 || last.Totals == nil ||
		*last.Totals != (domain.StandardChangeSuggestionScan{Customers: 1, ChangeRequests: 2, Clusters: 1, Skipped: 1}) {
		t.Fatalf("pages = %d, totals = %+v; want B computed and A skipped", pages, last.Totals)
	}

	resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.SkippedCustomers) != 1 || resp.SkippedCustomers[0].Customer.ID != sgAccountA || resp.SkippedCustomers[0].Reason != suggestionSkippedTooLarge {
		t.Fatalf("skippedCustomers = %+v", resp.SkippedCustomers)
	}
	if len(resp.Suggestions) != 1 || resp.Suggestions[0].Customer.ID != sgAccountB || resp.Scanned.Skipped != 1 {
		t.Errorf("suggestions = %+v, scanned = %+v", resp.Suggestions, resp.Scanned)
	}
	one, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{AccountID: sgAccountA})
	if len(one.SkippedCustomers) != 1 || one.Scanned != (domain.StandardChangeSuggestionScan{Skipped: 1}) {
		t.Errorf("customer A: skipped = %+v, scanned = %+v", one.SkippedCustomers, one.Scanned)
	}
	other, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{AccountID: sgAccountB})
	if len(other.SkippedCustomers) != 0 || other.SkippedCustomers == nil {
		t.Errorf("customer B: skipped = %+v, want an empty list", other.SkippedCustomers)
	}

	t.Run("the call's own deadline is an error, not a skip", func(t *testing.T) {
		suggestionCustomerTimeout = time.Minute
		repo := sgFixture()
		repo.slow = map[string]bool{sgAccountA: true}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := sgService(repo, true).Refresh(ctx, domain.RefreshStandardChangeSuggestionsRequest{})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want the call's deadline", err)
		}
		if len(repo.skipped) != 1 || len(repo.skipped[firstRunID(repo)]) != 0 {
			t.Errorf("skipped = %v, want none recorded", repo.skipped)
		}
	})
}

func firstRunID(repo *fakeSuggestionRepo) string {
	for id := range repo.runs {
		return id
	}
	return ""
}

func TestStandardChangeSuggestionService_Refresh_ListsTheCustomersOnce(t *testing.T) {
	repo := sgFixture()
	svc := sgService(repo, true)
	first, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{Limit: 1})
	if err != nil || !first.More {
		t.Fatal(first, err)
	}
	// A customer whose change request completes after the run started waits
	// for the next run.
	late := sgCand(99, "Deploy CApps", "CLOSED", sgPlan)
	late.AccountID = "cccccccc-0000-4000-8000-00000000000c"
	repo.cands = append(repo.cands, late)
	last, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{RunID: first.RunID, Cursor: first.NextCursor, Limit: 5})
	if err != nil || last.More || last.Totals.Customers != 2 {
		t.Fatalf("last = %+v, %v", last, err)
	}
	if len(repo.sinces) != 1 {
		t.Errorf("customers listed %d times, want once per run", len(repo.sinces))
	}
}
