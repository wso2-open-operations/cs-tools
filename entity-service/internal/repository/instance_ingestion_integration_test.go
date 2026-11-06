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

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Product and version fixtures. "wso2am" is the code of "WSO2 API Manager";
// "WSO2 API Manager Analytics" also contains "API Manager", and is longer.
const (
	ingestProductAM          = "6b000000-0000-4000-8000-000000000001"
	ingestProductAMAnalytics = "6b000000-0000-4000-8000-000000000002"
	ingestProductAxM         = "6b000000-0000-4000-8000-000000000003"
	ingestVersionAM420       = "6c000000-0000-4000-8000-000000000001"
	ingestVersionAMA420      = "6c000000-0000-4000-8000-000000000002"
	ingestVersionAM430       = "6c000000-0000-4000-8000-000000000003"
	ingestVersionAxM420      = "6c000000-0000-4000-8000-000000000004"
	ingestProjectWithKey     = "6d000000-0000-4000-8000-000000000001"
	ingestProjectNoKey       = "6d000000-0000-4000-8000-000000000002"
)

// instanceIngestionTestPool builds the usage tables from the real migration in
// a private schema and seeds a small product catalogue and two projects.
// Skipped without INSTANCE_INGESTION_TEST_DSN.
func instanceIngestionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INSTANCE_INGESTION_TEST_DSN")
	if dsn == "" {
		t.Skip("INSTANCE_INGESTION_TEST_DSN not set")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := fmt.Sprintf("instance_ingestion_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	// A session zone east of UTC: 23:30 UTC is already the next day there, so
	// a date taken from the session instead of the instant would show.
	cfg.ConnConfig.RuntimeParams["timezone"] = "Asia/Colombo"
	cfg.MaxConns = 30
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect with search_path: %v", err)
	}
	t.Cleanup(pool.Close)

	for _, ddl := range []string{
		`CREATE TABLE product (id UUID PRIMARY KEY, name VARCHAR(255) NOT NULL, code VARCHAR(100))`,
		`CREATE TABLE product_version (id UUID PRIMARY KEY, product_id UUID NOT NULL REFERENCES product(id), version VARCHAR(100) NOT NULL)`,
		`CREATE TABLE project (id UUID PRIMARY KEY, key VARCHAR(128), product_consumption_primary_secret_key VARCHAR(128))`,
		`CREATE TABLE deployment (id UUID PRIMARY KEY, number VARCHAR(50), project_id UUID REFERENCES project(id), is_active BOOLEAN, created_on TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	migration, err := os.ReadFile("../../migrations/0054_usage_tracking_tables.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply 0054: %v", err)
	}
	rollup, err := os.ReadFile("../../migrations/0245_usage_rollup_functions.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(rollup)); err != nil {
		t.Fatalf("apply 0245: %v", err)
	}
	monthlyNullSource, err := os.ReadFile("../../migrations/0250_usage_rollup_monthly_null_source.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(monthlyNullSource)); err != nil {
		t.Fatalf("apply 0250: %v", err)
	}
	reported, err := os.ReadFile("../../migrations/0247_deployment_node_reported_product.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(reported)); err != nil {
		t.Fatalf("apply 0247: %v", err)
	}
	history, err := os.ReadFile("../../migrations/0248_usage_report_import.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(history)); err != nil {
		t.Fatalf("apply 0248: %v", err)
	}

	for _, seed := range []string{
		`INSERT INTO product (id, name, code) VALUES
			('` + ingestProductAM + `', 'WSO2 API Manager', 'wso2am'),
			('` + ingestProductAMAnalytics + `', 'WSO2 API Manager Analytics', 'wso2am-analytics'),
			('` + ingestProductAxM + `', 'AxM Gateway', 'axm')`,
		`INSERT INTO product_version (id, product_id, version) VALUES
			('` + ingestVersionAM420 + `', '` + ingestProductAM + `', '4.2.0'),
			('` + ingestVersionAMA420 + `', '` + ingestProductAMAnalytics + `', '4.2.0'),
			('` + ingestVersionAM430 + `', '` + ingestProductAM + `', '4.3.0'),
			('` + ingestVersionAxM420 + `', '` + ingestProductAxM + `', '4.2.0')`,
		`INSERT INTO project (id, key, product_consumption_primary_secret_key) VALUES
			('` + ingestProjectWithKey + `', 'CUPRSUBDEV', 'abc123secret'),
			('` + ingestProjectNoKey + `', 'NOKEYPROJ', NULL)`,
		`INSERT INTO deployment (id, number, project_id, is_active, created_on) VALUES
			('6e000000-0000-4000-8000-000000000001', 'DEP-OLD', '` + ingestProjectNoKey + `', FALSE, NOW() - INTERVAL '2 days'),
			('6e000000-0000-4000-8000-000000000002', 'DEP-OLD', '` + ingestProjectWithKey + `', TRUE, NOW() - INTERVAL '1 day'),
			('6e000000-0000-4000-8000-000000000003', 'DEP-NOKEY', '` + ingestProjectNoKey + `', TRUE, NOW())`,
	} {
		if _, err := pool.Exec(ctx, seed); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return pool
}

// internalCtx is how an internal caller reaches the repository: with an
// identity stamped by the identity middleware.
func internalCtx() context.Context {
	return repository.WithSystemIdentity(context.Background())
}

type ingestNode struct {
	ProjectKey       string
	DeploymentNumber *string
	VersionID        *string
	CreatedBy        string
}

func readNodes(t *testing.T, pool *pgxpool.Pool, nodeID string) []ingestNode {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT project_key, deployment_number, product_version_id::text, created_by
		FROM deployment_node WHERE node_id = $1 ORDER BY created_on, id`, nodeID)
	if err != nil {
		t.Fatalf("read nodes: %v", err)
	}
	defer rows.Close()
	var out []ingestNode
	for rows.Next() {
		var n ingestNode
		if err := rows.Scan(&n.ProjectKey, &n.DeploymentNumber, &n.VersionID, &n.CreatedBy); err != nil {
			t.Fatalf("scan node: %v", err)
		}
		out = append(out, n)
	}
	return out
}

