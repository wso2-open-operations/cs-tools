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
// generateAll builds every row this tool inserts, in FK dependency order:
// users -> accounts -> account contacts -> projects -> project contacts ->
// deployments -> products/versions -> deployed products -> services ->
// SLA policies -> work items (+ their type-extension row) -> comments,
// time cards, watchers, tags, escalations, SLAs. All in one transaction
// (see main.go), so a failure partway through leaves nothing behind for the
// next `docker compose up` to retry from scratch.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Fixed role ids from scripts/csm-compose/seed-entity-service.sql, which
// migrate-and-seed.sh always loads before this program runs (see
// docker-compose.yml's dependency chain) -- reused rather than inserting new
// role rows, since recompute_user_type() (migration 000007) only recognizes
// the role *names* those two rows already carry ("internal"/"customer").
const (
	roleInternalID = "00000000-0000-0000-0000-000000000101"
	roleCustomerID = "00000000-0000-0000-0000-000000000102"
)

const (
	numInternalUsers = 6
	numExternalUsers = 6
	numAccounts      = 4
)

type genUser struct {
	id, email, firstName, lastName, fullName string
	internal                                 bool
}

type genAccount struct {
	id, name string
}

type genAccountContact struct {
	id, accountID, email, displayName string
}

type genProject struct {
	id, accountID, key string
}

type genDeployment struct {
	id, projectID string
}

type genProduct struct {
	id, name, unit string
}

type genProductVersion struct {
	id, productID, version string
}

type genDeployedProduct struct {
	id, projectID, deploymentID, productID, versionID string
}

type genSLAPolicy struct {
	id, target string
}

type genSubcategory struct {
	id, category string
}

type genWorkItem struct {
	id, number, wtype, accountID, projectID, deploymentID string
	deployedProductID                                     *string
	openedByID, assignedToID, contactUserID               *string
	createdOn                                             time.Time
}

// genSummary counts what generateAll inserted, for the marker row and the
// program's final log line.
type genSummary struct {
	users, accounts, accountContacts, projects, projectContacts   int
	deployments, products, productVersions, deployedProducts      int
	services, serviceOfferings, slaPolicies                       int
	workItems, comments, timeCards, timeCardApprovers             int
	watchers, tags, workItemTags, escalations, escalationNotified int
	slas                                                          int
}

func (s genSummary) String() string {
	return fmt.Sprintf(
		"users=%d accounts=%d account_contacts=%d projects=%d project_contacts=%d "+
			"deployments=%d products=%d product_versions=%d deployed_products=%d "+
			"services=%d service_offerings=%d sla_policies=%d work_items=%d comments=%d "+
			"time_cards=%d time_card_approvers=%d watchers=%d tags=%d work_item_tags=%d "+
			"escalations=%d escalation_notified=%d slas=%d",
		s.users, s.accounts, s.accountContacts, s.projects, s.projectContacts,
		s.deployments, s.products, s.productVersions, s.deployedProducts,
		s.services, s.serviceOfferings, s.slaPolicies, s.workItems, s.comments,
		s.timeCards, s.timeCardApprovers, s.watchers, s.tags, s.workItemTags,
		s.escalations, s.escalationNotified, s.slas)
}

