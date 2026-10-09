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

// Regression guard for the create-time assignment group: the CSM Portal's
// create form sends its "Assignment group" picker as
// CreateChangeRequestRequest.GroupID, and both Postgres create paths
// (CreateChangeRequest and CreateChangeRequestFromServiceNow) used to drop it
// on the floor -- the change request read back with no AssignedTeam. Skipped
// without CHANGE_REQUEST_TEST_DSN, same as change_request_repo_integration_test.go,
// whose seededGroupID/unknownGroupID this reuses.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestCreateIntegration

package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	crCreateSubject = "cr-create-assignment-group integration test"
	// crCreateSNID/crCreateSNNumber stand in for the identity ServiceNow
	// would have returned on the dual-write SN-first path.
	crCreateSNID     = "36666666-0000-0000-0000-0000000000c1"
	crCreateSNNumber = "CRAGTEST001"
)

// crCreateType returns a pointer to t -- a type is mandatory on every create.
func crCreateType(t domain.ChangeRequestType) *domain.ChangeRequestType { return &t }

func changeRequestCreatePool(t *testing.T) *repository.Scoped {
	t.Helper()
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	scoped := repository.NewScoped(pool)

	sys := repository.WithSystemIdentity(context.Background())
	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE subject = $1 OR id = $2`, crCreateSubject, crCreateSNID)
	}
	cleanup()
	t.Cleanup(cleanup)
	return scoped
}

// assertChangeRequestAssignedTeam checks work_item.assignment_group_id
// directly and through GetChangeRequestByID (the read the portal's detail
// page actually makes). want == "" means no assignment group at all.
func assertChangeRequestAssignedTeam(t *testing.T, scoped *repository.Scoped, repo repository.ChangeRequestRepository, id, want string) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())

	var got *string
	if err := scoped.QueryRow(sys, `SELECT assignment_group_id::TEXT FROM work_item WHERE id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read back assignment_group_id: %v", err)
	}
	if want == "" {
		if got != nil {
			t.Fatalf("work_item.assignment_group_id = %q, want NULL", *got)
		}
	} else if got == nil || *got != want {
		t.Fatalf("work_item.assignment_group_id = %v, want %q", got, want)
	}

	cr, err := repo.GetChangeRequestByID(sys, id)
	if err != nil {
		t.Fatalf("GetChangeRequestByID: %v", err)
	}
	if want == "" {
		if cr.AssignedTeam != nil {
			t.Fatalf("GetChangeRequestByID AssignedTeam = %+v, want nil", cr.AssignedTeam)
		}
		return
	}
	if cr.AssignedTeam == nil || cr.AssignedTeam.ID != want {
		t.Fatalf("GetChangeRequestByID AssignedTeam = %+v, want ID %q", cr.AssignedTeam, want)
	}
	if cr.AssignedTeam.Name == "" {
		t.Fatalf("GetChangeRequestByID AssignedTeam.Name is empty, want the seeded group's name")
	}
}

func TestChangeRequestCreateIntegration_PortalPersistsAssignmentGroup(t *testing.T) {
	scoped := changeRequestCreatePool(t)
	repo := repository.NewChangeRequestRepository(scoped)
	sys := repository.WithSystemIdentity(context.Background())

	groupID := seededGroupID
	resp, err := repo.CreateChangeRequest(sys, domain.CreateChangeRequestRequest{
		Subject: crCreateSubject,
		Type:    crCreateType(domain.ChangeRequestTypeNormal),
		GroupID: &groupID,
	}, "cr-create-test@test.local")
	if err != nil {
		t.Fatalf("CreateChangeRequest: %v", err)
	}
	assertChangeRequestAssignedTeam(t, scoped, repo, resp.ChangeRequest.ID, groupID)
}

func TestChangeRequestCreateIntegration_PortalWithoutAssignmentGroupLeavesItNull(t *testing.T) {
	scoped := changeRequestCreatePool(t)
	repo := repository.NewChangeRequestRepository(scoped)
	sys := repository.WithSystemIdentity(context.Background())

	resp, err := repo.CreateChangeRequest(sys, domain.CreateChangeRequestRequest{Subject: crCreateSubject, Type: crCreateType(domain.ChangeRequestTypeNormal)}, "cr-create-test@test.local")
	if err != nil {
		t.Fatalf("CreateChangeRequest: %v", err)
	}
	assertChangeRequestAssignedTeam(t, scoped, repo, resp.ChangeRequest.ID, "")
}