func ingestStr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestInstanceIngestionIntegration_NodeUpsert(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	id1, err := repo.UpsertDeploymentNode(ctx, "node-a", domain.InstanceNodeMeta{
		ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	nodes := readNodes(t, pool, "node-a")
	if len(nodes) != 1 || nodes[0].ProjectKey != "CUPRSUBDEV" || ingestStr(nodes[0].DeploymentNumber) != "DEP000001" ||
		ingestStr(nodes[0].VersionID) != ingestVersionAM420 || nodes[0].CreatedBy != domain.InstanceIngestionActor {
		t.Fatalf("registered node = %+v", nodes)
	}

	// Blank meta never clears what is stored, and a missing version leaves the
	// product version alone.
	id2, err := repo.UpsertDeploymentNode(ctx, "node-a", domain.InstanceNodeMeta{Product: "wso2am"})
	if err != nil || id2 != id1 {
		t.Fatalf("second upsert: id %q (want %q), err %v", id2, id1, err)
	}
	nodes = readNodes(t, pool, "node-a")
	if nodes[0].ProjectKey != "CUPRSUBDEV" || ingestStr(nodes[0].DeploymentNumber) != "DEP000001" || ingestStr(nodes[0].VersionID) != ingestVersionAM420 {
		t.Fatalf("blank meta changed the node: %+v", nodes[0])
	}

	// An upgrade moves the version; an unknown version clears it rather than
	// leaving the node on the version it left.
	if _, err := repo.UpsertDeploymentNode(ctx, "node-a", domain.InstanceNodeMeta{Product: "wso2am", Version: "4.3.0"}); err != nil {
		t.Fatal(err)
	}
	if v := readNodes(t, pool, "node-a")[0].VersionID; ingestStr(v) != ingestVersionAM430 {
		t.Fatalf("version after upgrade = %s", ingestStr(v))
	}
	if _, err := repo.UpsertDeploymentNode(ctx, "node-a", domain.InstanceNodeMeta{Product: "wso2am", Version: "9.9.9"}); err != nil {
		t.Fatal(err)
	}
	if v := readNodes(t, pool, "node-a")[0].VersionID; v != nil {
		t.Fatalf("version after an unknown version = %s, want NULL", *v)
	}
}

func TestInstanceIngestionIntegration_ProductResolutionIsDeterministic(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	cases := []struct {
		product string
		want    string
	}{
		{"wso2am", ingestVersionAM420},                      // exact code
		{"WSO2 API Manager Analytics", ingestVersionAMA420}, // exact name, though a shorter name also contains part of it
		{"API Manager", ingestVersionAM420},                 // contained in two names: the shorter one
		{"wso2am-analytics", ingestVersionAMA420},           // exact code of the longer one
		{"A_M", ""}, // _ is literal: "AxM Gateway" must not match
		{"%", ""},   // % is literal
	}
	for i, tc := range cases {
		node := fmt.Sprintf("resolve-%d", i)
		if _, err := repo.UpsertDeploymentNode(ctx, node, domain.InstanceNodeMeta{ProjectKey: "P", Product: tc.product, Version: "4.2.0"}); err != nil {
			t.Fatalf("%q: %v", tc.product, err)
		}
		got := readNodes(t, pool, node)[0].VersionID
		want := tc.want
		if want == "" {
			want = "<nil>"
		}
		if ingestStr(got) != want {
			t.Errorf("product %q resolved to %s, want %s", tc.product, ingestStr(got), want)
		}
	}
}

func TestInstanceIngestionIntegration_ConcurrentFirstHeartbeatsMakeOneNode(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	countedOn := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- repo.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{
				NodeID: "node-race", CountType: "TPS", Count: i,
				InstanceNodeMeta: domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP7"},
			}, countedOn.Add(time.Duration(i)*time.Minute)) // distinct reports; one instant twice is a retry, stored once
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent usage: %v", err)
		}
	}

	if nodes := readNodes(t, pool, "node-race"); len(nodes) != 1 {
		t.Fatalf("deployment_node rows for one node = %d, want 1", len(nodes))
	}
	var hourly, daily int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hourly_usage_summary`).Scan(&hourly); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM daily_usage_summary`).Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if hourly != n || daily != 1 {
		t.Fatalf("hourly rows = %d (want %d), daily rows = %d (want 1)", hourly, n, daily)
	}
}

