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

// CreateProductWorkItemRequest is the body of POST /cases/{id}/product-work-items.
// The fields after IdempotencyKey are those of the portal backend's
// POST /cases/{id}/github-issues dialog. The repository comes from the case's
// product, never from RepoOverride (accepted and ignored, as the portal backend
// does).
type CreateProductWorkItemRequest struct {
	CaseID         string `json:"-"`
	IdempotencyKey string `json:"idempotencyKey"`

	Title        string `json:"title"`
	Description  string `json:"description"`
	Reason       string `json:"reason"`
	RepoOverride *struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	} `json:"repoOverride"`
	UpdateLevel          string `json:"updateLevel"`
	PublicIssueURL       string `json:"publicIssueUrl"`
	Regression           bool   `json:"regression"`
	HotFixRequired       bool   `json:"hotFixRequired"`
	IssueTypeLabel       string `json:"issueTypeLabel"`
	PriorityLevel        string `json:"priorityLevel"`
	OnboardingInProgress bool   `json:"onboardingInProgress"`
}

// Product work item kinds (work_item_creation_request.kind).
const (
	ProductWorkItemKindProductIssue   = "PRODUCT_ISSUE"
	ProductWorkItemKindUntrackedPatch = "UNTRACKED_PATCH"
)

// ProductWorkItemIssue is the GitHub issue part of the create answer; its shape
// is the portal backend's webapp contract.
type ProductWorkItemIssue struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
	Repo   string `json:"repo"`
}

// ProductWorkItemRef names the registered work item.
type ProductWorkItemRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// CreateProductWorkItemResponse is the answer of POST /cases/{id}/product-work-items.
// Message and Issue keep the portal backend's webapp contract; Kind and
// WorkItem are new. WorkItem is null for an untracked patch.
type CreateProductWorkItemResponse struct {
	Message  string               `json:"message"`
	Issue    ProductWorkItemIssue `json:"issue"`
	Kind     string               `json:"kind"`
	WorkItem *ProductWorkItemRef  `json:"workItem"`
}
