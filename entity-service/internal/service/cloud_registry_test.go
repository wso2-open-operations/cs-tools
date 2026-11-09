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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These tests pin the registry-driven behaviour: the seven seeded clouds
// resolve exactly as the old hardcoded maps did, unknown and retired clouds
// are rejected, and a cloud that exists only as registry rows is routed,
// validated and weighted with no code change.

const (
	acmeParent    = "00000000-0000-0000-0000-0000000000a1"
	acmeOfferingA = "00000000-0000-0000-0000-0000000000b1"
	acmeOfferingB = "00000000-0000-0000-0000-0000000000b2"
)

// acmeCloud is a cloud that exists nowhere in the code: one region, two
// weighted offerings, and a service-name prefix that is not its slug.
func acmeCloud() domain.CloudDefinition {
	return domain.CloudDefinition{
		Slug:           "acme-cloud",
		DisplayName:    "Acme Cloud",
		OfferingKey:    "ACME_CLOUD",
		IncidentPrefix: "acme",
		Plan: domain.CloudAvailabilityPlan{
			Regions: []domain.AvailabilityRegion{{Key: "us - dp", ParentID: acmeParent}},
			Weights: map[string]float64{acmeOfferingA: 60, acmeOfferingB: 40},
		},
	}
}

// recordingDashboardRepo notes which key and prefix the service resolved.
type recordingDashboardRepo struct {
	*fakeDashboardRepo
	monitorsFor   string
	incidentsFor  string
	parentFor     []string
	incidentSince time.Time
}

func (r *recordingDashboardRepo) Monitors(_ context.Context, key string) ([]repository.MonitorRow, error) {
	r.monitorsFor = key
	return r.monitors, nil
}

func (r *recordingDashboardRepo) Incidents(_ context.Context, prefix string, since time.Time) ([]repository.IncidentRow, error) {
	r.incidentsFor, r.incidentSince = prefix, since
	return nil, nil
}

func (r *recordingDashboardRepo) ParentAvailabilities(ctx context.Context, parentID string) ([]repository.ParentAvailabilityRow, error) {
	r.parentFor = append(r.parentFor, parentID)
	return r.fakeDashboardRepo.ParentAvailabilities(ctx, parentID)
}

// (a) every seeded cloud resolves to what the old maps said.
func TestRegistry_SeededCloudsResolveAsBefore(t *testing.T) {
	for slug, key := range seededCloudOfferings {
		repo := &recordingDashboardRepo{fakeDashboardRepo: &fakeDashboardRepo{}}
		svc := NewCloudStatusDashboardService(repo)

		if _, err := svc.Monitors(context.Background(), " "+strings.ToUpper(slug)+" "); err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if repo.monitorsFor != key {
			t.Errorf("%s: monitors queried by %q, want %q", slug, repo.monitorsFor, key)
		}
		if _, err := svc.Incidents(context.Background(), slug); err != nil {
			t.Fatalf("%s incidents: %v", slug, err)
		}
		if repo.incidentsFor != slug {
			t.Errorf("%s: incidents matched by prefix %q, want the slug", slug, repo.incidentsFor)
		}
	}
}

func TestRegistry_SeededPlansAreUnchanged(t *testing.T) {
	svc := NewCloudStatusDashboardService(&fakeDashboardRepo{}).(*cloudStatusDashboardService)
	for slug, want := range seededCloudPlans {
		def, err := svc.validateCloud(context.Background(), slug)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if !reflect.DeepEqual(def.Plan, want) {
			t.Errorf("%s: plan differs from the golden fixture", slug)
		}
		if def.Plan.Weighted() != want.Weighted() {
			t.Errorf("%s: weighted = %v, want %v", slug, def.Plan.Weighted(), want.Weighted())
		}
	}
}

// (b) unknown and inactive clouds are rejected, and the message lists the
// valid slugs in the old order.
func TestRegistry_UnknownCloudIsRejected(t *testing.T) {
	svc := NewCloudStatusDashboardService(&fakeDashboardRepo{})
	_, err := svc.Monitors(context.Background(), "nope")
	var verr *apierror.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("got %v, want a validation error", err)
	}
	const want = "cloud must be one of asgardeo, choreo, bijira, devant, moesif, choreo-eu, agent-manager"
	if verr.Msg != want {
		t.Errorf("message = %q, want %q", verr.Msg, want)
	}
}

// An inactive cloud never leaves the repository (it filters is_active), so the
// service sees it exactly as it sees an unknown one: absent.
func TestRegistry_InactiveCloudIsRejected(t *testing.T) {
	var active []domain.CloudDefinition
	for _, d := range seededDefinitions() {
		if d.Slug != "moesif" { // moesif retired
			active = append(active, d)
		}
	}
	repo := &fakeDashboardRepo{registry: &fixtureClouds{defs: active}}
	svc := NewCloudStatusDashboardService(repo)

	for name, call := range map[string]func() error{
		"monitors":       func() error { _, err := svc.Monitors(context.Background(), "moesif"); return err },
		"incidents":      func() error { _, err := svc.Incidents(context.Background(), "moesif"); return err },
		"availability":   func() error { _, err := svc.Availabilities(context.Background(), "moesif"); return err },
		"history":        func() error { _, err := svc.AvailabilityHistory(context.Background(), "moesif"); return err },
		"incidentDetail": func() error { _, err := svc.IncidentDetail(context.Background(), "x", "moesif"); return err },
	} {
		var verr *apierror.ValidationError
		if err := call(); !errors.As(err, &verr) {
			t.Errorf("%s: got %v, want a validation error for a retired cloud", name, err)
		} else if strings.Contains(verr.Msg, "moesif") {
			t.Errorf("%s: the message still lists the retired cloud: %q", name, verr.Msg)
		}
	}
}

