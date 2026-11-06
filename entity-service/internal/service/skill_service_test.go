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
	"slices"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	skUserID    = "11111111-1111-4111-8111-111111111111"
	skSkillID   = "22222222-2222-4222-8222-222222222222"
	skOtherID   = "22222222-2222-4222-8222-222222222223"
	skNoScaleID = "22222222-2222-4222-8222-222222222224"
	skInactive  = "22222222-2222-4222-8222-222222222225"
	skScaleA    = "33333333-3333-4333-8333-333333333331"
	skScaleB    = "33333333-3333-4333-8333-333333333332"
	skLevelA1   = "44444444-4444-4444-8444-444444444441"
	skLevelA2   = "44444444-4444-4444-8444-444444444442"
	skLevelB1   = "44444444-4444-4444-8444-444444444443"
	skCategory  = "55555555-5555-4555-8555-555555555551"
	skEmail     = "jane.doe@example.com"
)

// fakeSkillRepo answers the reads from its fields and records the writes. The
// outcomes the real transaction would find (already held, inherited, ...) are
// set by the test.
type fakeSkillRepo struct {
	users  map[string]string // email -> platform user id
	lead   bool
	skills map[string]*domain.Skill
	levels map[string]string // level -> scale
	cats   map[string]bool
	scales map[string]bool

	addOutcome, setOutcome, removeOutcome domain.SkillOutcome
	createOutcome, updateOutcome          domain.SkillOutcome

	resolvedEmails []string
	leadChecks     int
	writes         []string // "add:<user>:<skill>:<level>:<actor>", ...
	lastSearch     domain.SearchUserSkillsRequest
	lastCreate     domain.CreateSkillRequest
	lastUpdate     domain.UpdateSkillRequest
}

func newFakeSkillRepo() *fakeSkillRepo {
	return &fakeSkillRepo{
		users: map[string]string{skEmail: skUserID},
		skills: map[string]*domain.Skill{
			skSkillID:   {ID: skSkillID, Name: "Kubernetes", Active: true, LevelTypeID: ptr(skScaleA)},
			skOtherID:   {ID: skOtherID, Name: "Terraform", Active: true, LevelTypeID: ptr(skScaleB)},
			skNoScaleID: {ID: skNoScaleID, Name: "Legacy", Active: true},
			skInactive:  {ID: skInactive, Name: "Retired", Active: false, LevelTypeID: ptr(skScaleA)},
		},
		levels: map[string]string{skLevelA1: skScaleA, skLevelA2: skScaleA, skLevelB1: skScaleB},
		cats:   map[string]bool{skCategory: true},
		scales: map[string]bool{skScaleA: true, skScaleB: true},
	}
}

func ptr[T any](v T) *T { return &v }

