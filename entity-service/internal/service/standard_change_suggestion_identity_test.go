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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// One subject, two changes: a firewall rule update done five times with a
// security-group plan and five times with a WAF plan (CS-2501 / CS-2511 on the
// local example data).
const (
	sgFirewall = "Update the firewall rules for partner integrations"
	sgSGPlan   = "<p>Edit the security group, add the partner CIDR ranges to the inbound rules, save and verify the connection</p>"
	sgWAFPlan  = "<p>Open the WAF console, create an allow rule for the partner user agents, deploy the rule set to production</p>"
)

func sgFirewallPair(from int) []repository.SuggestionCandidate {
	var out []repository.SuggestionCandidate
	for i := 0; i < 5; i++ {
		out = append(out, sgCand(from+i, sgFirewall, "CLOSED", sgSGPlan), sgCand(from+10+i, sgFirewall, "CLOSED", sgWAFPlan))
	}
	return out
}

func sgPlanOf(t *testing.T, resp domain.StandardChangeSuggestionsResponse, plan string, cands []repository.SuggestionCandidate) domain.StandardChangeSuggestion {
	t.Helper()
	byID := map[string]string{}
	for _, c := range cands {
		byID[c.ID] = c.ImplementationPlan
	}
	for _, s := range resp.Suggestions {
		if byID[s.Members[0].ID] == plan {
			return s
		}
	}
	t.Fatalf("no suggestion with plan %q in %+v", plan, resp.Suggestions)
	return domain.StandardChangeSuggestion{}
}

func TestSuggestionKeys_TwoClustersOfOneSubject(t *testing.T) {
	repo := &fakeSuggestionRepo{cands: sgFirewallPair(1)}
	svc := sgService(repo, true)
	refreshAll(t, svc, 0)
	resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
	if err != nil || len(resp.Suggestions) != 2 {
		t.Fatalf("suggestions = %+v, %v; want the two firewall changes", resp.Suggestions, err)
	}
	const subject = "update firewall rules partner integrations"
	a, b := resp.Suggestions[0], resp.Suggestions[1]
	if a.Key == b.Key || a.SubjectKey != subject || b.SubjectKey != subject {
		t.Fatalf("keys %q / %q, subjects %q / %q: want two keys of one subject", a.Key, b.Key, a.SubjectKey, b.SubjectKey)
	}
	for _, s := range []domain.StandardChangeSuggestion{a, b} {
		if s.Key != subject && !(strings.HasPrefix(s.Key, subject+"#") && validSuggestionKey(s.Key)) {
			t.Errorf("key %q is neither the subject key nor the subject key with a suffix", s.Key)
		}
	}

	// Dismissing one hides only that one.
	sgOne := sgPlanOf(t, resp, sgWAFPlan, repo.cands)
	if _, err := svc.DismissSuggestion(sgPersonCtx(), domain.DismissStandardChangeSuggestionRequest{AccountID: sgAccountA, Key: sgOne.Key, Reason: "one-off"}); err != nil {
		t.Fatalf("dismiss %q: %v", sgOne.Key, err)
	}
	repo.dismissals = map[string]map[string]domain.StandardChangeSuggestionDismissal{sgAccountA: {sgOne.Key: {ID: "d1"}}}
	resp, _ = svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
	if len(resp.Suggestions) != 1 || resp.Suggestions[0].Key == sgOne.Key {
		t.Fatalf("after dismissing %q: %+v", sgOne.Key, resp.Suggestions)
	}
}

func TestSuggestionKeys_KeptAcrossRefreshes(t *testing.T) {
	repo := &fakeSuggestionRepo{cands: sgFirewallPair(1)}
	svc := sgService(repo, true)
	refreshAll(t, svc, 0)
	first, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
	sgKey := sgPlanOf(t, first, sgSGPlan, repo.cands).Key
	wafKey := sgPlanOf(t, first, sgWAFPlan, repo.cands).Key

	// A day later: each change was done again, and the oldest of each left the
	// window. Its oldest member is no longer the same; its key is.
	for i := range repo.cands {
		if repo.cands[i].Number == "CHG0000005" || repo.cands[i].Number == "CHG0000015" {
			repo.cands[i].CompletedOn = sgNow.AddDate(-2, 0, 0)
		}
	}
	repo.cands = append(repo.cands, sgCand(30, sgFirewall, "CLOSED", sgSGPlan), sgCand(31, sgFirewall, "CLOSED", sgWAFPlan))
	refreshAll(t, svc, 0)
	second, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
	if got := sgPlanOf(t, second, sgSGPlan, repo.cands).Key; got != sgKey {
		t.Errorf("the security-group change's key moved from %q to %q", sgKey, got)
	}
	if got := sgPlanOf(t, second, sgWAFPlan, repo.cands).Key; got != wafKey {
		t.Errorf("the WAF change's key moved from %q to %q", wafKey, got)
	}
}

