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
//
// Work items (and everything hung off them: comments, time cards, watchers,
// tags, escalations, SLAs) -- split out from generate.go for size only, part
// of the same generateAll() call chain.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var workItemSpecs = []struct {
	wtype  string
	prefix string
	count  int
}{
	{"CASE", "CASE", 8},
	{"SERVICE_REQUEST", "SR", 4},
	{"CHANGE_REQUEST", "CR", 3},
	{"INCIDENT", "INC", 3},
	{"ENGAGEMENT", "ENG", 2},
	{"PROBLEM", "PRB", 2},
}

func genWorkItems(
	ctx context.Context, tx pgx.Tx,
	accounts []genAccount, projects []genProject, deployments []genDeployment,
	deployedProducts []genDeployedProduct, internalUsers, externalUsers []genUser,
	accountContacts []genAccountContact, incidentSubcats, problemSubcats []genSubcategory,
	serviceIDs, offeringIDs []string, summary *genSummary,
) ([]genWorkItem, error) {
	projectsByAccount := map[string][]genProject{}
	for _, p := range projects {
		projectsByAccount[p.accountID] = append(projectsByAccount[p.accountID], p)
	}
	deploymentsByProject := map[string][]genDeployment{}
	for _, d := range deployments {
		deploymentsByProject[d.projectID] = append(deploymentsByProject[d.projectID], d)
	}
	deployedByDeployment := map[string][]genDeployedProduct{}
	for _, dp := range deployedProducts {
		deployedByDeployment[dp.deploymentID] = append(deployedByDeployment[dp.deploymentID], dp)
	}
	emailToUserID := map[string]string{}
	for _, u := range externalUsers {
		emailToUserID[u.email] = u.id
	}
	contactsByAccount := map[string][]genAccountContact{}
	for _, ac := range accountContacts {
		contactsByAccount[ac.accountID] = append(contactsByAccount[ac.accountID], ac)
	}

	var out []genWorkItem
	counter := 0

	for _, spec := range workItemSpecs {
		for i := 0; i < spec.count; i++ {
			acc := pick(accounts)
			projs := projectsByAccount[acc.id]
			if len(projs) == 0 {
				continue
			}
			proj := pick(projs)
			deps := deploymentsByProject[proj.id]
			if len(deps) == 0 {
				continue
			}
			dep := pick(deps)

			var deployedProductID *string
			if dps := deployedByDeployment[dep.id]; len(dps) > 0 && randBool(0.7) {
				id := pick(dps).id
				deployedProductID = &id
			}

			var openedBy, contactUser *string
			if contacts := contactsByAccount[acc.id]; len(contacts) > 0 {
				c := pick(contacts)
				if uid, ok := emailToUserID[c.email]; ok {
					openedBy, contactUser = &uid, &uid
				}
			}
			if openedBy == nil && len(externalUsers) > 0 {
				uid := pick(externalUsers).id
				openedBy = &uid
			}
			assignedTo := pick(internalUsers).id

			counter++
			number := fmt.Sprintf("%s-%04d", spec.prefix, 1000+counter)
			id := newUUID()
			createdOn := randRecentTime(1, 120)
			updatedOn := notAfterNow(createdOn.Add(time.Duration(randRange(1, 72)) * time.Hour))
			subject := subjectForType(spec.wtype)
			description := descriptionForType(spec.wtype)

			if _, err := tx.Exec(ctx, `
				INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
					number, wso2_id, subject, type, account_id, project_id, deployment_id,
					deployed_product_id, contact_user_id, opened_by_user_id, assigned_to_id, description)
				VALUES ($1,$2,$3,'seed-generator','seed-generator',$4,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
				ON CONFLICT (id) DO NOTHING`,
				id, createdOn, updatedOn, number, subject, spec.wtype, acc.id, proj.id, dep.id,
				deployedProductID, contactUser, openedBy, assignedTo, description); err != nil {
				return nil, fmt.Errorf("work_item %s: %w", number, err)
			}

			if err := insertWorkItemExtension(ctx, tx, spec.wtype, id, updatedOn, assignedTo,
				openedBy, incidentSubcats, problemSubcats, serviceIDs, offeringIDs); err != nil {
				return nil, fmt.Errorf("%s extension for %s: %w", spec.wtype, number, err)
			}

			summary.workItems++
			out = append(out, genWorkItem{
				id: id, number: number, wtype: spec.wtype,
				accountID: acc.id, projectID: proj.id, deploymentID: dep.id,
				deployedProductID: deployedProductID,
				openedByID:        openedBy,
				assignedToID:      &assignedTo,
				contactUserID:     contactUser,
				createdOn:         createdOn,
			})
		}
	}
	return out, nil
}

