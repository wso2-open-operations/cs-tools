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

// ProductRegressionReportRequest asks for one monthly product regression
// report: the regression-tagged cases on a set of products whose fix ETA
// was shared with the customer during one calendar month.
type ProductRegressionReportRequest struct {
	// Products are the product names (compared ignoring case and
	// surrounding whitespace) the report covers. Required: the business
	// unit decides which products are its own.
	Products []string
	// Month is the calendar month reported, as "YYYY-MM". Empty means the
	// month before Now in the report's timezone -- the month a run on the
	// 1st reports on.
	Month string
	// Now is the instant the report is built for. The scheduled task passes
	// its firing time, so a retried run reports the same month.
	Now time.Time
}

// ProductRegressionReport is one monthly product regression report: the
// regression cases of one business unit's products, grouped by product.
type ProductRegressionReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	// Month is the calendar month reported, "YYYY-MM".
	Month string `json:"month"`
	// PeriodStart and PeriodEnd bound the month: a case is listed when its
	// fix ETA was shared at or after PeriodStart and before PeriodEnd.
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	// Tag is the tag that marks a case as a regression.
	Tag string `json:"tag"`
	// Products are the product names asked for, as given.
	Products []string `json:"products"`
	// TotalCases is the number of cases across every group.
	TotalCases int `json:"totalCases"`
	// Groups are the cases by product, products in name order, each
	// product's cases in the order their fix ETA was shared.
	Groups []ProductRegressionReportGroup `json:"groups"`
}

// ProductRegressionReportGroup is one product's regression cases.
type ProductRegressionReportGroup struct {
	Product string                       `json:"product"`
	Cases   []ProductRegressionReportRow `json:"cases"`
}

// ProductRegressionReportRow is one regression case in the report.
type ProductRegressionReportRow struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	// IssueKey is the case's customer-facing identifier.
	IssueKey string `json:"issueKey"`
	Subject  string `json:"subject"`
	// Product is the WSO2 product the case is filed against; the group it
	// lands in.
	Product string `json:"product"`
	// DeployedProduct is the customer's deployment of that product the
	// case concerns (name and version), blank when the case names none.
	DeployedProduct string `json:"deployedProduct"`
	// EtaSharedOn is when the fix ETA was shared with the customer; the
	// instant that placed the case in this month's report.
	EtaSharedOn time.Time `json:"etaSharedOn"`
}
