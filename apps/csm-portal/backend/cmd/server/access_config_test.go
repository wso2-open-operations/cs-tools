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

package main

import (
	"slices"
	"testing"
)

func TestLoadAccessConfig(t *testing.T) {
	envs := []string{
		"AUTH_VIEWER_ROLES", "AUTH_ESCALATOR_ROLES",
		"AUTH_ATTACHMENT_DOWNLOADER_ROLES", "AUTH_USAGE_METRICS_VIEWER_ROLES",
		"AUTH_SUPPORT_ENGINEER_ROLES", "AUTH_ADMIN_ROLES", "AUTH_TIMECARD_APPROVER_ROLES",
		"AUTH_DASHBOARD_DESIGNER_ROLES",
	}
	resetEnv := func(t *testing.T) {
		for _, name := range envs {
			t.Setenv(name, "")
		}
	}

	t.Run("unset roles have no names: nobody holds them", func(t *testing.T) {
		resetEnv(t)
		got := loadAccessConfig()
		for name, roles := range map[string][]string{
			"Viewer": got.Viewer, "Escalator": got.Escalator,
			"AttachmentDownloader": got.AttachmentDownloader, "UsageMetricsViewer": got.UsageMetricsViewer,
			"CsEngineer": got.CsEngineer, "Admin": got.Admin,
			"TimecardApprover": got.TimecardApprover, "DashboardDesigner": got.DashboardDesigner,
		} {
			if len(roles) != 0 {
				t.Errorf("%s = %v, want no names when the variable is unset", name, roles)
			}
		}
	})

	t.Run("a configured value is trimmed and may list several roles", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_ESCALATOR_ROLES", " test-notes , test-interns ,, ")
		got := loadAccessConfig()
		if want := []string{"test-notes", "test-interns"}; !slices.Equal(got.Escalator, want) {
			t.Errorf("Escalator = %v, want %v", got.Escalator, want)
		}
		if len(got.CsEngineer) != 0 {
			t.Errorf("CsEngineer = %v, want it untouched by another role's variable", got.CsEngineer)
		}
	})

	// The env var name stays AUTH_SUPPORT_ENGINEER_ROLES even though the
	// portal role and Go field were renamed to CsEngineer -- see
	// handler.AccessConfig.CsEngineer's own doc comment for why.
	t.Run("each role reads its own variable", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_SUPPORT_ENGINEER_ROLES", "test-se")
		t.Setenv("AUTH_ADMIN_ROLES", "test-adm")
		got := loadAccessConfig()
		if !slices.Equal(got.CsEngineer, []string{"test-se"}) || !slices.Equal(got.Admin, []string{"test-adm"}) {
			t.Errorf("CsEngineer = %v, Admin = %v", got.CsEngineer, got.Admin)
		}
	})

	t.Run("a value of only commas or spaces is empty", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_ADMIN_ROLES", " , ,")
		if got := loadAccessConfig().Admin; len(got) != 0 {
			t.Errorf("Admin = %v, want empty", got)
		}
	})
}