func subjectForType(wtype string) string {
	switch wtype {
	case "CASE":
		return fmt.Sprintf("%s %s", pick(caseSubjectVerbs), pick(caseSubjectProblems))
	case "SERVICE_REQUEST":
		return pick(srSubjects)
	case "CHANGE_REQUEST":
		return pick(crSubjects)
	case "INCIDENT":
		return pick(incidentSubjects)
	case "ENGAGEMENT":
		return pick(engagementSubjects)
	case "PROBLEM":
		return pick(problemSubjects)
	default:
		return "Generated work item"
	}
}

func descriptionForType(wtype string) string {
	if wtype == "CASE" {
		return fmt.Sprintf(pick(caseDescriptionTemplates), pick(caseSubjectProblems), pick(caseSubjectEnvs))
	}
	return "Generated for local dev testing (" + wtype + ")."
}

// insertWorkItemExtension inserts the type-specific row that extends
// work_item id (case/service_request/change_request/incident/engagement/
// problem -- see entity-service/migrations 000018, 000019, 000047, 000058,
// 000059). Every enum value used below is copied from the corresponding
// migration's CREATE TYPE list, not invented.
func insertWorkItemExtension(
	ctx context.Context, tx pgx.Tx, wtype, id string, updatedOn time.Time, assignedTo string,
	openedBy *string, incidentSubcats, problemSubcats []genSubcategory, serviceIDs, offeringIDs []string,
) error {
	switch wtype {
	case "CASE":
		severity := pick([]string{"S0", "S1", "S2", "S3", "S4"})
		issueType := pick([]string{"TOTAL_OUTAGE", "PARTIAL_OUTAGE", "PERFORMANCE_DEGRADATION", "QUESTION", "SECURITY_OR_COMPLIANCE", "ERROR"})
		state := pick([]string{"WORK_IN_PROGRESS", "AWAITING_INFO", "SOLUTION_PROPOSED", "CLOSED", "OPEN", "WAITING_ON_WSO2", "REOPENED"})
		workState := pick([]string{"ONGOING", "PAUSED"})
		var closedBy *string
		var closedOn, resolvedOn *time.Time
		var resolutionCode *string
		if state == "CLOSED" {
			cb := assignedTo
			closedBy = &cb
			co := updatedOn
			closedOn = &co
			ro := updatedOn.Add(-2 * time.Hour)
			resolvedOn = &ro
			rc := pick([]string{"SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED", "SOLVED_WORKAROUND_PROVIDED", "SOLVED_BY_CUSTOMER", "INCONCLUSIVE_OUT_OF_SCOPE"})
			resolutionCode = &rc
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO "case" (id, severity, issue_type, state, current_escalation_level,
				is_escalated, work_state, resolution_code, closed_by_user_id, closed_on, resolved_on)
			VALUES ($1,$2,$3,$4,'EL0',false,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO NOTHING`,
			id, severity, issueType, state, workState, resolutionCode, closedBy, closedOn, resolvedOn)
		return err

	case "SERVICE_REQUEST":
		state := pick([]string{"WORK_IN_PROGRESS", "AWAITING_INFO", "SOLUTION_PROPOSED", "CLOSED", "OPEN", "WAITING_ON_WSO2", "REOPENED"})
		cause := pick([]string{"SOLUTION_ARCHITECTURE", "USER_ERROR_CONFIGURATION", "PRODUCT_LIMITATION", "DOCUMENTATION_GAP", "UNKNOWN"})
		category := pick([]string{"General", "Access", "Configuration", "Licensing"})
		_, err := tx.Exec(ctx, `
			INSERT INTO service_request (id, state, cause, category)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (id) DO NOTHING`,
			id, state, cause, category)
		return err

	case "CHANGE_REQUEST":
		state := pick([]string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"})
		impact := pick([]string{"LOW", "MEDIUM", "HIGH"})
		priority := pick([]string{"LOW", "CRITICAL", "MODERATE", "HIGH"})
		category := pick([]string{"SOFTWARE", "NETWORK", "SERVICE", "HARDWARE", "OTHER"})
		risk := pick([]string{"HIGH", "MODERATE", "LOW"})
		crType := pick([]string{"INFRA", "GENERAL"})
		approval := pick([]string{"REQUESTED", "APPROVED", "REJECTED", "NOT_REQUESTED"})
		requestedBy := assignedTo
		if openedBy != nil {
			requestedBy = *openedBy
		}
		startOn := updatedOn.Add(48 * time.Hour)
		endOn := startOn.Add(2 * time.Hour)
		_, err := tx.Exec(ctx, `
			INSERT INTO change_request (id, start_on, end_on, impact, state, priority, category,
				risk, requested_by_user_id, approval, justification, change_request_type,
				is_customer_approved, is_customer_reviewed)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			ON CONFLICT (id) DO NOTHING`,
			id, startOn, endOn, impact, state, priority, category, risk, requestedBy, approval,
			"Generated change request for local dev testing.", crType,
			randBool(0.6), randBool(0.5))
		return err

	case "INCIDENT":
		priority := pick([]string{"CRITICAL", "HIGH", "MODERATE", "LOW"})
		state := pick([]string{"NEW", "IN_PROGRESS", "ON_HOLD", "RESOLVED", "CLOSED", "CANCELED"})
		impact := pick([]string{"HIGH", "MEDIUM", "LOW"})
		urgency := pick([]string{"HIGH", "MEDIUM", "LOW"})
		contactType := pick([]string{"EMAIL", "PHONE", "CHAT", "DIRECT"})
		var serviceID, serviceOfferingID *string
		if len(serviceIDs) > 0 {
			sid := pick(serviceIDs)
			serviceID = &sid
		}
		if len(offeringIDs) > 0 {
			oid := pick(offeringIDs)
			serviceOfferingID = &oid
		}
		var category, subcategoryID *string
		if len(incidentSubcats) > 0 {
			sc := pick(incidentSubcats)
			category, subcategoryID = &sc.category, &sc.id
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO incident (id, priority, state, service_id, opened_on, caller_id,
				category, subcategory_id, impact, urgency, service_offering_id, contact_type)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (id) DO NOTHING`,
			id, priority, state, serviceID, updatedOn, openedBy, category, subcategoryID,
			impact, urgency, serviceOfferingID, contactType)
		return err

	case "ENGAGEMENT":
		state := pick([]string{"WORK_IN_PROGRESS", "AWAITING_INFO", "SOLUTION_PROPOSED", "CLOSED", "OPEN", "WAITING_ON_WSO2", "REOPENED"})
		etype := pick([]string{"MIGRATION", "CONSULTANCY", "NEW_FEATURE_IMPROVEMENT", "FOLLOW_UP", "ONBOARDING"})
		paymentType := pick([]string{"PAID", "FOC"})
		startDate := updatedOn
		endDate := updatedOn.AddDate(0, 0, randRange(14, 90))
		_, err := tx.Exec(ctx, `
			INSERT INTO engagement (id, state, type, payment_type, start_date, end_date)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (id) DO NOTHING`,
			id, state, etype, paymentType, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		return err

	case "PROBLEM":
		state := pick([]string{"NEW", "ASSESS", "ROOT_CAUSE_ANALYSIS", "FIX_IN_PROGRESS", "RESOLVED", "CLOSED"})
		priority := pick([]string{"CRITICAL", "HIGH", "MODERATE", "LOW", "PLANNING"})
		var category *string
		var subcategoryID *string
		if len(problemSubcats) > 0 {
			sc := pick(problemSubcats)
			category, subcategoryID = &sc.category, &sc.id
		}
		isActive := state != "CLOSED" && state != "RESOLVED"
		_, err := tx.Exec(ctx, `
			INSERT INTO problem (id, opened_by_id, opened_on, state, is_active, priority,
				category, subcategory_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO NOTHING`,
			id, openedBy, updatedOn, state, isActive, priority, category, subcategoryID)
		return err

	default:
		return fmt.Errorf("insertWorkItemExtension: unhandled work item type %q", wtype)
	}
}

// ---- comments / time cards / watchers / tags / escalations / SLAs --------

func genComments(ctx context.Context, tx pgx.Tx, workItems []genWorkItem, internalUsers, externalUsers []genUser, summary *genSummary) error {
	emailByID := map[string]string{}
	for _, u := range internalUsers {
		emailByID[u.id] = u.email
	}
	for _, u := range externalUsers {
		emailByID[u.id] = u.email
	}

	for _, wi := range workItems {
		if !randBool(0.7) {
			continue
		}
		cursor := wi.createdOn
		n := randRange(1, 3)
		for i := 0; i < n; i++ {
			cursor = notAfterNow(cursor.Add(time.Duration(randRange(1, 48)) * time.Hour))
			authorEmail := "seed-generator"
			switch {
			case wi.assignedToID != nil && randBool(0.6):
				authorEmail = emailByID[*wi.assignedToID]
			case wi.openedByID != nil:
				authorEmail = emailByID[*wi.openedByID]
			}
			commentType, content := "COMMENT", pick(commentTemplates)
			if randBool(0.2) {
				commentType, content = "WORK_NOTE", pick(workNoteTemplates)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
				VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT (id) DO NOTHING`,
				newUUID(), cursor, authorEmail, commentType, wi.id, content); err != nil {
				return err
			}
			summary.comments++
		}
	}
	return nil
}

func genTimeCards(ctx context.Context, tx pgx.Tx, workItems []genWorkItem, projects []genProject, internalUsers []genUser, summary *genSummary) error {
	states := []string{"PENDING", "SUBMITTED", "APPROVED", "REJECTED", "RECALLED", "PROCESSED"}
	for _, wi := range workItems {
		if wi.wtype != "CASE" && wi.wtype != "SERVICE_REQUEST" {
			continue
		}
		if !randBool(0.5) {
			continue
		}
		n := randRange(1, 2)
		for i := 0; i < n; i++ {
			user := pick(internalUsers)
			state := pick(states)
			id := newUUID()
			createdOn := notAfterNow(wi.createdOn.Add(time.Duration(randRange(1, 72)) * time.Hour))
			var approvedBy *string
			if state == "APPROVED" || state == "PROCESSED" {
				ap := pick(internalUsers).id
				approvedBy = &ap
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
					case_id, customer_project_id, user_id, approved_by_id, work_date, is_billable,
					state, issue_complexity, analyzing_minutes, setting_up_minutes,
					reproducing_debugging_minutes, providing_solution_minutes, patching_minutes,
					work_log_comment)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6,$7,true,
					$8,$9,$10,$11,$12,$13,$14,$15)
				ON CONFLICT (id) DO NOTHING`,
				id, createdOn, wi.id, wi.projectID, user.id, approvedBy, createdOn.Format("2006-01-02"),
				state, pick([]string{"LOW", "MEDIUM", "HIGH"}), randRange(10, 60), randRange(0, 30),
				randRange(0, 45), randRange(0, 60), randRange(0, 20), pick(timeCardComments)); err != nil {
				return err
			}
			summary.timeCards++

			if approvedBy != nil && randBool(0.5) {
				if _, err := tx.Exec(ctx, `
					INSERT INTO time_card_approver (id, created_on, updated_on, created_by, updated_by,
						time_card_id, approver_id)
					VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4)
					ON CONFLICT (id) DO NOTHING`,
					newUUID(), createdOn, id, *approvedBy); err != nil {
					return err
				}
				summary.timeCardApprovers++
			}
		}
	}
	return nil
}

func genWatchers(ctx context.Context, tx pgx.Tx, workItems []genWorkItem, internalUsers []genUser, summary *genSummary) error {
	for _, wi := range workItems {
		if !randBool(0.3) {
			continue
		}
		for _, u := range pickN(internalUsers, randRange(1, 2)) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO work_item_watcher (id, work_item_id, user_id)
				VALUES ($1,$2,$3)
				ON CONFLICT (id) DO NOTHING`,
				newUUID(), wi.id, u.id); err != nil {
				return err
			}
			summary.watchers++
		}
	}
	return nil
}

func genTags(ctx context.Context, tx pgx.Tx, workItems []genWorkItem, summary *genSummary) error {
	var tagIDs []string
	createdOn := randRecentTime(300, 600)
	for _, name := range tagNames {
		id := newUUID()
		if _, err := tx.Exec(ctx, `
			INSERT INTO tag (id, created_on, updated_on, created_by, updated_by, name)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3)
			ON CONFLICT (id) DO NOTHING`,
			id, createdOn, name); err != nil {
			return err
		}
		summary.tags++
		tagIDs = append(tagIDs, id)
	}

	for _, wi := range workItems {
		if !randBool(0.5) {
			continue
		}
		for _, tid := range pickN(tagIDs, randRange(1, 2)) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO work_item_tag (id, created_on, updated_on, created_by, updated_by,
					work_item_id, tag_id)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4)
				ON CONFLICT (id) DO NOTHING`,
				newUUID(), wi.createdOn, wi.id, tid); err != nil {
				return err
			}
			summary.workItemTags++
		}
	}
	return nil
}

