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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeTeamRepo struct {
	exists     bool
	existsErr  error
	members    []repository.TeamMemberRow
	membersErr error

	teams      []domain.Team
	teamsTotal int
	teamsErr   error
	gotQuery   string
	gotLimit   int
	gotOffset  int

	chatWebhooks    []domain.TeamChatWebhook
	chatWebhooksErr error

	groupEmails    []domain.TeamGroupEmail
	groupEmailsErr error

	teamDetail domain.TeamDetail
	getTeamErr error

	setWebhookErr     error
	gotSetWebhookURL  *string
	gotSetWebhookCall bool

	assignLeadErr     error
	gotAssignLeadTeam string
	gotAssignLeadUser string
}

func (f *fakeTeamRepo) TeamExists(context.Context, string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeTeamRepo) GetTeamMembers(context.Context, string) ([]repository.TeamMemberRow, error) {
	return f.members, f.membersErr
}

func (f *fakeTeamRepo) SearchTeams(_ context.Context, q string, limit, offset int) ([]domain.Team, int, error) {
	f.gotQuery, f.gotLimit, f.gotOffset = q, limit, offset
	return f.teams, f.teamsTotal, f.teamsErr
}

func (f *fakeTeamRepo) ListChatWebhooks(context.Context) ([]domain.TeamChatWebhook, error) {
	return f.chatWebhooks, f.chatWebhooksErr
}

func (f *fakeTeamRepo) ListGroupEmails(context.Context) ([]domain.TeamGroupEmail, error) {
	return f.groupEmails, f.groupEmailsErr
}

func (f *fakeTeamRepo) GetTeamDetail(context.Context, string) (domain.TeamDetail, error) {
	if f.getTeamErr != nil {
		return domain.TeamDetail{}, f.getTeamErr
	}
	d := f.teamDetail
	// Reflects a prior SetChatWebhookURL call, mirroring the real repo's
	// read-your-writes behavior -- UpdateTeam re-reads via GetTeamDetail
	// after writing.
	if f.gotSetWebhookCall {
		d.ChatWebhookURL = f.gotSetWebhookURL
	}
	return d, nil
}

func (f *fakeTeamRepo) SetChatWebhookURL(_ context.Context, _ string, url *string) error {
	f.gotSetWebhookCall = true
	f.gotSetWebhookURL = url
	return f.setWebhookErr
}

func (f *fakeTeamRepo) AssignTeamLead(_ context.Context, teamID, userID string) error {
	f.gotAssignLeadTeam, f.gotAssignLeadUser = teamID, userID
	return f.assignLeadErr
}

const testTeamID = "11111111-1111-1111-1111-111111111111"

func TestTeamService_GetTeamMembers_RequiresAuthentication(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{exists: true}, alwaysUnrestrictedAccess{})

	_, err := svc.GetTeamMembers(contextWithUserIDToken(""), testTeamID)

	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnauthorizedError for a tokenless request, got %v", err)
	}
}

func TestTeamService_GetTeamMembers_NotFound(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{exists: false}, alwaysUnrestrictedAccess{})

	_, err := svc.GetTeamMembers(contextWithUserIDToken("tok"), testTeamID)

	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected NotFoundError for a missing team, got %v", err)
	}
}

func TestTeamService_GetTeamMembers_ReturnsRoster(t *testing.T) {
	role := "lead"
	email := "lead@wso2.com"
	repo := &fakeTeamRepo{
		exists: true,
		members: []repository.TeamMemberRow{
			{UserID: "u1", Name: "A Lead", Email: &email, Role: role},
		},
	}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.GetTeamMembers(contextWithUserIDToken("tok"), testTeamID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Members) != 1 || resp.Members[0].ID != "u1" || resp.Members[0].Role == nil || *resp.Members[0].Role != "lead" {
		t.Fatalf("unexpected members: %+v", resp.Members)
	}
}

