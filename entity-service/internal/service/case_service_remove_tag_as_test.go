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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// RemoveCaseTagAs is RemoveCaseTag with the actor given instead of read from the
// x-user-id-token: the same delete, the same ServiceNow mirror, the same errors.

func TestCaseService_RemoveCaseTagAs_NeedsNoTokenAndPassesTheActor(t *testing.T) {
	var gotActor string
	repo := &stubCaseRepo{removeCaseTag: func(_ context.Context, _, _, actor string) error { gotActor = actor; return nil }}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	// No token on the context: RemoveCaseTag would answer 401, and never reach the repository.
	if err := svc.RemoveCaseTag(context.Background(), testDeploymentUUID, testUUID); err == nil {
		t.Fatal("RemoveCaseTag without a token succeeded")
	}
	if err := svc.RemoveCaseTagAs(context.Background(), testDeploymentUUID, testUUID, "system-actor"); err != nil {
		t.Fatalf("RemoveCaseTagAs: %v", err)
	}
	if gotActor != "system-actor" {
		t.Errorf("actor = %q", gotActor)
	}
}

func TestCaseService_RemoveCaseTagAs_Validation(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{removeCaseTag: func(context.Context, string, string, string) error {
		t.Fatal("the repository was reached with an invalid id")
		return nil
	}}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	var ve *apierror.ValidationError
	if err := svc.RemoveCaseTagAs(context.Background(), "nope", testUUID, "a"); !errors.As(err, &ve) {
		t.Errorf("bad case id: %v", err)
	}
	if err := svc.RemoveCaseTagAs(context.Background(), testDeploymentUUID, "nope", "a"); !errors.As(err, &ve) {
		t.Errorf("bad tag id: %v", err)
	}
}

func TestCaseService_RemoveCaseTagAs_NotFoundPassesThrough(t *testing.T) {
	repo := &stubCaseRepo{removeCaseTag: func(context.Context, string, string, string) error {
		return &apierror.NotFoundError{Msg: "tag not found on this case"}
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	var nf *apierror.NotFoundError
	if err := svc.RemoveCaseTagAs(context.Background(), testDeploymentUUID, testUUID, "a"); !errors.As(err, &nf) {
		t.Errorf("err = %v, want not found", err)
	}
}

func TestCaseService_RemoveCaseTagAs_MirrorsToServiceNowLikeRemoveCaseTag(t *testing.T) {
	storedSysID := uuidToSysid(testSNSysID2)
	repo := &stubCaseRepo{
		removeCaseTag:     func(context.Context, string, string, string) error { return nil },
		getCaseTagSNSysID: func(context.Context, string, string) (*string, error) { return &storedSysID, nil },
	}
	type removeCall struct{ caseID, tagID string }
	called := make(chan removeCall, 1)
	mirror := &stubMirrorCaseService{removeCaseTagFn: func(_ context.Context, caseID, tagID string) error {
		called <- removeCall{caseID, tagID}
		return nil
	}}
	failures := &recordingSNWritebackFailures{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, NewSNWritebackDispatcher(failures), mirror, nil, "")

	if err := svc.RemoveCaseTagAs(context.Background(), testDeploymentUUID, testUUID, "system-actor"); err != nil {
		t.Fatalf("RemoveCaseTagAs: %v", err)
	}
	select {
	case got := <-called:
		if got.caseID != testDeploymentUUID || got.tagID != testSNSysID2 {
			t.Errorf("mirror call = %+v, want case %q tag %q", got, testDeploymentUUID, testSNSysID2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the ServiceNow mirror was not called")
	}
	if n := failures.count(); n != 0 {
		t.Errorf("writeback failures = %d", n)
	}
}
