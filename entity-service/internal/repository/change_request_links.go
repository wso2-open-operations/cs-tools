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

package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The change request's customer-scope fields -- Customer Project, Deployments
// and Deployment products -- and the rules that tie them together. The data
// model is migration 0191's header (its environment catalogue and join table
// were dropped again by migration 0192: a deployment already IS an environment
// instance, its role being deployment.type); the rules, which both create and
// PATCH (and the form's lookup) apply identically, are:
//
//   - Project: work_item.project_id. Must exist.
//   - Deployments: change_request_deployment. Each must exist, be active and
//     belong to the project (so deployments require a project).
//   - Deployment products: change_request_deployed_product. READ-ONLY and
//     derived: the active deployed products of the chosen deployments. A caller
//     may state them, but only as exactly that set.
//   - Customer Group: NOT a field. It is derived, live, from the project: its
//     REGISTERED portal-user contacts (loadProjectCustomerContacts). A client
//     that still sends customerGroupId, or environmentIds, is refused
//     (rejectRemovedChangeRequestFields).
//
// PATCH additionally enforces the edit window (changeRequestLinksLockedStates)
// for the deployments and deployment products, and the stricter customer
// requirements lock for the project: it can be chosen or changed only while the
// change request is New (change_request_customer_lock.go); from Request Approval
// on it is frozen, and what is chosen with it must belong to it.

// maxChangeRequestLinkIDs caps each id list of a selection.
const maxChangeRequestLinkIDs = 100

// changeRequestLinksLockedStates are the states from which a change request's
// project, deployments, environments and deployment products can no longer be
// changed: implementation has started (or the change is over), so the scope the
// change was approved for is fixed. New .. Scheduled (and a legacy NULL state)
// are the edit window.
var changeRequestLinksLockedStates = map[string]bool{
	"IMPLEMENT": true, "REVIEW": true, "CUSTOMER_REVIEW": true,
	"ROLLBACK": true, "CLOSED": true, "CANCELED": true,
}

// crQueryer is the read surface shared by *Scoped and pgx.Tx, so the same
// resolution runs inside a transaction (create, PATCH) and outside one
// (pre-flight validation, the form's lookup).
type crQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// linkDeploymentRow / linkProductRow keep the extra columns the lookup needs
// next to the EntityRef the detail needs.
type linkDeploymentRow struct {
	ref domain.EntityRef
	typ string
}

type linkProductRow struct {
	ref          domain.EntityRef
	deploymentID string
}

// resolvedChangeRequestLinks is a validated selection with everything derived.
type resolvedChangeRequestLinks struct {
	projectID   string
	deployments []linkDeploymentRow
	products    []linkProductRow
}

func (r resolvedChangeRequestLinks) deploymentIDs() []string {
	out := make([]string, len(r.deployments))
	for i, d := range r.deployments {
		out[i] = d.ref.ID
	}
	return out
}

func (r resolvedChangeRequestLinks) productIDs() []string {
	out := make([]string, len(r.products))
	for i, p := range r.products {
		out[i] = p.ref.ID
	}
	return out
}

func entityRefIDs(refs []domain.EntityRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.ID
	}
	return out
}

// resolveLinkOpts relaxes resolution for a PATCH that re-sends values the
// change request already holds.
type resolveLinkOpts struct {
	// grandfatheredDeployments are deployments already stored on the change
	// request: accepted even when deactivated since, so an unrelated edit does
	// not start failing.
	grandfatheredDeployments map[string]bool
	// acceptedProductSets are additional sets accepted for a stated
	// deploymentProductIds besides the derived one (the stored snapshot).
	acceptedProductSets [][]string
}

func normalizeUUIDList(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func sameIDSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]bool, len(a))
	for _, x := range a {
		m[strings.ToLower(x)] = true
	}
	for _, y := range b {
		if !m[strings.ToLower(y)] {
			return false
		}
	}
	return true
}

