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
	"sort"
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

// mustCluster clusters with no deadline, failing the test on an error.
func mustCluster(t *testing.T, cands []repository.SuggestionCandidate, threshold float64) []suggestionCluster {
	t.Helper()
	cls, err := clusterSuggestionCandidates(context.Background(), cands, threshold)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	return cls
}

func TestClusterSuggestionCandidates(t *testing.T) {
	t.Run("same subject and similar plans make one cluster, newest first", func(t *testing.T) {
		got := clusterKeys(mustCluster(t, []repository.SuggestionCandidate{
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
		got := clusterKeys(mustCluster(t, []repository.SuggestionCandidate{
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
		cls := mustCluster(t, []repository.SuggestionCandidate{
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
		got := clusterKeys(mustCluster(t, []repository.SuggestionCandidate{
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
		cls := mustCluster(t, []repository.SuggestionCandidate{clone, src, other}, 0.5)
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
		cls := mustCluster(t, cands, 0.5)
		if len(cls) != 1 {
			t.Fatalf("want one cluster, got %v", clusterKeys(cls))
		}
		return cls[0]
	}
	t.Run("eligible: counts, source and evidence", func(t *testing.T) {
		a := sgCand(1, "Deploy CApps", "CLOSED", sgPlan)
		a.HasDeviation = true // the newest deviated: not the source, still evidence
		b, c, d := sgCand(2, "Deploy CApps", "CLOSED", sgPlan), sgCand(3, "Deploy CApps", "CLOSED", sgPlan), sgCand(4, "Deploy CApps", "CLOSED", sgPlan)
		e := sgCand(5, "Deploy CApps", "CLOSED", sgPlan) // five closed: the default minimum
		b.Risk, c.Risk = sp("LOW"), sp("MODERATE")
		d.Impact = sp("MEDIUM")
		s := judgeSuggestionCluster(mk(a, b, c, d, e), sgCriteria, nil, nil)
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
		if strings.Join(ev, ",") != "CHG0000001,CHG0000003,CHG0000004,CHG0000005" {
			t.Errorf("evidence = %v", ev)
		}
		if s.Risk != (domain.StandardChangeSuggestionBreakdown{Low: 1, Moderate: 1, Unknown: 3}) {
			t.Errorf("risk = %+v (unknown is shown, not refused)", s.Risk)
		}
		if s.Impact.Moderate != 1 || s.Subject != "Deploy CApps" || s.SuccessRate != 1 {
			t.Errorf("impact = %+v, subject = %q, success = %v", s.Impact, s.Subject, s.SuccessRate)
		}
		if !s.LastCompletedOn.Equal(a.CompletedOn) || !s.FirstCompletedOn.Equal(e.CompletedOn) {
			t.Errorf("completed between %v and %v", s.FirstCompletedOn, s.LastCompletedOn)
		}
	})
	t.Run("each criterion it misses is named", func(t *testing.T) {
		a, b := sgCand(1, "Deploy CApps", "CLOSED", sgPlan), sgCand(2, "Deploy CApps", "ROLLBACK", sgPlan)
		a.Risk, b.Impact = sp("HIGH"), sp("HIGH")
		incidents := map[string][]domain.EntityRef{a.ID: {{ID: "i1", Name: "INC1"}}, b.ID: {{ID: "i1", Name: "INC1"}}}
		s := judgeSuggestionCluster(mk(a, b), sgCriteria, incidents, nil)
		want := []string{
			"closed 1 time in 12 months (needs 5)",
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

// fakeSuggestionRepo serves fixed candidates and keeps runs, rows and
// dismissals in memory, as the real one does in the database.
type fakeSuggestionRepo struct {
	cands       []repository.SuggestionCandidate
	sinces      []time.Time
	incidents   map[string][]domain.EntityRef
	proposals   map[string][]domain.EntityRef
	dismissals  map[string]map[string]domain.StandardChangeSuggestionDismissal
	dismissed   []string
	runs        map[string]*repository.SuggestionRun
	rows        map[string][]repository.SuggestionSnapshotRow            // run id -> rows
	runCustomer map[string]map[string]int                                // run id -> account -> change requests
	listed      map[string][]string                                      // run id -> the customers listed at its start
	skipped     map[string]map[string]string                             // run id -> account -> reason
	carried     map[string]map[string][]repository.SuggestionSnapshotRow // run id -> account -> keys a skip carried
	beforeRows  func()                                                   // called before SnapshotRows reads (once set)
	nextRun     int
	loaded      []string        // accounts whose candidates were read, in order
	slow        map[string]bool // accounts whose read waits until its context ends
}

func (f *fakeSuggestionRepo) NextCustomers(_ context.Context, runID, after string, limit int) ([]string, error) {
	var out []string
	for _, a := range f.listed[runID] {
		if a > after && len(out) < limit {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeSuggestionRepo) Candidates(ctx context.Context, account string, since time.Time) ([]repository.SuggestionCandidate, error) {
	f.loaded = append(f.loaded, account)
	if f.slow[account] {
		<-ctx.Done()
		return nil, fmt.Errorf("read candidates: %w", ctx.Err())
	}
	var out []repository.SuggestionCandidate
	for _, c := range f.cands {
		if c.AccountID == account && !c.CompletedOn.Before(since) {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeSuggestionRepo) IncidentsOf(context.Context, []string) (map[string][]domain.EntityRef, error) {
	return f.incidents, nil
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
func (f *fakeSuggestionRepo) StartRun(_ context.Context, crit domain.StandardChangeSuggestionCriteria, windowStart func(time.Time) time.Time) (repository.SuggestionRun, error) {
	if f.runs == nil {
		f.runs, f.rows, f.runCustomer = map[string]*repository.SuggestionRun{}, map[string][]repository.SuggestionSnapshotRow{}, map[string]map[string]int{}
		f.listed, f.skipped, f.carried = map[string][]string{}, map[string]map[string]string{}, map[string]map[string][]repository.SuggestionSnapshotRow{}
	}
	f.nextRun++
	run := repository.SuggestionRun{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", f.nextRun), StartedOn: sgNow, Criteria: crit}
	f.runs[run.ID] = &run
	f.runCustomer[run.ID], f.skipped[run.ID], f.carried[run.ID] = map[string]int{}, map[string]string{}, map[string][]repository.SuggestionSnapshotRow{}
	since := windowStart(run.StartedOn)
	f.sinces = append(f.sinces, since)
	set := map[string]bool{}
	for _, c := range f.cands {
		if !c.CompletedOn.Before(since) {
			set[c.AccountID] = true
		}
	}
	for a := range set {
		f.listed[run.ID] = append(f.listed[run.ID], a)
	}
	sort.Strings(f.listed[run.ID])
	return run, nil
}
func (f *fakeSuggestionRepo) GetRun(_ context.Context, id string) (*repository.SuggestionRun, error) {
	if r, ok := f.runs[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeSuggestionRepo) open(id string) error {
	if r, ok := f.runs[id]; !ok || r.CompletedOn != nil {
		return &apierror.ConflictError{Msg: "gone"}
	}
	return nil
}
func (f *fakeSuggestionRepo) SaveCustomer(_ context.Context, runID, account string, crs int, rows []repository.SuggestionSnapshotRow) error {
	if err := f.open(runID); err != nil {
		return err
	}
	kept := f.rows[runID][:0:0]
	for _, r := range f.rows[runID] {
		if r.AccountID != account {
			kept = append(kept, r)
		}
	}
	f.rows[runID] = append(kept, rows...)
	f.runCustomer[runID][account] = crs
	delete(f.skipped[runID], account)
	delete(f.carried[runID], account)
	return nil
}
func (f *fakeSuggestionRepo) SkipCustomer(_ context.Context, runID, account, reason string) error {
	if err := f.open(runID); err != nil {
		return err
	}
	delete(f.runCustomer[runID], account)
	f.skipped[runID][account] = reason
	// The keys of the latest published run come along (its rows, or its own carried ones).
	if latest, _ := f.LatestRun(context.Background()); latest != nil {
		keys, _ := f.SnapshotKeys(context.Background(), latest.ID, account)
		f.carried[runID][account] = keys
	}
	return nil
}
func (f *fakeSuggestionRepo) SkippedCustomers(_ context.Context, runID, account string) ([]domain.StandardChangeSuggestionSkipped, error) {
	out := []domain.StandardChangeSuggestionSkipped{}
	for a, reason := range f.skipped[runID] {
		if account == "" || a == account {
			out = append(out, domain.StandardChangeSuggestionSkipped{Customer: domain.EntityRef{ID: a}, Reason: reason})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Customer.ID < out[j].Customer.ID })
	return out, nil
}
func (f *fakeSuggestionRepo) PublishRun(_ context.Context, id string) (repository.SuggestionRun, error) {
	if err := f.open(id); err != nil {
		return repository.SuggestionRun{}, err
	}
	run := f.runs[id]
	done := sgNow.Add(time.Minute)
	run.CompletedOn = &done
	run.Counts = domain.StandardChangeSuggestionScan{Customers: len(f.runCustomer[id]), Clusters: len(f.rows[id]), Skipped: len(f.skipped[id])}
	for _, n := range f.runCustomer[id] {
		run.Counts.ChangeRequests += n
	}
	for _, r := range f.rows[id] {
		if r.Eligible {
			run.Counts.Eligible++
		}
	}
	for other := range f.runs {
		if other != id {
			delete(f.runs, other)
			delete(f.rows, other)
			delete(f.carried, other)
		}
	}
	return *run, nil
}
func (f *fakeSuggestionRepo) LatestRun(context.Context) (*repository.SuggestionRun, error) {
	for _, r := range f.runs {
		if r.CompletedOn != nil {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}
func (f *fakeSuggestionRepo) CustomerCounts(_ context.Context, runID, account string) (domain.StandardChangeSuggestionScan, error) {
	var out domain.StandardChangeSuggestionScan
	if n, ok := f.runCustomer[runID][account]; ok {
		out.Customers, out.ChangeRequests = 1, n
	}
	if _, ok := f.skipped[runID][account]; ok {
		out.Skipped = 1
	}
	for _, r := range f.rows[runID] {
		if r.AccountID == account {
			out.Clusters++
			if r.Eligible {
				out.Eligible++
			}
		}
	}
	return out, nil
}
func (f *fakeSuggestionRepo) SnapshotKeys(_ context.Context, runID, account string) ([]repository.SuggestionSnapshotRow, error) {
	var out []repository.SuggestionSnapshotRow
	for _, r := range f.rows[runID] {
		if r.AccountID == account {
			out = append(out, repository.SuggestionSnapshotRow{AccountID: account, Key: r.Key, MemberIDs: r.MemberIDs})
		}
	}
	return append(out, f.carried[runID][account]...), nil
}
func (f *fakeSuggestionRepo) SnapshotRows(_ context.Context, runID, account string, includeIneligible bool) ([]repository.SuggestionSnapshotRow, error) {
	if f.beforeRows != nil {
		f.beforeRows()
	}
	var out []repository.SuggestionSnapshotRow
	for _, r := range f.rows[runID] {
		if (account == "" || r.AccountID == account) && (includeIneligible || r.Eligible) {
			out = append(out, r)
		}
	}
	return out, nil
}

func sgService(repo *fakeSuggestionRepo, internal bool) *standardChangeSuggestionService {
	svc := NewStandardChangeSuggestionService(repo, stubAccess{scope: AccessScope{Unrestricted: internal}}, domain.StandardChangeSuggestionCriteria{}).(*standardChangeSuggestionService)
	svc.now = func() time.Time { return sgNow }
	return svc
}

func sgPersonCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane@wso2.com"})
}

// refreshAll drives a whole refresh the way the scheduled task does: page after
// page while more, then the published totals.
func refreshAll(t *testing.T, svc *standardChangeSuggestionService, limit int) (pages int, last domain.RefreshStandardChangeSuggestionsResponse) {
	t.Helper()
	req := domain.RefreshStandardChangeSuggestionsRequest{Limit: limit}
	for {
		resp, err := svc.Refresh(context.Background(), req)
		if err != nil {
			t.Fatalf("refresh page %d: %v", pages+1, err)
		}
		pages++
		if !resp.More {
			return pages, resp
		}
		req = domain.RefreshStandardChangeSuggestionsRequest{RunID: resp.RunID, Cursor: resp.NextCursor, Limit: limit}
	}
}

func sgFixture() *fakeSuggestionRepo {
	eligible := []repository.SuggestionCandidate{
		sgCand(1, "Deploy CApps", "CLOSED", sgPlan), sgCand(2, "Deploy CApps", "CLOSED", sgPlan), sgCand(3, "Deploy CApps", "CLOSED", sgPlan),
		sgCand(4, "Deploy CApps", "CLOSED", sgPlan), sgCand(5, "Deploy CApps", "CLOSED", sgPlan),
	}
	short := []repository.SuggestionCandidate{sgCand(11, "Restart gateway", "CLOSED", ""), sgCand(12, "Restart gateway", "CLOSED", "")}
	// The same subject at another customer never joins Acme's cluster.
	other := []repository.SuggestionCandidate{sgCand(13, "Deploy CApps", "CLOSED", sgPlan), sgCand(14, "Deploy CApps", "CLOSED", sgPlan)}
	for i := range other {
		other[i].AccountID, other[i].AccountName = sgAccountB, "Beta"
	}
	// Outside the 12-month window: never read.
	old := sgCand(15, "Deploy CApps", "CLOSED", sgPlan)
	old.CompletedOn = sgNow.AddDate(-2, 0, 0)
	return &fakeSuggestionRepo{cands: append(append(append(append([]repository.SuggestionCandidate{}, eligible...), short...), other...), old)}
}

func TestStandardChangeSuggestionService_Refresh(t *testing.T) {
	t.Run("one customer at a time, published by the last page", func(t *testing.T) {
		repo := sgFixture()
		svc := sgService(repo, true)
		pages, last := refreshAll(t, svc, 1)
		if pages != 2 || !reflect.DeepEqual(repo.loaded, []string{sgAccountA, sgAccountB}) {
			t.Fatalf("pages = %d, customers read = %v; want 2, A then B", pages, repo.loaded)
		}
		if last.ComputedOn == nil || last.Totals == nil ||
			*last.Totals != (domain.StandardChangeSuggestionScan{Customers: 2, ChangeRequests: 9, Clusters: 3, Eligible: 1}) {
			t.Fatalf("last page = %+v, totals %+v", last, last.Totals)
		}
		if !repo.sinces[0].Equal(sgNow.AddDate(-1, 0, 0)) {
			t.Errorf("window from %v, want 12 months before the run started", repo.sinces[0])
		}
	})
	t.Run("a page stops once its time is spent, after one customer", func(t *testing.T) {
		repo := sgFixture()
		svc := sgService(repo, true)
		clock := sgNow
		svc.now = func() time.Time { clock = clock.Add(suggestionRefreshBudget + time.Second); return clock }
		resp, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err != nil || resp.Customers != 1 || !resp.More || resp.NextCursor != sgAccountA {
			t.Fatalf("resp = %+v, %v; want one customer and more after A", resp, err)
		}
	})
	t.Run("an unpublished run is not read, and a page sent again counts its customer once", func(t *testing.T) {
		repo := sgFixture()
		svc := sgService(repo, true)
		first, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{Limit: 1})
		if err != nil || !first.More {
			t.Fatal(first, err)
		}
		if resp, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{}); resp.ComputedOn != nil || len(resp.Suggestions) != 0 {
			t.Fatalf("a half-done run was read: %+v", resp)
		}
		// The first page again (the scheduler retried it), then the rest.
		if _, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{RunID: first.RunID, Limit: 1}); err != nil {
			t.Fatal(err)
		}
		last, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{RunID: first.RunID, Cursor: first.NextCursor, Limit: 1})
		if err != nil || last.More || last.Totals.Customers != 2 || last.Totals.ChangeRequests != 9 {
			t.Fatalf("last = %+v, %v", last, err)
		}
		// A published run takes no more pages.
		_, err = svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{RunID: first.RunID, Cursor: first.NextCursor})
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) {
			t.Fatalf("a page of a published run: %v, want a 409", err)
		}
	})
	t.Run("refuses a cursor without its run, a bad limit, and anyone but staff or a service", func(t *testing.T) {
		svc := sgService(sgFixture(), true)
		var ve *apierror.ValidationError
		if _, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{Cursor: sgAccountA}); !errors.As(err, &ve) {
			t.Errorf("cursor without runId: %v", err)
		}
		if _, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{Limit: 501}); !errors.As(err, &ve) {
			t.Errorf("limit 501: %v", err)
		}
		var fe *apierror.ForbiddenError
		if _, err := sgService(sgFixture(), false).Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{}); !errors.As(err, &fe) {
			t.Errorf("an external caller: %v, want a 403", err)
		}
	})
}

func TestStandardChangeSuggestionService_GetSuggestions(t *testing.T) {
	t.Run("nothing computed yet is an empty list with no computedOn", func(t *testing.T) {
		resp, err := sgService(sgFixture(), true).GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if err != nil || resp.ComputedOn != nil || resp.Suggestions == nil || len(resp.Suggestions) != 0 || resp.Criteria != DefaultStandardChangeSuggestionCriteria {
			t.Fatalf("resp = %+v, %v", resp, err)
		}
	})
	repo := sgFixture()
	svc := sgService(repo, true)
	refreshAll(t, svc, 0)
	repo.loaded = nil

	t.Run("reads the snapshot only: eligible by default, computedOn, the run's counts", func(t *testing.T) {
		resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Suggestions) != 1 || resp.Suggestions[0].Key != "deploy capps" || resp.Suggestions[0].MemberCount != 5 {
			t.Fatalf("suggestions = %+v", resp.Suggestions)
		}
		if resp.ComputedOn == nil || resp.Scanned != (domain.StandardChangeSuggestionScan{Customers: 2, ChangeRequests: 9, Clusters: 3, Eligible: 1}) {
			t.Errorf("computedOn = %v, scanned = %+v", resp.ComputedOn, resp.Scanned)
		}
		if len(repo.loaded) != 0 {
			t.Errorf("a read loaded candidates of %v: it must never compute", repo.loaded)
		}
	})
	t.Run("ineligible on request, after the eligible", func(t *testing.T) {
		resp, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
		if len(resp.Suggestions) != 3 || !resp.Suggestions[0].Eligible || resp.Suggestions[1].Eligible {
			t.Fatalf("suggestions = %+v", resp.Suggestions)
		}
	})
	t.Run("one customer, with that customer's counts", func(t *testing.T) {
		resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{AccountID: " " + strings.ToUpper(sgAccountB), IncludeIneligible: true})
		if err != nil || len(resp.Suggestions) != 1 || resp.Suggestions[0].Customer.ID != sgAccountB {
			t.Fatalf("customer B: %+v, %v", resp.Suggestions, err)
		}
		if resp.Scanned != (domain.StandardChangeSuggestionScan{Customers: 1, ChangeRequests: 2, Clusters: 1}) {
			t.Errorf("scanned = %+v", resp.Scanned)
		}
		var ve *apierror.ValidationError
		if _, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{AccountID: "acme"}); !errors.As(err, &ve) {
			t.Fatalf("err = %v, want a 400", err)
		}
	})
	t.Run("open proposals and dismissals are read live", func(t *testing.T) {
		member := sgCand(1, "", "", "").ID
		repo.proposals = map[string][]domain.EntityRef{member: {{ID: "p1", Name: "SCP0000001"}}}
		repo.dismissals = map[string]map[string]domain.StandardChangeSuggestionDismissal{sgAccountA: {"deploy capps": {ID: "d1", Reason: "one-off migration"}}}
		defer func() { repo.proposals, repo.dismissals = nil, nil }()
		resp, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if len(resp.Suggestions) != 0 || resp.Scanned.Dismissed != 1 {
			t.Fatalf("suggestions = %+v, scanned = %+v", resp.Suggestions, resp.Scanned)
		}
		resp, _ = svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeDismissed: true})
		got := resp.Suggestions[0]
		if got.Dismissal == nil || got.Dismissal.Reason != "one-off migration" || len(got.OpenProposals) != 1 || got.OpenProposals[0].Name != "SCP0000001" {
			t.Fatalf("suggestion = %+v", got)
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
	svc := NewStandardChangeSuggestionService(&fakeSuggestionRepo{}, stubAccess{}, domain.StandardChangeSuggestionCriteria{MinClosed: 7, MinSuccessPercent: 150}).(*standardChangeSuggestionService)
	want := DefaultStandardChangeSuggestionCriteria
	want.MinClosed, want.MinSuccessPercent = 7, 100
	if svc.criteria != want {
		t.Errorf("criteria = %+v, want %+v (defaults for zero, capped at 100)", svc.criteria, want)
	}
}