func (f *fakeSkillRepo) ResolveUserID(_ context.Context, email string) (string, bool, error) {
	f.resolvedEmails = append(f.resolvedEmails, email)
	id, ok := f.users[email]
	return id, ok, nil
}
func (f *fakeSkillRepo) IsCRESRELead(context.Context, string) (bool, error) {
	f.leadChecks++
	return f.lead, nil
}
func (f *fakeSkillRepo) ListLevelTypes(context.Context) ([]domain.SkillLevelType, error) {
	return []domain.SkillLevelType{{ID: skScaleA}}, nil
}
func (f *fakeSkillRepo) ListCategories(context.Context) ([]domain.SkillCategory, error) {
	return []domain.SkillCategory{{ID: skCategory}}, nil
}
func (f *fakeSkillRepo) SearchSkills(context.Context, domain.SearchSkillsRequest) ([]domain.Skill, int, error) {
	return []domain.Skill{}, 0, nil
}
func (f *fakeSkillRepo) GetSkill(_ context.Context, id string) (*domain.Skill, error) {
	return f.skills[id], nil
}
func (f *fakeSkillRepo) LevelTypeOfLevel(_ context.Context, id string) (string, bool, error) {
	t, ok := f.levels[id]
	return t, ok, nil
}
func (f *fakeSkillRepo) LevelTypeExists(_ context.Context, id string) (bool, error) {
	return f.scales[id], nil
}
func (f *fakeSkillRepo) CategoriesExist(_ context.Context, ids []string) (bool, error) {
	for _, id := range ids {
		if !f.cats[id] {
			return false, nil
		}
	}
	return true, nil
}
func (f *fakeSkillRepo) ListUserSkills(context.Context, string) ([]domain.UserSkill, error) {
	return []domain.UserSkill{}, nil
}
func (f *fakeSkillRepo) row(skill, level string) domain.SkillWriteResult {
	return domain.SkillWriteResult{Row: &domain.UserSkill{SkillID: skill, Level: &domain.SkillLevel{ID: level}}}
}
func (f *fakeSkillRepo) AddUserSkill(_ context.Context, user, skill, level, actor string) (domain.SkillWriteResult, error) {
	f.writes = append(f.writes, strings.Join([]string{"add", user, skill, level, actor}, ":"))
	r := f.row(skill, level)
	r.Outcome = f.addOutcome
	return r, nil
}
func (f *fakeSkillRepo) SetUserSkillLevel(_ context.Context, user, skill, level, actor string) (domain.SkillWriteResult, error) {
	f.writes = append(f.writes, strings.Join([]string{"set", user, skill, level, actor}, ":"))
	r := f.row(skill, level)
	r.Outcome = f.setOutcome
	return r, nil
}
func (f *fakeSkillRepo) RemoveUserSkill(_ context.Context, user, skill, actor string) (domain.SkillWriteResult, error) {
	f.writes = append(f.writes, strings.Join([]string{"remove", user, skill, actor}, ":"))
	return domain.SkillWriteResult{Outcome: f.removeOutcome}, nil
}
func (f *fakeSkillRepo) ListUserSkillHistory(context.Context, string, string, int, int) ([]domain.SkillHistoryEntry, int, error) {
	return []domain.SkillHistoryEntry{}, 0, nil
}
func (f *fakeSkillRepo) SearchUserSkills(_ context.Context, req domain.SearchUserSkillsRequest) ([]domain.UserSkillRow, int, error) {
	f.lastSearch = req
	return []domain.UserSkillRow{}, 0, nil
}
func (f *fakeSkillRepo) CreateSkill(_ context.Context, req domain.CreateSkillRequest, actor string) (domain.SkillWriteResult, error) {
	f.lastCreate = req
	f.writes = append(f.writes, "create:"+actor)
	return domain.SkillWriteResult{Outcome: f.createOutcome, Skill: &domain.Skill{ID: skOtherID, Name: req.Name}}, nil
}
func (f *fakeSkillRepo) UpdateSkill(_ context.Context, id string, req domain.UpdateSkillRequest, actor string) (domain.SkillWriteResult, error) {
	f.lastUpdate = req
	f.writes = append(f.writes, "update:"+id+":"+actor)
	return domain.SkillWriteResult{Outcome: f.updateOutcome, Skill: &domain.Skill{ID: id}}, nil
}

func newSkillSvc(repo *fakeSkillRepo) SkillService {
	return NewSkillService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"})
}

// asCaller is a validated user token. The identity provider's own user id is
// set to something that is not the platform id, to show it is never used.
func asCaller(email string, roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{
		Validated: true, UserEmail: email, UserID: "idp-subject-not-a-platform-id", Roles: roles,
	})
}

func wantConflictErr(t *testing.T, err error, what string) {
	t.Helper()
	var c *apierror.ConflictError
	if !errors.As(err, &c) {
		t.Fatalf("%s: want ConflictError, got %v", what, err)
	}
}

func wantNotFoundErr(t *testing.T, err error, what string) {
	t.Helper()
	var n *apierror.NotFoundError
	if !errors.As(err, &n) {
		t.Fatalf("%s: want NotFoundError, got %v", what, err)
	}
}

