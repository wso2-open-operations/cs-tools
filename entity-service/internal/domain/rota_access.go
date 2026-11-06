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

package domain

import "time"

// RotaAccess is the Team Schedule's Rota access tab: who may change the rotas
// beyond the team leads, and what the caller may change about that.
//
// A rota admin may mark every team in one family (CRE, SRE or SME). A rota
// marker may mark one team -- its rotations, leave and allocations -- and
// nothing else: not the tags, not Generate month, not the paging chain.
type RotaAccess struct {
	CanManage  RotaAccessRights  `json:"canManage"`
	RotaAdmins []RotaAdminGrant  `json:"rotaAdmins"`
	Markers    []RotaMarkerGrant `json:"markers"`
}

// RotaAccessRights is what the caller may grant and take back.
type RotaAccessRights struct {
	// RotaAdmins is true for a CSM admin, who alone grants rota admin.
	RotaAdmins bool `json:"rotaAdmins"`
	// MarkerFamilies are the families whose teams' markers the caller may
	// change: every family for a CSM admin, their own for a rota admin.
	MarkerFamilies []string `json:"markerFamilies"`
	// MarkerTeams are the teams whose markers the caller may change as their
	// lead (lead or Americas team lead), whatever MarkerFamilies says.
	MarkerTeams []string `json:"markerTeams"`
	// MemberFamilies are the families whose teams' members the caller may
	// add and remove: the same rule as MarkerFamilies (every family for a
	// CSM admin, their own for a rota admin), named apart so the two can
	// diverge without a client reading one for the other. A team's lead
	// gets no member rights from MarkerTeams.
	MemberFamilies []string `json:"memberFamilies"`
}

// TeamMemberRemoval is what removing a person from a team found: whether
// they were on it, and what kept them on it. A removal that is kept is not
// written; the service turns the reason into the caller's refusal.
type TeamMemberRemoval struct {
	// Removed is true when the membership was deleted.
	Removed bool
	// Member is false when the person was not on the team at all.
	Member bool
	// Role is the membership's role (engineer, lead, ...).
	Role string
	// AlertTier is the person's Case Alerts responder tier (T1-T3), or "".
	AlertTier string
	// UpcomingTurns counts the person's turns on this team that have not
	// ended yet.
	UpcomingTurns int
}

// TeamMemberPatch is a change to one person's place on a team: their role
// (engineer or sub_lead only) and/or their Case Alerts responder tier.
type TeamMemberPatch struct {
	// Role is the new role, or nil to leave it.
	Role *string
	// AlertTierSet is true when the tier is to be written; AlertTier is then
	// the new tier (T1-T3), or nil to clear it.
	AlertTierSet bool
	AlertTier    *string
}

// TeamMembership is one person's place on one team.
type TeamMembership struct {
	TeamKey   string  `json:"teamKey"`
	UserID    string  `json:"userId"`
	Role      string  `json:"role"`
	AlertTier *string `json:"alertTier"`
}

// TeamMemberUpdate is what changing a membership found. A change that is
// kept is not written; the service turns the reason into the caller's refusal.
type TeamMemberUpdate struct {
	// Member is false when the person is not on the team.
	Member bool
	// Updated is true when the membership was written (false for a no-op).
	Updated bool
	// Lead is true when the person holds a lead-type role, which is never
	// changed here.
	Lead bool
	// TierHolder is the team's other holder of the requested tier, when the
	// tier is taken ("" otherwise). Each tier has one holder per team.
	TierHolder string
	// Membership is the membership as it stands after the call.
	Membership TeamMembership
}

// TeamMemberMove is what moving a person between two teams found: what kept
// them where they were, or where they ended up.
type TeamMemberMove struct {
	// Moved is true when this call moved them.
	Moved bool
	// OnSource and OnTarget say where they were when the call started.
	OnSource, OnTarget bool
	// Role, AlertTier and UpcomingTurns describe the source membership, as
	// for TeamMemberRemoval.
	Role          string
	AlertTier     string
	UpcomingTurns int
	// Lead is true when the source role is lead-type.
	Lead bool
	// Membership is the membership on the target after the call, when the
	// person is on it.
	Membership TeamMembership
}

// RotaAdminGrant is one person holding one family's rota admin role.
type RotaAdminGrant struct {
	Family    string    `json:"family"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	GrantedBy string    `json:"grantedBy"`
	GrantedOn time.Time `json:"grantedOn"`
	// Active is false when the holder is not internal staff, which a rota
	// role never makes anyone: the grant is listed so it can be removed, but
	// it does nothing.
	Active bool `json:"active"`
}

// RotaMarkerGrant is one person marking one team's rota.
type RotaMarkerGrant struct {
	TeamKey   string    `json:"teamKey"`
	Family    string    `json:"family"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	GrantedBy string    `json:"grantedBy"`
	GrantedOn time.Time `json:"grantedOn"`
	// Active is false once the holder has left the team or is not internal
	// staff: the grant stops working on its own and is listed so it can be
	// removed.
	Active bool `json:"active"`
}

// RotaEditTeams is which teams the caller may edit, and which of those only
// as a rota marker -- a marker may mark the team's days but may not generate
// its rota or change the shared tags, so the page needs to tell them apart.
type RotaEditTeams struct {
	TeamKeys           []string `json:"teamKeys"`
	MarkerOnlyTeamKeys []string `json:"markerOnlyTeamKeys"`
}
