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

package repository

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestSreAlertRepositoryIntegration_SearchesAndGets(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ctx := WithSystemIdentity(context.Background())
	store := NewAlertRepository(NewScoped(pool))
	numbers, err := store.ReserveAlertIDs(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	seen := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	for i, number := range numbers {
		row := integrationAlert(number, "it-alert-sre-svc")
		row.RawBody = []byte(`{"n":` + string(rune('0'+i)) + `}`)
		row.ReceivedOn = seen.Add(time.Duration(i) * time.Minute)
		row.Alert.Service = "it-alert-sre-svc"
		row.Alert.Environment = []string{"prod", "staging", "prod"}[i]
		row.Alert.Description = "disk 100% full"
		if _, err := store.InsertAlerts(ctx, []domain.NewAlert{row}); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSreAlertRepository(NewScoped(pool))

	items, total, err := repo.Search(ctx, domain.SreAlertSearchFilters{Services: []string{"it-alert-sre-svc"}, Environments: []string{"prod"}},
		domain.SreAlertSort{Field: "receivedOn", Order: "desc"}, 0, 10)
	if err != nil || total != 2 || len(items) != 2 || items[0].Number != numbers[2] || items[1].Number != numbers[0] || items[0].Incident != nil || len(items[0].ID) != 36 {
		t.Fatalf("Search = %+v total %d, %v; want the two prod alerts newest first with UUIDs and no incident", items, total, err)
	}
	// % in the search matches itself, not anything.
	if _, total, err := repo.Search(ctx, domain.SreAlertSearchFilters{SearchQuery: "100%", Services: []string{"it-alert-sre-svc"}},
		domain.SreAlertSort{}, 0, 10); err != nil || total != 3 {
		t.Fatalf("search 100%% matched %d, %v; want 3", total, err)
	}
	if _, total, err := repo.Search(ctx, domain.SreAlertSearchFilters{SearchQuery: "10_%", Services: []string{"it-alert-sre-svc"}},
		domain.SreAlertSort{}, 0, 10); err != nil || total != 0 {
		t.Fatalf("search 10_%% matched %d, %v; want 0, since _ is literal", total, err)
	}

	// The linked incident comes back as a record, and filters by its id.
	var incidentID string
	if err := NewScoped(pool).QueryRow(ctx, `SELECT i.id::text FROM incident i JOIN work_item w ON w.id = i.id LIMIT 1`).Scan(&incidentID); err == nil {
		if _, err := NewScoped(pool).Exec(ctx, `UPDATE alert SET incident_id = $2 WHERE number = $1`, numbers[0], incidentID); err != nil {
			t.Fatal(err)
		}
		linked, total, err := repo.Search(ctx, domain.SreAlertSearchFilters{IncidentID: incidentID}, domain.SreAlertSort{}, 0, 10)
		if err != nil || total < 1 || linked[0].Incident == nil || linked[0].Incident.ID != incidentID || linked[0].Incident.Number == "" || linked[0].Incident.State == "" {
			t.Fatalf("Search(incidentId) = %+v total %d, %v; want the linked alert with its incident's number and state", linked, total, err)
		}
	}

	byNumber, err := repo.Get(ctx, numbers[1])
	if err != nil || byNumber == nil || byNumber.Environment != "staging" || byNumber.RawBody == nil || *byNumber.RawBody != `{"n":1}` {
		t.Fatalf("Get(number) = %+v, %v; want the staging alert with its raw body", byNumber, err)
	}
	if byNumber.DefaultedFields == nil || len(byNumber.DefaultedFields) != 0 {
		t.Fatalf("Get(number) defaulted fields %v, want an empty list for an alert that defaulted nothing", byNumber.DefaultedFields)
	}
	if _, err := NewScoped(pool).Exec(ctx, `UPDATE alert SET defaulted_fields = '[{"field":"severity","value":"Critical"}]' WHERE number = $1`, numbers[0]); err != nil {
		t.Fatal(err)
	}
	if d, err := repo.Get(ctx, numbers[0]); err != nil || len(d.DefaultedFields) != 1 || d.DefaultedFields[0] != (domain.SreAlertDefaultedField{Field: "severity", Value: "Critical"}) {
		t.Fatalf("Get defaulted fields = %+v, %v; want severity defaulted to Critical", d, err)
	}
	// A field the source sent is a value; one it left empty is nil (null in JSON), not "".
	if byNumber.Severity == nil || *byNumber.Severity != "critical" || byNumber.Description == nil || *byNumber.Description != "disk 100% full" ||
		byNumber.Category != nil || byNumber.AssignmentGroup != nil {
		t.Fatalf("Get(number) severity %v category %v description %v assignment group %v; want critical, nil, the description, nil",
			byNumber.Severity, byNumber.Category, byNumber.Description, byNumber.AssignmentGroup)
	}
	byID, err := repo.Get(ctx, byNumber.ID)
	if err != nil || byID == nil || byID.Number != numbers[1] {
		t.Fatalf("Get(uuid) = %+v, %v; want the same alert", byID, err)
	}
	if missing, err := repo.Get(ctx, "not-a-uuid"); err != nil || missing != nil {
		t.Fatalf("Get(missing) = %+v, %v; want nil", missing, err)
	}

	facets, err := repo.Facets(ctx, seen.Add(-time.Hour), 200)
	if err != nil || !slices.Contains(facets.Environments, "staging") || !slices.Contains(facets.Services, "it-alert-sre-svc") {
		t.Fatalf("Facets = %+v, %v; want the stored environments and services", facets, err)
	}
}
