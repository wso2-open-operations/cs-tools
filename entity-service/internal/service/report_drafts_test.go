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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func srReq(ids ...string) domain.CreateCaseRequest {
	return domain.CreateCaseRequest{Type: "security_report_analysis", ProjectID: saProjectID, AttachmentIDs: ids}
}

func TestValidateReportDrafts_SecurityReportNeedsAFile(t *testing.T) {
	repo := newFakeStoredRepo()
	svc := newStoredSvc(repo)

	t.Run("a user's security report with no file is refused", func(t *testing.T) {
		_, err := validateReportDrafts(saCtx(t, saEmail), svc, srReq())
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != reportRequiredMessage {
			t.Fatalf("err = %v, want the report-required ValidationError", err)
		}
	})
	t.Run("a user's security report with a stored draft passes and returns the author", func(t *testing.T) {
		repo.reportDrafts = []domain.StoredAttachmentRow{reportRow(saReportA)}
		email, err := validateReportDrafts(saCtx(t, saEmail), svc, srReq(saReportA))
		if err != nil || email != saEmail {
			t.Fatalf("email=%q err=%v, want %q and no error", email, err, saEmail)
		}
	})
	t.Run("a draft that is not the caller's is refused", func(t *testing.T) {
		repo.reportDrafts = nil
		if _, err := validateReportDrafts(saCtx(t, saEmail), svc, srReq(saReportA)); err == nil {
			t.Fatal("a file the caller did not upload for this project must be refused")
		}
	})
	t.Run("a machine caller is not held to the minimum", func(t *testing.T) {
		if _, err := validateReportDrafts(context.Background(), svc, srReq()); err != nil {
			t.Fatalf("err = %v, want none for a caller with no user token", err)
		}
	})
	t.Run("another type needs none", func(t *testing.T) {
		req := domain.CreateCaseRequest{Type: "case", ProjectID: saProjectID}
		if _, err := validateReportDrafts(saCtx(t, saEmail), svc, req); err != nil {
			t.Fatalf("err = %v, want none", err)
		}
	})
	t.Run("files with nothing to check them are refused, not silently dropped", func(t *testing.T) {
		if _, err := validateReportDrafts(saCtx(t, saEmail), nil, srReq(saReportA)); err == nil {
			t.Fatal("ids with no linker must be refused")
		}
	})
}

func TestLinkReportDrafts_LinksOnlyWhenThereAreFiles(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.reportLinkWant = []string{saReportA}
	svc := newStoredSvc(repo)

	linkReportDrafts(context.Background(), svc, saParentID, srReq(), saEmail)
	if repo.reportLinkCall.calls != 0 {
		t.Error("nothing to link, nothing should be called")
	}
	linkReportDrafts(context.Background(), svc, saParentID, srReq(saReportA), saEmail)
	if c := repo.reportLinkCall; c.calls != 1 || c.workItem != saParentID || c.email != saEmail {
		t.Errorf("repo call = %+v, want one link to the new record as the author", c)
	}
	// A failure is logged, never panics or returns: the record already exists.
	repo.reportLinkWant = nil
	linkReportDrafts(context.Background(), svc, saParentID, srReq(saReportA), saEmail)
}
