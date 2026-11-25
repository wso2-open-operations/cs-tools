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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The visit and UTM context of a signup is stored per pairing, a value once
// known is kept, and the organisation detail and the dashboard read it back.
func TestRegistrationMetadataIsStoredKeptAndReported(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	ingest := NewIngestRepository(pool)
	const owner = "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b"
	const company = "meta-company-1"

	from, to := time.Now().AddDate(-1, 0, 0), time.Now().AddDate(1, 0, 0)
	before, err := NewAnalyticsRepository(pool).Dashboard(ctx, domain.AnalyticsRange{From: &from, To: &to})
	if err != nil {
		t.Fatalf("dashboard before: %v", err)
	}

	first, err := ingest.Register(ctx, domain.Registration{
		CompanyID: company, UserID: owner, InitiatingProduct: "identity-platform",
		Metadata: &domain.RegistrationMetadata{
			UTMSource: "google", UTMMedium: "cpc", UTMCampaign: "spring-launch",
			InitialUTMSource: "newsletter", ReferringDomain: "example.com", PreferredLanguage: "en-GB",
		},
	}, nil)
	if err != nil {
		t.Fatalf("first registration: %v", err)
	}

	// A later event for the same pairing fills what was missing and replaces nothing.
	if _, err := ingest.Register(ctx, domain.Registration{
		CompanyID: company, UserID: owner, InitiatingProduct: "identity-platform",
		Metadata: &domain.RegistrationMetadata{UTMSource: "bing", UTMCampaign: "other", SignInMethod: "github"},
	}, nil); err != nil {
		t.Fatalf("second registration: %v", err)
	}

	// Every product lands its own pairing with its own metadata.
	for key, src := range map[string]string{"api-platform": "linkedin", "integration-platform": "x"} {
		if _, err := ingest.Register(ctx, domain.Registration{
			CompanyID: company, UserID: owner, InitiatingProduct: key,
			Metadata: &domain.RegistrationMetadata{UTMSource: src},
		}, nil); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}

	d, err := NewOrganizationRepository(pool).Get(ctx, first.OrganizationID)
	if err != nil {
		t.Fatalf("get organization: %v", err)
	}
	m := d.RegistrationMetadata
	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	if m == nil || str(m.UTMSource) != "google" || str(m.UTMCampaign) != "spring-launch" ||
		str(m.UTMMedium) != "cpc" || str(m.InitialUTMSource) != "newsletter" ||
		str(m.ReferringDomain) != "example.com" || str(m.PreferredLanguage) != "en-GB" ||
		str(m.SignInMethod) != "github" || m.InitialUTMMedium != nil {
		t.Errorf("detail metadata = %+v", m)
	}
	if len(d.PlatformCards) != 3 {
		t.Errorf("platform cards = %d, want one per product", len(d.PlatformCards))
	}

	// An event with no metadata at all stores nothing.
	if _, err := ingest.Register(ctx, domain.Registration{
		CompanyID: "meta-company-2", UserID: owner, InitiatingProduct: "api-platform",
	}, nil); err != nil {
		t.Fatal(err)
	}
	var rows int
	_ = pool.QueryRow(ctx, `
		SELECT count(*) FROM plg_registration_metadata m
		JOIN plg_org_platform op ON op.id = m.org_platform_id
		JOIN plg_organization o ON o.id = op.organization_id
		WHERE o.moesif_company_id = 'meta-company-2'`).Scan(&rows)
	if rows != 0 {
		t.Errorf("an event with no metadata stored %d rows", rows)
	}

	// Outcome, classification and UTM source reach the dashboard.
	var pairing string
	if err := pool.QueryRow(ctx, `
		SELECT op.id::TEXT FROM plg_org_platform op JOIN plg_product p ON p.id = op.product_id
		WHERE op.organization_id::TEXT = $1 AND p.code = 'IAM'`, first.OrganizationID).Scan(&pairing); err != nil {
		t.Fatal(err)
	}
	seedValidationRun(t, pairing, "IAM", map[string]string{
		domain.TaskCodeEligibilityOutcome: "ELIGIBLE", domain.TaskCodeEmailClassification: "CORPORATE",
	})

	got, err := NewAnalyticsRepository(pool).Dashboard(ctx, domain.AnalyticsRange{From: &from, To: &to})
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	count := func(rows []domain.CountByLabel, label string) int {
		for _, r := range rows {
			if r.Label == label {
				return r.Count
			}
		}
		return 0
	}
	// An organisation counts once, under its first product: company 1 entered on
	// IAM (its API and Integration registrations came later), company 2 on API.
	delta := func(label string) int {
		return count(got.RegistrationsByEntryProduct, label) - count(before.RegistrationsByEntryProduct, label)
	}
	if delta("Identity & Access Management") != 1 || delta("API Platform") != 1 || delta("Integration Platform") != 0 {
		t.Errorf("entry products moved by IAM %d, API %d, Integration %d; want 1, 1, 0",
			delta("Identity & Access Management"), delta("API Platform"), delta("Integration Platform"))
	}
	// Counts are compared with the baseline: other tests share this database.
	gain := func(now, was []domain.CountByLabel, label string) int { return count(now, label) - count(was, label) }
	if gain(got.UTMSources, before.UTMSources, "google") != 1 || count(got.UTMSources, "NOT_RECORDED") < 1 {
		t.Errorf("utm sources = %+v", got.UTMSources)
	}
	if count(got.UTMSources, "linkedin") != 0 || count(got.UTMSources, "x") != 0 {
		t.Errorf("utm sources count a non-entry registration: %+v", got.UTMSources)
	}
	if gain(got.QualificationOverview, before.QualificationOverview, "ELIGIBLE") != 1 || count(got.QualificationOverview, "NOT_ASSESSED") < 1 {
		t.Errorf("qualification = %+v", got.QualificationOverview)
	}
	if gain(got.EmailClassification, before.EmailClassification, "CORPORATE") != 1 || count(got.EmailClassification, "NOT_CLASSIFIED") < 1 {
		t.Errorf("email classification = %+v", got.EmailClassification)
	}
	googleRow := func(rows []domain.UTMQualification) domain.UTMQualification {
		for _, q := range rows {
			if q.Label == "google" {
				return q
			}
		}
		return domain.UTMQualification{}
	}
	g, g0 := googleRow(got.QualifiedByUTM), googleRow(before.QualifiedByUTM)
	if g.Eligible-g0.Eligible != 1 || g.Total-g0.Total != 1 || g.NotAssessed-g0.NotAssessed != 0 {
		t.Errorf("qualified by utm = %+v (was %+v)", g, g0)
	}
	// Later registrations of the same organisation (linkedin, x) are not entry registrations.
	for _, q := range got.QualifiedByUTM {
		if q.Label == "linkedin" || q.Label == "x" {
			t.Errorf("qualified by utm counts a non-entry registration: %+v", q)
		}
	}
	if len(got.QualifiedByUTM) > maxUTMSources {
		t.Errorf("qualified by utm has %d rows, cap is %d", len(got.QualifiedByUTM), maxUTMSources)
	}
}

