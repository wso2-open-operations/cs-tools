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