func TestInstanceIngestionIntegration_UsageStatsRollUp(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	// 23:30 UTC on the 15th is the 16th in the session's zone: the day must be the 15th.
	countedOn := time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}
	for i, c := range []int{100, 250} {
		// Two reports a minute apart (the same instant twice would be one report, retried).
		if err := repo.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "node-u", CountType: "TRANSACTION_COUNT", Count: c, InstanceNodeMeta: meta}, countedOn.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	// A count with no deployment number takes the node's own.
	if err := repo.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "node-u", CountType: "CORES", Count: 8}, countedOn); err != nil {
		t.Fatal(err)
	}

	nodes := readNodes(t, pool, "node-u")
	if len(nodes) != 1 || nodes[0].ProjectKey != "CUPRSUBDEV" || ingestStr(nodes[0].VersionID) != ingestVersionAM420 {
		t.Fatalf("usage stats did not register the node from its meta: %+v", nodes)
	}

	rows, err := pool.Query(context.Background(), `
		SELECT count_type, value, deployment_number, summary_date::text, data_source::text
		FROM daily_usage_summary ORDER BY count_type`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var ct, day, src string
		var v int
		var dn *string
		if err := rows.Scan(&ct, &v, &dn, &day, &src); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s=%d %s %s %s", ct, v, ingestStr(dn), day, src))
	}
	want := []string{
		"CORES=8 DEP000001 2026-10-15 API_CALL",
		"TRANSACTION_COUNT=350 DEP000001 2026-10-15 API_CALL",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("daily rows:\n got %v\nwant %v", got, want)
	}

	var hourly int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hourly_usage_summary WHERE counted_on >= $1`, countedOn).Scan(&hourly); err != nil {
		t.Fatal(err)
	}
	if hourly != 3 {
		t.Fatalf("hourly rows from the counted instant on = %d, want 3", hourly)
	}
}

func TestInstanceIngestionIntegration_InfoSnapshotIsStoredOncePerHash(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	req := domain.InstanceInfoRequest{
		NodeID: "node-i", JDKVersion: "17.0.9", CoreCount: 8, Hash: "h1",
		DeploymentInfo:   map[string]any{"jdkVersion": "17.0.9"},
		InstanceNodeMeta: domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP2"},
	}
	first := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	later := first.Add(time.Hour)

	id1, err := repo.InsertDeploymentInformation(ctx, req, first)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := repo.InsertDeploymentInformation(ctx, req, later)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("same snapshot stored twice: %s and %s", id1, id2)
	}
	req.Hash, req.CoreCount = "h2", 0
	id3, err := repo.InsertDeploymentInformation(ctx, req, later)
	if err != nil || id3 == id1 {
		t.Fatalf("a new hash must be a new row: %s (%v)", id3, err)
	}

	var created, updated time.Time
	if err := pool.QueryRow(context.Background(), `SELECT payload_created_on, payload_updated_on FROM deployment_information WHERE id = $1::uuid`, id1).Scan(&created, &updated); err != nil {
		t.Fatal(err)
	}
	if !created.Equal(first) || !updated.Equal(later) {
		t.Fatalf("payload created/updated = %v / %v, want %v / %v", created, updated, first, later)
	}
	var cores *int
	if err := pool.QueryRow(context.Background(), `SELECT core_count FROM deployment_information WHERE id = $1::uuid`, id3).Scan(&cores); err != nil {
		t.Fatal(err)
	}
	if cores != nil {
		t.Fatalf("an unreported core count is stored as %d, want NULL", *cores)
	}
	if nodes := readNodes(t, pool, "node-i"); len(nodes) != 1 || nodes[0].ProjectKey != "CUPRSUBDEV" {
		t.Fatalf("info did not register the node from its meta: %+v", nodes)
	}
}

func TestInstanceIngestionIntegration_BatchImport(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	req := domain.DailyUsageBatchRequest{
		NodeID:           "node-zip",
		InstanceNodeMeta: domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP-ROOT", Product: "wso2am", Version: "4.2.0"},
		UserEmail:        "uploader@example.com",
		Usages: []domain.DailyUsageBatchItem{
			{Date: "2026-10-14", Type: "TRANSACTION_COUNT", Value: 99000},
			{Date: "2026-10-14", Type: "CORES", Value: 32},
			{Date: "2026-10-14", Type: "CORES", Value: 16, NodeID: "node-zip-2", DeploymentNumber: "DEP-OTHER"},
		},
	}
	imported, err := repo.ImportDailyUsagesBatch(ctx, req)
	if err != nil || imported != 3 {
		t.Fatalf("imported %d, err %v", imported, err)
	}
	// Re-importing the same file updates, never duplicates.
	req.Usages[0].Value = 100000
	if _, err := repo.ImportDailyUsagesBatch(ctx, req); err != nil {
		t.Fatal(err)
	}

	rows, err := pool.Query(context.Background(), `
		SELECT n.node_id, d.count_type, d.value, d.deployment_number, d.created_by, d.data_source::text
		FROM daily_usage_summary d JOIN deployment_node n ON n.id = d.deployment_node_id
		ORDER BY n.node_id, d.count_type`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var node, ct, by, src string
		var v int
		var dn *string
		if err := rows.Scan(&node, &ct, &v, &dn, &by, &src); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s=%d %s %s %s", node, ct, v, ingestStr(dn), by, src))
	}
	want := []string{
		"node-zip CORES=32 DEP-ROOT uploader@example.com FILE_UPLOAD",
		"node-zip TRANSACTION_COUNT=100000 DEP-ROOT uploader@example.com FILE_UPLOAD",
		"node-zip-2 CORES=16 DEP-OTHER uploader@example.com FILE_UPLOAD",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("daily rows:\n got %v\nwant %v", got, want)
	}
	if nodes := readNodes(t, pool, "node-zip-2"); len(nodes) != 1 || ingestStr(nodes[0].DeploymentNumber) != "DEP-OTHER" {
		t.Fatalf("override node = %+v", nodes)
	}
}

func TestInstanceIngestionIntegration_BatchIsAllOrNothing(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))

	// The last entry's node id is longer than the column, so the database
	// refuses it after the earlier entries were written: nothing may remain.
	_, err := repo.ImportDailyUsagesBatch(internalCtx(), domain.DailyUsageBatchRequest{
		NodeID:           "node-ok",
		InstanceNodeMeta: domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV"},
		Usages: []domain.DailyUsageBatchItem{
			{Date: "2026-10-14", Type: "CORES", Value: 4},
			{Date: "2026-10-14", Type: "CORES", Value: 4, NodeID: strings.Repeat("z", 40)},
		},
	})
	if err == nil {
		t.Fatal("want an error for an over-long node id")
	}
	for _, table := range []string{"deployment_node", "daily_usage_summary"} {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s holds %d rows after a failed import, want 0", table, n)
		}
	}
}

func TestInstanceIngestionIntegration_SecretKey(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	for _, key := range []string{"CUPRSUBDEV", ingestProjectWithKey} {
		got, err := repo.GetProjectSecretKey(ctx, key)
		if err != nil || got != "abc123secret" {
			t.Errorf("by %q: %q, %v", key, got, err)
		}
	}
	for _, key := range []string{"NOKEYPROJ", "UNKNOWN"} {
		_, err := repo.GetProjectSecretKey(ctx, key)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("by %q: want NotFoundError, got %T (%v)", key, err, err)
		}
	}
}

func TestInstanceIngestionIntegration_DeploymentConsumptionSecret(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()

	// Two deployments share the number: the active one decides the project.
	got, err := repo.GetDeploymentConsumptionSecret(ctx, "DEP-OLD")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != ingestProjectWithKey || got.ProjectKey != "CUPRSUBDEV" || got.SecretKey != "abc123secret" || got.DeploymentNumber != "DEP-OLD" {
		t.Fatalf("got %+v", got)
	}
	for _, number := range []string{"DEP-NOKEY", "DEP-UNKNOWN"} {
		_, err := repo.GetDeploymentConsumptionSecret(ctx, number)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("%s: want NotFoundError, got %T (%v)", number, err, err)
		}
	}
}

func TestInstanceIngestionIntegration_BatchRegistersReportNodesWithTheirProducts(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))

	imported, err := repo.ImportDailyUsagesBatch(internalCtx(), domain.DailyUsageBatchRequest{
		NodeID:           "node-a",
		InstanceNodeMeta: domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000002442"},
		Nodes: []domain.DailyUsageBatchNode{
			{NodeID: "node-a", Product: "wso2am", Version: "4.2.0"},
			{NodeID: "node-b", Product: "wso2am", Version: "4.3.0"},
		},
		Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Type: "CORES", Value: 4, NodeID: "node-a"}},
	})
	if err != nil || imported != 1 {
		t.Fatalf("imported %d, err %v", imported, err)
	}
	for node, want := range map[string]string{"node-a": ingestVersionAM420, "node-b": ingestVersionAM430} {
		nodes := readNodes(t, pool, node)
		if len(nodes) != 1 || ingestStr(nodes[0].VersionID) != want || ingestStr(nodes[0].DeploymentNumber) != "DEP000002442" {
			t.Errorf("%s = %+v, want version %s", node, nodes, want)
		}
	}
}

// A node keeps what it reported; one whose product or version is not in the catalogue is
// listed as unresolved (grouped by what it reported), one that resolves is not, and a node
// last seen before the window is left out.
func TestInstanceIngestionIntegration_UnresolvedProductsAreKeptAndListed(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	register := func(nodeID, product, version string) {
		t.Helper()
		meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: product, Version: version}
		if _, err := repo.UpsertDeploymentNode(ctx, nodeID, meta); err != nil {
			t.Fatal(err)
		}
	}
	reported := func(nodeID string) (product, version *string, resolved bool) {
		t.Helper()
		if err := pool.QueryRow(context.Background(),
			`SELECT reported_product, reported_version, product_version_id IS NOT NULL FROM deployment_node WHERE node_id = $1`,
			nodeID).Scan(&product, &version, &resolved); err != nil {
			t.Fatal(err)
		}
		return
	}

	register("node-known", "wso2am", "4.2.0")      // in the catalogue
	register("node-unknown", "wso2zz", "1.0.0")    // product not in the catalogue
	register("node-newversion", "wso2am", "9.9.9") // product known, version not
	register("node-old", "wso2yy", "2.0.0")        // unresolved, but last seen long ago
	if _, err := pool.Exec(context.Background(), `UPDATE deployment_node SET updated_on = NOW() - INTERVAL '10 days' WHERE node_id = 'node-old'`); err != nil {
		t.Fatal(err)
	}

	if p, v, ok := reported("node-known"); p == nil || *p != "wso2am" || v == nil || *v != "4.2.0" || !ok {
		t.Errorf("node-known: reported %v %v resolved %v", p, v, ok)
	}
	if p, _, ok := reported("node-unknown"); p == nil || *p != "wso2zz" || ok {
		t.Errorf("node-unknown: reported %v resolved %v", p, ok)
	}

	nodes, err := repo.ListUnresolvedProductNodes(ctx, time.Now().Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, fmt.Sprintf("%s %s %s", n.NodeID, n.ReportedProduct, ingestStr(n.ReportedVersion)))
	}
	want := []string{"node-newversion wso2am 9.9.9", "node-unknown wso2zz 1.0.0"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("unresolved nodes:\n got %v\nwant %v", got, want)
	}

	// Once the node reports a product the catalogue knows, it is no longer listed.
	register("node-unknown", "wso2am", "4.2.0")
	nodes, err = repo.ListUnresolvedProductNodes(ctx, time.Now().Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].NodeID != "node-newversion" {
		t.Errorf("after node-unknown resolved: %+v", nodes)
	}
}

// rollupNode registers a node and returns its uuid.
func rollupNode(t *testing.T, repo repository.InstanceIngestionRepository, nodeID string) string {
	t.Helper()
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}
	id, err := repo.UpsertDeploymentNode(internalCtx(), nodeID, meta)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// rollupDailyValues reads a node's daily rows as "type date source=value".
func rollupDailyValues(t *testing.T, pool *pgxpool.Pool, nodeID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT dus.count_type, dus.summary_date::text, dus.data_source::text, dus.value
		FROM daily_usage_summary dus JOIN deployment_node dn ON dn.id = dus.deployment_node_id
		WHERE dn.node_id = $1 ORDER BY dus.count_type, dus.summary_date, dus.data_source::text`, nodeID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var ct, day, src string
		var v int
		if err := rows.Scan(&ct, &day, &src, &v); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s %s=%d", ct, day, src, v))
	}
	return got
}

