package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// identitiesRepo records what reached the repository; every other method is
// unused here.
type identitiesRepo struct {
	repository.OrganizationRepository
	got []string
}

func (r *identitiesRepo) Identities(_ context.Context, ids []string) ([]domain.OrganizationIdentity, error) {
	r.got = ids
	return []domain.OrganizationIdentity{{OrganizationID: ids[0]}}, nil
}

func TestOrganizationIdentities_ValidatesBeforeReadingAnything(t *testing.T) {
	many := make([]string, 51)
	for i := range many {
		many[i] = "11111111-1111-1111-1111-111111111111"
	}
	for name, ids := range map[string][]string{
		"empty": nil, "more than 50": many, "not a uuid": {"org-1"},
	} {
		repo := &identitiesRepo{}
		_, err := NewOrganizationService(repo).Identities(context.Background(),
			domain.OrganizationIdentitiesRequest{OrganizationIDs: ids})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: want a validation error, got %v", name, err)
		}
		if repo.got != nil {
			t.Errorf("%s: the repository was called", name)
		}
	}
}

func TestOrganizationIdentities_PassesValidIDsThrough(t *testing.T) {
	repo := &identitiesRepo{}
	id := "11111111-1111-1111-1111-111111111111"
	res, err := NewOrganizationService(repo).Identities(context.Background(),
		domain.OrganizationIdentitiesRequest{OrganizationIDs: []string{id}})
	if err != nil || len(res.Identities) != 1 || len(repo.got) != 1 {
		t.Fatalf("got %+v, %v", res, err)
	}
}
