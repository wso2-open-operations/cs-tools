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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
)

type fakeUploadStore struct {
	removed []string
	errFor  map[string]error
}

func (f *fakeUploadStore) RemoveDir(_ context.Context, dir string) error {
	if err := f.errFor[dir]; err != nil {
		return err
	}
	f.removed = append(f.removed, dir)
	return nil
}

func TestCleanupUploads_RemovesTheFolderAndMarksTheRowFailed(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	repo := newFakeStoredRepo()
	repo.abandoned = []domain.AbandonedUpload{
		{Kind: domain.StoredAttachmentWorkItem, ID: "11111111-1111-4111-8111-111111111111"},
		{Kind: domain.StoredAttachmentComment, ID: "22222222-2222-4222-8222-222222222222"},   // folder already gone
		{Kind: domain.StoredAttachmentDeployment, ID: "33333333-3333-4333-8333-333333333333"}, // folder cannot be removed
	}
	store := &fakeUploadStore{errFor: map[string]error{
		filestore.StagingDir("22222222-2222-4222-8222-222222222222"): filestore.ErrNotFound,
		filestore.StagingDir("33333333-3333-4333-8333-333333333333"): errors.New("boom"),
	}}
	svc := NewAttachmentUploadCleanupService(repo, store, alwaysUnrestrictedAccess{}).(*attachmentUploadCleanupService)
	svc.now = func() time.Time { return now }

	res, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.Found != 3 || res.Cleaned != 2 || res.Failed != 1 {
		t.Errorf("result = %+v, want found 3, cleaned 2, failed 1", res)
	}
	if _, ok := repo.failed["11111111-1111-4111-8111-111111111111"]; !ok {
		t.Error("the abandoned row was not marked FAILED")
	}
	if _, ok := repo.failed["22222222-2222-4222-8222-222222222222"]; !ok {
		t.Error("a folder that was already gone must not stop the row being marked FAILED")
	}
	if _, ok := repo.failed["33333333-3333-4333-8333-333333333333"]; ok {
		t.Error("a row whose folder could not be removed must stay PENDING to be tried again")
	}
	if want := now.Add(-abandonedUploadAge); !repo.abandonedAt.Equal(want) || !repo.abandonedSys {
		t.Errorf("listed with cutoff %v (system=%v), want %v as the system", repo.abandonedAt, repo.abandonedSys, want)
	}
}

func TestCleanupUploads_InternalCallersOnly(t *testing.T) {
	var forbidden *apierror.ForbiddenError
	svc := NewAttachmentUploadCleanupService(newFakeStoredRepo(), &fakeUploadStore{}, stubAccess{})
	if _, err := svc.CleanupUploads(context.Background()); !errors.As(err, &forbidden) {
		t.Errorf("a restricted caller: err = %v, want ForbiddenError", err)
	}
}