func hourlyRow(t *testing.T, pool *pgxpool.Pool, nodeUUID, countType string, count int, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO hourly_usage_summary (id, created_on, updated_on, created_by, updated_by, deployment_node_id, count_type, count, counted_on)
		VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1::uuid, $2, $3, $4)`, nodeUUID, countType, count, at); err != nil {
		t.Fatal(err)
	}
}

func dailyRow(t *testing.T, pool *pgxpool.Pool, nodeUUID, day, countType string, value int, source string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO daily_usage_summary (id, created_on, updated_on, created_by, updated_by, deployment_node_id, deployment_number, summary_date, count_type, value, data_source)
		VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1::uuid, 'DEP000001', $2::date, $3, $4, $5::usage_data_source_enum)`,
		nodeUUID, day, countType, value, source); err != nil {
		t.Fatal(err)
	}
}

func monthlyValues(t *testing.T, pool *pgxpool.Pool, year, month int) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT node_id, count_type, count FROM monthly_usage_summary
		WHERE year = $1 AND month = $2 ORDER BY node_id, count_type`, year, month)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var node, ct string
		var v int
		if err := rows.Scan(&node, &ct, &v); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s=%d", node, ct, v))
	}
	return got
}

// The SQL counter rule has to stay in step with the one ingestion applies.
func TestUsageRollup_CounterRule(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	for countType, want := range map[string]bool{
		"TRANSACTION_COUNT": true, "M2M_TOKEN": true, "AGENT_DIRECT_TOKEN": true, "AGENT_DIRECT_REFRESH_TOKEN": true,
		"AGENT_OBO_TOKEN": true, "AGENT_OBO_REFRESH_TOKEN": true, "AGENT_PROVISION": true,
		"MAU": false, "TOTAL_USERS": false, "TOTAL_ROOT_ORGS": false, "TOTAL_B2B_ORGS": false,
		"API_COUNT": false, "MCP_API_COUNT": false, "CORES": false, "AGENTX": false,
	} {
		var got bool
		if err := pool.QueryRow(context.Background(), `SELECT usage_count_type_is_counter($1)`, countType).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("usage_count_type_is_counter(%q) = %v, want %v", countType, got, want)
		}
	}
}

// The daily rollup sums counters and takes the latest reading of levels from the
// UTC day's hourly rows, repairs a wrong stored value, leaves another source's row
// and other days alone, and writes nothing the second time.
func TestUsageRollup_DailyFromHourlyRows(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	node := rollupNode(t, repo, "node-r1")
	at := func(day, hour, min int) time.Time { return time.Date(2026, 10, day, hour, min, 0, 0, time.UTC) }

	hourlyRow(t, pool, node, "TRANSACTION_COUNT", 100, at(15, 8, 0))
	hourlyRow(t, pool, node, "TRANSACTION_COUNT", 250, at(15, 9, 0))
	hourlyRow(t, pool, node, "TRANSACTION_COUNT", 5, at(15, 23, 30)) // 23:30 UTC is already the 16th in the session zone
	hourlyRow(t, pool, node, "TRANSACTION_COUNT", 10, at(16, 1, 0))  // the next day
	hourlyRow(t, pool, node, "M2M_TOKEN", 20, at(15, 9, 0))
	hourlyRow(t, pool, node, "M2M_TOKEN", 30, at(15, 10, 0))
	hourlyRow(t, pool, node, "AGENT_OBO_TOKEN", 3, at(15, 9, 0))
	hourlyRow(t, pool, node, "MAU", 40, at(15, 11, 0))
	hourlyRow(t, pool, node, "MAU", 45, at(15, 12, 0))
	hourlyRow(t, pool, node, "TOTAL_USERS", 56, at(15, 12, 0))
	hourlyRow(t, pool, node, "TOTAL_USERS", 50, at(15, 9, 0)) // an earlier hour's report that arrived late

	dailyRow(t, pool, node, "2026-10-15", "TRANSACTION_COUNT", 250, "API_CALL")     // the old "last report" value
	dailyRow(t, pool, node, "2026-10-15", "TRANSACTION_COUNT", 7777, "FILE_UPLOAD") // an upload for the same day

	written, err := repo.RollupDailyUsage(ctx, "2026-10-15")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"AGENT_OBO_TOKEN 2026-10-15 API_CALL=3",
		"M2M_TOKEN 2026-10-15 API_CALL=50",
		"MAU 2026-10-15 API_CALL=45",
		"TOTAL_USERS 2026-10-15 API_CALL=56",
		"TRANSACTION_COUNT 2026-10-15 API_CALL=355",
		"TRANSACTION_COUNT 2026-10-15 FILE_UPLOAD=7777",
	}
	if got := rollupDailyValues(t, pool, "node-r1"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("daily rows:\n got %v\nwant %v", got, want)
	}
	if written != 5 { // four rows inserted, the stale TRANSACTION_COUNT corrected
		t.Errorf("rows written = %d, want 5", written)
	}

	again, err := repo.RollupDailyUsage(ctx, "2026-10-15")
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Errorf("a rerun over unchanged rows wrote %d rows, want 0", again)
	}

	// The next day is rolled up on its own.
	if _, err := repo.RollupDailyUsage(ctx, "2026-10-16"); err != nil {
		t.Fatal(err)
	}
	got := rollupDailyValues(t, pool, "node-r1")
	if len(got) != len(want)+1 || got[len(got)-1] != "TRANSACTION_COUNT 2026-10-16 API_CALL=10" {
		t.Errorf("after rolling up the 16th: %v", got)
	}
}

// The monthly rollup sums counters over the days and takes a level's latest day,
// counts a node-type-day once (the live row over an uploaded one), includes a node
// that has only uploaded days (an air-gapped product), skips other months, and writes
// nothing the second time.
func TestUsageRollup_MonthlyFromDailyRows(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	live := rollupNode(t, repo, "node-m1")
	airGapped := rollupNode(t, repo, "node-air")

	dailyRow(t, pool, live, "2026-10-01", "TRANSACTION_COUNT", 100, "API_CALL")
	dailyRow(t, pool, live, "2026-10-02", "TRANSACTION_COUNT", 200, "API_CALL")
	dailyRow(t, pool, live, "2026-10-02", "TRANSACTION_COUNT", 999, "FILE_UPLOAD") // the live row for that day counts
	dailyRow(t, pool, live, "2026-10-03", "TRANSACTION_COUNT", 50, "FILE_UPLOAD")  // no live row that day, so the upload counts
	dailyRow(t, pool, live, "2026-10-05", "MAU", 30, "API_CALL")
	dailyRow(t, pool, live, "2026-10-20", "MAU", 45, "API_CALL")
	dailyRow(t, pool, live, "2026-10-12", "MAU", 99, "FILE_UPLOAD")
	dailyRow(t, pool, live, "2026-09-30", "TRANSACTION_COUNT", 77, "API_CALL") // another month

	dailyRow(t, pool, airGapped, "2026-10-01", "M2M_TOKEN", 10, "FILE_UPLOAD")
	dailyRow(t, pool, airGapped, "2026-10-02", "M2M_TOKEN", 15, "FILE_UPLOAD")
	dailyRow(t, pool, airGapped, "2026-10-01", "TOTAL_USERS", 20, "FILE_UPLOAD")
	dailyRow(t, pool, airGapped, "2026-10-31", "TOTAL_USERS", 22, "FILE_UPLOAD")

	written, err := repo.RollupMonthlyUsage(ctx, 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"node-air M2M_TOKEN=25",
		"node-air TOTAL_USERS=22",
		"node-m1 MAU=45",
		"node-m1 TRANSACTION_COUNT=350",
	}
	if got := monthlyValues(t, pool, 2026, 10); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("monthly rows:\n got %v\nwant %v", got, want)
	}
	if written != 4 {
		t.Errorf("rows written = %d, want 4", written)
	}
	if got := monthlyValues(t, pool, 2026, 9); len(got) != 0 {
		t.Errorf("September was rolled up too: %v", got)
	}

	again, err := repo.RollupMonthlyUsage(ctx, 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Errorf("a rerun over unchanged rows wrote %d rows, want 0", again)
	}

	// A day that changes afterwards changes the month on the next run.
	dailyRow(t, pool, live, "2026-10-04", "TRANSACTION_COUNT", 1000, "API_CALL")
	if n, err := repo.RollupMonthlyUsage(ctx, 2026, 10); err != nil || n != 1 {
		t.Fatalf("after a new day: wrote %d rows, err %v", n, err)
	}
	if got := monthlyValues(t, pool, 2026, 10); got[3] != "node-m1 TRANSACTION_COUNT=1350" {
		t.Errorf("monthly rows after a new day: %v", got)
	}
}

// Reports arriving while the rollup runs must neither deadlock with it nor be lost:
// the rollup takes the same per-node locks ingestion does, and a final run over the
// finished hours gives the day's true total.
func TestUsageRollup_RunsAlongsideIngestion(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}
	day := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	rollupNode(t, repo, "node-c1")
	rollupNode(t, repo, "node-c2")

	const reports = 24
	var wg sync.WaitGroup
	errs := make(chan error, 2*reports+40)
	for _, node := range []string{"node-c1", "node-c2"} {
		for i := 0; i < reports; i++ {
			wg.Add(1)
			go func(node string, hour int) {
				defer wg.Done()
				req := domain.InstanceUsageIngestRequest{NodeID: node, CountType: "M2M_TOKEN", Count: 10, InstanceNodeMeta: meta}
				errs <- repo.InsertUsageStats(ctx, req, day.Add(time.Duration(hour)*time.Hour))
			}(node, i)
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, err := repo.RollupDailyUsage(ctx, "2026-10-15")
			errs <- err
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent call failed: %v", err)
		}
	}

	if _, err := repo.RollupDailyUsage(ctx, "2026-10-15"); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node-c1", "node-c2"} {
		want := []string{fmt.Sprintf("M2M_TOKEN 2026-10-15 API_CALL=%d", 10*reports)}
		if got := rollupDailyValues(t, pool, node); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s daily rows: got %v, want %v", node, got, want)
		}
	}
}

// dailyValues reads the node's daily rows as "type date source=value".
func dailyValues(t *testing.T, pool *pgxpool.Pool, nodeID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT dus.count_type, dus.summary_date::text, dus.data_source::text, dus.value
		FROM daily_usage_summary dus JOIN deployment_node dn ON dn.id = dus.deployment_node_id
		WHERE dn.node_id = $1 ORDER BY dus.count_type, dus.summary_date, dus.data_source::text`, nodeID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var ct, day, src string
		var v int
		if err := rows.Scan(&ct, &day, &src, &v); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s %s=%d", ct, day, src, v))
	}
	return got
}

