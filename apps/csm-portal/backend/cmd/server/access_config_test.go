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
		"AUTH_DASHBOARD_DESIGNER_ROLES", "AUTH_ANNOUNCEMENT_CREATOR_ROLES", "AUTH_SECURITY_TEAM_ROLES", "AUTH_ONBOARDING_LEAD_ROLES",
		"AUTH_PRODUCT_CATALOG_MANAGER_ROLES",
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
			"AnnouncementCreator": got.AnnouncementCreator, "SecurityTeam": got.SecurityTeam,
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

	// Creating an announcement needs this role on top of write access, so an
	// unset variable means only admin can create one -- see
	// handler.AccessConfig.AnnouncementCreator.
	t.Run("the product catalog manager role reads its own variable and is optional", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_PRODUCT_CATALOG_MANAGER_ROLES", " test-catalog-managers , test-leads ")
		got := loadAccessConfig()
		if want := []string{"test-catalog-managers", "test-leads"}; !slices.Equal(got.ProductCatalogManager, want) {
			t.Errorf("ProductCatalogManager = %v, want %v", got.ProductCatalogManager, want)
		}
		t.Setenv("AUTH_PRODUCT_CATALOG_MANAGER_ROLES", "")
		if got := loadAccessConfig().ProductCatalogManager; len(got) != 0 {
			t.Errorf("ProductCatalogManager = %v, want empty when unset", got)
		}
	})

	t.Run("the onboarding lead role reads its own variable and is optional", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_ONBOARDING_LEAD_ROLES", " test-onboarders , test-leads ")
		got := loadAccessConfig()
		if want := []string{"test-onboarders", "test-leads"}; !slices.Equal(got.OnboardingLead, want) {
			t.Errorf("OnboardingLead = %v, want %v", got.OnboardingLead, want)
		}
		t.Setenv("AUTH_ONBOARDING_LEAD_ROLES", "")
		if got := loadAccessConfig().OnboardingLead; len(got) != 0 {
			t.Errorf("OnboardingLead = %v, want empty when unset", got)
		}
	})

	t.Run("the announcement creator role reads its own variable", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_ANNOUNCEMENT_CREATOR_ROLES", " test-announcers , test-leads ")
		got := loadAccessConfig()
		if want := []string{"test-announcers", "test-leads"}; !slices.Equal(got.AnnouncementCreator, want) {
			t.Errorf("AnnouncementCreator = %v, want %v", got.AnnouncementCreator, want)
		}
		if len(got.CsEngineer) != 0 || len(got.Admin) != 0 {
			t.Errorf("CsEngineer = %v, Admin = %v, want both untouched by this variable", got.CsEngineer, got.Admin)
		}
	})

	// security_team is read-only (view, operations, security center) and
	// holds no write access of any kind -- see handler.AccessConfig.SecurityTeam.
	t.Run("the security team role reads its own variable", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_SECURITY_TEAM_ROLES", " test-security , test-secops ")
		got := loadAccessConfig()
		if want := []string{"test-security", "test-secops"}; !slices.Equal(got.SecurityTeam, want) {
			t.Errorf("SecurityTeam = %v, want %v", got.SecurityTeam, want)
		}
		if len(got.CsEngineer) != 0 || len(got.Admin) != 0 {
			t.Errorf("CsEngineer = %v, Admin = %v, want both untouched by this variable", got.CsEngineer, got.Admin)
		}
	})
}
