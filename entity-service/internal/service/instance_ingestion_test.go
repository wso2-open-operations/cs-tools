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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// mockInstanceIngestionRepo records what the service handed it.
type mockInstanceIngestionRepo struct {
	calls int

	nodeID     string
	meta       domain.InstanceNodeMeta
	info       domain.InstanceInfoRequest
	usage      domain.InstanceUsageIngestRequest
	batch      domain.DailyUsageBatchRequest
	reportedOn time.Time
	countedOn  time.Time

	secret    string
	secretErr error
}

func (m *mockInstanceIngestionRepo) UpsertDeploymentNode(_ context.Context, nodeID string, meta domain.InstanceNodeMeta) (string, error) {
	m.calls++
	m.nodeID, m.meta = nodeID, meta
	return "node-uuid", nil
}

func (m *mockInstanceIngestionRepo) InsertDeploymentInformation(_ context.Context, req domain.InstanceInfoRequest, reportedOn time.Time) (string, error) {
	m.calls++
	m.info, m.reportedOn = req, reportedOn
	return "info-uuid", nil
}

func (m *mockInstanceIngestionRepo) InsertUsageStats(_ context.Context, req domain.InstanceUsageIngestRequest, countedOn time.Time) error {
	m.calls++
	m.usage, m.countedOn = req, countedOn
	return nil
}

func (m *mockInstanceIngestionRepo) ImportDailyUsagesBatch(_ context.Context, req domain.DailyUsageBatchRequest) (int, error) {
	m.calls++
	m.batch = req
	return len(req.Usages), nil
}

func (m *mockInstanceIngestionRepo) GetProjectSecretKey(_ context.Context, _ string) (string, error) {
	m.calls++
	return m.secret, m.secretErr
}

func (m *mockInstanceIngestionRepo) GetDeploymentConsumptionSecret(_ context.Context, number string) (domain.DeploymentConsumptionSecretResponse, error) {
	m.calls++
	return domain.DeploymentConsumptionSecretResponse{DeploymentNumber: number}, m.secretErr
}

func newTestIngestionService(repo *mockInstanceIngestionRepo) *instanceIngestionService {
	return &instanceIngestionService{
		repo: repo,
		now:  func() time.Time { return time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC) },
	}
}

func wantValidation(t *testing.T, err error, contains string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *apierror.ValidationError, got %T (%v)", err, err)
	}
	if !strings.Contains(ve.Msg, contains) {
		t.Errorf("message %q does not mention %q", ve.Msg, contains)
	}
}

func TestInstanceIngestion_MetaIsTrimmedAndPassedOn(t *testing.T) {
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).UpsertInstanceMeta(context.Background(), domain.InstanceMetaRequest{
		NodeID: " node-1 ",
		InstanceNodeMeta: domain.InstanceNodeMeta{
			ProjectKey: " PROJ ", DeploymentNumber: " DEP001 ", Product: " wso2am ", Version: " 4.2.0 ",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "node-uuid" {
		t.Errorf("id = %q", resp.ID)
	}
	want := domain.InstanceNodeMeta{ProjectKey: "PROJ", DeploymentNumber: "DEP001", Product: "wso2am", Version: "4.2.0"}
	if repo.nodeID != "node-1" || repo.meta != want {
		t.Errorf("repo got node %q meta %+v, want node-1 %+v", repo.nodeID, repo.meta, want)
	}
}

func TestInstanceIngestion_RejectsBeforeTheRepository(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	ctx := context.Background()
	cases := []struct {
		name     string
		run      func(InstanceIngestionService) error
		contains string
	}{
		{"meta without node", func(s InstanceIngestionService) error {
			_, err := s.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: "  "})
			return err
		}, "nodeId is required"},
		{"node id too long", func(s InstanceIngestionService) error {
			_, err := s.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: long(37)})
			return err
		}, "nodeId must be at most 36"},
		{"deployment number too long", func(s InstanceIngestionService) error {
			_, err := s.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: "n", InstanceNodeMeta: domain.InstanceNodeMeta{DeploymentNumber: long(51)}})
			return err
		}, "deploymentNumber"},
		{"usage without count type", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n"})
			return err
		}, "countType is required"},
		{"negative count", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n", CountType: "TPS", Count: -1})
			return err
		}, "count must be between"},
		{"count beyond INTEGER", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n", CountType: "TPS", Count: 1 << 31})
			return err
		}, "count must be between"},
		{"unreadable time", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n", CountType: "TPS", CreatedTime: "tomorrow"})
			return err
		}, "createdTime"},
		{"info time in no known layout", func(s InstanceIngestionService) error {
			_, err := s.InsertInstanceInfo(ctx, domain.InstanceInfoRequest{NodeID: "n", CreatedTime: "15/10/2026 10:00"})
			return err
		}, "createdTime"},
		{"batch without node", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{})
			return err
		}, "nodeId is required"},
		{"batch entry with a bad date", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Usages: []domain.DailyUsageBatchItem{
				{Date: "2026-10-15", Type: "CORES", Value: 1},
				{Date: "2026-13-01", Type: "CORES", Value: 1},
			}})
			return err
		}, "usages[1].date"},
		{"batch entry with no type", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Value: 1}}})
			return err
		}, "usages[0].type is required"},
		{"batch entry with a negative value", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Type: "CORES", Value: -5}}})
			return err
		}, "usages[0].value"},
		{"batch node without id", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Nodes: []domain.DailyUsageBatchNode{{NodeID: " "}}})
			return err
		}, "nodes[0].nodeId is required"},
		{"deployment secret without number", func(s InstanceIngestionService) error {
			_, err := s.GetDeploymentConsumptionSecret(ctx, " ")
			return err
		}, "deployment number is required"},
		{"deployment number too long", func(s InstanceIngestionService) error {
			_, err := s.GetDeploymentConsumptionSecret(ctx, long(51))
			return err
		}, "deployment number must be at most 50"},
		{"secret without key", func(s InstanceIngestionService) error {
			_, err := s.GetProjectSecretKey(ctx, " ")
			return err
		}, "project key is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockInstanceIngestionRepo{}
			wantValidation(t, tc.run(newTestIngestionService(repo)), tc.contains)
			if repo.calls != 0 {
				t.Errorf("the repository was called %d times for a refused request", repo.calls)
			}
		})
	}
}

