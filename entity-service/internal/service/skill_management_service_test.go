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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// testSkillCapRoles gives each capability its own role, so a test can show
// that one role grants one task and nothing else.
var testSkillCapRoles = SkillCapabilityRoles{
	domain.SkillCapDirectory:       {"skill_directory_viewer"},
	domain.SkillCapReviewRequests:  {"skill_reviewer"},
	domain.SkillCapManageCatalogue: {"skill_curator"},
}

var errNotFaked = errors.New("not faked")

// unimplementedSkillManagement is embedded in fakeSkillRepo: every management
// call it does not override fails loudly.
type unimplementedSkillManagement struct{}

func (unimplementedSkillManagement) DefaultLevelTypeID(context.Context) (string, bool, error) {
	return skScaleA, true, nil
}
func (unimplementedSkillManagement) CreateCategory(context.Context, domain.CreateSkillCategoryRequest, string) (*domain.SkillCategory, domain.SkillOutcome, error) {
	return nil, 0, errNotFaked
}
func (unimplementedSkillManagement) UpdateCategory(context.Context, string, domain.UpdateSkillCategoryRequest, string) (*domain.SkillCategory, domain.SkillOutcome, error) {
	return nil, 0, errNotFaked
}
func (unimplementedSkillManagement) CheckSkillRequest(context.Context, string) (domain.CheckSkillRequestResponse, error) {
	return domain.CheckSkillRequestResponse{Similar: []domain.SkillRef{{ID: "a", Name: "Go", Active: true}, {ID: "b", Name: "Golang", Active: false}}}, nil
}
func (unimplementedSkillManagement) CreateSkillRequest(_ context.Context, userID, norm string, req domain.CreateSkillRequestRequest, _ string) (*domain.SkillRequest, domain.SkillOutcome, error) {
	return &domain.SkillRequest{ID: "r1", RequestedName: req.Name, RequestedBy: domain.SkillPersonRef{ID: userID}}, domain.SkillOutcomeDone, nil
}
func (unimplementedSkillManagement) GetSkillRequest(_ context.Context, id string) (*domain.SkillRequest, error) {
	return &domain.SkillRequest{ID: id, RequestedName: "Ballerina", Status: domain.SkillRequestPending,
		Category: &domain.SkillCategoryRef{ID: skCategory}}, nil
}
func (unimplementedSkillManagement) ListSkillRequests(context.Context, string, []domain.SkillRequestStatus, int, int) ([]domain.SkillRequest, int, error) {
	return []domain.SkillRequest{}, 0, nil
}
func (unimplementedSkillManagement) WithdrawSkillRequest(context.Context, string, string, string) (*domain.SkillRequest, domain.SkillOutcome, error) {
	return nil, 0, errNotFaked
}
func (unimplementedSkillManagement) RejectSkillRequest(_ context.Context, id, _, reason, _ string) (*domain.SkillRequest, domain.SkillOutcome, error) {
	return &domain.SkillRequest{ID: id, Status: domain.SkillRequestRejected, ReviewComment: &reason}, domain.SkillOutcomeDone, nil
}
func (unimplementedSkillManagement) GetUserSkillProfile(context.Context, string) (*domain.UserSkillProfileResponse, error) {
	return nil, nil
}

func (f *fakeSkillRepo) CountPendingSkillRequests(context.Context) (int, error) {
	return f.pendingCount, nil
}
func (f *fakeSkillRepo) ApproveSkillRequest(_ context.Context, a domain.SkillApproval) (*domain.SkillRequest, domain.SkillOutcome, error) {
	f.lastApproval = a
	return &domain.SkillRequest{ID: a.RequestID, Status: domain.SkillRequestApproved}, domain.SkillOutcomeDone, nil
}
func (f *fakeSkillRepo) SearchSkillDirectory(_ context.Context, req domain.SearchSkillDirectoryRequest) ([]domain.SkillDirectoryPerson, int, error) {
	f.lastDirectory = req
	return []domain.SkillDirectoryPerson{}, 0, nil
}

