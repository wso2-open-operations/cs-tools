package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Cloud signups land per product by id only; the Moesif contract is unchanged.
func TestIngestResolvesCloudInitiatingProduct(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewIngestRepository(pool)
	var err error

	type row struct {
		name    *string
		person  bool
		owner   *string
		product string
	}
	read := func(company string) row {
		var r row
		err := pool.QueryRow(ctx, `
			SELECT o.organization_name, o.registered_user IS NOT NULL, o.owner_user_id::TEXT,
			       COALESCE(string_agg(p.code, ',' ORDER BY p.code), '')
			FROM plg_organization o
			LEFT JOIN plg_org_platform op ON op.organization_id = o.id
			LEFT JOIN plg_product p ON p.id = op.product_id
			WHERE o.moesif_company_id = $1
			GROUP BY o.id`, company).Scan(&r.name, &r.person, &r.owner, &r.product)
		if err != nil {
			t.Fatalf("read %s: %v", company, err)
		}
		return r
	}
	reg := func(r domain.Registration) (*domain.IngestResult, error) {
		return repo.Register(ctx, r, nil)
	}
	const owner = "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b"

	// Each Cloud name lands on its product, by id only, even with a name and email sent.
	for key, want := range map[string]string{
		"wso2cloud": "WSO2_CLOUD", "core": "WSO2_CLOUD", "agent-manager": "AGENT_PLATFORM", "api-platform": "API_PLATFORM",
		"integration-platform": "INTEGRATION_PLATFORM", "identity-platform": "IAM",
	} {
		company := "cloudkey-" + key
		res, err := reg(domain.Registration{CompanyID: company, UserID: owner, InitiatingProduct: strings.ToUpper(key),
			OrganizationName: "Should Not Be Stored " + key, RegisteredEmail: key + "@leak.example"})
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if res.Status != "created" || res.OrganizationName != "" || res.PersonEmail != "" {
			t.Errorf("%s: result %+v", key, res)
		}
		got := read(company)
		if got.name != nil || got.person || got.owner == nil || *got.owner != owner || got.product != want {
			t.Errorf("%s: stored name=%v person=%v owner=%v product=%q; want no name, no person, owner, %s",
				key, got.name, got.person, got.owner, got.product, want)
		}
	}

	// Both fields agreeing is fine; disagreeing is refused and writes nothing.
	if _, err := reg(domain.Registration{CompanyID: "cloudkey-agree", UserID: owner,
		InitiatingProduct: "identity-platform", InitiatedPlatform: "iam"}); err != nil {
		t.Errorf("agreeing fields refused: %v", err)
	}
	_, err = reg(domain.Registration{CompanyID: "cloudkey-clash", UserID: owner,
		InitiatingProduct: "identity-platform", InitiatedPlatform: "API_PLATFORM"})
	if err == nil || !strings.Contains(err.Error(), "initiatedPlatform") {
		t.Errorf("clash: err = %v", err)
	}
	_, err = reg(domain.Registration{CompanyID: "cloudkey-unknown", UserID: owner, InitiatingProduct: "choreo"})
	if err == nil || !strings.Contains(err.Error(), "unknown initiatingProduct: choreo") {
		t.Errorf("unknown: err = %v", err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM plg_organization WHERE moesif_company_id IN ('cloudkey-clash','cloudkey-unknown')`).Scan(&n)
	if n != 0 {
		t.Errorf("a refused registration left %d organisations", n)
	}

	// The old contract is unchanged: a Moesif IAM registration keeps its name and person.
	if _, err := reg(domain.Registration{CompanyID: "cloudkey-legacy", InitiatedPlatform: "IAM",
		OrganizationName: "Legacy Org", RegisteredEmail: "owner@legacy.example", FirstName: "Lee"}); err != nil {
		t.Fatalf("legacy: %v", err)
	}
	if got := read("cloudkey-legacy"); got.name == nil || *got.name != "Legacy Org" || !got.person || got.product != "IAM" {
		t.Errorf("legacy stored %+v", got)
	}
	// And the old codes still work as initiatedPlatform with id-only payloads.
	if _, err := reg(domain.Registration{CompanyID: "cloudkey-oldcode", UserID: owner, InitiatedPlatform: "API_PLATFORM"}); err != nil {
		t.Errorf("old code: %v", err)
	}

	// A second Cloud signup for the same company on another product adds a pairing; the owner stays.
	res, err := reg(domain.Registration{CompanyID: "cloudkey-wso2cloud", UserID: "119a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b",
		InitiatingProduct: "api-platform"})
	if err != nil || res.Status != "updated" {
		t.Errorf("second product: %+v, %v", res, err)
	}
	if got := read("cloudkey-wso2cloud"); got.product != "API_PLATFORM,WSO2_CLOUD" || *got.owner != owner {
		t.Errorf("second product stored %+v (owner %v)", got, *got.owner)
	}
}
