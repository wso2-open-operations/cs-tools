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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
)

type fakeShareStore struct {
	shares  []filestore.Share
	deleted []string
	delErr  map[string]error
	listErr error
}

func (f *fakeShareStore) ListShares(_ context.Context, limit, offset int) ([]filestore.Share, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if offset >= len(f.shares) {
		return nil, nil
	}
	end := offset + limit
	if end > len(f.shares) {
		end = len(f.shares)
	}
	return f.shares[offset:end], nil
}

func (f *fakeShareStore) DeleteShare(_ context.Context, id string) error {
	if err := f.delErr[id]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func TestCleanupShares_RemovesOnlyOurExpiredOrSpentShares(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	ms := func(d time.Duration) int64 { return now.Add(d).UnixMilli() }
	store := &fakeShareStore{shares: []filestore.Share{
		{ID: "live", Name: "dl", ExpiresAt: ms(30 * time.Second), MaxTokens: 1},
		{ID: "just-expired", Name: "dl", ExpiresAt: ms(-time.Minute), MaxTokens: 1}, // inside the grace
		{ID: "old", Name: "dl", ExpiresAt: ms(-time.Hour), MaxTokens: 1},
		{ID: "spent", Name: "up", ExpiresAt: ms(10 * time.Minute), MaxTokens: 1, UsedTokens: 1},
		{ID: "not-ours", Name: "someone-elses", ExpiresAt: ms(-time.Hour), MaxTokens: 1},
		{ID: "no-expiry", Name: "dl", ExpiresAt: 0, MaxTokens: 0},
		{ID: "gone", Name: "dl", ExpiresAt: ms(-2 * time.Hour), MaxTokens: 1},
		{ID: "stuck", Name: "dl", ExpiresAt: ms(-2 * time.Hour), MaxTokens: 1},
	}, delErr: map[string]error{"gone": filestore.ErrNotFound, "stuck": errors.New("boom")}}
	svc := NewAttachmentShareCleanupService(store, alwaysUnrestrictedAccess{}).(*attachmentShareCleanupService)
	svc.now = func() time.Time { return now }

	res, err := svc.CleanupShares(context.Background())
	if err != nil {
		t.Fatalf("CleanupShares: %v", err)
	}
	want := map[string]bool{"old": true, "spent": true}
	if len(store.deleted) != 2 || !want[store.deleted[0]] || !want[store.deleted[1]] {
		t.Errorf("deleted = %v, want old and spent only", store.deleted)
	}
	if res.Scanned != 8 || res.Deleted != 3 || res.Failed != 1 {
		t.Errorf("result = %+v, want scanned 8, deleted 3 (a share already gone counts), failed 1", res)
	}
}

func TestCleanupShares_InternalCallersOnlyAndStorageErrorsAre503(t *testing.T) {
	var forbidden *apierror.ForbiddenError
	svc := NewAttachmentShareCleanupService(&fakeShareStore{}, stubAccess{})
	if _, err := svc.CleanupShares(context.Background()); !errors.As(err, &forbidden) {
		t.Errorf("a restricted caller: err = %v, want ForbiddenError", err)
	}
	var su *apierror.ServiceUnavailableError
	down := NewAttachmentShareCleanupService(&fakeShareStore{listErr: errors.New("down")}, alwaysUnrestrictedAccess{})
	if _, err := down.CleanupShares(context.Background()); !errors.As(err, &su) {
		t.Errorf("listing failure: err = %v, want ServiceUnavailableError", err)
	}
}
