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
