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
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// memCaseCommentTemplateRepo keeps templates in memory and matches the owner
// the same way the Postgres repository does (owner_user_id IS NOT DISTINCT
// FROM the given owner), so the ownership tests below exercise the real rule.
type memCaseCommentTemplateRepo struct {
	rows   []memTemplateRow
	nextID int
}

type memTemplateRow struct {
	owner *string
	t     domain.CaseCommentTemplate
	actor string
}

func sameOwner(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *memCaseCommentTemplateRepo) List(_ context.Context, userID string) ([]domain.CaseCommentTemplate, error) {
	var out []domain.CaseCommentTemplate
	for _, r := range m.rows {
		if r.owner == nil || *r.owner == userID {
			out = append(out, r.t)
		}
	}
	return out, nil
}

func (m *memCaseCommentTemplateRepo) Count(_ context.Context, owner *string) (int, error) {
	n := 0
	for _, r := range m.rows {
		if sameOwner(r.owner, owner) {
			n++
		}
	}
	return n, nil
}

func (m *memCaseCommentTemplateRepo) Create(_ context.Context, owner *string, req domain.CreateCaseCommentTemplateRequest, actor string) (domain.CaseCommentTemplate, error) {
	m.nextID++
	t := domain.CaseCommentTemplate{
		ID:        fmt.Sprintf("00000000-0000-0000-0000-%012d", m.nextID),
		Name:      req.Name,
		Body:      req.Body,
		AppliesTo: req.AppliesTo,
		Shared:    owner == nil,
	}
	m.rows = append(m.rows, memTemplateRow{owner: owner, t: t, actor: actor})
	return t, nil
}

func (m *memCaseCommentTemplateRepo) Update(_ context.Context, id string, owner *string, req domain.UpdateCaseCommentTemplateRequest, actor string) (domain.CaseCommentTemplate, error) {
	for i, r := range m.rows {
		if r.t.ID == id && sameOwner(r.owner, owner) {
			if req.Name != nil {
				m.rows[i].t.Name = *req.Name
			}
			if req.Body != nil {
				m.rows[i].t.Body = *req.Body
			}
			if req.AppliesTo != nil {
				m.rows[i].t.AppliesTo = *req.AppliesTo
			}
			m.rows[i].actor = actor
			return m.rows[i].t, nil
		}
	}
	return domain.CaseCommentTemplate{}, &apierror.NotFoundError{Msg: "comment template not found"}
}

func (m *memCaseCommentTemplateRepo) Delete(_ context.Context, id string, owner *string) error {
	for i, r := range m.rows {
		if r.t.ID == id && sameOwner(r.owner, owner) {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return nil
		}
	}
	return &apierror.NotFoundError{Msg: "comment template not found"}
}

// templateUsers maps each test email to its own user id.
func templateUsers() stubUserRepo {
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: "user-" + strings.SplitN(email, "@", 2)[0], Email: email}, nil
		},
	}
}

func templateCtx(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
}

func newTemplateReq(name string, appliesTo domain.CommentTemplateAppliesTo) domain.CreateCaseCommentTemplateRequest {
	return domain.CreateCaseCommentTemplateRequest{Name: name, Body: "<p>Hello</p>", AppliesTo: appliesTo}
}

func TestCaseCommentTemplateService_PersonalTemplatesAreTheOwnersOnly(t *testing.T) {
	repo := &memCaseCommentTemplateRepo{}
	svc := NewCaseCommentTemplateService(repo, templateUsers())
	alice, bob := templateCtx("alice@example.com"), templateCtx("bob@example.com")

	mine, err := svc.CreatePersonal(alice, newTemplateReq("  Ack  ", domain.CommentTemplateAppliesToComment))
	if err != nil {
		t.Fatalf("CreatePersonal: %v", err)
	}
	if mine.Name != "Ack" || mine.Shared {
		t.Errorf("created %+v, want trimmed name and a personal template", mine)
	}
	if got := repo.rows[0].actor; got != "alice@example.com" {
		t.Errorf("recorded editor = %q, want the token's email", got)
	}

	list, err := svc.List(bob)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Templates) != 0 {
		t.Errorf("bob sees %d templates, want none of alice's", len(list.Templates))
	}

	name := "Hijacked"
	var nf *apierror.NotFoundError
	if _, err := svc.UpdatePersonal(bob, mine.ID, domain.UpdateCaseCommentTemplateRequest{Name: &name}); !errors.As(err, &nf) {
		t.Errorf("bob updating alice's template: err = %v, want not found", err)
	}
	if err := svc.DeletePersonal(bob, mine.ID); !errors.As(err, &nf) {
		t.Errorf("bob deleting alice's template: err = %v, want not found", err)
	}
	// The shared routes cannot reach a personal template either.
	if err := svc.DeleteShared(bob, mine.ID); !errors.As(err, &nf) {
		t.Errorf("shared delete of a personal template: err = %v, want not found", err)
	}
	if err := svc.DeletePersonal(alice, mine.ID); err != nil {
		t.Errorf("alice deleting her own template: %v", err)
	}
}