// A counter a product publishes every hour adds up over the day, a level keeps
// its latest reading (even when an older hour's report arrives late), each day
// has its own row, and an uploaded FILE_UPLOAD row is left alone.
func TestInstanceIngestionIntegration_DailyValueIsDerivedFromHourlyRows(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	send := func(ct string, count int, when time.Time) {
		t.Helper()
		req := domain.InstanceUsageIngestRequest{NodeID: "node-d", CountType: ct, Count: count, InstanceNodeMeta: meta}
		if err := repo.InsertUsageStats(ctx, req, when); err != nil {
			t.Fatal(err)
		}
	}

	// Counters: one report per hour, each the amount since the previous one.
	for _, c := range []int{100, 250, 50} {
		send("TRANSACTION_COUNT", c, at(15, 8+c/50))
	}
	send("M2M_TOKEN", 20, at(15, 9))
	send("M2M_TOKEN", 30, at(15, 10))
	send("AGENT_OBO_TOKEN", 3, at(15, 9))
	send("AGENT_OBO_TOKEN", 4, at(15, 10))
	send("TRANSACTION_COUNT", 10, at(16, 1)) // the next day is its own row

	// Levels: the latest reading of the day stands, whatever order they arrive in.
	send("MAU", 40, at(15, 11))
	send("MAU", 45, at(15, 12))
	send("TOTAL_USERS", 56, at(15, 12))
	send("TOTAL_USERS", 50, at(15, 9)) // late report from an earlier hour

	// An uploaded day for the same node and type sits beside the live one.
	if _, err := repo.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{
		NodeID: "node-d", InstanceNodeMeta: meta,
		Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Type: "TRANSACTION_COUNT", Value: 7777}},
	}); err != nil {
		t.Fatal(err)
	}
	send("TRANSACTION_COUNT", 5, at(15, 14)) // live data after the upload

	got := dailyValues(t, pool, "node-d")
	want := []string{
		"AGENT_OBO_TOKEN 2026-10-15 API_CALL=7",
		"M2M_TOKEN 2026-10-15 API_CALL=50",
		"MAU 2026-10-15 API_CALL=45",
		"TOTAL_USERS 2026-10-15 API_CALL=56",
		"TRANSACTION_COUNT 2026-10-15 API_CALL=405",
		"TRANSACTION_COUNT 2026-10-15 FILE_UPLOAD=7777",
		"TRANSACTION_COUNT 2026-10-16 API_CALL=10",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("daily rows:\n got %v\nwant %v", got, want)
	}
}

