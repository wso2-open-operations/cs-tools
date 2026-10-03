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
)

type stubTeamMemberRepo struct {
	gotTeamKeys []string
	gotRoles    []string
	members     []domain.TeamMemberEntry
	err         error
}

func (s *stubTeamMemberRepo) MembersByTeamKeys(_ context.Context, teamKeys, roles, alertTiers, teamTypes []string) ([]domain.TeamMemberEntry, error) {
	s.gotTeamKeys, s.gotRoles = teamKeys, roles
	return s.members, s.err
}

// A rung that reaches nobody is a normal answer, not a failure: an ABT with no
// sub lead yet simply escalates past LEVEL_1. It must serialize as [] rather
// than null, or a caller decoding into a slice cannot tell "none" from
// "absent".
func TestMembersByTeamKeys_NoMatchesIsEmptyNotNull(t *testing.T) {
	repo := &stubTeamMemberRepo{}
	resp, err := NewTeamMemberService(repo, alwaysUnrestrictedAccess{}).MembersByTeamKeys(context.Background(), []string{"vega"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Members == nil {
		t.Error("Members = nil, want an empty slice")
	}
	if resp.Count != 0 {
		t.Errorf("Count = %d, want 0", resp.Count)
	}
}

// A trailing comma in a query string produces a blank entry. Dropping it here
// keeps it out of the SQL, where it would match no team and quietly narrow
// nothing.
func TestMembersByTeamKeys_TrimsBlanks(t *testing.T) {
	repo := &stubTeamMemberRepo{}
	_, err := NewTeamMemberService(repo, alwaysUnrestrictedAccess{}).MembersByTeamKeys(
		context.Background(), []string{" vega ", "", "  "}, []string{"lead", " "}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.gotTeamKeys) != 1 || repo.gotTeamKeys[0] != "vega" {
		t.Errorf("teamKeys = %v, want [vega]", repo.gotTeamKeys)
	}
	if len(repo.gotRoles) != 1 || repo.gotRoles[0] != "lead" {
		t.Errorf("roles = %v, want [lead]", repo.gotRoles)
	}
}

func TestMembersByTeamKeys_RejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		teamKeys []string
		roles    []string
	}{
		"no team keys":        {nil, nil},
		"only blank keys":     {[]string{" ", ""}, nil},
		"role not in the set": {[]string{"vega"}, []string{"deputy"}},
		// 'member' was renamed to 'engineer' by migration 000106. Asking for
		// it is now a typo, and saying so beats returning nothing.
		"the retired role": {[]string{"vega"}, []string{"member"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &stubTeamMemberRepo{}
			_, err := NewTeamMemberService(repo, alwaysUnrestrictedAccess{}).MembersByTeamKeys(context.Background(), tc.teamKeys, tc.roles, nil, nil)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want a ValidationError", err)
			}
			if repo.gotTeamKeys != nil {
				t.Error("the repository was called despite invalid input")
			}
		})
	}
}

// Every role the CHECK allows must be accepted, or the ladder loses a rung to
// a validation error rather than to missing data.
func TestMembersByTeamKeys_AcceptsEveryRole(t *testing.T) {
	for _, role := range []string{"engineer", "sub_lead", "lead", "cre_head", "cs_head"} {
		t.Run(role, func(t *testing.T) {
			repo := &stubTeamMemberRepo{}
			if _, err := NewTeamMemberService(repo, alwaysUnrestrictedAccess{}).MembersByTeamKeys(
				context.Background(), []string{"vega"}, []string{role}, nil, nil); err != nil {
				t.Fatalf("role %q rejected: %v", role, err)
			}
		})
	}
}

// The rota and the org chart behind it are staff data: who works where, at
// what rank, with their address. There is no narrower scope a customer could
// be given, so anyone not unrestricted sees none of it. Without this the
// endpoint answers a tokenless caller in full, which is how it shipped before
// this test existed.
func TestMembersByTeamKeys_RejectsAnyoneNotInternal(t *testing.T) {
	repo := &stubTeamMemberRepo{}
	svc := NewTeamMemberService(repo, stubAccess{scope: AccessScope{Unrestricted: false}})

	_, err := svc.MembersByTeamKeys(context.Background(), []string{"vega"}, nil, nil, nil)

	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want a ForbiddenError", err)
	}
	if repo.gotTeamKeys != nil {
		t.Error("the repository was reached despite the caller being external")
	}
}

// A failure deciding the scope must not read as permission granted.
func TestMembersByTeamKeys_ScopeErrorIsNotAllowed(t *testing.T) {
	repo := &stubTeamMemberRepo{}
	svc := NewTeamMemberService(repo, stubAccess{err: errors.New("identity lookup failed")})

	if _, err := svc.MembersByTeamKeys(context.Background(), []string{"vega"}, nil, nil, nil); err == nil {
		t.Fatal("err = nil, want the scope failure surfaced")
	}
	if repo.gotTeamKeys != nil {
		t.Error("the repository was reached despite the scope being unknown")
	}
}