func generateAll(ctx context.Context, tx pgx.Tx) (genSummary, error) {
	var summary genSummary

	users, err := genUsers(ctx, tx, &summary)
	if err != nil {
		return summary, fmt.Errorf("users: %w", err)
	}
	var internalUsers, externalUsers []genUser
	for _, u := range users {
		if u.internal {
			internalUsers = append(internalUsers, u)
		} else {
			externalUsers = append(externalUsers, u)
		}
	}

	accounts, err := genAccounts(ctx, tx, internalUsers, &summary)
	if err != nil {
		return summary, fmt.Errorf("accounts: %w", err)
	}

	accountContacts, err := genAccountContacts(ctx, tx, accounts, externalUsers, &summary)
	if err != nil {
		return summary, fmt.Errorf("account contacts: %w", err)
	}

	projects, err := genProjects(ctx, tx, accounts, &summary)
	if err != nil {
		return summary, fmt.Errorf("projects: %w", err)
	}

	if err := genProjectContacts(ctx, tx, projects, accountContacts, &summary); err != nil {
		return summary, fmt.Errorf("project contacts: %w", err)
	}

	deployments, err := genDeployments(ctx, tx, projects, &summary)
	if err != nil {
		return summary, fmt.Errorf("deployments: %w", err)
	}

	products, versions, err := genProducts(ctx, tx, &summary)
	if err != nil {
		return summary, fmt.Errorf("products: %w", err)
	}

	deployedProducts, err := genDeployedProducts(ctx, tx, deployments, products, versions, &summary)
	if err != nil {
		return summary, fmt.Errorf("deployed products: %w", err)
	}

	serviceIDs, offeringIDs, err := genServices(ctx, tx, &summary)
	if err != nil {
		return summary, fmt.Errorf("services: %w", err)
	}

	slaPolicies, err := genSLAPolicies(ctx, tx, &summary)
	if err != nil {
		return summary, fmt.Errorf("sla policies: %w", err)
	}

	incidentSubcats, err := loadSubcategories(ctx, tx, "incident_subcategory")
	if err != nil {
		return summary, fmt.Errorf("loading incident_subcategory: %w", err)
	}
	problemSubcats, err := loadSubcategories(ctx, tx, "problem_subcategory")
	if err != nil {
		return summary, fmt.Errorf("loading problem_subcategory: %w", err)
	}

	workItems, err := genWorkItems(ctx, tx, accounts, projects, deployments, deployedProducts,
		internalUsers, externalUsers, accountContacts, incidentSubcats, problemSubcats,
		serviceIDs, offeringIDs, &summary)
	if err != nil {
		return summary, fmt.Errorf("work items: %w", err)
	}

	if err := genComments(ctx, tx, workItems, internalUsers, externalUsers, &summary); err != nil {
		return summary, fmt.Errorf("comments: %w", err)
	}

	if err := genTimeCards(ctx, tx, workItems, projects, internalUsers, &summary); err != nil {
		return summary, fmt.Errorf("time cards: %w", err)
	}

	if err := genWatchers(ctx, tx, workItems, internalUsers, &summary); err != nil {
		return summary, fmt.Errorf("watchers: %w", err)
	}

	if err := genTags(ctx, tx, workItems, &summary); err != nil {
		return summary, fmt.Errorf("tags: %w", err)
	}

	if err := genEscalations(ctx, tx, workItems, internalUsers, &summary); err != nil {
		return summary, fmt.Errorf("escalations: %w", err)
	}

	if err := genSLAs(ctx, tx, workItems, slaPolicies, &summary); err != nil {
		return summary, fmt.Errorf("slas: %w", err)
	}

	return summary, nil
}

// ---- users ----------------------------------------------------------------