func TestInstanceIngestion_TimesAreReadAsUTC(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-10-15 23:30:00", time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)},
		{"2026-10-16T05:00:00+05:30", time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)},
		{"2026-10-15T23:30:00Z", time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)},
		{"", time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		repo := &mockInstanceIngestionRepo{}
		if _, err := newTestIngestionService(repo).InsertUsageStats(context.Background(), domain.InstanceUsageIngestRequest{
			NodeID: "n", CountType: "TPS", Count: 3, CreatedTime: tc.in,
		}); err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !repo.countedOn.Equal(tc.want) || repo.countedOn.Location() != time.UTC {
			t.Errorf("%q: countedOn = %v, want %v (UTC)", tc.in, repo.countedOn, tc.want)
		}
	}
}

func TestInstanceIngestion_InfoWithoutHashIsHashedFromTheSnapshot(t *testing.T) {
	info := map[string]any{"jdkVersion": "17", "resourceProfile": map[string]any{"cpuCores": 8}}
	hashOf := func(v any) string {
		repo := &mockInstanceIngestionRepo{}
		if _, err := newTestIngestionService(repo).InsertInstanceInfo(context.Background(), domain.InstanceInfoRequest{NodeID: "n", DeploymentInfo: v}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return repo.info.Hash
	}
	a, b := hashOf(info), hashOf(info)
	if len(a) != 64 || a != b {
		t.Errorf("hash = %q / %q, want the same 64-character SHA-256 for the same snapshot", a, b)
	}
	if c := hashOf(map[string]any{"jdkVersion": "21"}); c == a {
		t.Error("a different snapshot got the same hash")
	}

	repo := &mockInstanceIngestionRepo{}
	if _, err := newTestIngestionService(repo).InsertInstanceInfo(context.Background(), domain.InstanceInfoRequest{NodeID: "n", Hash: " given ", DeploymentInfo: info}); err != nil {
		t.Fatal(err)
	}
	if repo.info.Hash != "given" {
		t.Errorf("a sent hash must be kept, got %q", repo.info.Hash)
	}
}

func TestInstanceIngestion_BatchIsTrimmedAndCounted(t *testing.T) {
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).ImportDailyUsagesBatch(context.Background(), domain.DailyUsageBatchRequest{
		NodeID:    "node-1",
		UserEmail: " someone@example.com ",
		Usages: []domain.DailyUsageBatchItem{
			{Date: " 2026-10-15 ", Type: " CORES ", Value: 8, NodeID: " node-2 ", DeploymentNumber: " DEP9 "},
			{Date: "2026-10-15", Type: "TRANSACTION_COUNT", Value: 500},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ImportedCount != 2 || resp.Status != "SUCCESS" {
		t.Errorf("response = %+v", resp)
	}
	got := repo.batch.Usages[0]
	if got.Date != "2026-10-15" || got.Type != "CORES" || got.NodeID != "node-2" || got.DeploymentNumber != "DEP9" {
		t.Errorf("entry not trimmed: %+v", got)
	}
	if repo.batch.UserEmail != "someone@example.com" {
		t.Errorf("userEmail = %q", repo.batch.UserEmail)
	}
}

func TestInstanceIngestion_SecretKeyErrorsPassThrough(t *testing.T) {
	repo := &mockInstanceIngestionRepo{secretErr: &apierror.NotFoundError{Msg: "no key"}}
	_, err := newTestIngestionService(repo).GetProjectSecretKey(context.Background(), "PROJ")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("want NotFoundError, got %T", err)
	}

	repo = &mockInstanceIngestionRepo{secret: "s3cret"}
	resp, err := newTestIngestionService(repo).GetProjectSecretKey(context.Background(), " PROJ ")
	if err != nil || resp.SecretKey != "s3cret" {
		t.Fatalf("resp = %+v, err = %v", resp, err)
	}
}
