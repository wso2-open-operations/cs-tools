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
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestAccountTeamRef(t *testing.T) {
	if got := accountTeamRef(nil, strPtr("Team Nova")); got != nil {
		t.Fatalf("accountTeamRef(nil, name) = %+v, want nil", got)
	}
	if got := accountTeamRef(strPtr(""), strPtr("Team Nova")); got != nil {
		t.Fatalf("accountTeamRef(empty id, name) = %+v, want nil", got)
	}
	got := accountTeamRef(strPtr("team-id"), strPtr("Team Nova"))
	if got == nil || got.ID != "team-id" || got.Name != "Team Nova" {
		t.Fatalf("accountTeamRef = %+v, want {ID: team-id, Name: Team Nova}", got)
	}
	got = accountTeamRef(strPtr("team-id"), nil)
	if got == nil || got.ID != "team-id" || got.Name != "" {
		t.Fatalf("accountTeamRef with nil name = %+v, want {ID: team-id, Name: \"\"}", got)
	}
}

func TestAccountRowToView_PopulatesCreAndSreTeam(t *testing.T) {
	row := repository.AccountRow{
		ID:          "account-id",
		Name:        "Acme",
		CreTeamID:   strPtr("cre-id"),
		CreTeamName: strPtr("Team Nova"),
		SreTeamID:   strPtr("sre-id"),
		SreTeamName: strPtr("Team Orion"),
		CreatedBy:   "someone@wso2.com",
	}

	view := accountRowToView(row)
	if view.CreTeam == nil || view.CreTeam.ID != "cre-id" || view.CreTeam.Name != "Team Nova" {
		t.Fatalf("view.CreTeam = %+v, want {ID: cre-id, Name: Team Nova}", view.CreTeam)
	}
	if view.SreTeam == nil || view.SreTeam.ID != "sre-id" || view.SreTeam.Name != "Team Orion" {
		t.Fatalf("view.SreTeam = %+v, want {ID: sre-id, Name: Team Orion}", view.SreTeam)
	}

	detail := accountRowToDetail(row)
	if detail.CreTeam == nil || detail.CreTeam.ID != "cre-id" {
		t.Fatalf("detail.CreTeam = %+v, want ID cre-id", detail.CreTeam)
	}
	if detail.SreTeam == nil || detail.SreTeam.ID != "sre-id" {
		t.Fatalf("detail.SreTeam = %+v, want ID sre-id", detail.SreTeam)
	}
}

// TestAccountRowToView_PopulatesSubRegion pins a real bug: SearchAccounts
// never selected account.sub_region at all, so a caller filtering accounts
// by sub-region (a finer geography than Region, e.g. "ASIA"/"NA - WEST"
// under Region "APAC"/"NA") had no field to filter on and silently matched
// nothing.
func TestAccountRowToView_PopulatesSubRegion(t *testing.T) {
	row := repository.AccountRow{
		ID:        "account-id",
		Name:      "Acme",
		Region:    strPtr("APAC"),
		SubRegion: strPtr("ASIA"),
		CreatedBy: "someone@wso2.com",
	}
	view := accountRowToView(row)
	if view.SubRegion == nil || *view.SubRegion != "ASIA" {
		t.Fatalf("view.SubRegion = %v, want \"ASIA\"", view.SubRegion)
	}
}

// TestAccountRowToViewAndDetail_CarryCountry pins that account.country reaches
// the wire as a top-level `country` string on both the search view and the
// detail response, and as an explicit null when the account has none. The CSM
// portal shows it beside Region on the account detail page.
func TestAccountRowToViewAndDetail_CarryCountry(t *testing.T) {
	withCountry := repository.AccountRow{
		ID: "account-id", Name: "Acme", Country: strPtr("Sri Lanka"), CreatedBy: "someone@wso2.com",
	}
	withoutCountry := repository.AccountRow{ID: "account-id", Name: "Acme", CreatedBy: "someone@wso2.com"}

	for name, build := range map[string]func(repository.AccountRow) any{
		"view":   func(r repository.AccountRow) any { return accountRowToView(r) },
		"detail": func(r repository.AccountRow) any { return accountRowToDetail(r) },
	} {
		t.Run(name, func(t *testing.T) {
			for label, tc := range map[string]struct {
				row  repository.AccountRow
				want any
			}{
				"set":   {withCountry, "Sri Lanka"},
				"unset": {withoutCountry, nil},
			} {
				raw, err := json.Marshal(build(tc.row))
				if err != nil {
					t.Fatalf("%s/%s: marshal: %v", name, label, err)
				}
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("%s/%s: unmarshal: %v", name, label, err)
				}
				got, present := body["country"]
				if !present {
					t.Fatalf("%s/%s: response has no top-level \"country\" key: %s", name, label, raw)
				}
				if got != tc.want {
					t.Fatalf("%s/%s: country = %v, want %v", name, label, got, tc.want)
				}
			}
		})
	}
}

func TestAccountRowToView_NilTeamsWhenUnset(t *testing.T) {
	row := repository.AccountRow{ID: "account-id", Name: "Acme", CreatedBy: "someone@wso2.com"}

	view := accountRowToView(row)
	if view.CreTeam != nil {
		t.Fatalf("view.CreTeam = %+v, want nil", view.CreTeam)
	}
	if view.SreTeam != nil {
		t.Fatalf("view.SreTeam = %+v, want nil", view.SreTeam)
	}
}

// TestAccountRow_PartnerFlags pins isPartner (classification "Partner"),
// hasPrimaryPartner (the stored primary partner) and lifeCycle on both shapes.
func TestAccountRow_PartnerFlags(t *testing.T) {
	partner, customer, prospect := " partner ", "Customer", "Prospect"
	cases := []struct {
		name              string
		classification    *string
		hasPrimaryPartner bool
		lifeCycle         *string
		wantIsPartner     bool
	}{
		{"partner, case and space insensitive", &partner, false, &customer, true},
		{"customer with a primary partner", &customer, true, &customer, false},
		{"no classification, no life cycle", nil, false, nil, false},
		{"prospect", nil, false, &prospect, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := repository.AccountRow{Classification: tc.classification, HasPrimaryPartner: tc.hasPrimaryPartner, LifeCycle: tc.lifeCycle}
			view, detail := accountRowToView(row), accountRowToDetail(row)
			for _, got := range [][2]*bool{{view.IsPartner, view.HasPrimaryPartner}, {detail.IsPartner, detail.HasPrimaryPartner}} {
				if got[0] == nil || *got[0] != tc.wantIsPartner || got[1] == nil || *got[1] != tc.hasPrimaryPartner {
					t.Fatalf("isPartner/hasPrimaryPartner = %v/%v, want %v/%v", got[0], got[1], tc.wantIsPartner, tc.hasPrimaryPartner)
				}
			}
			for _, got := range []*string{view.LifeCycle, detail.LifeCycle} {
				if (got == nil) != (tc.lifeCycle == nil) || (got != nil && *got != *tc.lifeCycle) {
					t.Fatalf("lifeCycle = %v, want %v", got, tc.lifeCycle)
				}
			}
		})
	}
}
