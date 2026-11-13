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

// DeprecatedProductUsageReportRequest asks for the deprecated product usage
// report: the product versions with a Deprecated support status that
// customers still run in an active Primary Production deployment.
type DeprecatedProductUsageReportRequest struct {
	// AsOf is the report date, "YYYY-MM-DD", shown in the report; the list is
	// the versions' current status whatever the date. Empty means today in
	// the report's timezone.
	AsOf string
	// Now is the instant the report is built for.
	Now time.Time
}

// DeprecatedProductUsageReport is the monthly deprecated product usage report, grouped by
// the account's region, then account, then project.
type DeprecatedProductUsageReport struct {
	GeneratedOn time.Time `json:"generatedOn"`
	// AsOf is the report date, "YYYY-MM-DD".
	AsOf string `json:"asOf"`
	// TotalVersions counts the (project, product version) entries listed;
	// TotalAccounts the accounts they belong to.
	TotalVersions int                            `json:"totalVersions"`
	TotalAccounts int                            `json:"totalAccounts"`
	Regions       []DeprecatedProductUsageRegion `json:"regions"`
}

// DeprecatedProductUsageRegion is one region's accounts. Region is nil for
// accounts with no region; that group is listed last.
type DeprecatedProductUsageRegion struct {
	Region   *string                         `json:"region"`
	Accounts []DeprecatedProductUsageAccount `json:"accounts"`
}

// DeprecatedProductUsageAccount is one account and its projects with deprecated usage.
type DeprecatedProductUsageAccount struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	// CsAbt is the account's CRE (ABT) team name; Region its region. Each
	// is nil when the account has none.
	CsAbt  *string `json:"csAbt"`
	Region *string `json:"region"`
	// AccountManager and TechnicalOwner are nil when the account has none.
	AccountManager *DeprecatedProductUsagePerson   `json:"accountManager"`
	TechnicalOwner *DeprecatedProductUsagePerson   `json:"technicalOwner"`
	Projects       []DeprecatedProductUsageProject `json:"projects"`
}

// DeprecatedProductUsagePerson is an account manager or technical owner. A name
// or email the user record lacks is left out.
type DeprecatedProductUsagePerson struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// DeprecatedProductUsageProject is one project and the deprecated versions it runs.
type DeprecatedProductUsageProject struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	// ProjectKey is nil when the project has none.
	ProjectKey *string                         `json:"projectKey"`
	Versions   []DeprecatedProductUsageVersion `json:"versions"`
}

// DeprecatedProductUsageVersion is one deprecated product version a project runs in
// Primary Production, listed once however many such deployments run it.
type DeprecatedProductUsageVersion struct {
	ProductVersionID string `json:"productVersionId"`
	ProductName      string `json:"productName"`
	Version          string `json:"version"`
	// SupportStatus is the enum text (uppercase), nil when unset.
	SupportStatus *string `json:"supportStatus"`
	// EolDate is the version's support EOL date, "YYYY-MM-DD"; nil when the
	// version has none yet.
	EolDate *string `json:"eolDate"`
}

// DeprecatedProductUsageRow is one (project, product version) row as the
// repository reads it, before grouping. Text it lacks is "" here; the
// report turns that into null (GroupDeprecatedProductUsage).
type DeprecatedProductUsageRow struct {
	Region, AccountID, AccountName, CsAbt           string
	AccountManagerName, AccountManagerEmail         string
	TechnicalOwnerName, TechnicalOwnerEmail         string
	ProjectID, ProjectName, ProjectKey              string
	ProductVersionID, ProductName, Version, EolDate string
	SupportStatus                                   *string
}
