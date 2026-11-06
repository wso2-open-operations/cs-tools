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
	"slices"
	"testing"
)

// A portal admin (the global admin role in the database, with no admin role on
// the token) runs the Rota Access tab like a CSM admin: every family's
// markers, and the rota admin grants.
func TestRotaAccess_PortalAdminManagesEverything(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, portalAdmin: true, teamFamily: map[string]string{"apollo": "SRE"}}
	svc, _ := newRotaAccess(repo)
	ctx := callerWithRoles("portal.admin@example.com", "cs_engineer")

	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CanManage.RotaAdmins || !slices.Equal(got.CanManage.MarkerFamilies, []string{"CRE", "SRE", "SME"}) {
		t.Fatalf("a portal admin got %+v, want rota admins and every family", got.CanManage)
	}
	if err := svc.GrantRotaAdmin(ctx, "sre", someoneID); err != nil {
		t.Fatalf("a portal admin was refused a rota admin grant: %v", err)
	}
	if err := svc.GrantRotaMarker(ctx, "apollo", someoneID); err != nil {
		t.Fatalf("a portal admin was refused a marker: %v", err)
	}
}

// A team's lead sees the tab and changes their own team's markers, and no
// other team's; they still cannot grant rota admin.
func TestRotaAccess_LeadManagesOwnTeamsMarkers(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"vega": "CRE", "castor": "CRE", "apollo": "SRE"}}
	svc, sched := newRotaAccess(repo)
	sched.led = []string{"vega"}
	ctx := callerWithRoles("vega.lead@example.com", "cs_engineer")

	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.CanManage.RotaAdmins || len(got.CanManage.MarkerFamilies) != 0 || !slices.Equal(got.CanManage.MarkerTeams, []string{"vega"}) {
		t.Fatalf("a lead got %+v, want only markerTeams [vega]", got.CanManage)
	}
	if !repo.listed || !slices.Equal(repo.listedTeams, []string{"vega"}) {
		t.Fatalf("the lead's markers were not listed for their team: listed=%v teams=%v", repo.listed, repo.listedTeams)
	}

	if err := svc.GrantRotaMarker(ctx, "vega", someoneID); err != nil {
		t.Fatalf("a lead was refused a marker on their own team: %v", err)
	}
	if err := svc.RevokeRotaMarker(ctx, "vega", someoneID); err != nil {
		t.Fatalf("a lead was refused removing a marker on their own team: %v", err)
	}
	wantForbidden(t, svc.GrantRotaMarker(ctx, "castor", someoneID), "a lead naming another team's marker")
	wantForbidden(t, svc.GrantRotaMarker(ctx, "apollo", someoneID), "a lead naming another family's marker")
	wantForbidden(t, svc.GrantRotaAdmin(ctx, "cre", someoneID), "a lead granting rota admin")
	if len(got.CanManage.MemberFamilies) != 0 {
		t.Fatalf("a lead got memberFamilies %v, want none", got.CanManage.MemberFamilies)
	}
	wantForbidden(t, svc.AddTeamMember(ctx, "vega", someoneID), "a lead adding a member to their own team")
	wantForbidden(t, svc.RemoveTeamMember(ctx, "vega", someoneID), "a lead removing a member from their own team")
}

// Somebody who leads nothing and holds no role still sees an empty tab.
func TestRotaAccess_NobodyStillGetsAnEmptyTab(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true}
	svc, _ := newRotaAccess(repo)
	got, err := svc.Get(callerWithRoles("engineer@example.com", "cs_engineer"))
	if err != nil {
		t.Fatal(err)
	}
	if got.CanManage.RotaAdmins || len(got.CanManage.MarkerFamilies) != 0 || len(got.CanManage.MarkerTeams) != 0 || repo.listed {
		t.Fatalf("an engineer got %+v (listed=%v), want an empty tab", got.CanManage, repo.listed)
	}
	if got.CanManage.MarkerTeams == nil {
		t.Fatal("markerTeams must be [] on the wire, not null")
	}
}