func genUsers(ctx context.Context, tx pgx.Tx, summary *genSummary) ([]genUser, error) {
	var out []genUser
	usedNames := map[string]bool{}

	makeUser := func(internal bool) genUser {
		var first, last string
		for {
			first, last = pick(firstNames), pick(lastNames)
			key := first + last
			if !usedNames[key] {
				usedNames[key] = true
				break
			}
		}
		domain := "wso2.com"
		if !internal {
			domain = pick(emailProviders)
		}
		email := fmt.Sprintf("%s.%s+%d@%s", lowerASCII(first), lowerASCII(last), randRange(1000, 9999), domain)
		return genUser{
			id:        newUUID(),
			email:     email,
			firstName: first,
			lastName:  last,
			fullName:  first + " " + last,
			internal:  internal,
		}
	}

	for i := 0; i < numInternalUsers; i++ {
		out = append(out, makeUser(true))
	}
	for i := 0; i < numExternalUsers; i++ {
		out = append(out, makeUser(false))
	}

	for _, u := range out {
		createdOn := randRecentTime(60, 400)
		if _, err := tx.Exec(ctx, `
			INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
				user_name, name, first_name, last_name, email, is_active, is_system_user)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6,$3,true,false)
			ON CONFLICT (id) DO NOTHING`,
			u.id, createdOn, u.email, u.fullName, u.firstName, u.lastName); err != nil {
			return nil, err
		}

		roleID := roleInternalID
		if !u.internal {
			roleID = roleCustomerID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_role (id, created_on, updated_on, user_id, role_id)
			VALUES ($1,$2,$2,$3,$4) ON CONFLICT (id) DO NOTHING`,
			newUUID(), createdOn, u.id, roleID); err != nil {
			return nil, err
		}
		summary.users++
	}
	return out, nil
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// ---- accounts / contacts ---------------------------------------------------

func genAccounts(ctx context.Context, tx pgx.Tx, internalUsers []genUser, summary *genSummary) ([]genAccount, error) {
	var out []genAccount
	usedNames := map[string]bool{}

	for i := 0; i < numAccounts; i++ {
		var name string
		for {
			name = fmt.Sprintf("%s %s", pick(companyAdjectives), pick(companyNouns))
			if !usedNames[name] {
				usedNames[name] = true
				break
			}
		}
		id := newUUID()
		number := fmt.Sprintf("ACC-%04d", 2000+i)
		sfID := fmt.Sprintf("SF-ACC-%04d", 2000+i)
		csm := pick(internalUsers)
		techOwner := pick(internalUsers)
		acctMgr := pick(internalUsers)
		createdOn := randRecentTime(180, 500)
		activation := createdOn.AddDate(0, 0, -randRange(0, 30))

		if _, err := tx.Exec(ctx, `
			INSERT INTO account (id, created_on, updated_on, created_by, updated_by,
				name, number, sf_id, activation_date, customer_success_manager_id,
				technical_owner_id, account_manager_id)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO NOTHING`,
			id, createdOn, name, number, sfID, activation.Format("2006-01-02"),
			csm.id, techOwner.id, acctMgr.id); err != nil {
			return nil, err
		}
		summary.accounts++
		out = append(out, genAccount{id: id, name: name})
	}
	return out, nil
}

func genAccountContacts(ctx context.Context, tx pgx.Tx, accounts []genAccount, externalUsers []genUser, summary *genSummary) ([]genAccountContact, error) {
	var out []genAccountContact
	if len(externalUsers) == 0 {
		return out, nil
	}
	ei := 0
	nextExternal := func() genUser {
		u := externalUsers[ei%len(externalUsers)]
		ei++
		return u
	}

	for _, acc := range accounts {
		n := randRange(1, 2)
		for i := 0; i < n; i++ {
			eu := nextExternal()
			id := newUUID()
			createdOn := randRecentTime(60, 300)
			if _, err := tx.Exec(ctx, `
				INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by,
					is_active, user_name, is_primary_contact, account_id)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',true,$3,$4,$5)
				ON CONFLICT (id) DO NOTHING`,
				id, createdOn, eu.fullName+" <"+eu.email+">", i == 0, acc.id); err != nil {
				return nil, err
			}
			summary.accountContacts++
			out = append(out, genAccountContact{id: id, accountID: acc.id, email: eu.email, displayName: eu.fullName})
		}
	}
	return out, nil
}

// ---- projects / project contacts / deployments -----------------------------

func genProjects(ctx context.Context, tx pgx.Tx, accounts []genAccount, summary *genSummary) ([]genProject, error) {
	var out []genProject
	for ai, acc := range accounts {
		n := randRange(1, 2)
		for pi := 0; pi < n; pi++ {
			id := newUUID()
			key := fmt.Sprintf("GEN%02d-PROJ%d", ai+1, pi+1)
			sfID := fmt.Sprintf("SF-PROJ-GEN-%02d-%d", ai+1, pi+1)
			name := acc.name + " " + pick([]string{"Production", "Platform", "Core Services", "Integration"})
			createdOn := randRecentTime(60, 400)
			if _, err := tx.Exec(ctx, `
				INSERT INTO project (id, created_on, updated_on, created_by, updated_by,
					key, sf_id, name, account_id, is_active, start_date)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6,true,$7)
				ON CONFLICT (id) DO NOTHING`,
				id, createdOn, key, sfID, name, acc.id, createdOn.Format("2006-01-02")); err != nil {
				return nil, err
			}
			summary.projects++
			out = append(out, genProject{id: id, accountID: acc.id, key: key})
		}
	}
	return out, nil
}

func genProjectContacts(ctx context.Context, tx pgx.Tx, projects []genProject, accountContacts []genAccountContact, summary *genSummary) error {
	byAccount := map[string][]genAccountContact{}
	for _, ac := range accountContacts {
		byAccount[ac.accountID] = append(byAccount[ac.accountID], ac)
	}

	states := []string{"INVITED", "RE-INVITED", "DEACTIVATED"}
	for _, proj := range projects {
		contacts := byAccount[proj.accountID]
		if len(contacts) == 0 {
			continue
		}
		// The first project_contact for every project is always REGISTERED,
		// so at least one generated external user can log into the customer
		// portal and see real data via RegisteredProjectIDs (see
		// entity-service/internal/repository/access_repo.go) -- any other
		// account_contact attached to the same project varies across the
		// remaining lifecycle states for list-view variety.
		for ci, ac := range contacts {
			state := "REGISTERED"
			if ci > 0 {
				state = pick(states)
			}
			createdOn := randRecentTime(30, 200)
			if _, err := tx.Exec(ctx, `
				INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by,
					email, state, account_contact_id, project_id)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6)
				ON CONFLICT (id) DO NOTHING`,
				newUUID(), createdOn, ac.email, state, ac.id, proj.id); err != nil {
				return err
			}
			summary.projectContacts++
		}
	}
	return nil
}

func genDeployments(ctx context.Context, tx pgx.Tx, projects []genProject, summary *genSummary) ([]genDeployment, error) {
	var out []genDeployment
	types := []string{"PRIMARY_PRODUCTION", "STAGING", "UAT", "DEVELOPMENT", "QA"}
	names := map[string]string{
		"PRIMARY_PRODUCTION": "Production",
		"STAGING":            "Staging",
		"UAT":                "UAT",
		"DEVELOPMENT":        "Development",
		"QA":                 "QA",
	}
	for di, proj := range projects {
		n := randRange(1, 2)
		usedTypes := map[string]bool{}
		for i := 0; i < n; i++ {
			t := "PRIMARY_PRODUCTION"
			if i > 0 {
				for {
					t = pick(types)
					if !usedTypes[t] {
						break
					}
				}
			}
			usedTypes[t] = true
			id := newUUID()
			number := fmt.Sprintf("DEP-GEN-%03d", di*10+i)
			createdOn := randRecentTime(60, 380)
			if _, err := tx.Exec(ctx, `
				INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by,
					number, name, description, type, is_active, project_id)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6,true,$7)
				ON CONFLICT (id) DO NOTHING`,
				id, createdOn, number, names[t], "Generated "+names[t]+" deployment", t, proj.id); err != nil {
				return nil, err
			}
			summary.deployments++
			out = append(out, genDeployment{id: id, projectID: proj.id})
		}
	}
	return out, nil
}

// ---- products ---------------------------------------------------------------

func genProducts(ctx context.Context, tx pgx.Tx, summary *genSummary) ([]genProduct, []genProductVersion, error) {
	var products []genProduct
	var versions []genProductVersion

	for _, p := range productCatalog {
		id := newUUID()
		createdOn := randRecentTime(300, 600)
		if _, err := tx.Exec(ctx, `
			INSERT INTO product (id, created_on, updated_on, created_by, updated_by,
				manufacturer, category, business_unit, unit, name)
			VALUES ($1,$2,$2,'seed-generator','seed-generator','WSO2','SOFTWARE',$3,$4,$5)
			ON CONFLICT (id) DO NOTHING`,
			id, createdOn, p.businessUnit, p.unit, p.name); err != nil {
			return nil, nil, err
		}
		summary.products++
		products = append(products, genProduct{id: id, name: p.name, unit: p.unit})

		vs := pickN(productVersionNumbers, 2)
		for _, v := range vs {
			vid := newUUID()
			if _, err := tx.Exec(ctx, `
				INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by,
					version, product_id, current_support_status, release_date)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,$5,$6)
				ON CONFLICT (id) DO NOTHING`,
				vid, createdOn, v, id, pick([]string{"AVAILABLE", "EXTENDED", "DEPRECATED"}),
				createdOn.Format("2006-01-02")); err != nil {
				return nil, nil, err
			}
			summary.productVersions++
			versions = append(versions, genProductVersion{id: vid, productID: id, version: v})
		}
	}
	return products, versions, nil
}

func genDeployedProducts(ctx context.Context, tx pgx.Tx, deployments []genDeployment, products []genProduct, versions []genProductVersion, summary *genSummary) ([]genDeployedProduct, error) {
	var out []genDeployedProduct
	versionsByProduct := map[string][]genProductVersion{}
	for _, v := range versions {
		versionsByProduct[v.productID] = append(versionsByProduct[v.productID], v)
	}

	n := 0
	for _, dep := range deployments {
		count := randRange(1, 3)
		for i := 0; i < count; i++ {
			p := pick(products)
			vs := versionsByProduct[p.id]
			if len(vs) == 0 {
				continue
			}
			v := pick(vs)
			id := newUUID()
			n++
			number := fmt.Sprintf("DP-GEN-%04d", n)
			createdOn := randRecentTime(30, 300)
			if _, err := tx.Exec(ctx, `
				INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by,
					number, name, active, life_cycle_stage_status, life_cycle_stage, core_count,
					deployment_id, product_id, version_id, product_category)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,true,'AVAILABLE','OPERATIONAL',$5,$6,$7,$8,'PDP')
				ON CONFLICT (id) DO NOTHING`,
				id, createdOn, number, p.name+" "+v.version, randRange(2, 16), dep.id, p.id, v.id); err != nil {
				return nil, err
			}
			summary.deployedProducts++
			out = append(out, genDeployedProduct{id: id, deploymentID: dep.id, productID: p.id, versionID: v.id})
		}
	}
	return out, nil
}

// ---- services (for incident.service_id/service_offering_id variety) -------

func genServices(ctx context.Context, tx pgx.Tx, summary *genSummary) (serviceIDs, offeringIDs []string, err error) {
	names := []string{"API Management", "Identity & Access", "Integration Runtime"}
	for i, name := range names {
		id := newUUID()
		createdOn := randRecentTime(300, 600)
		if _, err := tx.Exec(ctx, `
			INSERT INTO service (id, created_on, updated_on, created_by, updated_by,
				name, status, number, business_criticality, consumer_type, state)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,'OPERATIONAL',$4,'SOMEWHAT_CRITICAL','BOTH','PUBLISHED')
			ON CONFLICT (id) DO NOTHING`,
			id, createdOn, name, fmt.Sprintf("SVC-GEN-%03d", i+1)); err != nil {
			return nil, nil, err
		}
		summary.services++
		serviceIDs = append(serviceIDs, id)

		for j := 0; j < 2; j++ {
			oid := newUUID()
			if _, err := tx.Exec(ctx, `
				INSERT INTO service_offering (id, created_on, updated_on, created_by, updated_by,
					name, number, status, business_criticality, parent_id, consumer_type, state)
				VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,$4,'OPERATIONAL','SOMEWHAT_CRITICAL',$5,'BOTH','PUBLISHED')
				ON CONFLICT (id) DO NOTHING`,
				oid, createdOn, name+" Offering "+fmt.Sprint(j+1), fmt.Sprintf("SVO-GEN-%03d-%d", i+1, j+1), id); err != nil {
				return nil, nil, err
			}
			summary.serviceOfferings++
			offeringIDs = append(offeringIDs, oid)
		}
	}
	return serviceIDs, offeringIDs, nil
}

// ---- SLA policies -----------------------------------------------------------

func genSLAPolicies(ctx context.Context, tx pgx.Tx, summary *genSummary) ([]genSLAPolicy, error) {
	var out []genSLAPolicy
	targets := []struct {
		target   string
		duration string
	}{
		{"RESPONSE", "4 hours"},
		{"WORKAROUND", "1 day"},
		{"RESOLUTION", "5 days"},
	}
	for _, t := range targets {
		id := newUUID()
		createdOn := randRecentTime(300, 600)
		if _, err := tx.Exec(ctx, `
			INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by,
				name, is_active, target, duration, schedule)
			VALUES ($1,$2,$2,'seed-generator','seed-generator',$3,true,$4,$5::interval,'24x7')
			ON CONFLICT (id) DO NOTHING`,
			id, createdOn, "Generated "+t.target+" SLA", t.target, t.duration); err != nil {
			return nil, err
		}
		summary.slaPolicies++
		out = append(out, genSLAPolicy{id: id, target: t.target})
	}
	return out, nil
}

// ---- subcategory lookups (already seeded by entity-service migrations) ----

func loadSubcategories(ctx context.Context, tx pgx.Tx, table string) ([]genSubcategory, error) {
	// table is always one of the two literal names this file calls with
	// (never user input), so this is not a SQL-injection-relevant format.
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id::text, category::text FROM %s`, table)) //nolint:gosec // fixed internal table name
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []genSubcategory
	for rows.Next() {
		var s genSubcategory
		if err := rows.Scan(&s.id, &s.category); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