func TestTeamService_SearchTeams_PassesQueryAndPagination(t *testing.T) {
	repo := &fakeTeamRepo{
		teams:      []domain.Team{{ID: testTeamID, Name: "apollo", Type: "cre"}},
		teamsTotal: 7,
	}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.SearchTeams(context.Background(), domain.SearchTeamsRequest{
		Filters:    &domain.SearchTeamsFilters{SearchQuery: "apo"},
		Pagination: domain.Pagination{Limit: 5, Offset: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotQuery != "apo" || repo.gotLimit != 5 || repo.gotOffset != 10 {
		t.Fatalf("repo got (%q, %d, %d), want (apo, 5, 10)", repo.gotQuery, repo.gotLimit, repo.gotOffset)
	}
	if resp.Total != 7 || len(resp.Teams) != 1 || resp.Teams[0].Name != "apollo" || resp.Limit != 5 || resp.Offset != 10 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestTeamService_SearchTeams_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewTeamService(&fakeTeamRepo{teamsErr: boom}, alwaysUnrestrictedAccess{})

	if _, err := svc.SearchTeams(context.Background(), domain.SearchTeamsRequest{}); !errors.Is(err, boom) {
		t.Fatalf("expected repo error, got %v", err)
	}
}

func TestTeamService_ListChatWebhooks_RequiresInternalCaller(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, restrictedAccess{})

	_, err := svc.ListChatWebhooks(context.Background())

	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError for a non-internal caller, got %v", err)
	}
}

func TestTeamService_ListChatWebhooks_ReturnsConfiguredTeams(t *testing.T) {
	repo := &fakeTeamRepo{
		chatWebhooks: []domain.TeamChatWebhook{
			{Name: "apollo", ChatWebhookURL: "https://chat.googleapis.com/v1/spaces/apollo"},
		},
	}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.ListChatWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Teams) != 1 || resp.Teams[0].Name != "apollo" || resp.Teams[0].ChatWebhookURL != "https://chat.googleapis.com/v1/spaces/apollo" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestTeamService_ListChatWebhooks_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewTeamService(&fakeTeamRepo{chatWebhooksErr: boom}, alwaysUnrestrictedAccess{})

	if _, err := svc.ListChatWebhooks(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("expected repo error, got %v", err)
	}
}

func TestTeamService_ListGroupEmails_RequiresInternalCaller(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, restrictedAccess{})

	_, err := svc.ListGroupEmails(context.Background())

	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError for a non-internal caller, got %v", err)
	}
}

func TestTeamService_ListGroupEmails_ReturnsConfiguredTeams(t *testing.T) {
	repo := &fakeTeamRepo{
		groupEmails: []domain.TeamGroupEmail{
			{Key: "americas_cre_team", GroupEmail: "americas-cre@example.test"},
		},
	}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.ListGroupEmails(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Teams) != 1 || resp.Teams[0].Key != "americas_cre_team" || resp.Teams[0].GroupEmail != "americas-cre@example.test" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestTeamService_ListGroupEmails_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewTeamService(&fakeTeamRepo{groupEmailsErr: boom}, alwaysUnrestrictedAccess{})

	if _, err := svc.ListGroupEmails(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("expected repo error, got %v", err)
	}
}

func TestTeamService_GetTeam_RequiresInternalCaller(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, restrictedAccess{})

	_, err := svc.GetTeam(context.Background(), testTeamID)

	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError for a non-internal caller, got %v", err)
	}
}

func TestTeamService_GetTeam_RejectsMalformedID(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, alwaysUnrestrictedAccess{})

	_, err := svc.GetTeam(context.Background(), "not-a-uuid")

	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError for a malformed id, got %v", err)
	}
}

func TestTeamService_GetTeam_ReturnsStoredValue(t *testing.T) {
	url := "https://chat.googleapis.com/v1/spaces/apollo"
	repo := &fakeTeamRepo{teamDetail: domain.TeamDetail{
		ID: testTeamID, Name: "Apollo", Type: "cre", ChatWebhookURL: &url,
	}}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.GetTeam(context.Background(), testTeamID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != testTeamID || resp.Name != "Apollo" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.ChatWebhookURL == nil || *resp.ChatWebhookURL != url {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestTeamService_UpdateTeam_RequiresInternalCaller(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, restrictedAccess{})

	url := "https://chat.googleapis.com/v1/spaces/apollo"
	_, err := svc.UpdateTeam(context.Background(), testTeamID, domain.UpdateTeamRequest{ChatWebhookURL: &url})

	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError for a non-internal caller, got %v", err)
	}
}

func TestTeamService_UpdateTeam_RejectsWhenNoFieldGiven(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, alwaysUnrestrictedAccess{})

	_, err := svc.UpdateTeam(context.Background(), testTeamID, domain.UpdateTeamRequest{})

	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError when no field is given, got %v", err)
	}
}

