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

package sweep

import (
	"context"
	"encoding/json"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
)

// project is the subset of csm-integration-service's Project shape this
// component reads. closureState, endDate, and account are undocumented in
// csm-integration-service's openapi.yaml but confirmed present in the real
// response via direct Postman testing against staging — this holds for both
// GetProject and SearchProjects's response items: entity-service added a
// nested account {id, name} reference to both (SearchProjects's item shape
// gained it via ProjectView.Account, confirmed against entity-service's own
// domain type and re-verified live via Postman). Account is nil only when a
// project genuinely has no linked account. Always go through accountID(),
// never read Account directly, so callers don't need to duplicate the nil
// check.
type project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// ProjectKey is the short project identifier (e.g. "APPSUB") used in
	// notice bodies, alongside Name. Tagged "key", not "projectKey" —
	// csm-integration-service's own openapi.yaml documents this field as
	// "projectKey", but the live response actually names it "key"
	// (confirmed directly via Postman against the dedicated test project;
	// "projectKey" silently produced an always-empty ProjectKey until this
	// was caught). Trust the wire over the spec if they disagree again.
	ProjectKey string             `json:"key"`
	Account    *projectAccountRef `json:"account"`
	// StartDate is nil only when genuinely absent on the wire — mirrors
	// EndDate's existing nullable-pointer convention.
	StartDate *time.Time `json:"startDate"`
	EndDate   *time.Time `json:"endDate"`
	// ClosureState is the derived roll-up over EndDateClosureState,
	// InvoiceDueDateClosureState, and ComplianceViolationClosureState — not
	// settable directly, and not what suspend()'s idempotency guard checks
	// (see EndDateClosureState's doc comment).
	ClosureState *string `json:"closureState"`
	// EndDateClosureState is the specific per-dimension state this
	// component's suspend() writes ("Suspended") and later reads back to
	// decide whether suspend already happened. Confirmed via a real
	// suspended project (Postman, project acac149b-eba1-4714-fcf5-f5dabad0cdb1)
	// that this can progress past "Suspended" to "Closed" via a process
	// outside this component — suspend()'s guard treats any non-"Open"
	// value as already handled, not just an exact "Suspended" match.
	EndDateClosureState    *string         `json:"endDateClosureState"`
	SuspensionProcessState json.RawMessage `json:"suspensionProcessState"`
}

// projectAccountRef is the nested account reference on both GetProject's and
// SearchProjects's response shapes. id and name are used (name for the
// notice subject line); the upstream shape carries more (activationDate,
// tier, region, ...) that this component doesn't need.
type projectAccountRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// accountID returns the project's account ID, or "" if the project
// genuinely has no linked account.
func (p project) accountID() string {
	if p.Account == nil {
		return ""
	}
	return p.Account.ID
}

// searchProjectsResponse mirrors csm-integration-service's ProjectSearchResponse.
type searchProjectsResponse struct {
	Projects []project `json:"projects"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"hasMore"`
}

type projectContactDTO struct {
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

type projectContactSearchResponse struct {
	Contacts []projectContactDTO `json:"contacts"`
}

type accountContactDTO struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	IsPrimary bool   `json:"isPrimary"`
}

type accountContactSearchResponse struct {
	Contacts []accountContactDTO `json:"contacts"`
}

// personRefDTO mirrors entity-service's PersonRef shape as it appears
// embedded on an Account (technicalOwner/accountManager/
// renewalAccountManager). Email is nullable — confirmed present but
// genuinely absent for some real accounts.
type personRefDTO struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

// accountDTO is the subset of GetAccount's response this component reads.
// TechnicalOwner and RenewalAccountManager are confirmed present on the real
// response (verified directly against the live API) alongside
// AccountManager; all three now feed the notice-content redesign's
// Recipients (see sweep.go's resolveAccountContacts).
type accountDTO struct {
	AccountManager        *personRefDTO `json:"accountManager"`
	TechnicalOwner        *personRefDTO `json:"technicalOwner"`
	RenewalAccountManager *personRefDTO `json:"renewalAccountManager"`
}

// entityReader is the minimal read surface processProject needs. Satisfied
// directly by *entity.Client — reads are never dry-run-gated.
type entityReader interface {
	SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error)
	SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error)
	GetAccount(ctx context.Context, id string) ([]byte, error)
}

// sweepReader is everything Run needs: entityReader plus SearchProjects and
// GetProject, the two extra read methods the outer loop uses that
// processProject doesn't. GetProject backs the TEST_PROJECT_ID scoped-run
// path — fetching one project directly instead of paginating the broad
// search. Satisfied directly by *entity.Client.
type sweepReader interface {
	entityReader
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	GetProject(ctx context.Context, id string) ([]byte, error)
}

// pagination mirrors entity-service's Pagination shape.
type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// searchProjectsRequest mirrors the fields of entity-service's
// SearchProjectsRequest this component uses. closureStatus/sortBy/sortOrder
// are top-level fields (confirmed directly against
// entity-service/internal/domain/entity.go's SearchProjectsRequest), not
// nested under a filters key. searchQuery is omitted entirely rather than
// sent as "" — an explicit empty searchQuery causes a 400 (confirmed quirk).
type searchProjectsRequest struct {
	Pagination    pagination `json:"pagination"`
	ClosureStatus string     `json:"closureStatus"`
	SortBy        string     `json:"sortBy"`
	SortOrder     string     `json:"sortOrder"`
}

// Result summarizes one full Run: how many projects were evaluated, how
// many were skipped via EXCLUDED_PROJECT_IDS, and any per-project failures
// encountered along the way. A non-empty Failures list is a "soft"
// outcome — Run's own error return is reserved for a fatal page-fetch
// failure that prevented the sweep from completing at all.
type Result struct {
	ProjectsEvaluated int
	ProjectsExcluded  int
	Failures          []ProjectFailure
}

// ProjectFailure records a single project's processProject failure.
type ProjectFailure struct {
	ProjectID string
	Err       error
}

// projectUpdater is the minimal write surface processProject needs.
// Satisfied by *entity.Client for real writes; a dry-run implementation
// (logs instead of calling UpdateProject) is injected instead when
// DRY_RUN is set — processProject itself never branches on a dry-run flag.
type projectUpdater interface {
	UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
}

// notifier is the minimal send surface processProject needs. Satisfied by
// *notify.LoggingNotifier and *notify.EmailNotifier. Send reports, per
// call, whether that specific notice was actually delivered — not a
// blanket "this notifier type sends for real" signal. recordNoticeSent
// uses this per-call result to avoid claiming a notification succeeded
// when this particular one didn't (e.g. a customer notice silently
// filtered out by EmailNotifier's WSO2-only staging safeguard).
type notifier interface {
	Send(ctx context.Context, n notify.Notice) (delivered bool, err error)
}