// (c) a cloud added purely as registry rows is validated, routed and weighted.
func TestRegistry_NewCloudNeedsNoCode(t *testing.T) {
	reg := &fixtureClouds{defs: append(seededDefinitions(), acmeCloud())}
	repo := &recordingDashboardRepo{fakeDashboardRepo: &fakeDashboardRepo{
		registry: reg,
		parentAvail: map[string][]repository.ParentAvailabilityRow{
			acmeParent: {
				{ServiceOfferingID: acmeOfferingA, Window: "LAST_30_DAYS", Availability: 100},
				{ServiceOfferingID: acmeOfferingB, Window: "LAST_30_DAYS", Availability: 90},
				// Not in the weight table: skipped, never NaN.
				{ServiceOfferingID: "00000000-0000-0000-0000-0000000000b3", Window: "LAST_30_DAYS", Availability: 10},
			},
		},
	}}
	svc := NewCloudStatusDashboardService(repo)
	ctx := context.Background()

	if _, err := svc.Monitors(ctx, "acme-cloud"); err != nil {
		t.Fatalf("a registered cloud must validate: %v", err)
	}
	if repo.monitorsFor != "ACME_CLOUD" {
		t.Errorf("monitors queried by %q, want ACME_CLOUD", repo.monitorsFor)
	}
	if _, err := svc.Incidents(ctx, "ACME-cloud"); err != nil {
		t.Fatalf("incidents: %v", err)
	}
	if repo.incidentsFor != "acme" {
		t.Errorf("incidents matched by %q, want the registered prefix %q", repo.incidentsFor, "acme")
	}

	got, err := svc.Availabilities(ctx, "acme-cloud")
	if err != nil {
		t.Fatalf("availabilities: %v", err)
	}
	windows, ok := got["us - dp"]
	if !ok {
		t.Fatalf("region key missing from %v", got)
	}
	// 100 x 60% + 90 x 40% = 96, as the string form a weighted cloud emits.
	var thirty string
	for _, w := range windows {
		if w.Duration == "Last 30 days" {
			thirty = w.Availability.Value
		}
	}
	if thirty != "96.000" {
		t.Errorf("30-day figure = %q, want 96.000", thirty)
	}
	if !reflect.DeepEqual(repo.parentFor, []string{acmeParent}) {
		t.Errorf("queried parents %v, want only the registered one", repo.parentFor)
	}
}

// The webhook side routes the new cloud too, with no slug table to edit.
func TestRegistry_NewCloudIsRoutableForWebhooks(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		registry: &fixtureClouds{defs: append(seededDefinitions(), acmeCloud())},
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "a", Number: "OUT900", Cloud: "ACME_CLOUD", Event: domain.CloudStatusEventOutageBegin, Timestamp: "2026-09-28T10:00:00Z"},
		},
		pending: []domain.PendingCloudStatusWebhook{
			{ID: "w1", OutageID: "a", Cloud: "ACME_CLOUD", Event: domain.CloudStatusEventOutageBegin},
			{ID: "w2", OutageID: "b", Cloud: "RETIRED_CLOUD", Event: domain.CloudStatusEventOutageBegin},
		},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got.Recorded != 1 || got.SkippedNoCloud != 0 {
		t.Errorf("got %+v, want the new cloud recorded", got)
	}

	resp, err := svc.PendingWebhooks(context.Background())
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(resp.Webhooks) != 1 || resp.Webhooks[0].Cloud != "acme-cloud" {
		t.Errorf("pending = %+v, want one webhook addressed to acme-cloud", resp.Webhooks)
	}
}

// The cache serves a snapshot for its TTL, refreshes after it, and survives a
// failed refresh with the previous snapshot rather than failing the dashboard.
func TestCloudRegistry_Cache(t *testing.T) {
	ctx := context.Background()
	src := &fixtureClouds{defs: seededDefinitions()}
	reg := newCloudRegistry(src.Clouds)
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg.now = func() time.Time { return clock }

	if _, err := reg.Catalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Catalog(ctx); err != nil || src.calls != 1 {
		t.Fatalf("second read inside the TTL: err=%v calls=%d, want a cached snapshot", err, src.calls)
	}

	// A cloud inserted in the database appears once the TTL has passed.
	src.defs = append(src.defs, acmeCloud())
	clock = clock.Add(cloudRegistryTTL + time.Second)
	c, err := reg.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Lookup("acme-cloud"); !ok || src.calls != 2 {
		t.Errorf("after the TTL: found=%v calls=%d, want a refreshed snapshot", ok, src.calls)
	}

	// A failing refresh keeps the last good snapshot.
	src.err = errFixtureDB
	clock = clock.Add(cloudRegistryTTL + time.Second)
	c, err = reg.Catalog(ctx)
	if err != nil {
		t.Fatalf("a failed refresh must serve the previous snapshot, got %v", err)
	}
	if _, ok := c.Lookup("choreo"); !ok {
		t.Error("previous snapshot lost")
	}

	// With nothing cached the error surfaces.
	if _, err := newCloudRegistry(src.Clouds).Catalog(ctx); !errors.Is(err, errFixtureDB) {
		t.Errorf("got %v, want the load error", err)
	}
}