func TestCapabilitiesFollowTheConfiguredRoles(t *testing.T) {
	repo := newFakeSkillRepo()
	repo.pendingCount = 3
	svc := newSkillSvc(repo)

	cases := []struct {
		name               string
		roles              []string
		dir, review, admin bool
	}{
		{"an engineer", nil, false, false, false},
		{"an admin", []string{"cs_admin"}, true, true, true},
		{"a directory viewer", []string{"skill_directory_viewer"}, true, false, false},
		{"a reviewer", []string{"skill_reviewer"}, false, true, false},
		{"a curator", []string{"skill_curator"}, false, false, true},
	}
	for _, c := range cases {
		got, err := svc.GetMyCapabilities(asCaller(skEmail, c.roles...))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.CanViewDirectory != c.dir || got.CanReviewRequests != c.review || got.CanManageCatalogue != c.admin {
			t.Fatalf("%s: got %+v", c.name, got)
		}
		if c.review != (got.PendingRequestCount != nil) {
			t.Fatalf("%s: pending count %v", c.name, got.PendingRequestCount)
		}
	}

	// With no roles configured, only admins hold any capability.
	adminsOnly := NewSkillService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, nil)
	got, err := adminsOnly.GetMyCapabilities(asCaller(skEmail, "skill_directory_viewer", "skill_reviewer", "skill_curator"))
	if err != nil || got.CanViewDirectory || got.CanReviewRequests || got.CanManageCatalogue {
		t.Fatalf("unconfigured roles granted something: %+v %v", got, err)
	}
}

func TestInactiveCatalogueIsForCatalogueManagersOnly(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)

	_, _ = svc.SearchSkills(asCaller(skEmail), domain.SearchSkillsRequest{IncludeInactive: true})
	if repo.lastSkillSearch.IncludeInactive || repo.lastSkillSearch.WithHolderCounts {
		t.Fatalf("an engineer's search reached the repository as %+v", repo.lastSkillSearch)
	}
	_, _ = svc.ListCategories(asCaller(skEmail))
	if repo.categoriesWithInactive {
		t.Fatal("an engineer was sent inactive categories")
	}

	_, _ = svc.SearchSkills(asCaller(skEmail, "skill_curator"), domain.SearchSkillsRequest{IncludeInactive: true})
	if !repo.lastSkillSearch.IncludeInactive || !repo.lastSkillSearch.WithHolderCounts {
		t.Fatalf("a curator's search reached the repository as %+v", repo.lastSkillSearch)
	}
	_, _ = svc.ListCategories(asCaller(skEmail, "skill_curator"))
	if !repo.categoriesWithInactive {
		t.Fatal("a curator was not sent inactive categories")
	}

	// Similar names: an engineer is not told about inactive skills.
	resp, err := svc.CheckSkillRequest(asCaller(skEmail), domain.CheckSkillRequestRequest{Name: "go"})
	if err != nil || len(resp.Similar) != 1 || !resp.Similar[0].Active {
		t.Fatalf("engineer's similar list: %+v %v", resp.Similar, err)
	}
}

func TestSkillRequestValidation(t *testing.T) {
	svc := newSkillSvc(newFakeSkillRepo())
	ctx := asCaller(skEmail)

	_, err := svc.CreateSkillRequest(ctx, domain.CreateSkillRequestRequest{Name: "Ballerina"})
	wantInvalid(t, err, "no level")
	_, err = svc.CreateSkillRequest(ctx, domain.CreateSkillRequestRequest{Name: "Ballerina", LevelID: skLevelB1})
	wantInvalid(t, err, "a level of another scale")
	_, err = svc.CreateSkillRequest(ctx, domain.CreateSkillRequestRequest{Name: "  ", LevelID: skLevelA1})
	wantInvalid(t, err, "a blank name")
	got, err := svc.CreateSkillRequest(ctx, domain.CreateSkillRequestRequest{Name: "  Ballerina ", LevelID: skLevelA1})
	if err != nil || got.RequestedName != "Ballerina" || got.RequestedBy.ID != skUserID {
		t.Fatalf("create: %+v %v", got, err)
	}
	if normalizeSkillName("  Apache   Kafka ") != "apache kafka" {
		t.Fatal("normalizeSkillName")
	}
}

