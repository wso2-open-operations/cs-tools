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

package server

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type stubGithubSync struct{ service.GithubSyncService }

func TestNewGithubDeliveryService(t *testing.T) {
	// A pool that never connects: nothing here runs a query.
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/x?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	inner := &stubGithubSync{}
	on := &config.Config{GitInternalsEnabled: true, EngineeringEntityBaseURL: "http://eng.invalid"}
	off := &config.Config{}

	// Flag off: exactly what it was, nil included.
	if got := newGithubDeliveryService(off, pool, inner); got != service.GithubSyncService(inner) {
		t.Error("flag off must return the customer sync untouched")
	}
	if got := newGithubDeliveryService(off, pool, nil); got != nil {
		t.Error("flag off with no customer sync must stay nil")
	}
	// No database, or the ServiceNow data source: untouched too.
	if got := newGithubDeliveryService(on, nil, inner); got != service.GithubSyncService(inner) {
		t.Error("no database must return the customer sync untouched")
	}
	sn := &config.Config{GitInternalsEnabled: true, EngineeringEntityBaseURL: "http://eng.invalid", DataSource: config.DataSourceServiceNow}
	if got := newGithubDeliveryService(sn, pool, inner); got != service.GithubSyncService(inner) {
		t.Error("the ServiceNow data source must return the customer sync untouched")
	}
	// Flag on: wrapped, also when the customer sync is off (the route then exists for
	// product issues only).
	if got := newGithubDeliveryService(on, pool, inner); got == nil || got == service.GithubSyncService(inner) {
		t.Error("flag on must wrap the customer sync")
	}
	got := newGithubDeliveryService(on, pool, nil)
	if got == nil {
		t.Fatal("flag on with the customer sync off must still serve product issues")
	}
	if _, err := got.CreateServiceRequestFromIssue(context.Background(), domain.CreateServiceRequestFromIssueRequest{}); err == nil {
		t.Error("the customer-only calls must refuse while the customer sync is off")
	}
}

func TestNewGithubDeliveryService_WarnsWhileNoBotLoginIsSet(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/x?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	cfg := &config.Config{GitInternalsEnabled: true, EngineeringEntityBaseURL: "http://eng.invalid"}
	newGithubDeliveryService(cfg, pool, nil)
	if got := strings.Count(buf.String(), "GIT_INTERNALS_BOT_LOGINS"); got != 1 {
		t.Errorf("expected one warning with no bot login set, log: %q", buf.String())
	}
	buf.Reset()
	cfg.GitInternalsBotLogins = []string{"portal-bot"}
	newGithubDeliveryService(cfg, pool, nil)
	if buf.Len() != 0 {
		t.Errorf("no warning expected once a bot login is set, log: %q", buf.String())
	}
	buf.Reset()
	newGithubDeliveryService(&config.Config{}, pool, nil)
	if buf.Len() != 0 {
		t.Errorf("no warning expected with git internals off, log: %q", buf.String())
	}
}
