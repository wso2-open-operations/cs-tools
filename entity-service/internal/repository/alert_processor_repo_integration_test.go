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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// drainIDs drains the outbox once, publishing everything, and returns the alert ids it was handed.
func drainIDs(t *testing.T, outbox *AlertOutbox) []string {
	t.Helper()
	var ids []string
	_, err := outbox.Drain(context.Background(), 100, func(batch []alertprocessor.Event) []string {
		for _, e := range batch {
			ids = append(ids, e.AlertID)
		}
		return ids
	})
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	return ids
}

func TestAlertOutboxIntegration_QueuesStoredAlertsOnce(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ctx := WithSystemIdentity(context.Background())
	repo := NewAlertRepository(NewScoped(pool), true)
	outbox := NewAlertOutbox(NewScoped(pool))
	ids, err := repo.ReserveAlertIDs(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	errorRow := integrationAlert(ids[2], "it-alert-outbox-2", 0)
	errorRow.Alert, errorRow.ErrorReason, errorRow.Error, errorRow.RawBody = nil, "UnknownSource", "no transform", []byte("x")
	message := []domain.NewAlert{integrationAlert(ids[0], "it-alert-outbox-1", 0), integrationAlert(ids[1], "it-alert-outbox-1", 1)}
	message[0].RawBody = []byte("{}")
	for _, rows := range [][]domain.NewAlert{message, message, {errorRow}} {
		if _, err := repo.InsertAlerts(ctx, rows); err != nil {
			t.Fatalf("InsertAlerts: %v", err)
		}
	}

	got := drainIDs(t, outbox)
	if !slices.Equal(got, ids[:2]) {
		t.Fatalf("drained %v, want the two alert rows once each and never the error row", got)
	}
	if again := drainIDs(t, outbox); len(again) != 0 {
		t.Fatalf("second drain handed %v, want nothing once published", again)
	}
}

func TestAlertGroupsIntegration_KeepsGroupAndLinks(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ctx := WithSystemIdentity(context.Background())
	repo := NewAlertRepository(NewScoped(pool), false)
	groups := NewAlertGroups(NewScoped(pool))
	ids, err := repo.ReserveAlertIDs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	row := integrationAlert(ids[0], "it-alert-group-1", 0)
	row.RawBody = []byte("{}")
	if _, err := repo.InsertAlerts(ctx, []domain.NewAlert{row}); err != nil {
		t.Fatal(err)
	}
	var incidentID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM incident LIMIT 1`).Scan(&incidentID); err != nil {
		t.Skipf("no incident to link to: %v", err)
	}
	fingerprint := "it-fingerprint-" + ids[0]
	t.Cleanup(func() {
		_, _ = NewScoped(pool).Exec(ctx, `DELETE FROM alert_group WHERE fingerprint = $1`, fingerprint)
	})
	seen := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

	err = groups.InGroup(ctx, fingerprint, func(tx alertprocessor.GroupTx) error {
		if g, err := tx.Group(ctx); err != nil || g != nil {
			t.Fatalf("Group = %+v, %v; want none yet", g, err)
		}
		if linked, err := tx.AlertLinked(ctx, ids[0]); err != nil || linked {
			t.Fatalf("AlertLinked = %v, %v; want false", linked, err)
		}
		if err := tx.SaveGroup(ctx, alertprocessor.Group{IncidentID: incidentID, FirstSeenOn: seen, LastSeenOn: seen, AlertCount: 1}); err != nil {
			return err
		}
		return tx.LinkAlert(ctx, ids[0], incidentID)
	})
	if err != nil {
		t.Fatalf("InGroup: %v", err)
	}

	err = groups.InGroup(ctx, fingerprint, func(tx alertprocessor.GroupTx) error {
		g, err := tx.Group(ctx)
		if err != nil || g == nil || g.IncidentID != incidentID || g.AlertCount != 1 || !g.FirstSeenOn.Equal(seen) {
			t.Fatalf("Group = %+v, %v; want the saved group", g, err)
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
