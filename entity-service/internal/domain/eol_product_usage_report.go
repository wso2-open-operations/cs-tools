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

// EolProductUsageReportRequest asks for the EOL product usage report: the
// product versions past their EOL date that customers still run in an
// active Primary Production deployment.
type EolProductUsageReportRequest struct {
	// AsOf is the report date, "YYYY-MM-DD": a version is EOL when its
	// support EOL date is before it. Empty means today in the report's
	// timezone.
	AsOf string
	// Now is the instant the report is built for.
	Now time.Time
}

// EolProductUsageReport is the monthly EOL product usage report, grouped by
// the account's region, then account, then project.
type EolProductUsageReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	// AsOf is the report date, "YYYY-MM-DD".
	AsOf string `json:"asOf"`
	// TotalVersions counts the (project, product version) entries listed;
	// TotalAccounts the accounts they belong to.
	TotalVersions int                     `json:"totalVersions"`
	TotalAccounts int                     `json:"totalAccounts"`
	Regions       []EolProductUsageRegion `json:"regions"`
}

// EolProductUsageRegion is one region's accounts. Region is "" for
// accounts with no region; that group is listed last.
type EolProductUsageRegion struct {
	Region   string                   `json:"region"`
	Accounts []EolProductUsageAccount `json:"accounts"`
}

// EolProductUsageAccount is one account and its projects with EOL usage.
type EolProductUsageAccount struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	// CsAbt is the account's CRE (ABT) team name, "" when it has none.
	CsAbt  string `json:"csAbt"`
	Region string `json:"region"`
	// AccountManager and TechnicalOwner are nil when the account has none.
	AccountManager *EolProductUsagePerson   `json:"accountManager"`
	TechnicalOwner *EolProductUsagePerson   `json:"technicalOwner"`
	Projects       []EolProductUsageProject `json:"projects"`
}

// EolProductUsagePerson is an account manager or technical owner.
type EolProductUsagePerson struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// EolProductUsageProject is one project and the EOL versions it runs.
type EolProductUsageProject struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	// ProjectKey is "" when the project has none.
	ProjectKey string                   `json:"projectKey"`
	Versions   []EolProductUsageVersion `json:"versions"`
}

// EolProductUsageVersion is one EOL product version a project runs in
// Primary Production, listed once however many such deployments run it.
type EolProductUsageVersion struct {
	ProductVersionID string `json:"productVersionId"`
	ProductName      string `json:"productName"`
	Version          string `json:"version"`
	// SupportStatus is the enum text (uppercase), nil when unset.
	SupportStatus *string `json:"supportStatus"`
	// EolDate is the version's support EOL date, "YYYY-MM-DD".
	EolDate string `json:"eolDate"`
}

// EolProductUsageRow is one (project, product version) row as the
// repository reads it, before grouping.
type EolProductUsageRow struct {
	Region, AccountID, AccountName, CsAbt           string
	AccountManagerName, AccountManagerEmail         string
	TechnicalOwnerName, TechnicalOwnerEmail         string
	ProjectID, ProjectName, ProjectKey              string
	ProductVersionID, ProductName, Version, EolDate string
	SupportStatus                                   *string
}
