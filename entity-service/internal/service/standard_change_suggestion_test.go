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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	sgAccountA = "aaaaaaaa-0000-4000-8000-00000000000a"
	sgAccountB = "bbbbbbbb-0000-4000-8000-00000000000b"
)

var sgNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

const sgPlan = "<p>Stop the gateway, copy the CApp archive to the deployment folder, start the gateway and check the health endpoint</p>"

func sgCand(n int, subject, state, plan string) repository.SuggestionCandidate {
	return repository.SuggestionCandidate{
		ID:                 fmt.Sprintf("00000000-0000-4000-8000-%012d", n),
		Number:             fmt.Sprintf("CHG%07d", n),
		Subject:            subject,
		State:              state,
		CompletedOn:        sgNow.AddDate(0, 0, -n),
		ImplementationPlan: plan,
		AccountID:          sgAccountA,
		AccountName:        "Acme",
	}
}

func sp(s string) *string { return &s }

func TestSuggestionKey(t *testing.T) {
	for in, want := range map[string]string{
		"[ACME] Deploy CApps v1.2.3 to Pekin prod on 2026-03-01 (CHG0012345)": "deploy capps pekin prod",
		"Deploy capps to PEKIN prod - 14/03/2026 10:30 PM":                    "deploy capps pekin prod",
		"Deploy CApps to Pekin prod, March 3rd":                               "deploy capps pekin prod",
		"<b>Rotate</b> the TLS certificate &amp; restart gw-01":               "rotate tls certificate restart gw",
		"WSO2 API Manager 4.2.0 update level 57":                              "wso apimanager update level",
		"12345":                                                               "",
	} {
		// "API Manager": words are kept apart, nothing is merged.
		want = strings.ReplaceAll(want, "apimanager", "api manager")
		if got := suggestionKey(in); got != want {
			t.Errorf("suggestionKey(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("abcdefgh ", 100)
	if got := suggestionKey(long); len(got) > suggestionMaxKey || strings.HasSuffix(got, " ") {
		t.Errorf("a long subject's key is %d bytes (cap %d) or ends in a space", len(got), suggestionMaxKey)
	}
	// A key is its own key (a dismissal sends it back).
	for _, s := range []string{"[X] Restart node 3 at 10:00", "Deploy CApps"} {
		k := suggestionKey(s)
		if suggestionKey(k) != k {
			t.Errorf("suggestionKey is not stable on %q", k)
		}
	}
}

func TestJaccard(t *testing.T) {
	a, b := suggestionWordSet("stop copy start check"), suggestionWordSet("stop copy restart check")
	if got := jaccard(a, b); got != 0.6 {
		t.Errorf("jaccard = %v, want 0.6 (3 of 5)", got)
	}
	if jaccard(wordSet{}, wordSet{}) != 0 {
		t.Error("two empty sets are not similar")
	}
}

func clusterKeys(cls []suggestionCluster) map[string][]string {
	out := map[string][]string{}
	for _, cl := range cls {
		for _, n := range cl.nodes {
			out[cl.key] = append(out[cl.key], n.c.Number)
		}
	}
	return out
}

func TestClusterSuggestionCandidates(t *testing.T) {
	t.Run("same subject and similar plans make one cluster, newest first", func(t *testing.T) {
		got := clusterKeys(clusterSuggestionCandidates([]repository.SuggestionCandidate{
			sgCand(3, "Deploy CApps to Pekin prod 2026-01-03", "CLOSED", sgPlan),
			sgCand(1, "[ACME] Deploy CApps to Pekin prod", "CLOSED", sgPlan+" and notify the customer"),
			sgCand(2, "Deploy capps to pekin PROD v2.1", "ROLLBACK", sgPlan),
		}, 0.5))
		want := map[string][]string{"deploy capps pekin prod": {"CHG0000001", "CHG0000002", "CHG0000003"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("clusters = %v, want %v", got, want)
		}
	})
	t.Run("a dissimilar plan splits the subject; a lone member is no cluster", func(t *testing.T) {
		got := clusterKeys(clusterSuggestionCandidates([]repository.SuggestionCandidate{
			sgCand(1, "Restart the gateway", "CLOSED", sgPlan),
			sgCand(2, "Restart the gateway", "CLOSED", sgPlan),
			sgCand(3, "Restart the gateway", "CLOSED", "<p>Drain the Kubernetes node, cordon it, reboot the virtual machine, uncordon</p>"),
		}, 0.5))
		if want := map[string][]string{"restart gateway": {"CHG0000001", "CHG0000002"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("clusters = %v, want %v", got, want)
		}
	})
	t.Run("a member with no plan joins the largest confirmed cluster, never bridging", func(t *testing.T) {
		other := "<p>Drain the Kubernetes node, cordon it, reboot the virtual machine, uncordon</p>"
		cls := clusterSuggestionCandidates([]repository.SuggestionCandidate{
			sgCand(1, "Restart the gateway", "CLOSED", sgPlan),
			sgCand(2, "Restart the gateway", "CLOSED", sgPlan),
			sgCand(3, "Restart the gateway", "CLOSED", ""),
			sgCand(4, "Restart the gateway", "CLOSED", other),
			sgCand(5, "Restart the gateway", "CLOSED", other),
			sgCand(6, "Restart the gateway", "CLOSED", other),
		}, 0.5)
		got := clusterKeys(cls)
		if len(cls) != 2 {
			t.Fatalf("clusters = %v, want two (the plans differ)", got)
		}
		for _, cl := range cls {
			var nums []string
			for _, n := range cl.nodes {
				nums = append(nums, n.c.Number)
			}
			if len(nums) == 4 && strings.Join(nums, ",") != "CHG0000003,CHG0000004,CHG0000005,CHG0000006" {
				t.Errorf("the planless member joined %v, want the largest cluster (4, 5, 6)", nums)
			}
		}
	})
	t.Run("members with no plans at all cluster by subject", func(t *testing.T) {
		got := clusterKeys(clusterSuggestionCandidates([]repository.SuggestionCandidate{
			sgCand(1, "Renew the certificate", "CLOSED", ""),
			sgCand(2, "Renew certificate", "CLOSED", "<p>&nbsp;</p>"),
		}, 0.5))
		if want := map[string][]string{"renew certificate": {"CHG0000001", "CHG0000002"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("clusters = %v, want %v", got, want)
		}
	})
	t.Run("a clone joins its source's cluster whatever its subject", func(t *testing.T) {
		src := sgCand(2, "Deploy CApps to Pekin prod", "CLOSED", sgPlan)
		other := sgCand(9, "Deploy CApps to Pekin prod", "CLOSED", sgPlan)
		clone := sgCand(1, "Promote the Pekin release to DR", "CLOSED", "<p>Something entirely different was typed here today</p>")
		clone.ClonedFrom = sp(strings.ToUpper(src.ID))
		cls := clusterSuggestionCandidates([]repository.SuggestionCandidate{clone, src, other}, 0.5)
		if len(cls) != 1 || len(cls[0].nodes) != 3 || cls[0].key != "deploy capps pekin prod" {
			t.Fatalf("clusters = %v, want the clone in the source's cluster", clusterKeys(cls))
		}
		if cls[0].core[0] {
			t.Error("the clone (newest) is counted in the subject core; want it joined by lineage")
		}
	})
}

var sgCriteria = DefaultStandardChangeSuggestionCriteria

func TestJudgeSuggestionCluster(t *testing.T) {
	mk := func(cands ...repository.SuggestionCandidate) suggestionCluster {
		cls := clusterSuggestionCandidates(cands, 0.5)
		if len(cls) != 1 {
			t.Fatalf("want one cluster, got %v", clusterKeys(cls))
		}
		return cls[0]
	}
	t.Run("eligible: counts, source and evidence", func(t *testing.T) {
		a := sgCand(1, "Deploy CApps", "CLOSED", sgPlan)
		a.HasDeviation = true // the newest deviated: not the source, still evidence
		b, c, d := sgCand(2, "Deploy CApps", "CLOSED", sgPlan), sgCand(3, "Deploy CApps", "CLOSED", sgPlan), sgCand(4, "Deploy CApps", "CLOSED", sgPlan)
		b.Risk, c.Risk = sp("LOW"), sp("MODERATE")
		d.Impact = sp("MEDIUM")
		s := judgeSuggestionCluster(mk(a, b, c, d), sgCriteria, nil, nil)
		if !s.Eligible || len(s.Unmet) != 0 {
			t.Fatalf("eligible = %v, unmet = %v", s.Eligible, s.Unmet)
		}
		if s.ProposeFrom == nil || s.ProposeFrom.Name != "CHG0000002" {
			t.Fatalf("proposeFrom = %+v, want the newest clean member CHG0000002", s.ProposeFrom)
		}
		var ev []string
		for _, e := range s.Evidence {
			ev = append(ev, e.Name)
		}
		if strings.Join(ev, ",") != "CHG0000001,CHG0000003,CHG0000004" {
			t.Errorf("evidence = %v", ev)
		}
		if s.Risk != (domain.StandardChangeSuggestionBreakdown{Low: 1, Moderate: 1, Unknown: 2}) {
			t.Errorf("risk = %+v (unknown is shown, not refused)", s.Risk)
		}
		if s.Impact.Moderate != 1 || s.Subject != "Deploy CApps" || s.SuccessRate != 1 {
			t.Errorf("impact = %+v, subject = %q, success = %v", s.Impact, s.Subject, s.SuccessRate)
		}
		if !s.LastCompletedOn.Equal(a.CompletedOn) || !s.FirstCompletedOn.Equal(d.CompletedOn) {
			t.Errorf("completed between %v and %v", s.FirstCompletedOn, s.LastCompletedOn)
		}
	})
	t.Run("each criterion it misses is named", func(t *testing.T) {
		a, b := sgCand(1, "Deploy CApps", "CLOSED", sgPlan), sgCand(2, "Deploy CApps", "ROLLBACK", sgPlan)
		a.Risk, b.Impact = sp("HIGH"), sp("HIGH")
		incidents := map[string][]domain.EntityRef{a.ID: {{ID: "i1", Name: "INC1"}}, b.ID: {{ID: "i1", Name: "INC1"}}}
		s := judgeSuggestionCluster(mk(a, b), sgCriteria, incidents, nil)
		want := []string{
			"closed 1 time in 12 months (needs 3)",
			"success rate 50% (needs 90%)",
			"1 incident linked to a member",
			"1 member has high risk",
			"1 member has high impact",
		}
		if s.Eligible || !reflect.DeepEqual(s.Unmet, want) {
			t.Fatalf("unmet = %q, want %q", s.Unmet, want)
		}
		if len(s.Incidents) != 1 {
			t.Errorf("incidents = %v, want INC1 once", s.Incidents)
		}
	})
	t.Run("no clean member to start from", func(t *testing.T) {
		a, b := sgCand(1, "Deploy CApps", "CLOSED", sgPlan), sgCand(2, "Deploy CApps", "ROLLBACK", sgPlan)
		a.HasDeviation = true
		if s := judgeSuggestionCluster(mk(a, b), sgCriteria, nil, nil); s.ProposeFrom != nil || len(s.Evidence) != 1 {
			t.Errorf("proposeFrom = %+v, evidence = %v", s.ProposeFrom, s.Evidence)
		}
	})
	t.Run("evidence stops at 99 beside the source", func(t *testing.T) {
		var cands []repository.SuggestionCandidate
		for i := 1; i <= 150; i++ {
			cands = append(cands, sgCand(i, "Deploy CApps", "CLOSED", sgPlan))
		}
		s := judgeSuggestionCluster(mk(cands...), sgCriteria, nil, nil)
		if len(s.Evidence) != 99 || s.MemberCount != 150 || len(s.Members) != 100 || s.ClosedCount != 150 {
			t.Errorf("evidence %d, members %d listed of %d, closed %d", len(s.Evidence), len(s.Members), s.MemberCount, s.ClosedCount)
		}
	})
}

func TestMatchSuggestionTemplate(t *testing.T) {
	cl := clusterSuggestionCandidates([]repository.SuggestionCandidate{
		sgCand(1, "Deploy CApps to Pekin prod", "CLOSED", sgPlan),
		sgCand(2, "Deploy CApps to Pekin prod", "CLOSED", sgPlan),
	}, 0.5)[0]
	tpl := func(subject, plan string) repository.SuggestionTemplate {
		return repository.SuggestionTemplate{ID: "t1", Number: "STD0000001", Name: "Deploy CApps", State: "ACTIVE", AccountID: sgAccountA, Subject: subject, ImplementationPlan: plan}
	}
	for name, tc := range map[string]struct {
		tpls []repository.SuggestionTemplate
		want domain.StandardChangeSuggestionMatch
	}{
		"no template": {nil, domain.StandardChangeSuggestionNew},
		"covered":     {[]repository.SuggestionTemplate{tpl("Deploy CApps to {{env}}", sgPlan)}, domain.StandardChangeSuggestionCovered},
		"close":       {[]repository.SuggestionTemplate{tpl("Deploy CApps to {{env}}", "<p>Stop the gateway, copy the archive, start it</p>")}, domain.StandardChangeSuggestionClose},
		"far":         {[]repository.SuggestionTemplate{tpl("Rotate the TLS certificate", "<p>Generate a CSR, have it signed, import the keystore</p>")}, domain.StandardChangeSuggestionNew},
	} {
		t.Run(name, func(t *testing.T) {
			got, ref := matchSuggestionTemplate(cl, tc.tpls, sgCriteria)
			if got != tc.want {
				t.Fatalf("match = %s (template %+v), want %s", got, ref, tc.want)
			}
			if (ref == nil) != (got == domain.StandardChangeSuggestionNew) {
				t.Errorf("template = %+v for %s", ref, got)
			}
		})
	}
}

// fakeSuggestionRepo serves fixed data and records dismissals.
type fakeSuggestionRepo struct {
	cands      []repository.SuggestionCandidate
	since      time.Time
	account    string
	incidents  map[string][]domain.EntityRef
	proposals  map[string][]domain.EntityRef
	templates  []repository.SuggestionTemplate
	dismissals map[string]map[string]domain.StandardChangeSuggestionDismissal
	dismissed  []string
}

func (f *fakeSuggestionRepo) Candidates(_ context.Context, account string, since time.Time) ([]repository.SuggestionCandidate, error) {
	f.account, f.since = account, since
	return f.cands, nil
}
func (f *fakeSuggestionRepo) IncidentsOf(context.Context, []string) (map[string][]domain.EntityRef, error) {
	return f.incidents, nil
}
func (f *fakeSuggestionRepo) LiveTemplates(context.Context, []string) ([]repository.SuggestionTemplate, error) {
	return f.templates, nil
}
func (f *fakeSuggestionRepo) OpenProposalsOf(context.Context, []string) (map[string][]domain.EntityRef, error) {
	return f.proposals, nil
}
func (f *fakeSuggestionRepo) Dismissals(context.Context, []string) (map[string]map[string]domain.StandardChangeSuggestionDismissal, error) {
	return f.dismissals, nil
}
func (f *fakeSuggestionRepo) Dismiss(_ context.Context, account, key, reason, actor string) (domain.StandardChangeSuggestionDismissal, error) {
	f.dismissed = append(f.dismissed, account+"|"+key+"|"+reason+"|"+actor)
	return domain.StandardChangeSuggestionDismissal{ID: "d1", Reason: reason}, nil
}
func (f *fakeSuggestionRepo) Restore(context.Context, string) error { return nil }

func sgService(repo *fakeSuggestionRepo, internal bool) *standardChangeSuggestionService {
	svc := NewStandardChangeSuggestionService(repo, stubAccess{scope: AccessScope{Unrestricted: internal}}, domain.StandardChangeSuggestionCriteria{}).(*standardChangeSuggestionService)
	svc.now = func() time.Time { return sgNow }
	return svc
}

func sgPersonCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane@wso2.com"})
}

func TestStandardChangeSuggestionService_GetSuggestions(t *testing.T) {
	eligible := []repository.SuggestionCandidate{
		sgCand(1, "Deploy CApps", "CLOSED", sgPlan), sgCand(2, "Deploy CApps", "CLOSED", sgPlan), sgCand(3, "Deploy CApps", "CLOSED", sgPlan),
	}
	short := []repository.SuggestionCandidate{sgCand(4, "Restart gateway", "CLOSED", ""), sgCand(5, "Restart gateway", "CLOSED", "")}
	// The same subject at another customer never joins Acme's cluster.
	other := sgCand(6, "Deploy CApps", "CLOSED", sgPlan)
	other.AccountID, other.AccountName = sgAccountB, "Beta"
	repo := &fakeSuggestionRepo{cands: append(append(append([]repository.SuggestionCandidate{}, eligible...), short...), other)}

	t.Run("eligible only by default, window from the criteria", func(t *testing.T) {
		resp, err := sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Suggestions) != 1 || resp.Suggestions[0].Key != "deploy capps" || resp.Suggestions[0].MemberCount != 3 {
			t.Fatalf("suggestions = %+v", resp.Suggestions)
		}
		if resp.Scanned != (domain.StandardChangeSuggestionScan{ChangeRequests: 6, Clusters: 2, Eligible: 1}) {
			t.Errorf("scanned = %+v", resp.Scanned)
		}
		if !repo.since.Equal(sgNow.AddDate(-1, 0, 0)) || resp.Criteria != DefaultStandardChangeSuggestionCriteria {
			t.Errorf("since = %v, criteria = %+v", repo.since, resp.Criteria)
		}
	})
	t.Run("ineligible on request, after the eligible", func(t *testing.T) {
		resp, _ := sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
		if len(resp.Suggestions) != 2 || !resp.Suggestions[0].Eligible || resp.Suggestions[1].Eligible {
			t.Fatalf("suggestions = %+v", resp.Suggestions)
		}
	})
	t.Run("a dismissed suggestion is hidden, or shown with its dismissal", func(t *testing.T) {
		repo.dismissals = map[string]map[string]domain.StandardChangeSuggestionDismissal{sgAccountA: {"deploy capps": {ID: "d1", Reason: "one-off migration"}}}
		defer func() { repo.dismissals = nil }()
		resp, _ := sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if len(resp.Suggestions) != 0 || resp.Scanned.Dismissed != 1 {
			t.Fatalf("suggestions = %+v, scanned = %+v", resp.Suggestions, resp.Scanned)
		}
		resp, _ = sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeDismissed: true})
		if len(resp.Suggestions) != 1 || resp.Suggestions[0].Dismissal == nil || resp.Suggestions[0].Dismissal.Reason != "one-off migration" {
			t.Fatalf("suggestions = %+v", resp.Suggestions)
		}
	})
	t.Run("overrides apply and are held to their range", func(t *testing.T) {
		two, months := 2, 6
		resp, err := sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{MinClosed: &two, WindowMonths: &months})
		if err != nil || len(resp.Suggestions) != 2 || resp.Criteria.MinClosed != 2 || !repo.since.Equal(sgNow.AddDate(0, -6, 0)) {
			t.Fatalf("err = %v, suggestions = %d, criteria = %+v", err, len(resp.Suggestions), resp.Criteria)
		}
		bad := 101
		_, err = sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{MinSuccessPercent: &bad})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "minSuccessPercent") {
			t.Fatalf("err = %v, want a 400 naming minSuccessPercent", err)
		}
	})
	t.Run("accountId is passed down, and must be a UUID", func(t *testing.T) {
		if _, err := sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{AccountID: " " + strings.ToUpper(sgAccountA)}); err != nil || repo.account != sgAccountA {
			t.Fatalf("err = %v, account = %q", err, repo.account)
		}
		var ve *apierror.ValidationError
		if _, err := sgService(repo, true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{AccountID: "acme"}); !errors.As(err, &ve) {
			t.Fatalf("err = %v, want a 400", err)
		}
	})
	t.Run("staff only", func(t *testing.T) {
		var fe *apierror.ForbiddenError
		if _, err := sgService(repo, false).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{}); !errors.As(err, &fe) {
			t.Fatalf("err = %v, want a 403", err)
		}
	})
}