func linkValidationf(format string, args ...any) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// Messages for the fields that are no longer accepted.
const (
	customerGroupIDRemovedMsg = "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"
	environmentIDsRemovedMsg  = "environmentIds is no longer supported: deployments carry the environment"
)

// rejectRemovedChangeRequestFields refuses a client that still sends
// customerGroupId or environmentIds, rather than silently dropping them: the
// Customer Group is derived live from the Customer Project's registered
// contacts, and a deployment already carries its environment (deployment.type).
// Applied by the service before anything else is looked at, so a refusal can
// never leave a ServiceNow record behind either.
func rejectRemovedChangeRequestFields(customerGroupSent, environmentIDsSent bool) error {
	if customerGroupSent {
		return &apierror.ValidationError{Msg: customerGroupIDRemovedMsg}
	}
	if environmentIDsSent {
		return &apierror.ValidationError{Msg: environmentIDsRemovedMsg}
	}
	return nil
}

// RejectRemovedCreateFields / RejectRemovedPatchFields are the entry points the
// service layer uses.
func RejectRemovedCreateFields(req domain.CreateChangeRequestRequest) error {
	return rejectRemovedChangeRequestFields(req.CustomerGroupID != nil, req.EnvironmentIDs != nil)
}

func RejectRemovedPatchFields(req domain.PatchChangeRequestRequest) error {
	return rejectRemovedChangeRequestFields(req.CustomerGroupID != nil, req.EnvironmentIDs != nil)
}

// customerContactsSQL selects the REGISTERED portal-user contacts of a project:
// a project_contact in state REGISTERED holding the PORTAL_USER project role
// (the same "registered contact with role X on project Y" chain
// callerIsRegisteredPortalContact uses), with the name and the "user"
// row resolved the way ProjectContactRepository does it (account_contact.user_name
// matched to "user".user_name, case-insensitively). A contact whose "user" row is
// deactivated is not listed. Contacts are the customer's own people, so they are
// never mixed across projects: the only input is the project id.
//
//	$1 project id (uuid)
//
// Columns: project_contact.id, display name, email, "user".id (NULL when the
// contact has no "user" row yet).
const customerContactsSQL = `
	SELECT pc.id::text,
	       COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), pc.email),
	       COALESCE(u.email, pc.email),
	       u.id::text
	FROM project_contact pc
	JOIN account_contact ac ON ac.id = pc.account_contact_id
	LEFT JOIN "user" u ON LOWER(u.user_name) = LOWER(ac.user_name)
	WHERE pc.project_id = $1::uuid
	  AND pc.state = 'REGISTERED'::project_contact_state_enum
	  AND COALESCE(u.is_active, true)
	  AND EXISTS (
	      SELECT 1
	      FROM project_contact_group pcg
	      JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
	      JOIN project_role pr ON pr.id = pgr.project_role_id
	      WHERE pcg.project_contact_id = pc.id AND pr.role = 'PORTAL_USER'::project_role_enum)
	ORDER BY 2, pc.id`

// projectCustomerContact is a registered contact with the "user" id that can
// act as an approver (empty when the contact has no "user" row).
type projectCustomerContact struct {
	contact domain.ChangeRequestCustomerContact
	userID  string
}

// loadProjectCustomerContacts lists the project's registered portal-user
// contacts (name order, never nil). A blank project has none.
func loadProjectCustomerContacts(ctx context.Context, q crQueryer, projectID string) ([]projectCustomerContact, error) {
	out := []projectCustomerContact{}
	if strings.TrimSpace(projectID) == "" {
		return out, nil
	}
	rows, err := q.Query(ctx, customerContactsSQL, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project customer contacts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c projectCustomerContact
		var email, userID *string
		if err := rows.Scan(&c.contact.ID, &c.contact.Name, &email, &userID); err != nil {
			return nil, fmt.Errorf("scan project customer contact: %w", err)
		}
		c.contact.Email = stringOrEmpty(email)
		c.userID = stringOrEmpty(userID)
		out = append(out, c)
	}
	return out, rows.Err()
}

