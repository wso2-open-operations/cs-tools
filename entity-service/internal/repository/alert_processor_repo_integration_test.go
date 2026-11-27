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
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// storeAlerts reserves ids and stores one single-alert message per row the change builds.
func storeAlerts(t *testing.T, repo AlertRepository, n int, change func(i int, row *domain.NewAlert)) []string {
	t.Helper()
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		row := integrationAlert(id, "it-alert-proc-"+id, 0)
		row.RawBody = []byte("{}")
		change(i, &row)
		if _, _, err := repo.InsertAlerts(ctx, []domain.NewAlert{row}); err != nil {
			t.Fatalf("InsertAlerts %s: %v", id, err)
		}
	}
	return ids
}

func TestAlertGroupsIntegration_ReadsTheGroupFromAlerts(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ctx := WithSystemIdentity(context.Background())
	// Through Scoped, because incident is FORCE ROW LEVEL SECURITY and a raw pool query sees no rows.
	var incidentID string
	err := NewScoped(pool).QueryRow(ctx, `SELECT id::text FROM incident LIMIT 1`).Scan(&incidentID)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("no incident to link to")
	}
	if err != nil {
		t.Fatalf("find an incident: %v", err)
	}
	seen := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	// One fingerprint: the same five fields on every alert.
	key := model.Alert{Source: "it", Service: "it-group-" + time.Now().Format("150405.000000"), MetricName: "cpu", Environment: "prod", UniqueIdentifier: "it-group"}
	ids := storeAlerts(t, NewAlertRepository(NewScoped(pool)), 3, func(i int, row *domain.NewAlert) {
		row.Alert, row.ReceivedOn = key, seen.Add(time.Duration(i)*time.Minute)
	})
	groups := NewAlertGroups(NewScoped(pool))

	err = groups.InGroup(ctx, key, func(tx alertprocessor.GroupTx) error {
		if g, err := tx.Group(ctx); err != nil || g != nil {
			t.Fatalf("Group = %+v, %v; want none before any alert is linked", g, err)
		}
		if err := tx.LinkAlert(ctx, ids[0], incidentID); err != nil {
			return err
		}
		return tx.LinkAlert(ctx, ids[1], incidentID)
	})
	if err != nil {
		t.Fatalf("InGroup: %v", err)
	}

	err = groups.InGroup(ctx, key, func(tx alertprocessor.GroupTx) error {
		g, err := tx.Group(ctx)
		if err != nil || g == nil || g.IncidentID != incidentID || g.AlertCount != 2 || !g.FirstSeenOn.Equal(seen) || !g.LastSeenOn.Equal(seen.Add(time.Minute)) {
			t.Fatalf("Group = %+v, %v; want the incident with its two alerts", g, err)
		}
		unlinked, err := tx.Unlinked(ctx, seen, seen.Add(time.Hour), ids[0])
		if err != nil || len(unlinked) != 1 || unlinked[0].AlertID != ids[2] || unlinked[0].Alert.MetricName != "cpu" {
			t.Fatalf("Unlinked = %+v, %v; want only the third alert, with its alert", unlinked, err)
		}
		linked, err := tx.AlertLinked(ctx, ids[0])
		if err != nil || !linked {
			t.Fatalf("AlertLinked = %v, %v; want true", linked, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InGroup: %v", err)
	}
}
