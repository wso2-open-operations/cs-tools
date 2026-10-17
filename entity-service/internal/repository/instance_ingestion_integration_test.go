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
			}, countedOn)
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
	for _, c := range []int{100, 250} {
		if err := repo.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "node-u", CountType: "TRANSACTION_COUNT", Count: c, InstanceNodeMeta: meta}, countedOn); err != nil {
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
		"TRANSACTION_COUNT=250 DEP000001 2026-10-15 API_CALL",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("daily rows:\n got %v\nwant %v", got, want)
	}

	var hourly int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hourly_usage_summary WHERE counted_on = $1`, countedOn).Scan(&hourly); err != nil {
		t.Fatal(err)
	}
	if hourly != 3 {
		t.Fatalf("hourly rows at the counted instant = %d, want 3", hourly)
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