func TestOwnProfileAddChangeRemove(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail)

	got, err := svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID, LevelID: skLevelA1})
	if err != nil || got.SkillID != skSkillID {
		t.Fatalf("AddMySkill = %+v, %v", got, err)
	}
	if _, err := svc.SetMySkillLevel(ctx, skSkillID, domain.SetMySkillLevelRequest{LevelID: skLevelA2}); err != nil {
		t.Fatalf("SetMySkillLevel: %v", err)
	}
	if err := svc.RemoveMySkill(ctx, skSkillID); err != nil {
		t.Fatalf("RemoveMySkill: %v", err)
	}
	want := []string{
		"add:" + skUserID + ":" + skSkillID + ":" + skLevelA1 + ":" + skEmail,
		"set:" + skUserID + ":" + skSkillID + ":" + skLevelA2 + ":" + skEmail,
		"remove:" + skUserID + ":" + skSkillID + ":" + skEmail,
	}
	if !slices.Equal(repo.writes, want) {
		t.Fatalf("writes = %v, want %v", repo.writes, want)
	}
	// The caller is found by the token's email, and the platform user id
	// that came back is what every write used, not the identity provider's.
	for _, e := range repo.resolvedEmails {
		if e != skEmail {
			t.Fatalf("resolved %q, want the token's email", e)
		}
	}
}

func TestOwnProfileRefusesALevelOutsideTheSkillsScale(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail)

	// A real level, of the other scale.
	_, err := svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID, LevelID: skLevelB1})
	wantInvalid(t, err, "add with a level of another scale")
	_, err = svc.SetMySkillLevel(ctx, skSkillID, domain.SetMySkillLevelRequest{LevelID: skLevelB1})
	wantInvalid(t, err, "change to a level of another scale")
	// Not a level at all.
	_, err = svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID, LevelID: "99999999-9999-4999-8999-999999999999"})
	wantInvalid(t, err, "add with an unknown level")
	_, err = svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID, LevelID: "nope"})
	wantInvalid(t, err, "add with a malformed level")
	_, err = svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID})
	wantInvalid(t, err, "add with no level")
	// A skill with no scale cannot take any level.
	_, err = svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skNoScaleID, LevelID: skLevelA1})
	wantConflictErr(t, err, "add a skill with no scale")
	if len(repo.writes) != 0 {
		t.Fatalf("a refused request wrote %v", repo.writes)
	}
}

func TestOwnProfileDuplicateAddIsRefused(t *testing.T) {
	repo := newFakeSkillRepo()
	repo.addOutcome = domain.SkillOutcomeAlreadyHeld
	_, err := newSkillSvc(repo).AddMySkill(asCaller(skEmail), domain.AddMySkillRequest{SkillID: skSkillID, LevelID: skLevelA1})
	wantConflictErr(t, err, "second add of the same skill")
}

func TestOwnProfileRefusesAnInactiveSkill(t *testing.T) {
	repo := newFakeSkillRepo()
	_, err := newSkillSvc(repo).AddMySkill(asCaller(skEmail), domain.AddMySkillRequest{SkillID: skInactive, LevelID: skLevelA1})
	wantConflictErr(t, err, "add an inactive skill")
	if len(repo.writes) != 0 {
		t.Fatalf("an inactive skill was written: %v", repo.writes)
	}
	// An unknown skill is a 404, not a 409.
	_, err = newSkillSvc(repo).AddMySkill(asCaller(skEmail), domain.AddMySkillRequest{SkillID: "99999999-9999-4999-8999-999999999999", LevelID: skLevelA1})
	wantNotFoundErr(t, err, "add an unknown skill")
}

func TestOwnProfileInheritedRowIsImmutable(t *testing.T) {
	repo := newFakeSkillRepo()
	repo.addOutcome, repo.setOutcome, repo.removeOutcome = domain.SkillOutcomeInherited, domain.SkillOutcomeInherited, domain.SkillOutcomeInherited
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail)
	_, err := svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID, LevelID: skLevelA1})
	wantConflictErr(t, err, "add over an inherited row")
	_, err = svc.SetMySkillLevel(ctx, skSkillID, domain.SetMySkillLevelRequest{LevelID: skLevelA2})
	wantConflictErr(t, err, "change an inherited row")
	wantConflictErr(t, svc.RemoveMySkill(ctx, skSkillID), "remove an inherited row")
}

func TestOwnProfileSkillNotHeldIs404(t *testing.T) {
	repo := newFakeSkillRepo()
	repo.setOutcome, repo.removeOutcome = domain.SkillOutcomeNotHeld, domain.SkillOutcomeNotHeld
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail)
	_, err := svc.SetMySkillLevel(ctx, skSkillID, domain.SetMySkillLevelRequest{LevelID: skLevelA2})
	wantNotFoundErr(t, err, "change a skill not held")
	wantNotFoundErr(t, svc.RemoveMySkill(ctx, skSkillID), "remove a skill not held")
}