// customerContactRefs is loadProjectCustomerContacts for the response shape.
func customerContactRefs(ctx context.Context, q crQueryer, projectID string) ([]domain.ChangeRequestCustomerContact, error) {
	contacts, err := loadProjectCustomerContacts(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChangeRequestCustomerContact, len(contacts))
	for i, c := range contacts {
		out[i] = c.contact
	}
	return out, nil
}

// resolveChangeRequestLinks validates sel and derives the deployment products
// from the chosen deployments, per the rules above.
// Returns a ValidationError (a 400) naming the offending field and id.
func resolveChangeRequestLinks(ctx context.Context, q crQueryer, sel domain.ChangeRequestLinkSelection, opts resolveLinkOpts) (resolvedChangeRequestLinks, error) {
	var res resolvedChangeRequestLinks
	depIDs := normalizeUUIDList(sel.DeploymentIDs)
	prodIDs := normalizeUUIDList(sel.DeploymentProductIDs)
	for field, n := range map[string]int{"deploymentIds": len(depIDs), "deploymentProductIds": len(prodIDs)} {
		if n > maxChangeRequestLinkIDs {
			return res, linkValidationf("%s must contain at most %d entries", field, maxChangeRequestLinkIDs)
		}
	}

	if sel.ProjectID != nil && strings.TrimSpace(*sel.ProjectID) != "" {
		res.projectID = strings.ToLower(strings.TrimSpace(*sel.ProjectID))
		var exists bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project WHERE id = $1::uuid)`, res.projectID).Scan(&exists); err != nil {
			return res, fmt.Errorf("resolve change request links: check project: %w", err)
		}
		if !exists {
			return res, linkValidationf("projectId does not refer to an existing project: %s", res.projectID)
		}
	}

	if len(depIDs) == 0 {
		if len(prodIDs) > 0 {
			return res, linkValidationf("deploymentProductIds requires deploymentIds: deployment products come from the selected deployments")
		}
		return res, nil
	}
	if res.projectID == "" {
		return res, linkValidationf("projectId is required when deploymentIds are provided")
	}

	// Deployments: exist, active, in the project.
	rows, err := q.Query(ctx, `
		SELECT d.id::text, d.name, d.project_id::text, COALESCE(d.is_active, false), lower(d.type::text)
		FROM deployment d
		WHERE d.id = ANY($1::text[]::uuid[])`, depIDs)
	if err != nil {
		return res, fmt.Errorf("resolve change request links: query deployments: %w", err)
	}
	type depInfo struct {
		row     linkDeploymentRow
		project *string
		active  bool
	}
	found := make(map[string]depInfo, len(depIDs))
	for rows.Next() {
		var (
			id, name, typ string
			project       *string
			active        bool
		)
		if err := rows.Scan(&id, &name, &project, &active, &typ); err != nil {
			rows.Close()
			return res, fmt.Errorf("resolve change request links: scan deployment: %w", err)
		}
		found[id] = depInfo{row: linkDeploymentRow{ref: domain.EntityRef{ID: id, Name: name}, typ: typ}, project: project, active: active}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("resolve change request links: iterate deployments: %w", err)
	}
	for _, id := range depIDs {
		d, ok := found[id]
		switch {
		case !ok:
			return res, linkValidationf("deploymentIds contains an unknown deployment: %s", id)
		case d.project == nil || !strings.EqualFold(*d.project, res.projectID):
			return res, linkValidationf("deploymentIds contains a deployment that does not belong to the selected project: %s", id)
		case !d.active && !opts.grandfatheredDeployments[id]:
			return res, linkValidationf("deploymentIds contains an inactive deployment: %s", id)
		}
		res.deployments = append(res.deployments, d.row)
	}
	sort.SliceStable(res.deployments, func(i, j int) bool {
		if res.deployments[i].ref.Name != res.deployments[j].ref.Name {
			return res.deployments[i].ref.Name < res.deployments[j].ref.Name
		}
		return res.deployments[i].ref.ID < res.deployments[j].ref.ID
	})

	// Deployment products: always the active deployed products of the chosen
	// deployments.
	prows, err := q.Query(ctx, `
		SELECT dp.id::text, p.name, pv.version, dp.deployment_id::text
		FROM deployed_product dp
		JOIN product p ON p.id = dp.product_id
		LEFT JOIN product_version pv ON pv.id = dp.version_id
		WHERE dp.deployment_id = ANY($1::text[]::uuid[])
		  AND (dp.active IS NULL OR dp.active = TRUE)
		ORDER BY p.name, pv.version NULLS FIRST, dp.id`, depIDs)
	if err != nil {
		return res, fmt.Errorf("resolve change request links: query deployed products: %w", err)
	}
	for prows.Next() {
		var id, name, depID string
		var version *string
		if err := prows.Scan(&id, &name, &version, &depID); err != nil {
			prows.Close()
			return res, fmt.Errorf("resolve change request links: scan deployed product: %w", err)
		}
		if version != nil && *version != "" {
			name += " " + *version
		}
		res.products = append(res.products, linkProductRow{ref: domain.EntityRef{ID: id, Name: name}, deploymentID: depID})
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return res, fmt.Errorf("resolve change request links: iterate deployed products: %w", err)
	}
	if sel.DeploymentProductIDs != nil {
		ok := sameIDSet(prodIDs, res.productIDs())
		for _, set := range opts.acceptedProductSets {
			ok = ok || sameIDSet(prodIDs, set)
		}
		if !ok {
			return res, linkValidationf("deploymentProductIds is read-only: it is derived from the selected deployments and must match them exactly")
		}
	}
	return res, nil
}

// changeRequestLinksSet converts a resolution to the domain's detail shape.
func (r resolvedChangeRequestLinks) linkSet() domain.ChangeRequestLinkSet {
	set := domain.ChangeRequestLinkSet{
		ProjectID:          r.projectID,
		Deployments:        make([]domain.EntityRef, 0, len(r.deployments)),
		DeploymentProducts: make([]domain.EntityRef, 0, len(r.products)),
	}
	for _, d := range r.deployments {
		set.Deployments = append(set.Deployments, d.ref)
	}
	for _, p := range r.products {
		set.DeploymentProducts = append(set.DeploymentProducts, p.ref)
	}
	return set
}

// firstOrNil returns the first id of ids, or nil.
func firstOrNil(ids []string) *string {
	if len(ids) == 0 {
		return nil
	}
	v := ids[0]
	return &v
}

// writeChangeRequestLinkRows replaces one join table's rows for the change
// request with ids (empty clears it).
func writeChangeRequestLinkRows(ctx context.Context, tx pgx.Tx, table, column, crID string, ids []string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE change_request_id = $1::uuid`, table), crID); err != nil {
		return fmt.Errorf("clear %s: %w", table, err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		`INSERT INTO %s (change_request_id, %s) SELECT $1::uuid, unnest($2::text[]::uuid[]) ON CONFLICT DO NOTHING`, table, column),
		crID, ids); err != nil {
		return fmt.Errorf("write %s: %w", table, err)
	}
	return nil
}

