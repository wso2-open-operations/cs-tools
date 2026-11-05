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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestCleanupUploads_MarksAbandonedRowsFailed(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	repo := newFakeStoredRepo()
	repo.abandoned = []domain.AbandonedUpload{
		{Kind: domain.StoredAttachmentWorkItem, ID: "11111111-1111-4111-8111-111111111111"},
		{Kind: domain.StoredAttachmentDeployment, ID: "33333333-3333-4333-8333-333333333333"},
	}
	svc := NewAttachmentUploadCleanupService(repo, alwaysUnrestrictedAccess{}).(*attachmentUploadCleanupService)
	svc.now = func() time.Time { return now }

	res, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.Found != 2 || res.Cleaned != 2 || res.Failed != 0 {
		t.Errorf("result = %+v, want found 2, cleaned 2, failed 0", res)
	}
	for _, id := range []string{"11111111-1111-4111-8111-111111111111", "33333333-3333-4333-8333-333333333333"} {
		if _, ok := repo.failed[id]; !ok {
			t.Errorf("the abandoned row %s was not marked FAILED", id)
		}
	}
	if want := now.Add(-abandonedUploadAge); !repo.abandonedAt.Equal(want) || !repo.abandonedSys {
		t.Errorf("listed with cutoff %v (system=%v), want %v as the system", repo.abandonedAt, repo.abandonedSys, want)
	}
}

func TestCleanupUploads_InternalCallersOnly(t *testing.T) {
	var forbidden *apierror.ForbiddenError
	svc := NewAttachmentUploadCleanupService(newFakeStoredRepo(), stubAccess{})
	if _, err := svc.CleanupUploads(context.Background()); !errors.As(err, &forbidden) {
		t.Errorf("a restricted caller: err = %v, want ForbiddenError", err)
	}
}