func TestAssignSuggestionIdentities(t *testing.T) {
	const subject = "update firewall rules partner integrations"
	clusters := func() []suggestionCluster { return mustCluster(t, sgFirewallPair(1), 0.5) }
	keys := func(cls []suggestionCluster) map[string]string { // plan -> key
		out := map[string]string{}
		for _, cl := range cls {
			out[cl.nodes[0].c.ImplementationPlan] = cl.id
		}
		return out
	}
	ids := func(cl suggestionCluster) []string {
		var out []string
		for _, n := range cl.nodes {
			out = append(out, n.c.ID)
		}
		return out
	}

	t.Run("a subject that split: the larger overlap keeps the bare key, a dismissal made before keeps matching it", func(t *testing.T) {
		cls := clusters()
		sg, waf := cls[0], cls[1]
		if sg.nodes[0].c.ImplementationPlan != sgSGPlan {
			sg, waf = waf, sg
		}
		// The previous run saw one cluster: three security-group members and two WAF ones.
		prev := []repository.SuggestionSnapshotRow{{Key: subject, MemberIDs: append(ids(sg)[:3:3], ids(waf)[:2]...)}}
		assignSuggestionIdentities(cls, prev)
		k := keys(cls)
		if k[sgSGPlan] != subject || !strings.HasPrefix(k[sgWAFPlan], subject+"#") || !validSuggestionKey(k[sgWAFPlan]) {
			t.Fatalf("keys = %v; want the security-group change (3 shared) to keep %q", k, subject)
		}
	})
	t.Run("no previous run: the larger cluster gets the bare key, the other one from its oldest core member", func(t *testing.T) {
		in := append(sgFirewallPair(1), sgCand(40, sgFirewall, "CLOSED", sgWAFPlan)) // six WAF
		cls := mustCluster(t, in, 0.5)
		assignSuggestionIdentities(cls, nil)
		k := keys(cls)
		if k[sgWAFPlan] != subject || k[sgSGPlan] != subject+"#"+suggestionIDSuffix(sgCand(5, "", "", "").ID) {
			t.Fatalf("keys = %v", k)
		}
		// The same input gives the same keys.
		again := mustCluster(t, in, 0.5)
		assignSuggestionIdentities(again, nil)
		if !mapsEqual(keys(again), k) {
			t.Errorf("minted %v, then %v", k, keys(again))
		}
	})
	t.Run("a previous key is given to one cluster only", func(t *testing.T) {
		cls := clusters()
		prev := []repository.SuggestionSnapshotRow{
			{Key: subject + "#aaaaaaaaaaaa", MemberIDs: append(ids(cls[0])[:2:2], ids(cls[1])[:1]...)},
			{Key: subject + "#aaaaaaaaaaaa", MemberIDs: ids(cls[1])}, // an old duplicate row
			{Key: "another subject", MemberIDs: ids(cls[0])},         // another subject never lends its key
		}
		assignSuggestionIdentities(cls, prev)
		if cls[0].id == cls[1].id || (cls[0].id != subject+"#aaaaaaaaaaaa" && cls[1].id != subject+"#aaaaaaaaaaaa") {
			t.Fatalf("keys %q / %q", cls[0].id, cls[1].id)
		}
		if cls[1].id != subject+"#aaaaaaaaaaaa" {
			t.Errorf("the cluster sharing 5 got %q; want the inherited key", cls[1].id)
		}
	})
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestValidSuggestionKey(t *testing.T) {
	for key, want := range map[string]bool{
		"update firewall rules partner integrations":              true,
		"update firewall rules partner integrations#0a1b2c3d4e5f": true,
		"update firewall rules partner integrations#0A1B2C3D4E5F": false, // lower case only
		"update firewall rules partner integrations#0a1b":         false,
		"update firewall rules partner integrations#":             false,
		"Update the firewall rules":                               false, // not normalised
		"#0a1b2c3d4e5f":                                           false,
		"":                                                        false,
	} {
		if got := validSuggestionKey(key); got != want {
			t.Errorf("validSuggestionKey(%q) = %v, want %v", key, got, want)
		}
	}
}

// A customer skipped by one refresh keeps its keys in the next: the skip carried
// them over. Without that the next run would mint them afresh, largest first,
// and the WAF change (grown the larger) would take the bare key from the
// security-group one, moving a dismissal with it.
func TestSuggestionKeys_KeptAcrossASkippedRefresh(t *testing.T) {
	repo := &fakeSuggestionRepo{cands: sgFirewallPair(1)}
	svc := sgService(repo, true)
	refreshAll(t, svc, 0)
	first, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
	sgKey := sgPlanOf(t, first, sgSGPlan, repo.cands).Key
	wafKey := sgPlanOf(t, first, sgWAFPlan, repo.cands).Key
	if sgKey == wafKey {
		t.Fatalf("one key for both: %q", sgKey)
	}

	old := suggestionCustomerTimeout
	suggestionCustomerTimeout = 20 * time.Millisecond
	t.Cleanup(func() { suggestionCustomerTimeout = old })
	repo.slow = map[string]bool{sgAccountA: true}
	refreshAll(t, svc, 0) // skipped
	repo.slow = nil
	suggestionCustomerTimeout = old

	// The WAF change was done twice more, so it is now the larger cluster.
	repo.cands = append(repo.cands, sgCand(40, sgFirewall, "CLOSED", sgWAFPlan), sgCand(41, sgFirewall, "CLOSED", sgWAFPlan))
	refreshAll(t, svc, 0)
	third, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
	if got := sgPlanOf(t, third, sgSGPlan, repo.cands).Key; got != sgKey {
		t.Errorf("after a skipped refresh the security-group change's key is %q, want %q", got, sgKey)
	}
	if got := sgPlanOf(t, third, sgWAFPlan, repo.cands).Key; got != wafKey {
		t.Errorf("after a skipped refresh the WAF change's key is %q, want %q", got, wafKey)
	}
}

// A cluster whose largest subject group changed (members joined by clone
// lineage outgrew the subject it was named by) keeps its key: previous keys are
// matched on shared change requests alone, whatever the subject.
func TestAssignSuggestionIdentities_KeyKeptWhenTheSubjectChanges(t *testing.T) {
	cls := mustCluster(t, sgGroup(5, "Promote the Pekin release to DR"), 0.5)
	if len(cls) != 1 {
		t.Fatalf("clusters = %d", len(cls))
	}
	var ids []string
	for _, n := range cls[0].nodes {
		ids = append(ids, n.c.ID)
	}
	prev := []repository.SuggestionSnapshotRow{{Key: "deploy capps pekin prod#0a1b2c3d4e5f", MemberIDs: ids[:3]}}
	assignSuggestionIdentities(cls, prev)
	if cls[0].id != "deploy capps pekin prod#0a1b2c3d4e5f" || cls[0].key != "promote pekin release dr" {
		t.Fatalf("id = %q, subject key = %q; want the previous key kept and the new subject reported", cls[0].id, cls[0].key)
	}
	if s := judgeSuggestionCluster(cls[0], sgCriteria, nil, nil); s.Key != cls[0].id || s.SubjectKey != "promote pekin release dr" {
		t.Errorf("suggestion key %q, subjectKey %q", s.Key, s.SubjectKey)
	}
}

// A refresh that publishes while GET reads deletes the run GET started on; GET
// notices and reads again, so it never answers one run's totals with no rows.
func TestStandardChangeSuggestionService_GetSuggestions_ReadsAgainWhenARefreshPublishes(t *testing.T) {
	repo := sgFixture()
	svc := sgService(repo, true)
	refreshAll(t, svc, 0)
	first, _ := repo.LatestRun(context.Background())
	repo.beforeRows = func() {
		repo.beforeRows = nil
		refreshAll(t, svc, 0) // publishes a new run, deleting the first
	}
	resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	latest, _ := repo.LatestRun(context.Background())
	if latest.ID == first.ID {
		t.Fatal("the refresh in between did not publish")
	}
	if len(resp.Suggestions) != 1 || resp.Scanned.Eligible != 1 {
		t.Fatalf("suggestions = %d, scanned = %+v; want the new run's one eligible suggestion", len(resp.Suggestions), resp.Scanned)
	}
}