// writeChangeRequestDeployments does not use the generic writeChangeRequestLinkRows
// helper: change_request_deployment is also defined by csm-sync-service's own
// 0136_change_request_deployment_table.sql, mirrored verbatim into this repo's
// migrations and numbered to run before this table's own 0191, so that
// surrogate-id schema (id UUID PRIMARY KEY, no default, plus a UNIQUE
// (change_request_id, deployment_id) this ON CONFLICT relies on) is the one
// that actually exists -- 0191's own CREATE TABLE IF NOT EXISTS for the same
// name is a no-op by the time it runs. An id-less INSERT here fails NOT NULL
// on id; change_request_deployed_product has no such collision (0137, this
// table's csm-sync-service namesake, never defines deployed_product), so it
// keeps using the generic helper unchanged.
func writeChangeRequestDeployments(ctx context.Context, tx pgx.Tx, crID string, ids []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM change_request_deployment WHERE change_request_id = $1::uuid`, crID); err != nil {
		return fmt.Errorf("clear change_request_deployment: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO change_request_deployment (id, change_request_id, deployment_id)
		 SELECT gen_random_uuid(), $1::uuid, unnest($2::text[]::uuid[]) ON CONFLICT DO NOTHING`,
		crID, ids); err != nil {
		return fmt.Errorf("write change_request_deployment: %w", err)
	}
	return nil
}

