package paging

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeTeamMembers struct {
	members []teamMember
	err     error
	keys    []string
	roles   []string
}

func (f *fakeTeamMembers) TeamMembers(_ context.Context, teamKeys, roles, _, _ []string) ([]teamMember, error) {
	f.keys, f.roles = teamKeys, roles
	return f.members, f.err
}

func TestCREHeads_ConfiguredHeadWinsWithoutALookup(t *testing.T) {
	entity := &fakeTeamMembers{}
	h := CREHeads{head: Person{Name: "Head", Email: " Head@Example.com "}, leadership: defaultLeadershipTeamKey, entity: entity}

	got, err := h.CREHeadEmails(context.Background())
	if err != nil || !slices.Equal(got, []string{"head@example.com"}) {
		t.Fatalf("CREHeadEmails() = %v, %v; want [head@example.com], nil", got, err)
	}
	if entity.keys != nil {
		t.Error("entity-service was asked although heads.cre is configured")
	}
}

func TestCREHeads_FallsBackToTheLeadershipTeamsCREHeadRole(t *testing.T) {
	entity := &fakeTeamMembers{members: []teamMember{
		{Email: "A@example.com"}, {Email: "a@example.com"}, {Email: ""}, {Email: "b@example.com"},
	}}
	h := CREHeads{leadership: "leaders", entity: entity}

	got, err := h.CREHeadEmails(context.Background())
	if err != nil || !slices.Equal(got, []string{"a@example.com", "b@example.com"}) {
		t.Fatalf("CREHeadEmails() = %v, %v; want [a@example.com b@example.com], nil", got, err)
	}
	if !slices.Equal(entity.keys, []string{"leaders"}) || !slices.Equal(entity.roles, []string{roleCREHead}) {
		t.Errorf("asked teams %v roles %v, want [leaders] [%s]", entity.keys, entity.roles, roleCREHead)
	}
}

func TestCREHeads_LookupErrorIsReturned(t *testing.T) {
	h := CREHeads{leadership: "leaders", entity: &fakeTeamMembers{err: errors.New("down")}}
	if _, err := h.CREHeadEmails(context.Background()); err == nil {
		t.Fatal("CREHeadEmails() error = nil, want the lookup's error")
	}
}

func TestNewCREHeads_DefaultsTheLeadershipTeamAndToleratesNoClient(t *testing.T) {
	h := NewCREHeads(LadderConfig{}, nil)
	if h.leadership != defaultLeadershipTeamKey {
		t.Errorf("leadership = %q, want %q", h.leadership, defaultLeadershipTeamKey)
	}
	got, err := h.CREHeadEmails(context.Background())
	if err != nil || len(got) != 0 {
		t.Errorf("CREHeadEmails() = %v, %v; want nothing, nil", got, err)
	}

	var cfg LadderConfig
	cfg.Teams.Leadership = " leaders "
	cfg.Heads.CRE = Person{Email: "head@example.com"}
	if h := NewCREHeads(cfg, nil); h.leadership != "leaders" || h.head.Email != "head@example.com" {
		t.Errorf("NewCREHeads(cfg) = %+v, want the configured leadership team and head", h)
	}
}
