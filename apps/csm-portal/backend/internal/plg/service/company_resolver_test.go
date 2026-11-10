package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/cloudclient"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

const (
	orgMoesif  = "11111111-1111-1111-1111-111111111111"
	orgCloud   = "22222222-2222-2222-2222-222222222222"
	orgNoID    = "33333333-3333-3333-3333-333333333333"
	orgMissing = "44444444-4444-4444-4444-444444444444"
	orgNoOwner = "55555555-5555-5555-5555-555555555555"
)

type fakeIdentities map[string]domain.OrganizationIdentity

func (f fakeIdentities) Identities(_ context.Context, ids []string) ([]domain.OrganizationIdentity, error) {
	out := []domain.OrganizationIdentity{}
	for _, id := range ids {
		if it, ok := f[id]; ok {
			out = append(out, it)
		}
	}
	return out, nil
}

type fakeCloud struct {
	mu        sync.Mutex
	orgCalls  int
	userCalls int
	orgErr    error
	userErr   error
	org       cloudclient.Org
	user      cloudclient.UserEmail
}

func (f *fakeCloud) GetOrg(context.Context, string) (*cloudclient.Org, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orgCalls++
	if f.orgErr != nil {
		return nil, f.orgErr
	}
	o := f.org
	return &o, nil
}

func (f *fakeCloud) GetUserEmail(context.Context, string) (*cloudclient.UserEmail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userCalls++
	if f.userErr != nil {
		return nil, f.userErr
	}
	u := f.user
	return &u, nil
}

func strp(s string) *string { return &s }

func identities() fakeIdentities {
	return fakeIdentities{
		orgMoesif: {OrganizationID: orgMoesif, MoesifCompanyID: strp("m-1"),
			OrganizationName: strp("acme"), RegisteredEmail: strp("owner@acme.com")},
		orgCloud:   {OrganizationID: orgCloud, MoesifCompanyID: strp("cloud-1"), OwnerUserID: strp("user-1")},
		orgNoID:    {OrganizationID: orgNoID},
		orgNoOwner: {OrganizationID: orgNoOwner, MoesifCompanyID: strp("cloud-2")},
	}
}

func goodCloud() *fakeCloud {
	return &fakeCloud{
		org:  cloudclient.Org{UUID: "cloud-1", FriendlyName: "Acme Cloud", Handle: "acme"},
		user: cloudclient.UserEmail{UserUUID: "user-1", OrgUUID: "cloud-1", Email: "admin@acme.io"},
	}
}

func resolveOne(t *testing.T, r *CompanyResolver, id string) *domain.CompanyIdentity {
	t.Helper()
	res, err := r.Resolve(context.Background(), domain.ResolveCompaniesRequest{OrganizationIDs: []string{id}})
	if err != nil {
		t.Fatalf("resolve %s: %v", id, err)
	}
	if len(res.Identities) == 0 {
		return nil
	}
	return &res.Identities[0]
}

func TestCompanyResolver_StoredValuesNeedNoCloudCall(t *testing.T) {
	cloud := goodCloud()
	got := resolveOne(t, NewCompanyResolver(identities(), cloud), orgMoesif)
	if got.Source != domain.IdentityFromDatabase || *got.OrganizationName != "acme" || *got.Email != "owner@acme.com" {
		t.Errorf("got %+v", got)
	}
	if cloud.orgCalls != 0 {
		t.Errorf("a stored identity called the cloud %d times", cloud.orgCalls)
	}
}

func TestCompanyResolver_NameFromTheOrgAndEmailFromTheOwner(t *testing.T) {
	cloud := goodCloud()
	got := resolveOne(t, NewCompanyResolver(identities(), cloud), orgCloud)
	if got.Source != domain.IdentityFromCloud || *got.OrganizationName != "Acme Cloud" || *got.Email != "admin@acme.io" {
		t.Fatalf("got %+v", got)
	}
	if cloud.orgCalls != 1 || cloud.userCalls != 1 {
		t.Fatalf("want one org call and one user call, got %d and %d", cloud.orgCalls, cloud.userCalls)
	}
}

func TestCompanyResolver_NothingIsCached(t *testing.T) {
	cloud := goodCloud()
	r := NewCompanyResolver(identities(), cloud)
	resolveOne(t, r, orgCloud)
	resolveOne(t, r, orgCloud)
	if cloud.orgCalls != 2 || cloud.userCalls != 2 {
		t.Errorf("a second reveal reused an answer: %d org calls, %d user calls", cloud.orgCalls, cloud.userCalls)
	}
}