func writeChangeRequestProducts(ctx context.Context, tx pgx.Tx, crID string, ids []string) error {
	return writeChangeRequestLinkRows(ctx, tx, "change_request_deployed_product", "deployed_product_id", crID, ids)
}

// loadChangeRequestLinks reads the stored deployments and deployment products
// of a change request (name order, never nil).
func loadChangeRequestLinks(ctx context.Context, q crQueryer, crID string) (deployments, products []domain.EntityRef, err error) {
	read := func(query string) ([]domain.EntityRef, error) {
		rows, err := q.Query(ctx, query, crID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []domain.EntityRef{}
		for rows.Next() {
			var ref domain.EntityRef
			var name *string
			if err := rows.Scan(&ref.ID, &name); err != nil {
				return nil, err
			}
			ref.Name = stringOrEmpty(name)
			out = append(out, ref)
		}
		return out, rows.Err()
	}
	if deployments, err = read(`
		SELECT d.id::text, d.name FROM change_request_deployment l JOIN deployment d ON d.id = l.deployment_id
		WHERE l.change_request_id = $1::uuid ORDER BY d.name, d.id`); err != nil {
		return nil, nil, fmt.Errorf("read change request deployments: %w", err)
	}
	if products, err = read(`
		SELECT dp.id::text, p.name || COALESCE(' ' || NULLIF(pv.version, ''), '')
		FROM change_request_deployed_product l
		JOIN deployed_product dp ON dp.id = l.deployed_product_id
		LEFT JOIN product p ON p.id = dp.product_id
		LEFT JOIN product_version pv ON pv.id = dp.version_id
		WHERE l.change_request_id = $1::uuid ORDER BY p.name, pv.version NULLS FIRST, dp.id`); err != nil {
		return nil, nil, fmt.Errorf("read change request deployment products: %w", err)
	}
	return deployments, products, nil
}

// insertChangeRequestJournalEntry appends a comment row of the given type
// (COMMENT = the customer-visible "Additional comments", WORK_NOTE = "Work
// notes") to the change request.
func insertChangeRequestJournalEntry(ctx context.Context, tx pgx.Tx, crID, commentType, createdBy, content string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		VALUES (gen_random_uuid(), NOW(), $1, $2::comment_type_enum, $3::uuid, $4)`,
		createdBy, commentType, crID, content); err != nil {
		return fmt.Errorf("insert change request %s: %w", strings.ToLower(commentType), err)
	}
	return nil
}

// changeRequestLinkPlan is what a PATCH decided to write for the scope fields.
type changeRequestLinkPlan struct {
	writeDeployments bool
	writeProducts    bool
	links            resolvedChangeRequestLinks
	// setSingulars mirrors the first chosen deployment / deployed product
	// into work_item.deployment_id / deployed_product_id, which the list
	// views and the single-valued PATCH fields still read.
	setSingulars      bool
	deploymentID      *string
	deployedProductID *string
}

// planChangeRequestLinks decides, for a PATCH carrying scope fields, what to
// write -- or returns the 400 that explains why not. snap is the change request
// as it stands, read by the caller AFTER it locked the work_item row (and the
// change_request row) in this transaction (lockChangeRequestForPatch), so the
// stored project and state it judges against are current. Returns nil when the
// request has no scope field. A field re-sent with the value already stored is a
// no-op and is never refused, whatever the state.
//
// The Customer Project can change only in the creation phase (New): after it the
// stored project is frozen (change_request_customer_lock.go), and the
// deployments and deployment products, which keep their until-implement window,
// are resolved against that stored project -- they can no longer be moved to
// another one together with it.
func planChangeRequestLinks(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, snap changeRequestGateSnapshot) (*changeRequestLinkPlan, error) {
	if req.ProjectID == nil && req.DeploymentIDs == nil && req.DeploymentProductIDs == nil {
		return nil, nil
	}
	if req.DeploymentIDs != nil && (req.DeploymentID != nil || req.DeployedProductID != nil) {
		return nil, linkValidationf("deploymentId and deployedProductId cannot be combined with deploymentIds: send deploymentIds only")
	}

	storedProject := snap.projectID
	storedDeps, storedProds, err := loadChangeRequestLinks(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("patch change request: %w", err)
	}
	storedDepIDs, storedProdIDs := entityRefIDs(storedDeps), entityRefIDs(storedProds)

	effProject := storedProject
	projectChanged := false
	if req.ProjectID != nil {
		effProject = req.ProjectID
		projectChanged = storedProject == nil || !strings.EqualFold(*storedProject, *req.ProjectID)
		// The caller has applied this rule already; it is kept here so that what
		// this function plans can never be a moved project after New.
		if err := checkCustomerProjectEdit(snap.state, storedProject, req.ProjectID); err != nil {
			return nil, err
		}
	}
	effDeps := storedDepIDs
	depsChanged := false
	if req.DeploymentIDs != nil {
		effDeps = normalizeUUIDList(*req.DeploymentIDs)
		depsChanged = !sameIDSet(effDeps, storedDepIDs)
	}
	if projectChanged && req.DeploymentIDs == nil && len(storedDepIDs) > 0 {
		return nil, linkValidationf("projectId cannot be changed without deploymentIds: the stored deployments belong to the current project (send deploymentIds for the new project; an empty array clears them)")
	}

	needResolve := projectChanged || depsChanged || req.DeploymentProductIDs != nil
	plan := &changeRequestLinkPlan{}
	if !needResolve {
		return plan, nil
	}

	sel := domain.ChangeRequestLinkSelection{ProjectID: effProject, DeploymentIDs: effDeps}
	if req.DeploymentProductIDs != nil {
		sel.DeploymentProductIDs = *req.DeploymentProductIDs
	}
	grand := make(map[string]bool, len(storedDepIDs))
	for _, d := range storedDepIDs {
		grand[strings.ToLower(d)] = true
	}
	links, err := resolveChangeRequestLinks(ctx, tx, sel, resolveLinkOpts{
		grandfatheredDeployments: grand,
		acceptedProductSets:      [][]string{storedProdIDs},
	})
	if err != nil {
		return nil, err
	}
	plan.links = links

	plan.writeDeployments = depsChanged
	plan.writeProducts = depsChanged || (req.DeploymentProductIDs != nil && !sameIDSet(links.productIDs(), storedProdIDs) && !sameIDSet(normalizeUUIDList(*req.DeploymentProductIDs), storedProdIDs))

	// Edit window (the project part of it is the stricter rule above, New only).
	var changedField string
	switch {
	case projectChanged:
		changedField = "projectId"
	case plan.writeDeployments:
		changedField = "deploymentIds"
	case plan.writeProducts:
		changedField = "deploymentProductIds"
	}
	if changedField != "" && changeRequestLinksLockedStates[snap.state] {
		return nil, linkValidationf("%s can no longer be changed: the change request is in state %q (project, deployments and deployment products are editable only before implementation starts)",
			changedField, strings.ToLower(snap.state))
	}

	if plan.writeDeployments {
		plan.setSingulars = true
		plan.deploymentID = firstOrNil(links.deploymentIDs())
		plan.deployedProductID = firstOrNil(links.productIDs())
	}
	return plan, nil
}

// applyChangeRequestLinkPlan writes the join rows the plan decided on.
func applyChangeRequestLinkPlan(ctx context.Context, tx pgx.Tx, id string, plan *changeRequestLinkPlan) error {
	if plan == nil {
		return nil
	}
	if plan.writeDeployments {
		if err := writeChangeRequestDeployments(ctx, tx, id, plan.links.deploymentIDs()); err != nil {
			return err
		}
	}
	if plan.writeProducts {
		if err := writeChangeRequestProducts(ctx, tx, id, plan.links.productIDs()); err != nil {
			return err
		}
	}
	return nil
}

// ValidateChangeRequestLinks implements ChangeRequestRepository.
//
// crvis: internal callers only: it is the create form's pre-flight, reached from POST /change-requests (internalOnly); it reads no change request
func (r *changeRequestRepo) ValidateChangeRequestLinks(ctx context.Context, sel domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
	res, err := resolveChangeRequestLinks(ctx, r.db, sel, resolveLinkOpts{})
	if err != nil {
		return domain.ChangeRequestLinkSet{}, err
	}
	return res.linkSet(), nil
}

// GetChangeRequestLinkOptions implements ChangeRequestRepository.
//
// crvis: internal callers only: POST /change-requests/link-options is wrapped by internalOnly (server/routes.go); it reads no change request, only the project's deployments
func (r *changeRequestRepo) GetChangeRequestLinkOptions(ctx context.Context, req domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error) {
	project := strings.ToLower(strings.TrimSpace(req.ProjectID))
	resp := domain.ChangeRequestLinkOptionsResponse{
		Deployments:        []domain.ChangeRequestDeploymentOption{},
		DeploymentProducts: []domain.ChangeRequestDeploymentProductOption{},
		CustomerContacts:   []domain.ChangeRequestCustomerContact{},
	}
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project WHERE id = $1::uuid)`, project).Scan(&exists); err != nil {
		return resp, fmt.Errorf("link options: check project: %w", err)
	}
	if !exists {
		return resp, linkValidationf("projectId does not refer to an existing project: %s", project)
	}
	// The read-only Customer Group of a change request on this project.
	contacts, err := customerContactRefs(ctx, r.db, project)
	if err != nil {
		return resp, fmt.Errorf("link options: %w", err)
	}
	resp.CustomerContacts = contacts

	rows, err := r.db.Query(ctx, `
		SELECT d.id::text, d.name, lower(d.type::text)
		FROM deployment d
		WHERE d.project_id = $1::uuid AND d.is_active = TRUE
		ORDER BY d.name, d.id`, project)
	if err != nil {
		return resp, fmt.Errorf("link options: query deployments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var opt domain.ChangeRequestDeploymentOption
		var typ *string
		if err := rows.Scan(&opt.ID, &opt.Name, &typ); err != nil {
			return resp, fmt.Errorf("link options: scan deployment: %w", err)
		}
		opt.Type = stringOrEmpty(typ)
		resp.Deployments = append(resp.Deployments, opt)
	}
	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("link options: iterate deployments: %w", err)
	}
	rows.Close()

	if len(req.DeploymentIDs) == 0 {
		return resp, nil
	}
	res, err := resolveChangeRequestLinks(ctx, r.db, domain.ChangeRequestLinkSelection{ProjectID: &project, DeploymentIDs: req.DeploymentIDs}, resolveLinkOpts{})
	if err != nil {
		return resp, err
	}
	depNames := make(map[string]domain.EntityRef, len(res.deployments))
	for _, d := range res.deployments {
		depNames[d.ref.ID] = d.ref
	}
	for _, p := range res.products {
		resp.DeploymentProducts = append(resp.DeploymentProducts, domain.ChangeRequestDeploymentProductOption{
			ID: p.ref.ID, Name: p.ref.Name, Deployment: depNames[p.deploymentID],
		})
	}
	return resp, nil
}