func TestCaseCommentTemplateService_SharedTemplatesAreSeenByEveryoneAndNotReachableAsPersonal(t *testing.T) {
	repo := &memCaseCommentTemplateRepo{}
	svc := NewCaseCommentTemplateService(repo, templateUsers())
	admin, engineer := templateCtx("admin@example.com"), templateCtx("eng@example.com")

	shared, err := svc.CreateShared(admin, newTemplateReq("Closing note", domain.CommentTemplateAppliesToBoth))
	if err != nil {
		t.Fatalf("CreateShared: %v", err)
	}
	if !shared.Shared {
		t.Errorf("created %+v, want a shared template", shared)
	}
	if _, err := svc.CreatePersonal(engineer, newTemplateReq("Mine", domain.CommentTemplateAppliesToWorkNote)); err != nil {
		t.Fatalf("CreatePersonal: %v", err)
	}

	list, err := svc.List(engineer)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Templates) != 2 {
		t.Fatalf("engineer sees %d templates, want own + shared", len(list.Templates))
	}

	var nf *apierror.NotFoundError
	if err := svc.DeletePersonal(engineer, shared.ID); !errors.As(err, &nf) {
		t.Errorf("personal delete of a shared template: err = %v, want not found", err)
	}
	body := "<p>Updated</p>"
	if _, err := svc.UpdateShared(admin, shared.ID, domain.UpdateCaseCommentTemplateRequest{Body: &body}); err != nil {
		t.Errorf("UpdateShared: %v", err)
	}
}

func TestCaseCommentTemplateService_Validation(t *testing.T) {
	svc := NewCaseCommentTemplateService(&memCaseCommentTemplateRepo{}, templateUsers())
	ctx := templateCtx("alice@example.com")
	long := strings.Repeat("x", domain.MaxCaseCommentTemplateNameLength+1)
	tooBig := strings.Repeat("x", domain.MaxCaseCommentTemplateBodyLength+1)

	cases := []struct {
		name string
		req  domain.CreateCaseCommentTemplateRequest
	}{
		{"blank name", domain.CreateCaseCommentTemplateRequest{Name: "  ", Body: "b", AppliesTo: "BOTH"}},
		{"long name", domain.CreateCaseCommentTemplateRequest{Name: long, Body: "b", AppliesTo: "BOTH"}},
		{"blank body", domain.CreateCaseCommentTemplateRequest{Name: "n", Body: " ", AppliesTo: "BOTH"}},
		{"body too big", domain.CreateCaseCommentTemplateRequest{Name: "n", Body: tooBig, AppliesTo: "BOTH"}},
		{"unknown type", domain.CreateCaseCommentTemplateRequest{Name: "n", Body: "b", AppliesTo: "both"}},
		{"missing type", domain.CreateCaseCommentTemplateRequest{Name: "n", Body: "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ve *apierror.ValidationError
			if _, err := svc.CreatePersonal(ctx, tc.req); !errors.As(err, &ve) {
				t.Errorf("err = %v, want a validation error", err)
			}
		})
	}

	var ve *apierror.ValidationError
	if _, err := svc.UpdatePersonal(ctx, "not-a-uuid", domain.UpdateCaseCommentTemplateRequest{}); !errors.As(err, &ve) {
		t.Errorf("bad id: err = %v, want a validation error", err)
	}
	if _, err := svc.UpdatePersonal(ctx, "00000000-0000-0000-0000-000000000001", domain.UpdateCaseCommentTemplateRequest{}); !errors.As(err, &ve) {
		t.Errorf("empty update: err = %v, want a validation error", err)
	}
}

func TestCaseCommentTemplateService_CapsPersonalTemplates(t *testing.T) {
	repo := &memCaseCommentTemplateRepo{}
	svc := NewCaseCommentTemplateService(repo, templateUsers())
	ctx := templateCtx("alice@example.com")
	for i := 0; i < domain.MaxPersonalCaseCommentTemplates; i++ {
		if _, err := svc.CreatePersonal(ctx, newTemplateReq(fmt.Sprintf("t%d", i), "BOTH")); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	var ve *apierror.ValidationError
	if _, err := svc.CreatePersonal(ctx, newTemplateReq("one too many", "BOTH")); !errors.As(err, &ve) {
		t.Errorf("err = %v, want the cap's validation error", err)
	}
	// Another user's cap is their own.
	if _, err := svc.CreatePersonal(templateCtx("bob@example.com"), newTemplateReq("t0", "BOTH")); err != nil {
		t.Errorf("bob's first template: %v", err)
	}
}

func TestCaseCommentTemplateService_RequiresAUserToken(t *testing.T) {
	svc := NewCaseCommentTemplateService(&memCaseCommentTemplateRepo{}, templateUsers())
	var ue *apierror.UnauthorizedError
	// A service credential alone (no user) cannot list, create a personal
	// template, or write a shared one: there is no owner or editor to record.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true})
	if _, err := svc.List(ctx); !errors.As(err, &ue) {
		t.Errorf("List: err = %v, want unauthorized", err)
	}
	if _, err := svc.CreateShared(ctx, newTemplateReq("n", "BOTH")); !errors.As(err, &ue) {
		t.Errorf("CreateShared: err = %v, want unauthorized", err)
	}
}