// seedValidationRun gives a pairing an attached registration run whose tasks
// hold the given text values, as the validation job leaves one.
func seedValidationRun(t *testing.T, pairing, product string, values map[string]string) {
	t.Helper()
	ctx := context.Background()
	// Its own playbook, named for this test: playbook names are unique and the
	// validation tests author theirs too.
	var playbookID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO plg_playbook (product_id, name, lifecycle_stage, playbook_type, display_order, active)
		VALUES ((SELECT id FROM plg_product WHERE code = $1), 'Registration metadata test', 'REGISTRATION', 'PROGRESSIVE', 1, TRUE)
		RETURNING id::TEXT`, product).Scan(&playbookID); err != nil {
		t.Fatalf("seed playbook: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO plg_playbook_task (playbook_id, code, name, sequence_no, value_type, options) VALUES
		($1::UUID, 'REGISTERED_EMAIL_CLASSIFICATION', 'Email', 1, 'SINGLE_SELECT',
		    '[{"code":"CORPORATE","label":"Corporate"},{"code":"PERSONAL","label":"Personal"}]'),
		($1::UUID, 'ELIGIBILITY_ASSESSMENT_OUTCOME', 'Outcome', 2, 'SINGLE_SELECT',
		    '[{"code":"ELIGIBLE","label":"Eligible"},{"code":"MONITOR","label":"Monitor"},{"code":"EXCLUDED","label":"Excluded"}]')`,
		playbookID); err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO plg_playbook_run (org_platform_id, playbook_id, added_by)
		VALUES ($1::UUID, $2::UUID, NULL) RETURNING id::TEXT`, pairing, playbookID).Scan(&runID); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO plg_playbook_run_task
		    (playbook_run_id, playbook_task_id, code, name, description, sequence_no, value_type, options)
		SELECT $1::UUID, t.id, t.code, t.name, t.description, t.sequence_no, t.value_type, t.options
		FROM   plg_playbook_task t WHERE t.playbook_id = $2::UUID`, runID, playbookID); err != nil {
		t.Fatalf("seed run tasks: %v", err)
	}
	for code, v := range values {
		if _, err := testPool.Exec(ctx, `
			UPDATE plg_playbook_run_task SET value_text = $3 WHERE playbook_run_id = $1::UUID AND code = $2`,
			runID, code, v); err != nil {
			t.Fatalf("set %s: %v", code, err)
		}
	}
}

// The organisation detail shows the earliest registration's visit context, and
// shows none when that registration has none: a later product's is not borrowed.
func TestOrganizationDetailDoesNotBorrowALaterRegistrationsMetadata(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	ingest := NewIngestRepository(pool)
	const owner = "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b"
	const company = "meta-company-3"
	now := time.Now()

	first, err := ingest.Register(ctx, domain.Registration{
		CompanyID: company, UserID: owner, InitiatingProduct: "api-platform",
		CreatedOn: &domain.SourceTime{Time: now.Add(-2 * time.Hour)},
	}, nil)
	if err != nil {
		t.Fatalf("first registration: %v", err)
	}
	if _, err := ingest.Register(ctx, domain.Registration{
		CompanyID: company, UserID: owner, InitiatingProduct: "integration-platform",
		CreatedOn: &domain.SourceTime{Time: now.Add(-time.Hour)},
		Metadata:  &domain.RegistrationMetadata{UTMSource: "later-product"},
	}, nil); err != nil {
		t.Fatalf("later registration: %v", err)
	}

	d, err := NewOrganizationRepository(pool).Get(ctx, first.OrganizationID)
	if err != nil {
		t.Fatalf("get organization: %v", err)
	}
	if d.RegistrationMetadata != nil {
		t.Errorf("metadata of a later registration shown as the original: %+v", d.RegistrationMetadata)
	}
}