func TestCompanyResolver_NoOwnerGivesTheNameOnly(t *testing.T) {
	cloud := goodCloud()
	got := resolveOne(t, NewCompanyResolver(identities(), cloud), orgNoOwner)
	if got.Source != domain.IdentityFromCloud || *got.OrganizationName != "Acme Cloud" || got.Email != nil {
		t.Errorf("got %+v", got)
	}
	if cloud.userCalls != 0 {
		t.Errorf("an organisation with no owner asked for an email")
	}
}

func TestCompanyResolver_CloudNotFoundIsUnresolved(t *testing.T) {
	cloud := &fakeCloud{orgErr: cloudclient.ErrNotFound}
	got := resolveOne(t, NewCompanyResolver(identities(), cloud), orgCloud)
	if got.Source != domain.IdentityUnresolved || got.OrganizationName != nil {
		t.Errorf("got %+v", got)
	}
}

func TestCompanyResolver_FailureAndRateLimitAreFailed(t *testing.T) {
	for _, err := range []error{errors.New("status 503"), cloudclient.ErrRateLimited} {
		got := resolveOne(t, NewCompanyResolver(identities(), &fakeCloud{orgErr: err}), orgCloud)
		if got.Source != domain.IdentityFailed {
			t.Errorf("%v: got %+v", err, got)
		}
	}
}

func TestCompanyResolver_EmailFailureKeepsTheName(t *testing.T) {
	for _, err := range []error{cloudclient.ErrNotFound, errors.New("status 500")} {
		cloud := goodCloud()
		cloud.userErr = err
		got := resolveOne(t, NewCompanyResolver(identities(), cloud), orgCloud)
		if got.Source != domain.IdentityFromCloud || *got.OrganizationName != "Acme Cloud" || got.Email != nil {
			t.Errorf("%v: got %+v", err, got)
		}
	}
}

func TestCompanyResolver_OwnerOfAnotherOrgGivesNoEmail(t *testing.T) {
	cloud := goodCloud()
	cloud.user.OrgUUID = "someone-else"
	got := resolveOne(t, NewCompanyResolver(identities(), cloud), orgCloud)
	if got.Email != nil {
		t.Errorf("an email from a different org was returned")
	}
}

func TestCompanyResolver_UnknownOrganisationIsNeverRevealed(t *testing.T) {
	if got := resolveOne(t, NewCompanyResolver(identities(), goodCloud()), orgMissing); got != nil {
		t.Errorf("an unknown organisation was revealed: %+v", got)
	}
}

func TestCompanyResolver_NoCompanyIDOrNoCloud(t *testing.T) {
	if got := resolveOne(t, NewCompanyResolver(identities(), goodCloud()), orgNoID); got.Source != domain.IdentityUnresolved {
		t.Errorf("no company id: got %+v", got)
	}
	if got := resolveOne(t, NewCompanyResolver(identities(), nil), orgCloud); got.Source != domain.IdentityFailed {
		t.Errorf("no cloud configured: got %+v", got)
	}
}

func TestCompanyResolver_RefusesABadRequest(t *testing.T) {
	r := NewCompanyResolver(identities(), nil)
	many := make([]string, 51)
	for i := range many {
		many[i] = orgMoesif
	}
	for name, ids := range map[string][]string{
		"empty": nil, "too many": many, "not a uuid": {"org-1"},
	} {
		_, err := r.Resolve(context.Background(), domain.ResolveCompaniesRequest{OrganizationIDs: ids})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: want a validation error, got %v", name, err)
		}
	}
}

func TestCompanyResolver_RevealAnswersOneOrganisation(t *testing.T) {
	r := NewCompanyResolver(identities(), goodCloud())

	got, err := r.Reveal(context.Background(), orgMoesif)
	if err != nil || got.OrganizationID != orgMoesif || got.Source != domain.IdentityFromDatabase {
		t.Fatalf("Reveal = %+v, %v", got, err)
	}

	var nf *apierror.NotFoundError
	if _, err := r.Reveal(context.Background(), orgMissing); !errors.As(err, &nf) {
		t.Errorf("unknown organisation: err = %v, want not found", err)
	}
	var ve *apierror.ValidationError
	if _, err := r.Reveal(context.Background(), "not-a-uuid"); !errors.As(err, &ve) {
		t.Errorf("bad id: err = %v, want a validation error", err)
	}
}
