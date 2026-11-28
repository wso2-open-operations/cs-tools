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

func closedFile(id, number, key string) domain.StoredAttachmentRow {
	return domain.StoredAttachmentRow{ID: id, Number: number, Name: number + ".pdf", Status: "COMPLETE", StorageKey: &key}
}

func newRetentionSvc(repo *fakeStoredRepo, blob *fakeBlob, now time.Time) *attachmentRetentionService {
	svc := NewAttachmentRetentionService(repo, alwaysUnrestrictedAccess{}, blob).(*attachmentRetentionService)
	svc.now = func() time.Time { return now }
	return svc
}

func TestExpire_RemovesTheFileThenMarksTheRow(t *testing.T) {
	now := time.Date(2026, 11, 12, 3, 0, 0, 0, time.UTC)
	repo := newFakeStoredRepo()
	repo.closedFiles = []domain.StoredAttachmentRow{closedFile("a1", "WIA1", "csm/a1"), closedFile("a2", "WIA2", "csm/a2")}
	blob := newFakeBlob()

	res, err := newRetentionSvc(repo, blob, now).ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 90})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Found != 2 || res.Expired != 2 || res.Failed != 0 || res.DryRun {
		t.Errorf("result = %+v", res)
	}
	if len(blob.deleted) != 2 || blob.deleted[0] != "csm/a1" || len(repo.expired) != 2 {
		t.Errorf("deleted = %v, expired = %v", blob.deleted, repo.expired)
	}
	if want := now.AddDate(0, 0, -90); !repo.closedBefore.Equal(want) || !repo.closedSys || repo.closedLimit != defaultRetentionLimit {
		t.Errorf("listed with cutoff %v (system=%v, limit=%d), want %v as the system with the default limit", repo.closedBefore, repo.closedSys, repo.closedLimit, want)
	}
}

func TestExpire_DryRunChangesNothing(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.closedFiles = []domain.StoredAttachmentRow{closedFile("a1", "WIA1", "csm/a1")}
	blob := newFakeBlob()
	res, err := newRetentionSvc(repo, blob, time.Now()).ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 90, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Expired != 1 || len(res.Attachments) != 1 || res.Attachments[0] != "WIA1" {
		t.Errorf("result = %+v, want the one attachment listed", res)
	}
	if len(blob.deleted) != 0 || len(repo.expired) != 0 {
		t.Error("a dry run removed something")
	}
}

// A file that cannot be removed keeps its row COMPLETE, so the next run tries it again.
func TestExpire_AFileThatCannotBeRemovedIsKept(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.closedFiles = []domain.StoredAttachmentRow{closedFile("a1", "WIA1", "csm/a1")}
	blob := newFakeBlob()
	blob.deleteErr = errors.New("storage is down")
	res, err := newRetentionSvc(repo, blob, time.Now()).ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 || res.Expired != 0 || len(repo.expired) != 0 {
		t.Errorf("result = %+v, expired = %v; want one failure and the row untouched", res, repo.expired)
	}
}

func TestExpire_RefusesTooShortARetention(t *testing.T) {
	var verr *apierror.ValidationError
	_, err := newRetentionSvc(newFakeStoredRepo(), newFakeBlob(), time.Now()).ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 7})
	if !errors.As(err, &verr) {
		t.Errorf("err = %v, want ValidationError", err)
	}
}

func TestExpire_InternalCallersOnly(t *testing.T) {
	var forbidden *apierror.ForbiddenError
	svc := NewAttachmentRetentionService(newFakeStoredRepo(), stubAccess{}, newFakeBlob())
	if _, err := svc.ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 90}); !errors.As(err, &forbidden) {
		t.Errorf("err = %v, want ForbiddenError", err)
	}
}

func TestExpire_NeedsStorage(t *testing.T) {
	var unavailable *apierror.ServiceUnavailableError
	svc := NewAttachmentRetentionService(newFakeStoredRepo(), alwaysUnrestrictedAccess{}, nil)
	if _, err := svc.ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 90}); !errors.As(err, &unavailable) {
		t.Errorf("err = %v, want ServiceUnavailableError", err)
	}
}

func TestExpire_TheLimitIsBounded(t *testing.T) {
	repo := newFakeStoredRepo()
	svc := newRetentionSvc(repo, newFakeBlob(), time.Now())
	for limit, want := range map[int]int{0: defaultRetentionLimit, 50: 50, 10000: maxRetentionLimit} {
		if _, err := svc.ExpireClosedWorkItemFiles(context.Background(), domain.ExpireAttachmentsRequest{ClosedForDays: 90, Limit: limit}); err != nil {
			t.Fatal(err)
		}
		if repo.closedLimit != want {
			t.Errorf("limit %d was asked as %d, want %d", limit, repo.closedLimit, want)
		}
	}
}