func TestReviewingNeedsTheReviewerCapability(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)

	_, err := svc.ApproveSkillRequest(asCaller(skEmail, "skill_curator"), skSkillID, domain.ApproveSkillRequestRequest{Mode: "create"})
	wantForbidden(t, err, "a curator approving")
	_, err = svc.RejectSkillRequest(asCaller(skEmail), skSkillID, domain.RejectSkillRequestRequest{Reason: "no"})
	wantForbidden(t, err, "an engineer rejecting")

	reviewer := asCaller(skEmail, "skill_reviewer")
	_, err = svc.RejectSkillRequest(reviewer, skSkillID, domain.RejectSkillRequestRequest{Reason: "  "})
	wantInvalid(t, err, "a reject without a reason")
	_, err = svc.ApproveSkillRequest(reviewer, skSkillID, domain.ApproveSkillRequestRequest{Mode: "merge"})
	wantInvalid(t, err, "an unknown mode")
	_, err = svc.ApproveSkillRequest(reviewer, skSkillID, domain.ApproveSkillRequestRequest{Mode: "existing"})
	wantInvalid(t, err, "existing without a skill")

	// Create defaults to the requested name and category.
	if _, err := svc.ApproveSkillRequest(reviewer, skSkillID, domain.ApproveSkillRequestRequest{Mode: "create"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	a := repo.lastApproval
	if a.Name != "Ballerina" || a.CategoryID == nil || *a.CategoryID != skCategory || a.ReviewerID != skUserID || a.Actor != skEmail {
		t.Fatalf("approval reached the repository as %+v", a)
	}
}

func TestDirectoryCriteriaAreCleaned(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail, "skill_directory_viewer")

	_, err := svc.SearchSkillDirectory(asCaller(skEmail, "skill_reviewer"), domain.SearchSkillDirectoryRequest{})
	wantForbidden(t, err, "a reviewer searching the directory")
	_, err = svc.SearchSkillDirectory(ctx, domain.SearchSkillDirectoryRequest{Match: "SOME"})
	wantInvalid(t, err, "an unknown match")
	_, err = svc.SearchSkillDirectory(ctx, domain.SearchSkillDirectoryRequest{Criteria: []domain.SkillCriterion{{SkillID: skSkillID, MinLevelValue: -1}}})
	wantInvalid(t, err, "a negative minimum")

	_, err = svc.SearchSkillDirectory(ctx, domain.SearchSkillDirectoryRequest{
		Match:    "any",
		TeamKeys: []string{" Vega "},
		Criteria: []domain.SkillCriterion{{SkillID: skSkillID, MinLevelValue: 1}, {SkillID: skSkillID, MinLevelValue: 3}, {SkillID: skOtherID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	d := repo.lastDirectory
	if d.Match != "ANY" || len(d.Criteria) != 2 || d.Criteria[0].MinLevelValue != 3 || d.TeamKeys[0] != "vega" || d.Pagination.Limit != skillDirectoryDefaultLimit {
		t.Fatalf("directory search reached the repository as %+v", d)
	}
}

func TestUpdateSkillKeepsACategoryItIsAlreadyFiledUnder(t *testing.T) {
	const parent = "55555555-5555-4555-8555-555555555552"
	repo := newFakeSkillRepo()
	repo.cats[parent] = true
	repo.notLeaf = map[string]bool{parent: true}
	// A migrated skill filed under a parent category.
	repo.skills[skSkillID].Categories = []domain.SkillCategoryRef{{ID: parent, Name: "Engineering"}}
	svc := newSkillSvc(repo)
	curator := asCaller(skEmail, "skill_curator")

	// Keeping it while adding a leaf is accepted.
	if _, err := svc.UpdateSkill(curator, skSkillID, domain.UpdateSkillRequest{CategoryIDs: &[]string{parent, skCategory}}); err != nil {
		t.Fatalf("keeping an existing category was refused: %v", err)
	}
	// Filing another skill under it anew is not.
	_, err := svc.UpdateSkill(curator, skOtherID, domain.UpdateSkillRequest{CategoryIDs: &[]string{parent}})
	wantInvalid(t, err, "newly filing under a parent category")
	_, err = svc.CreateSkill(curator, domain.CreateSkillRequest{Name: "Rust", LevelTypeID: skScaleA, CategoryIDs: []string{parent}})
	wantInvalid(t, err, "creating a skill under a parent category")
}