func TestStandardChangeSuggestionService_Dismiss(t *testing.T) {
	repo := &fakeSuggestionRepo{}
	svc := sgService(repo, true)
	if _, err := svc.DismissSuggestion(sgPersonCtx(), domain.DismissStandardChangeSuggestionRequest{AccountID: sgAccountA, Key: "deploy capps", Reason: "  one-off  "}); err != nil {
		t.Fatal(err)
	}
	if want := sgAccountA + "|deploy capps|one-off|jane@wso2.com"; len(repo.dismissed) != 1 || repo.dismissed[0] != want {
		t.Fatalf("dismissed = %v, want %q", repo.dismissed, want)
	}
	for name, req := range map[string]domain.DismissStandardChangeSuggestionRequest{
		"no account":  {Key: "deploy capps", Reason: "x"},
		"no key":      {AccountID: sgAccountA, Reason: "x"},
		"not a key":   {AccountID: sgAccountA, Key: "Deploy CApps", Reason: "x"},
		"no reason":   {AccountID: sgAccountA, Key: "deploy capps", Reason: "  "},
		"long reason": {AccountID: sgAccountA, Key: "deploy capps", Reason: strings.Repeat("é", 1001)},
	} {
		var ve *apierror.ValidationError
		if _, err := svc.DismissSuggestion(sgPersonCtx(), req); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a 400", name, err)
		}
	}
	var ue *apierror.UnauthorizedError
	if _, err := svc.DismissSuggestion(context.Background(), domain.DismissStandardChangeSuggestionRequest{AccountID: sgAccountA, Key: "deploy capps", Reason: "x"}); !errors.As(err, &ue) {
		t.Errorf("a machine caller: err = %v, want a 401", err)
	}
	var ve *apierror.ValidationError
	if err := svc.RestoreSuggestion(sgPersonCtx(), "d1"); !errors.As(err, &ve) {
		t.Errorf("restore of a non-UUID: err = %v, want a 400", err)
	}
}

func TestNewStandardChangeSuggestionService_Criteria(t *testing.T) {
	svc := NewStandardChangeSuggestionService(&fakeSuggestionRepo{}, stubAccess{}, domain.StandardChangeSuggestionCriteria{MinClosed: 5, MinSuccessPercent: 150, CoveredPercent: 40}).(*standardChangeSuggestionService)
	want := DefaultStandardChangeSuggestionCriteria
	want.MinClosed, want.MinSuccessPercent, want.CoveredPercent, want.ClosePercent = 5, 100, 40, 40
	if svc.criteria != want {
		t.Errorf("criteria = %+v, want %+v (defaults for zero, capped at 100, close no wider than covered)", svc.criteria, want)
	}
}