func TestOwnProfileNeedsAnInternalUserWithAToken(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)

	// A service credential: no user token.
	_, err := svc.GetMySkills(auth.WithIdentity(context.Background(), auth.Identity{Validated: true}))
	wantForbidden(t, err, "no user token")
	// A token whose email has no active internal user record.
	_, err = svc.GetMySkills(asCaller("nobody@example.com"))
	wantForbidden(t, err, "unknown user")
	// Somebody who is not internal at all.
	denied := NewSkillService(repo, stubAccess{}, []string{"cs_admin"})
	_, err = denied.GetMySkills(asCaller(skEmail))
	wantForbidden(t, err, "not internal")
	if len(repo.writes) != 0 {
		t.Fatalf("wrote %v", repo.writes)
	}
}

func TestManagerReadNeedsAnAdminOrACREorSRELead(t *testing.T) {
	req := domain.SearchUserSkillsRequest{TeamKeys: []string{" Vega ", ""}}

	// Neither: refused, and the filters never reach the repository.
	repo := newFakeSkillRepo()
	_, err := newSkillSvc(repo).SearchUserSkills(asCaller(skEmail), req)
	wantForbidden(t, err, "an ordinary engineer")

	// A service credential is refused even though it is internal.
	_, err = newSkillSvc(repo).SearchUserSkills(auth.WithIdentity(context.Background(), auth.Identity{Validated: true, Roles: []string{"cs_admin"}}), req)
	wantForbidden(t, err, "a service credential")

	// A team lead.
	repo = newFakeSkillRepo()
	repo.lead = true
	if _, err := newSkillSvc(repo).SearchUserSkills(asCaller(skEmail), req); err != nil {
		t.Fatalf("a team lead: %v", err)
	}
	if !slices.Equal(repo.lastSearch.TeamKeys, []string{"vega"}) {
		t.Fatalf("team keys reached the repository as %v, want [vega]", repo.lastSearch.TeamKeys)
	}

	// A CSM admin, who need not lead anything.
	repo = newFakeSkillRepo()
	if _, err := newSkillSvc(repo).SearchUserSkills(asCaller(skEmail, "cs_admin"), req); err != nil {
		t.Fatalf("a CSM admin: %v", err)
	}
	// Only the exact admin role counts.
	_, err = newSkillSvc(repo).SearchUserSkills(asCaller(skEmail, "CS_ADMIN"), req)
	wantForbidden(t, err, "a near-miss role")
}

func TestManagerSearchValidatesItsFilters(t *testing.T) {
	repo := newFakeSkillRepo()
	repo.lead = true
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail)
	_, err := svc.SearchUserSkills(ctx, domain.SearchUserSkillsRequest{SkillIDs: []string{"nope"}})
	wantInvalid(t, err, "a malformed skill id")
	_, err = svc.SearchUserSkills(ctx, domain.SearchUserSkillsRequest{MinLevelValue: ptr(-1)})
	wantInvalid(t, err, "a negative minimum")
	_, err = svc.SearchUserSkills(ctx, domain.SearchUserSkillsRequest{Pagination: domain.Pagination{Limit: 201}})
	wantInvalid(t, err, "a page over the cap")
	if _, err := svc.SearchUserSkills(ctx, domain.SearchUserSkillsRequest{}); err != nil || repo.lastSearch.Pagination.Limit != skillDefaultLimit {
		t.Fatalf("default page: limit %d, err %v", repo.lastSearch.Pagination.Limit, err)
	}
}