func TestChangeRequestCreateIntegration_PortalUnknownAssignmentGroupIsValidationError(t *testing.T) {
	scoped := changeRequestCreatePool(t)
	repo := repository.NewChangeRequestRepository(scoped)
	sys := repository.WithSystemIdentity(context.Background())

	groupID := unknownGroupID
	_, err := repo.CreateChangeRequest(sys, domain.CreateChangeRequestRequest{
		Subject: crCreateSubject,
		Type:    crCreateType(domain.ChangeRequestTypeNormal),
		GroupID: &groupID,
	}, "cr-create-test@test.local")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("CreateChangeRequest(unknown groupId) err = %v (%T), want *apierror.ValidationError", err, err)
	}
}

func TestChangeRequestCreateIntegration_FromServiceNowPersistsAssignmentGroup(t *testing.T) {
	scoped := changeRequestCreatePool(t)
	repo := repository.NewChangeRequestRepository(scoped)
	sys := repository.WithSystemIdentity(context.Background())

	groupID := seededGroupID
	resp, err := repo.CreateChangeRequestFromServiceNow(sys, domain.CreateChangeRequestRequest{
		Subject: crCreateSubject,
		Type:    crCreateType(domain.ChangeRequestTypeNormal),
		GroupID: &groupID,
	}, crCreateSNID, crCreateSNNumber, "cr-create-test@test.local")
	if err != nil {
		t.Fatalf("CreateChangeRequestFromServiceNow: %v", err)
	}
	if resp.ChangeRequest.ID != crCreateSNID {
		t.Fatalf("CreateChangeRequestFromServiceNow id = %q, want %q", resp.ChangeRequest.ID, crCreateSNID)
	}
	assertChangeRequestAssignedTeam(t, scoped, repo, crCreateSNID, groupID)
}

// A group id that is not a row of "group" (a stale id from a form opened before the group
// search listed real groups, or a direct API call) is refused before anything is written, in
// words the person on the form can act on: why, and which field to change; no id, no
// field name, no table.
func TestChangeRequestCreateIntegration_AGroupThatIsNotAServiceNowGroupIsRefusedInWords(t *testing.T) {
	scoped := changeRequestCreatePool(t)
	repo := repository.NewChangeRequestRepository(scoped)
	sys := repository.WithSystemIdentity(context.Background())

	const notAGroupID = "dddddddd-0000-4000-8000-0000000cc001"

	check := func(t *testing.T, err error) {
		t.Helper()
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
		}
		for _, want := range []string{"cannot be used", "not an assignment group in ServiceNow", `"Assignment group"`} {
			if !strings.Contains(ve.Msg, want) {
				t.Errorf("message %q does not contain %q", ve.Msg, want)
			}
		}
		for _, bad := range []string{notAGroupID, "groupId", "assignment_group_id", "table", "SQLSTATE"} {
			if strings.Contains(ve.Msg, bad) {
				t.Errorf("message %q shows %q to the person on the form", ve.Msg, bad)
			}
		}
	}

	t.Run("the pre-flight the dual-write create runs before ServiceNow", func(t *testing.T) {
		g := notAGroupID
		_, err := repo.ValidateChangeRequestLinks(sys, domain.ChangeRequestLinkSelection{AssignmentGroupID: &g})
		check(t, err)
	})
	t.Run("the plain PostgreSQL create", func(t *testing.T) {
		g := notAGroupID
		_, err := repo.CreateChangeRequest(sys, domain.CreateChangeRequestRequest{
			Subject: crCreateSubject, Type: crCreateType(domain.ChangeRequestTypeNormal), GroupID: &g,
		}, "cr-create-test@test.local")
		check(t, err)
	})
	t.Run("nothing was written", func(t *testing.T) {
		var n int
		if err := scoped.QueryRow(sys, `SELECT COUNT(*) FROM work_item WHERE subject = $1`, crCreateSubject).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		if n != 0 {
			t.Errorf("%d work items were written for a refused create", n)
		}
	})
}