// Every import leaves one history row, in the same transaction: a failed import leaves
// none. The listing filters by project and deployment, newest first.
func TestInstanceIngestionIntegration_ImportsAreRecordedInTheirHistory(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	valid := true
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}

	first := domain.DailyUsageBatchRequest{
		NodeID: "node-h1", InstanceNodeMeta: meta, UserEmail: "uploader@example.com",
		Nodes:         []domain.DailyUsageBatchNode{{NodeID: "node-h1"}, {NodeID: "node-h2"}},
		Usages:        []domain.DailyUsageBatchItem{{Date: "2026-10-14", Type: "TRANSACTION_COUNT", Value: 10}, {Date: "2026-10-15", Type: "TRANSACTION_COUNT", Value: 20}},
		ExportVersion: "1.0", ExportedAt: "2026-10-16T05:00:00.123Z", ExportValid: &valid,
	}
	if _, err := repo.ImportDailyUsagesBatch(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := domain.DailyUsageBatchRequest{
		NodeID: "node-h3", InstanceNodeMeta: domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000002"},
		Usages:     []domain.DailyUsageBatchItem{{Date: "2026-10-15", Type: "CORES", Value: 4}},
		ExportedAt: "not a time",
	}
	if _, err := repo.ImportDailyUsagesBatch(ctx, second); err != nil {
		t.Fatal(err)
	}
	// A batch that fails part-way (a type too long for its column) records nothing.
	failing := domain.DailyUsageBatchRequest{
		NodeID: "node-h4", InstanceNodeMeta: meta,
		Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Type: strings.Repeat("X", 101), Value: 1}},
	}
	if _, err := repo.ImportDailyUsagesBatch(ctx, failing); err == nil {
		t.Fatal("the failing batch was accepted")
	}

	all, err := repo.ListUsageReportImports(ctx, "CUPRSUBDEV", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("history rows = %d, want 2 (the failed import must leave none): %+v", len(all), all)
	}
	newest, oldest := all[0], all[1]
	if ingestStr(newest.DeploymentNumber) != "DEP000002" || newest.ExportedAt != nil || newest.CreatedBy != domain.InstanceIngestionActor || newest.ImportedCount != 1 {
		t.Errorf("newest: %+v (an unparseable exportedAt is unknown; no uploader means the service)", newest)
	}
	if oldest.CreatedBy != "uploader@example.com" || ingestStr(oldest.ExportVersion) != "1.0" || oldest.ExportValid == nil || !*oldest.ExportValid ||
		oldest.ExportedAt == nil || !oldest.ExportedAt.Equal(time.Date(2026, 10, 16, 5, 0, 0, 123000000, time.UTC)) ||
		oldest.NodeCount != 2 || oldest.ImportedCount != 2 {
		t.Errorf("oldest: %+v", oldest)
	}

	byDeployment, err := repo.ListUsageReportImports(ctx, "", "DEP000001", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(byDeployment) != 1 || byDeployment[0].ID != oldest.ID {
		t.Errorf("by deployment: %+v", byDeployment)
	}
}

