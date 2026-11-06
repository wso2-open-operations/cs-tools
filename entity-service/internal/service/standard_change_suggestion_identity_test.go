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