func TestTeamService_UpdateTeam_RejectsNonHTTPSURL(t *testing.T) {
	repo := &fakeTeamRepo{}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	bad := "http://example.com/webhook"
	_, err := svc.UpdateTeam(context.Background(), testTeamID, domain.UpdateTeamRequest{ChatWebhookURL: &bad})

	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError for a non-https url, got %v", err)
	}
	if repo.gotSetWebhookCall {
		t.Fatal("repo should not be called for an invalid url")
	}
}

func TestTeamService_UpdateTeam_RejectsNonChatGoogleapisHost(t *testing.T) {
	repo := &fakeTeamRepo{}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	for _, bad := range []string{
		"https://evil.example/v1/spaces/apollo",
		"https://chat.googleapis.com.evil.example/v1/spaces/apollo",
		"https://chat.googleapis.com@evil.example/v1/spaces/apollo",
		"https://169.254.169.254/latest/meta-data",
	} {
		t.Run(bad, func(t *testing.T) {
			bad := bad
			_, err := svc.UpdateTeam(context.Background(), testTeamID, domain.UpdateTeamRequest{ChatWebhookURL: &bad})
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError for %q, got %v", bad, err)
			}
			if repo.gotSetWebhookCall {
				t.Fatalf("repo should not be called for %q", bad)
			}
		})
	}
}

func TestTeamService_UpdateTeam_ClearsOnEmptyString(t *testing.T) {
	repo := &fakeTeamRepo{teamDetail: domain.TeamDetail{ID: testTeamID, Name: "Apollo", Type: "cre"}}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	blank := "   "
	resp, err := svc.UpdateTeam(context.Background(), testTeamID, domain.UpdateTeamRequest{ChatWebhookURL: &blank})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ChatWebhookURL != nil {
		t.Fatalf("expected a cleared webhook, got %+v", resp)
	}
	if repo.gotSetWebhookURL != nil {
		t.Fatalf("expected repo to be called with nil, got %v", *repo.gotSetWebhookURL)
	}
}

func TestTeamService_UpdateTeam_StoresTrimmedHTTPSURL(t *testing.T) {
	repo := &fakeTeamRepo{teamDetail: domain.TeamDetail{ID: testTeamID, Name: "Apollo", Type: "cre"}}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	padded := "  https://chat.googleapis.com/v1/spaces/apollo  "
	resp, err := svc.UpdateTeam(context.Background(), testTeamID, domain.UpdateTeamRequest{ChatWebhookURL: &padded})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://chat.googleapis.com/v1/spaces/apollo"
	if resp.ChatWebhookURL == nil || *resp.ChatWebhookURL != want {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if repo.gotSetWebhookURL == nil || *repo.gotSetWebhookURL != want {
		t.Fatalf("expected repo to be called with %q, got %v", want, repo.gotSetWebhookURL)
	}
}

func TestTeamService_AssignTeamLead_RequiresInternalCaller(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, restrictedAccess{})

	err := svc.AssignTeamLead(context.Background(), testTeamID, testTeamID)

	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError for a non-internal caller, got %v", err)
	}
}

func TestTeamService_AssignTeamLead_RejectsMalformedUserID(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{}, alwaysUnrestrictedAccess{})

	err := svc.AssignTeamLead(context.Background(), testTeamID, "not-a-uuid")

	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError for a malformed userId, got %v", err)
	}
}

func TestTeamService_AssignTeamLead_PassesThroughToRepo(t *testing.T) {
	const userID = "22222222-2222-2222-2222-222222222222"
	repo := &fakeTeamRepo{}
	svc := NewTeamService(repo, alwaysUnrestrictedAccess{})

	if err := svc.AssignTeamLead(context.Background(), testTeamID, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotAssignLeadTeam != testTeamID || repo.gotAssignLeadUser != userID {
		t.Fatalf("repo got (%q, %q), want (%q, %q)", repo.gotAssignLeadTeam, repo.gotAssignLeadUser, testTeamID, userID)
	}
}

func TestTeamService_AssignTeamLead_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewTeamService(&fakeTeamRepo{assignLeadErr: boom}, alwaysUnrestrictedAccess{})

	const userID = "22222222-2222-2222-2222-222222222222"
	if err := svc.AssignTeamLead(context.Background(), testTeamID, userID); !errors.Is(err, boom) {
		t.Fatalf("expected repo error, got %v", err)
	}
}