func TestCatalogueWritesAreForACSMAdminOnly(t *testing.T) {
	repo := newFakeSkillRepo()
	repo.lead = true // a team lead still may not edit the catalogue
	svc := newSkillSvc(repo)
	create := domain.CreateSkillRequest{Name: "Go", LevelTypeID: skScaleA}

	_, err := svc.CreateSkill(asCaller(skEmail), create)
	wantForbidden(t, err, "a team lead creating a skill")
	_, err = svc.UpdateSkill(asCaller(skEmail), skSkillID, domain.UpdateSkillRequest{Active: ptr(false)})
	wantForbidden(t, err, "a team lead deactivating a skill")
	_, err = svc.CreateSkill(auth.WithIdentity(context.Background(), auth.Identity{Validated: true, Roles: []string{"cs_admin"}}), create)
	wantForbidden(t, err, "a service credential")
	if len(repo.writes) != 0 {
		t.Fatalf("a refused write reached the repository: %v", repo.writes)
	}

	admin := asCaller(skEmail, "cs_admin")
	got, err := svc.CreateSkill(admin, domain.CreateSkillRequest{Name: "  Go  ", LevelTypeID: skScaleA, CategoryIDs: []string{skCategory, skCategory}})
	if err != nil || got.ID == "" {
		t.Fatalf("admin create = %+v, %v", got, err)
	}
	if repo.lastCreate.Name != "Go" || len(repo.lastCreate.CategoryIDs) != 1 {
		t.Fatalf("create reached the repository as %+v, want a trimmed name and one category", repo.lastCreate)
	}
	if _, err := svc.UpdateSkill(admin, skSkillID, domain.UpdateSkillRequest{Active: ptr(false)}); err != nil {
		t.Fatalf("admin deactivate: %v", err)
	}
	if !slices.Contains(repo.writes, "update:"+skSkillID+":"+skEmail) {
		t.Fatalf("writes = %v", repo.writes)
	}
}

func TestCatalogueWriteValidation(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	admin := asCaller(skEmail, "cs_admin")
	ok := domain.CreateSkillRequest{Name: "Go", LevelTypeID: skScaleA}

	bad := ok
	bad.Name = "   "
	_, err := svc.CreateSkill(admin, bad)
	wantInvalid(t, err, "blank name")
	bad = ok
	bad.LevelTypeID = ""
	_, err = svc.CreateSkill(admin, bad)
	wantInvalid(t, err, "no level scale")
	bad = ok
	bad.LevelTypeID = "99999999-9999-4999-8999-999999999999"
	_, err = svc.CreateSkill(admin, bad)
	wantInvalid(t, err, "unknown level scale")
	bad = ok
	bad.CategoryIDs = []string{"99999999-9999-4999-8999-999999999999"}
	_, err = svc.CreateSkill(admin, bad)
	wantInvalid(t, err, "unknown category")
	bad = ok
	bad.Name = strings.Repeat("x", skillNameMax+1)
	_, err = svc.CreateSkill(admin, bad)
	wantInvalid(t, err, "long name")

	_, err = svc.UpdateSkill(admin, skSkillID, domain.UpdateSkillRequest{})
	wantInvalid(t, err, "an empty patch")
	_, err = svc.UpdateSkill(admin, "nope", domain.UpdateSkillRequest{Active: ptr(true)})
	wantInvalid(t, err, "a malformed id")
	if len(repo.writes) != 0 {
		t.Fatalf("invalid writes reached the repository: %v", repo.writes)
	}

	repo.createOutcome = domain.SkillOutcomeNameTaken
	_, err = svc.CreateSkill(admin, ok)
	wantConflictErr(t, err, "a name already taken")
	repo.updateOutcome = domain.SkillOutcomeLevelsInUse
	_, err = svc.UpdateSkill(admin, skSkillID, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleB)})
	wantConflictErr(t, err, "changing the scale of a skill people hold")
	repo.updateOutcome = domain.SkillOutcomeNotFound
	_, err = svc.UpdateSkill(admin, skSkillID, domain.UpdateSkillRequest{Active: ptr(true)})
	wantNotFoundErr(t, err, "an unknown skill")
}

func TestCatalogueReadsAreForInternalCallersOnly(t *testing.T) {
	repo := newFakeSkillRepo()
	denied := NewSkillService(repo, stubAccess{}, []string{"cs_admin"})
	ctx := asCaller(skEmail)
	_, err := denied.ListLevelTypes(ctx)
	wantForbidden(t, err, "levels")
	_, err = denied.ListCategories(ctx)
	wantForbidden(t, err, "categories")
	_, err = denied.SearchSkills(ctx, domain.SearchSkillsRequest{})
	wantForbidden(t, err, "search")
	_, err = denied.GetSkill(ctx, skSkillID)
	wantForbidden(t, err, "get")

	svc := newSkillSvc(repo)
	if _, err := svc.GetSkill(ctx, skSkillID); err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	_, err = svc.GetSkill(ctx, "99999999-9999-4999-8999-999999999999")
	wantNotFoundErr(t, err, "an unknown skill")
	_, err = svc.SearchSkills(ctx, domain.SearchSkillsRequest{Pagination: domain.Pagination{Limit: 500}})
	wantInvalid(t, err, "a page over the cap")
}