func genEscalations(ctx context.Context, tx pgx.Tx, workItems []genWorkItem, internalUsers []genUser, summary *genSummary) error {
	var caseItems []genWorkItem
	for _, wi := range workItems {
		if wi.wtype == "CASE" {
			caseItems = append(caseItems, wi)
		}
	}
	if len(caseItems) == 0 {
		return nil
	}
	n := 3
	if n > len(caseItems) {
		n = len(caseItems)
	}
	levels := []string{"EL1", "EL2", "EL3"}

	for i, wi := range pickN(caseItems, n) {
		level := levels[i%len(levels)]

		if _, err := tx.Exec(ctx, `
			UPDATE "case" SET is_escalated = true, current_escalation_level = $1
			WHERE id = $2`, level, wi.id); err != nil {
			return err
		}

		escID := newUUID()
		createdOn := wi.createdOn.Add(24 * time.Hour)
		if _, err := tx.Exec(ctx, `
			INSERT INTO case_escalation (id, created_on, updated_on, created_by, updated_by,
				work_item_id, current_level, previous_level, reason)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,'EL0',$5)
			ON CONFLICT (id) DO NOTHING`,
			escID, createdOn, wi.id, level, pick(escalationReasons)); err != nil {
			return err
		}
		summary.escalations++

		for _, u := range pickN(internalUsers, randRange(1, 2)) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO case_escalation_notification_list (id, case_escalation_id, user_id)
				VALUES ($1,$2,$3)
				ON CONFLICT (id) DO NOTHING`,
				newUUID(), escID, u.id); err != nil {
				return err
			}
			summary.escalationNotified++
		}
	}
	return nil
}

func genSLAs(ctx context.Context, tx pgx.Tx, workItems []genWorkItem, policies []genSLAPolicy, summary *genSummary) error {
	if len(policies) == 0 {
		return nil
	}
	stages := []string{"IN_PROGRESS", "ACHIEVED", "BREACHED", "PAUSED", "COMPLETED"}
	durationByTarget := map[string]string{
		"RESPONSE":   "4 hours",
		"WORKAROUND": "1 day",
		"RESOLUTION": "5 days",
	}

	for _, wi := range workItems {
		if wi.wtype != "CASE" && wi.wtype != "SERVICE_REQUEST" && wi.wtype != "INCIDENT" {
			continue
		}
		if !randBool(0.6) {
			continue
		}
		policy := pick(policies)
		stage := pick(stages)
		startOn := wi.createdOn

		if _, err := tx.Exec(ctx, `
			INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id,
				sla_policy_id, schedule, is_active, stage, has_breached, start_on, duration)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,'24x7',true,$5,$6,$7,$8::interval)
			ON CONFLICT (id) DO NOTHING`,
			newUUID(), startOn, wi.id, policy.id, stage, stage == "BREACHED", startOn,
			durationByTarget[policy.target]); err != nil {
			return err
		}
		summary.slas++
	}
	return nil
}