// monthly_usage_summary has no unique key, so two rollups of the same month must not run
// at once: the second waits for the first to commit, then finds the rows already there.
func TestUsageRollup_MonthlyRunsForOneMonthAreSerialised(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	node := rollupNode(t, repo, "node-serial")
	dailyRow(t, pool, node, "2026-10-01", "TRANSACTION_COUNT", 100, "API_CALL")

	bg := context.Background()
	first, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Rollback(bg) }()
	if _, err := first.Exec(bg, `SELECT set_config('app.is_internal', 'true', true)`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := first.QueryRow(bg, `SELECT rollup_monthly_usage(2026, 10)`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("first rollup: wrote %d, err %v", n, err)
	}

	type result struct {
		n   int
		err error
	}
	second := make(chan result, 1)
	go func() {
		n, err := repo.RollupMonthlyUsage(internalCtx(), 2026, 10)
		second <- result{n, err}
	}()
	select {
	case r := <-second:
		t.Fatalf("the second rollup did not wait for the first: wrote %d, err %v", r.n, r.err)
	case <-time.After(500 * time.Millisecond):
	}

	if err := first.Commit(bg); err != nil {
		t.Fatal(err)
	}
	r := <-second
	if r.err != nil || r.n != 0 {
		t.Fatalf("second rollup after the first committed: wrote %d, err %v (want 0)", r.n, r.err)
	}
	if got := monthlyValues(t, pool, 2026, 10); strings.Join(got, "|") != "node-serial TRANSACTION_COUNT=100" {
		t.Errorf("monthly rows: %v", got)
	}
}