// A client may send a UUID in upper case. What reaches the repository is the
// lower-case form the database returns, so a comparison against the stored
// scale (UpdateSkill's "is it changing?") and the per-skill lock key agree.
func TestCatalogueWritesStoreIdsInLowerCase(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	admin := asCaller(skEmail, "cs_admin")
	upper := strings.ToUpper(skScaleA)
	upperCat := strings.ToUpper(skCategory)

	if _, err := svc.CreateSkill(admin, domain.CreateSkillRequest{Name: "Go", LevelTypeID: upper, CategoryIDs: []string{upperCat}}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.lastCreate.LevelTypeID != skScaleA {
		t.Fatalf("create stored levelTypeId %q, want %q", repo.lastCreate.LevelTypeID, skScaleA)
	}
	if got := repo.lastCreate.CategoryIDs; len(got) != 1 || got[0] != skCategory {
		t.Fatalf("create stored categoryIds %v", got)
	}

	req := domain.UpdateSkillRequest{LevelTypeID: &upper}
	if _, err := svc.UpdateSkill(admin, strings.ToUpper(skSkillID), req); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.lastUpdate.LevelTypeID == nil || *repo.lastUpdate.LevelTypeID != skScaleA {
		t.Fatalf("update stored levelTypeId %v, want %q", repo.lastUpdate.LevelTypeID, skScaleA)
	}
	if want := "update:" + skSkillID + ":"; !strings.HasPrefix(repo.writes[len(repo.writes)-1], want) {
		t.Fatalf("update id reached the repository as %q, want it lower-case", repo.writes[len(repo.writes)-1])
	}

	// Removal keys a lock on the skill id, so it must be the canonical text.
	if err := svc.RemoveMySkill(asCaller(skEmail), strings.ToUpper(skSkillID)); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if want := "remove:" + skUserID + ":" + skSkillID + ":"; !strings.HasPrefix(repo.writes[len(repo.writes)-1], want) {
		t.Fatalf("remove reached the repository as %q", repo.writes[len(repo.writes)-1])
	}
}

// The scale can be swapped between the service's check and the write; the
// repository then refuses, and the caller gets a 409 (not-found: 404).
func TestOwnProfileMapsAScaleSwappedUnderTheWrite(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	ctx := asCaller(skEmail)

	repo.addOutcome = domain.SkillOutcomeLevelNotInScale
	_, err := svc.AddMySkill(ctx, domain.AddMySkillRequest{SkillID: skSkillID, LevelID: skLevelA1})
	wantConflictErr(t, err, "add after the scale was swapped")
	repo.setOutcome = domain.SkillOutcomeLevelNotInScale
	_, err = svc.SetMySkillLevel(ctx, skSkillID, domain.SetMySkillLevelRequest{LevelID: skLevelA1})
	wantConflictErr(t, err, "change after the scale was swapped")
	repo.setOutcome = domain.SkillOutcomeNotFound
	_, err = svc.SetMySkillLevel(ctx, skSkillID, domain.SetMySkillLevelRequest{LevelID: skLevelA1})
	wantNotFoundErr(t, err, "change of a skill deleted under the write")
}

// A skill deactivated between the service's check and the write's lock comes
// back from the repository as SkillInactive, which is the same 409 as the
// service's own inactive check.
func TestOwnProfileMapsASkillDeactivatedUnderTheWrite(t *testing.T) {
	repo := newFakeSkillRepo()
	svc := newSkillSvc(repo)
	repo.addOutcome = domain.SkillOutcomeSkillInactive
	_, err := svc.AddMySkill(asCaller(skEmail), domain.AddMySkillRequest{SkillID: skSkillID, LevelID: skLevelA1})
	wantConflictErr(t, err, "add after the skill was deactivated")
}
