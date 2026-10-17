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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestInstanceIngestionValidation checks the repository refuses a request it
// cannot key before it touches the database (the repository is built without one).
func TestInstanceIngestionValidation(t *testing.T) {
	ctx := context.Background()
	repo := NewInstanceIngestionRepository(nil)

	if _, err := repo.UpsertDeploymentNode(ctx, " ", domain.InstanceNodeMeta{}); err == nil {
		t.Error("UpsertDeploymentNode: want an error for an empty nodeId")
	}
	if _, err := repo.InsertDeploymentInformation(ctx, domain.InstanceInfoRequest{}, time.Now()); err == nil {
		t.Error("InsertDeploymentInformation: want an error for an empty nodeId")
	}
	if err := repo.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{}, time.Now()); err == nil {
		t.Error("InsertUsageStats: want an error for an empty nodeId")
	}
	if err := repo.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n"}, time.Now()); err == nil {
		t.Error("InsertUsageStats: want an error for an empty countType")
	}
	if _, err := repo.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{}); err == nil {
		t.Error("ImportDailyUsagesBatch: want an error for an empty nodeId")
	}
	if _, err := repo.GetProjectSecretKey(ctx, "  "); err == nil {
		t.Error("GetProjectSecretKey: want an error for a blank key")
	}
}

// TestInstanceIngestion_RefusesAContextWithNoIdentity pins that the repository
// no longer falls back to the system identity: a caller that reached it without
// the identity middleware gets ErrNoCallerIdentity, never unrestricted access.
func TestInstanceIngestion_RefusesAContextWithNoIdentity(t *testing.T) {
	repo := NewInstanceIngestionRepository(NewScoped(nil))
	_, err := repo.UpsertDeploymentNode(context.Background(), "node-1", domain.InstanceNodeMeta{})
	if err != ErrNoCallerIdentity {
		t.Fatalf("err = %v, want ErrNoCallerIdentity", err)
	}
	_, err = repo.GetProjectSecretKey(context.Background(), "PROJ")
	if err == nil {
		t.Fatal("GetProjectSecretKey with no identity: want an error")
	}
}
