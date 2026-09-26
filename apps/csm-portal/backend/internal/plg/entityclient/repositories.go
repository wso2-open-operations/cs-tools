package entityclient

import (
	"context"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// The six repository interfaces the services are written against, served over
// HTTP. The signatures are the interfaces' — nothing above this file knows the
// data arrives over a network.

// ---------------------------------------------------------------------------
// Reference
// ---------------------------------------------------------------------------

func (c *Client) ListProducts(ctx context.Context) ([]domain.Product, error) {
	var out struct {
		Products []domain.Product `json:"products"`
	}
	if err := c.get(ctx, "/plg/products", &out); err != nil {
		return nil, err
	}
	return out.Products, nil
}

// ListCSUsers returns the active engineers, for the owner pickers.
func (c *Client) ListCSUsers(ctx context.Context) ([]domain.UserRef, error) {
	active := true
	var out domain.SearchUsersResponse
	body := domain.SearchUsersRequest{
		// UPPERCASE, matching csm-portal's user_type_enum. The slice restricts
		// every user query to INTERNAL anyway, so this is belt and braces — but
		// a stale lowercase value here would have been a filter that matched
		// nothing and an owner picker that was simply empty.
		Filters:    domain.UserSearchFilters{Active: &active, UserTypes: []string{"INTERNAL"}},
		Pagination: domain.Pagination{Limit: 100},
	}
	if err := c.post(ctx, "/plg/users/search", body, &out); err != nil {
		return nil, err
	}
	return out.Users, nil
}

// GetCSUser resolves one engineer by email.
//
// This is the hop identity turns on: the caller's token carries an address,
// every write carries an id, and this resolves one to the other. Nil for an
// unknown email rather than an error — the caller decides what that means, and
// here it means 403.
func (c *Client) GetCSUser(ctx context.Context, email string) (*domain.UserRef, error) {
	// Active as well as internal. The slice restricts every user query to
	// INTERNAL on its own, so an external or system account already resolves to
	// nothing — but is_active is a separate fact, and without this an engineer
	// who has been offboarded could still act while being invisible in the
	// owner pickers, which filter it. Being hidden from the team list and still
	// able to write is the wrong half of the check to enforce.
	//
	// This is also what IDENTITY.md records the merge as needing; it is cheaper
	// to have it true now than to remember it later.
	active := true
	var out domain.SearchUsersResponse
	body := domain.SearchUsersRequest{
		Filters:    domain.UserSearchFilters{Emails: []string{email}, Active: &active},
		Pagination: domain.Pagination{Limit: 1},
	}
	if err := c.post(ctx, "/plg/users/search", body, &out); err != nil {
		return nil, err
	}
	if len(out.Users) == 0 {
		return nil, nil
	}
	return &out.Users[0], nil
}

func (c *Client) LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error) {
	var out domain.LifecycleCatalogue
	if err := c.get(ctx, "/plg/lifecycle", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Organisations
// ---------------------------------------------------------------------------

func (c *Client) Search(ctx context.Context, req domain.SearchOrganizationsRequest) ([]domain.OrganizationSummary, int, error) {
	var out domain.SearchOrganizationsResponse
	if err := c.post(ctx, "/plg/organizations/search", req, &out); err != nil {
		return nil, 0, err
	}
	return out.Organizations, out.Total, nil
}

func (c *Client) Get(ctx context.Context, id string) (*domain.OrganizationDetail, error) {
	var out domain.OrganizationDetail
	if err := c.get(ctx, "/plg/organizations/"+esc(id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Patch(ctx context.Context, req domain.PatchOrganizationRequest) error {
	return c.patch(ctx, "/plg/organizations/"+esc(req.ID),
		map[string]any{"ownerId": req.OwnerID}, nil)
}

// ---------------------------------------------------------------------------
// The pairing — and the guarded writes
// ---------------------------------------------------------------------------

func (c *Client) GetPairing(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error) {
	var out domain.ProductDetail
	if err := c.get(ctx,
		"/plg/organizations/"+esc(orgID)+"/products/"+esc(productCode), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchPairing applies a stage, tier or trial change.
//
// THE BFF DECIDING WHAT ZERO MEANS. entity-service reports RowsAffected 0 when
// the expected stage no longer held; that it constitutes a conflict — and that
// the caller should be told which stage is actually current — is PLG's rule,
// and this is where it is applied.
func (c *Client) PatchPairing(ctx context.Context, req domain.PatchOrgPlatformRequest, actorID string) error {
	var out struct {
		RowsAffected int                   `json:"rowsAffected"`
		CurrentStage domain.LifecycleStage `json:"currentStage"`
	}
	// NOTE FOR ANYONE ADDING A FIELD. This body is a map, so a new field on
	// PatchOrgPlatformRequest does NOT fail the build here — it is silently
	// dropped, and the symptom is a request that returns 200 and changes
	// nothing. Add the field here as well as to the struct.
	body := map[string]any{
		"expectedStage": req.ExpectedStage, "lifecycleStage": req.LifecycleStage,
		"healthState": req.HealthState,
		"reason":      req.Reason, "subscriptionTier": req.SubscriptionTier,
		"trialEndDate": req.TrialEndDate, "clearTrialEndDate": req.ClearTrialEndDate,
		"trialExtendedDate":      req.TrialExtendedDate,
		"clearTrialExtendedDate": req.ClearTrialExtendedDate,
		"actorId":                actorID,
	}
	if err := c.patch(ctx,
		"/plg/organizations/"+esc(req.OrganizationID)+"/products/"+esc(req.ProductCode),
		body, &out); err != nil {
		return err
	}
	// Zero rows means a failed precondition, and only a stage change HAS one. A
	// health-only request that changed nothing simply asked for what was already
	// true, which is not a conflict.
	if out.RowsAffected == 0 && req.LifecycleStage != nil {
		return &apierror.ConflictError{Msg: "the stage changed while you were editing; it is now " +
			string(out.CurrentStage)}
	}
	return nil
}

// Acknowledge claims a registration.
//
// RowsAffected 0 means it was already acknowledged — the decision travels back
// as a row count rather than being raised inside a transaction this service
// does not have.
func (c *Client) Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actorID string) error {
	var out writeResult
	body := map[string]any{"ownerId": req.OwnerID, "actorId": actorID}
	if err := c.post(ctx,
		"/plg/registrations/"+esc(req.OrgPlatformID)+"/acknowledge", body, &out); err != nil {
		return err
	}
	if out.RowsAffected == 0 {
		return &apierror.ConflictError{Msg: "this registration has already been acknowledged"}
	}
	return nil
}

func (c *Client) AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actorID string) error {
	body := map[string]any{"playbookId": req.PlaybookID, "actorId": actorID}
	return c.post(ctx,
		"/plg/organizations/"+esc(req.OrganizationID)+"/products/"+esc(req.ProductCode)+"/playbook-runs",
		body, nil)
}

// DetachRun removes a run. Zero rows means it was closed, and closed runs are
// not detachable — that would erase a recorded outcome.
func (c *Client) DetachRun(ctx context.Context, runID string) (string, string, error) {
	var out writeResult
	if err := c.delete(ctx, "/plg/playbook-runs/"+esc(runID), &out); err != nil {
		return "", "", err
	}
	if out.RowsAffected == 0 {
		return "", "", &apierror.ConflictError{
			Msg: "this playbook has been closed; removing it would erase the record"}
	}
	return out.OrganizationID, out.ProductCode, nil
}

func (c *Client) PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actorID string) (string, string, error) {
	var out writeResult
	body := map[string]any{
		"boolValue": req.BoolValue, "numberValue": req.NumberValue,
		"textValue": req.TextValue, "checkedCodes": req.CheckedCodes,
		"clearValue": req.ClearValue, "actorId": actorID,
	}
	if err := c.patch(ctx, "/plg/playbook-run-tasks/"+esc(req.ID), body, &out); err != nil {
		return "", "", err
	}
	return out.OrganizationID, out.ProductCode, nil
}

func (c *Client) RunTaskShape(ctx context.Context, taskID string) (domain.RunTaskShape, error) {
	var out domain.RunTaskShape
	err := c.get(ctx, "/plg/playbook-run-tasks/"+esc(taskID)+"/shape", &out)
	return out, err
}

func (c *Client) CreateNote(ctx context.Context, req domain.CreateNoteRequest, actorID string) error {
	body := map[string]any{"body": req.Body, "actorId": actorID}
	return c.post(ctx,
		"/plg/organizations/"+esc(req.OrganizationID)+"/products/"+esc(req.ProductCode)+"/notes",
		body, nil)
}

// UpdateNote corrects a note's wording.
//
// Zero rows means the caller is not the author. 403, not 409: the request was
// understood and refused, and it will be refused every time.
func (c *Client) UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actorID string) (string, string, error) {
	var out writeResult
	body := map[string]any{"body": req.Body, "actorId": actorID}
	if err := c.patch(ctx, "/plg/notes/"+esc(req.ID), body, &out); err != nil {
		return "", "", err
	}
	if out.RowsAffected == 0 {
		return "", "", &apierror.ForbiddenError{Msg: "only the engineer who wrote a note can edit it"}
	}
	return out.OrganizationID, out.ProductCode, nil
}

func (c *Client) SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) ([]domain.RegistrationItem, int, error) {
	var out domain.SearchRegistrationsResponse
	if err := c.post(ctx, "/plg/registrations/search", req, &out); err != nil {
		return nil, 0, err
	}
	return out.Registrations, out.Total, nil
}

func (c *Client) LocatePairing(ctx context.Context, orgPlatformID string) (string, string, error) {
	var out domain.PairingLocation
	if err := c.get(ctx, "/plg/pairings/"+esc(orgPlatformID)+"/location", &out); err != nil {
		return "", "", err
	}
	return out.OrganizationID, out.ProductCode, nil
}

// ---------------------------------------------------------------------------
// Playbook templates
// ---------------------------------------------------------------------------

func (c *Client) ListAll(ctx context.Context) ([]domain.Playbook, error) {
	return c.listPlaybooks(ctx, "")
}

func (c *Client) ListByProduct(ctx context.Context, productCode string) ([]domain.Playbook, error) {
	return c.listPlaybooks(ctx, productCode)
}

func (c *Client) listPlaybooks(ctx context.Context, productCode string) ([]domain.Playbook, error) {
	path := "/plg/playbooks"
	if productCode != "" {
		path += "?product=" + url.QueryEscape(productCode)
	}
	var out struct {
		Playbooks []domain.Playbook `json:"playbooks"`
	}
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Playbooks, nil
}

// ListForStage is unused by the services — the pairing detail already carries
// the available playbooks — and exists only to satisfy the interface.
// Returning an error rather than an empty list makes an accidental caller
// obvious instead of quietly wrong.
func (c *Client) ListForStage(ctx context.Context, productID string, stage domain.LifecycleStage, kinds []domain.PlaybookType) ([]domain.Playbook, error) {
	return nil, &apierror.ValidationError{
		Msg: "ListForStage is not served over the entity contract; read availablePlaybooks from the pairing"}
}

func (c *Client) GetPlaybook(ctx context.Context, id string) (*domain.Playbook, error) {
	var out domain.Playbook
	if err := c.get(ctx, "/plg/playbooks/"+esc(id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreatePlaybook(ctx context.Context, req domain.CreatePlaybookRequest) (string, error) {
	var out struct {
		PlaybookID string `json:"playbookId"`
	}
	if err := c.post(ctx, "/plg/products/"+esc(req.ProductCode)+"/playbooks", req, &out); err != nil {
		return "", err
	}
	return out.PlaybookID, nil
}

func (c *Client) PatchPlaybook(ctx context.Context, req domain.PatchPlaybookRequest) error {
	return c.patch(ctx, "/plg/playbooks/"+esc(req.ID), req, nil)
}

func (c *Client) ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) error {
	return c.put(ctx, "/plg/playbooks/"+esc(req.PlaybookID)+"/tasks", req, nil)
}

func (c *Client) DeletePlaybook(ctx context.Context, id string) error {
	return c.delete(ctx, "/plg/playbooks/"+esc(id), nil)
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

func (c *Client) Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error) {
	path := "/plg/analytics/dashboard"
	q := url.Values{}
	if rng.From != nil {
		q.Set("from", rng.From.Format("2006-01-02"))
	}
	if rng.To != nil {
		q.Set("to", rng.To.Format("2006-01-02"))
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out domain.DashboardAnalytics
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) WorkQueue(ctx context.Context, f domain.WorkQueueFilters) (*domain.WorkQueueResponse, error) {
	body := domain.SearchWorkQueueRequest{Filters: domain.WorkQueueSearchFilters{
		HealthStates: f.HealthStates,
		OwnerIDs:     f.OwnerIDs, OrganizationIDs: f.OrganizationIDs,
		ProductCodes: f.ProductCodes, LifecycleStages: f.LifecycleStages,
		Reasons: f.Reasons, PlaybookIDs: f.PlaybookIDs, TaskCodes: f.TaskCodes,
	}}
	var out domain.WorkQueueResponse
	if err := c.post(ctx, "/plg/work-queue/search", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