// A report the usage tracker retries is stored once, so a counter is not counted twice,
// and duplicates already in the table (from before, or synced) are counted once by both
// the live recompute and the daily rollup.
func TestInstanceIngestionIntegration_RetriedReportsAreCountedOnce(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}
	at := func(hour int) time.Time { return time.Date(2026, 10, 15, hour, 0, 0, 0, time.UTC) }
	send := func(count int, when time.Time) {
		t.Helper()
		req := domain.InstanceUsageIngestRequest{NodeID: "node-retry", CountType: "TRANSACTION_COUNT", Count: count, InstanceNodeMeta: meta}
		if err := repo.InsertUsageStats(ctx, req, when); err != nil {
			t.Fatal(err)
		}
	}

	send(100, at(8))
	send(100, at(8)) // the same report again, after a timeout
	send(50, at(9))
	var hourly int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM hourly_usage_summary h JOIN deployment_node dn ON dn.id = h.deployment_node_id
		WHERE dn.node_id = 'node-retry'`).Scan(&hourly); err != nil {
		t.Fatal(err)
	}
	if hourly != 2 {
		t.Errorf("hourly rows = %d, want 2 (the retry is not stored)", hourly)
	}
	if got := dailyValues(t, pool, "node-retry"); strings.Join(got, "|") != "TRANSACTION_COUNT 2026-10-15 API_CALL=150" {
		t.Errorf("daily after a retry: %v", got)
	}

	// A duplicate already in the table is counted once, live and by the rollup.
	var nodeUUID string
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM deployment_node WHERE node_id = 'node-retry'`).Scan(&nodeUUID); err != nil {
		t.Fatal(err)
	}
	hourlyRow(t, pool, nodeUUID, "TRANSACTION_COUNT", 50, at(9))
	send(10, at(10))
	if got := dailyValues(t, pool, "node-retry"); strings.Join(got, "|") != "TRANSACTION_COUNT 2026-10-15 API_CALL=160" {
		t.Errorf("daily with a stored duplicate: %v", got)
	}
	if _, err := repo.RollupDailyUsage(ctx, "2026-10-15"); err != nil {
		t.Fatal(err)
	}
	if got := dailyValues(t, pool, "node-retry"); strings.Join(got, "|") != "TRANSACTION_COUNT 2026-10-15 API_CALL=160" {
		t.Errorf("daily after the rollup: %v", got)
	}
}

// With the rollup switched on, an uploaded report rebuilds the months its rows fall in,
// however old; with it off (the default) it leaves monthly_usage_summary alone.
func TestInstanceIngestionIntegration_ImportRebuildsTheMonthsItTouches(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	ctx := internalCtx()
	meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: "wso2am", Version: "4.2.0"}
	batch := func(node string) domain.DailyUsageBatchRequest {
		return domain.DailyUsageBatchRequest{NodeID: node, InstanceNodeMeta: meta, Usages: []domain.DailyUsageBatchItem{
			{Date: "2026-03-30", Type: "TRANSACTION_COUNT", Value: 40},
			{Date: "2026-03-31", Type: "TRANSACTION_COUNT", Value: 60},
			{Date: "2026-04-01", Type: "TOTAL_USERS", Value: 7},
		}}
	}

	off := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	if _, err := off.ImportDailyUsagesBatch(ctx, batch("node-off")); err != nil {
		t.Fatal(err)
	}
	if got := monthlyValues(t, pool, 2026, 3); len(got) != 0 {
		t.Errorf("rollup off still wrote March: %v", got)
	}

	on := repository.NewInstanceIngestionRepository(repository.NewScoped(pool), repository.WithMonthlyRollupOnImport(true))
	if _, err := on.ImportDailyUsagesBatch(ctx, batch("node-on")); err != nil {
		t.Fatal(err)
	}
	// Each run rebuilds the whole month, so node-off's March is now in as well.
	if got := monthlyValues(t, pool, 2026, 3); strings.Join(got, "|") != "node-off TRANSACTION_COUNT=100|node-on TRANSACTION_COUNT=100" {
		t.Errorf("March: %v", got)
	}
	if got := monthlyValues(t, pool, 2026, 4); strings.Join(got, "|") != "node-off TOTAL_USERS=7|node-on TOTAL_USERS=7" {
		t.Errorf("April: %v", got)
	}
}

// The daily rollup runs a chunk of nodes per transaction; a day with more nodes than one
// chunk is still rolled up whole.
func TestUsageRollup_DailyRunsInChunksAndCoversEveryNode(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	const nodes = 230 // more than two chunks of 100
	for i := 0; i < nodes; i++ {
		// Mixed case: the rollup locks in byte order, as the batch import does.
		id := rollupNode(t, repo, fmt.Sprintf("Node-%03d", i))
		if i%2 == 0 {
			id = rollupNode(t, repo, fmt.Sprintf("node-%03d", i))
		}
		hourlyRow(t, pool, id, "TRANSACTION_COUNT", 1, time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC))
	}
	written, err := repo.RollupDailyUsage(ctx, "2026-10-15")
	if err != nil {
		t.Fatal(err)
	}
	if written != nodes {
		t.Errorf("rows written = %d, want %d", written, nodes)
	}
}

// unresolved_since records when a node's reports stopped matching the catalogue: kept
// across heartbeats, cleared once it matches, set afresh if it goes unresolved again.
func TestInstanceIngestionIntegration_UnresolvedSinceTracksTheMiss(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	repo := repository.NewInstanceIngestionRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	report := func(product, version string) {
		t.Helper()
		meta := domain.InstanceNodeMeta{ProjectKey: "CUPRSUBDEV", DeploymentNumber: "DEP000001", Product: product, Version: version}
		if _, err := repo.UpsertDeploymentNode(ctx, "node-since", meta); err != nil {
			t.Fatal(err)
		}
	}
	since := func() *time.Time {
		t.Helper()
		var v *time.Time
		if err := pool.QueryRow(context.Background(), `SELECT unresolved_since FROM deployment_node WHERE node_id = 'node-since'`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	report("wso2zz", "1.0.0")
	first := since()
	if first == nil {
		t.Fatal("a node that went unresolved has no unresolved_since")
	}
	report("wso2zz", "1.0.0") // a later heartbeat
	if got := since(); got == nil || !got.Equal(*first) {
		t.Errorf("a heartbeat moved unresolved_since: %v, want %v", got, *first)
	}
	nodes, err := repo.ListUnresolvedProductNodes(ctx, time.Now().Add(-time.Hour), 10)
	if err != nil || len(nodes) != 1 || !nodes[0].UnresolvedSince.Equal(*first) {
		t.Fatalf("listed %+v, err %v", nodes, err)
	}

	report("wso2am", "4.2.0")
	if got := since(); got != nil {
		t.Errorf("a resolved node keeps unresolved_since %v", got)
	}
	report("wso2zz", "1.0.0")
	if got := since(); got == nil || !got.After(*first) {
		t.Errorf("going unresolved again: %v, first %v", got, *first)
	}
}
