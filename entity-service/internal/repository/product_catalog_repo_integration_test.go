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
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These run the catalogue writes against a live PostgreSQL instance with
// migrations 0270-0272, because the cross-field rules live in SQL inside the
// write transaction. Skipped unless ENTITY_TEST_DATABASE_URL is set (see
// account_repo_salesforce_integration_test.go for the setup).

const (
	catITProductCoded   = "5fc00000-0000-4000-8000-000000000001"
	catITProductNoCode  = "5fc00000-0000-4000-8000-000000000002"
	catITActor          = "catalog-it@wso2.com"
	catITVersionPrefix  = "it-"
	catITProductCodeVal = "itcode"
)

func newProductCatalogIntegrationRepo(t *testing.T) (*productCatalogRepo, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	clean := func() {
		ids := []string{catITProductCoded, catITProductNoCode}
		if _, err := pool.Exec(ctx, `DELETE FROM product_version_carbon WHERE product_version_id IN
			(SELECT id FROM product_version WHERE product_id = ANY($1::uuid[]))`, ids); err != nil {
			t.Fatalf("clean carbon: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM product_version WHERE product_id = ANY($1::uuid[])`, ids); err != nil {
			t.Fatalf("clean versions: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM product WHERE id = ANY($1::uuid[])`, ids); err != nil {
			t.Fatalf("clean products: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	if _, err := pool.Exec(ctx, `INSERT INTO product (id, name, category, code, created_on, created_by, updated_on, updated_by)
		VALUES ($1::uuid, 'IT Coded Product', 'SOFTWARE', $3, NOW(), 'it', NOW(), 'it'),
		       ($2::uuid, 'IT Uncoded Product', 'SOFTWARE', NULL, NOW(), 'it', NOW(), 'it')`,
		catITProductCoded, catITProductNoCode, catITProductCodeVal); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	return &productCatalogRepo{db: pool}, pool
}

func catITStr(column, cast, v string) CatalogSet {
	return CatalogSet{Column: column, Cast: cast, Value: &v}
}
func catITBool(column string, v bool) CatalogSet {
	return CatalogSet{Column: column, Cast: "%s::boolean", Value: &v}
}
func catITDate(column, v string) CatalogSet { return catITStr(column, "%s::date", v) }

// catITCreate adds a version of product with the given profiles and listing sets.
func catITCreate(t *testing.T, r *productCatalogRepo, product, version string, profiles []string, sets []CatalogSet, rowSets ...CatalogSet) ([]string, error) {
	t.Helper()
	all := append([]CatalogSet{
		catITStr("product_id", "%s::uuid", product),
		catITStr("version", "%s", catITVersionPrefix+version),
	}, sets...)
	return r.ApplyChange(context.Background(), catITActor, CatalogApply{
		EntityType: domain.CatalogEntityProductVersion, Operation: CatalogOpCreate,
		Sets: all, RowSets: rowSets, Profiles: profiles,
	})
}

func catITUpdate(r *productCatalogRepo, id string, sets []CatalogSet, rowSets ...CatalogSet) ([]string, error) {
	return r.ApplyChange(context.Background(), catITActor, CatalogApply{
		EntityType: domain.CatalogEntityProductVersion, Operation: CatalogOpUpdate, EntityID: id,
		Sets: sets, RowSets: rowSets,
	})
}

func catITRetire(r *productCatalogRepo, id string) ([]string, error) {
	return r.ApplyChange(context.Background(), catITActor, CatalogApply{
		EntityType: domain.CatalogEntityProductVersion, Operation: CatalogOpRetire, EntityID: id,
	})
}

func catITWantRule(t *testing.T, err error, substr string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, substr) {
		t.Fatalf("want a validation error containing %q, got %v", substr, err)
	}
}

func catITPublicID(t *testing.T, pool *pgxpool.Pool, id string) int64 {
	t.Helper()
	var pid int64
	if err := pool.QueryRow(context.Background(), `SELECT public_id FROM product_version WHERE id = $1::uuid`, id).Scan(&pid); err != nil {
		t.Fatalf("public id: %v", err)
	}
	return pid
}

func TestProductCatalogIntegration_VersionWithNoProfileCannotBeAddedTwice(t *testing.T) {
	r, _ := newProductCatalogIntegrationRepo(t)
	if _, err := catITCreate(t, r, catITProductCoded, "1.0.0", []string{""}, nil); err != nil {
		t.Fatalf("first add: %v", err)
	}
	_, err := catITCreate(t, r, catITProductCoded, "1.0.0", []string{""}, nil)
	catITWantRule(t, err, "already has a row for no deployment profile")
}

func TestProductCatalogIntegration_DateOrderUsesStoredValues(t *testing.T) {
	r, _ := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "2.0.0", []string{"All In One"}, []CatalogSet{
		catITDate("release_date", "2026-01-01"), catITDate("support_eol_date", "2030-01-01"),
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	_, err = catITUpdate(r, ids[0], []CatalogSet{catITDate("earliest_possible_support_eol_date", "2040-01-01")})
	catITWantRule(t, err, "EOL date is before the earliest possible EOL date")
	_, err = catITUpdate(r, ids[0], []CatalogSet{catITDate("support_eol_announcement_date", "2031-01-01")})
	catITWantRule(t, err, "announcement date is after the EOL date")
	_, err = catITUpdate(r, ids[0], []CatalogSet{catITDate("earliest_possible_support_eol_date", "2025-06-01")})
	catITWantRule(t, err, "earliest possible EOL date is before the release date")
	if _, err := catITUpdate(r, ids[0], []CatalogSet{
		catITDate("earliest_possible_support_eol_date", "2028-01-01"), catITDate("support_eol_announcement_date", "2029-01-01"),
	}); err != nil {
		t.Fatalf("valid dates refused: %v", err)
	}
}

func TestProductCatalogIntegration_PublicVersionNeedsAnAbbreviation(t *testing.T) {
	r, pool := newProductCatalogIntegrationRepo(t)
	_, err := catITCreate(t, r, catITProductNoCode, "1.0.0", []string{"All In One"}, []CatalogSet{catITBool("is_public", true)})
	catITWantRule(t, err, "no public abbreviation")
	if _, err := catITCreate(t, r, catITProductNoCode, "1.0.0", []string{"All In One"}, []CatalogSet{
		catITBool("is_public", true), catITStr("public_abbreviation", "%s", "itabbr"),
	}); err != nil {
		t.Fatalf("with abbreviation: %v", err)
	}
	// A row published before the rule (legacy data) can still be edited, but
	// not re-published without an abbreviation.
	legacy, err := catITCreate(t, r, catITProductNoCode, "1.1.0", []string{"All In One"}, []CatalogSet{
		catITStr("public_abbreviation", "%s", "tmp"), catITBool("is_public", true),
	})
	if err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE product_version SET public_abbreviation = NULL WHERE id = $1::uuid`, legacy[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := catITUpdate(r, legacy[0], []CatalogSet{catITDate("support_eol_date", "2031-01-01")}); err != nil {
		t.Fatalf("edit of a legacy row refused: %v", err)
	}
	_, err = catITUpdate(r, legacy[0], []CatalogSet{catITBool("show_in_support_matrix", true)})
	catITWantRule(t, err, "no public abbreviation")
	// The product with a code may publish without a version abbreviation.
	if _, err := catITCreate(t, r, catITProductCoded, "1.0.0", []string{"All In One"}, []CatalogSet{catITBool("show_in_support_matrix", true)}); err != nil {
		t.Fatalf("coded product: %v", err)
	}
}

func TestProductCatalogIntegration_JoinOnlyALiveListingOfTheSameVersion(t *testing.T) {
	r, pool := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "3.0.0", []string{"All In One"}, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	pid := catITPublicID(t, pool, ids[0])
	join := CatalogSet{Column: "public_id", Cast: "%s::bigint", Value: &pid}
	_, err = catITCreate(t, r, catITProductCoded, "3.1.0", []string{"Gateway"}, nil, join)
	catITWantRule(t, err, "not a live listing of this product and version")
	_, err = catITCreate(t, r, catITProductNoCode, "3.0.0", []string{"Gateway"}, nil, join)
	catITWantRule(t, err, "not a live listing of this product and version")
	far := int64(987654)
	_, err = catITCreate(t, r, catITProductCoded, "3.0.0", []string{"Gateway"}, nil, CatalogSet{Column: "public_id", Cast: "%s::bigint", Value: &far})
	catITWantRule(t, err, "not a live listing")
	joined, err := catITCreate(t, r, catITProductCoded, "3.0.0", []string{"Gateway"}, nil, join)
	if err != nil {
		t.Fatalf("join the same version: %v", err)
	}
	if got := catITPublicID(t, pool, joined[0]); got != pid {
		t.Fatalf("joined public id = %d, want %d", got, pid)
	}
}

func TestProductCatalogIntegration_RetiredRowEditTouchesOnlyItself(t *testing.T) {
	r, pool := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "4.0.0", []string{"All In One", "Gateway"}, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE product_version SET retired_on = NOW() WHERE id = $1::uuid`, ids[1]); err != nil {
		t.Fatalf("retire one row: %v", err)
	}
	got, err := catITUpdate(r, ids[1], []CatalogSet{catITStr("description", "%s", "retired only")})
	if err != nil || len(got) != 1 || got[0] != ids[1] {
		t.Fatalf("patch of a retired row reached %v (err %v)", got, err)
	}
	var live *string
	if err := pool.QueryRow(ctx, `SELECT description FROM product_version WHERE id = $1::uuid`, ids[0]).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != nil {
		t.Fatalf("live sibling changed to %q", *live)
	}
}

func TestProductCatalogIntegration_UnpublishedRowsMoveTogether(t *testing.T) {
	r, pool := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "5.0.0", []string{"All In One", "Gateway"}, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE product_version SET public_id = NULL WHERE id = ANY($1::uuid[])`, ids); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	got, err := catITUpdate(r, ids[0], []CatalogSet{catITDate("support_eol_date", "2031-01-01")})
	if err != nil || len(got) != 2 {
		t.Fatalf("patch reached %v (err %v), want both rows", got, err)
	}
	retired, err := catITRetire(r, ids[1])
	if err != nil || len(retired) != 2 {
		t.Fatalf("retire reached %v (err %v), want both rows", retired, err)
	}
}

func TestProductCatalogIntegration_SplitListingStillEditableOnOtherFields(t *testing.T) {
	r, pool := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "6.0.0", []string{"All In One", "Gateway"}, []CatalogSet{catITDate("release_date", "2026-01-01")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE product_version SET release_date = '2026-02-01' WHERE id = $1::uuid`, ids[1]); err != nil {
		t.Fatalf("split: %v", err)
	}
	if _, err := catITUpdate(r, ids[0], []CatalogSet{catITStr("description", "%s", "x")}); err != nil {
		t.Fatalf("unrelated edit refused: %v", err)
	}
	if _, err := catITUpdate(r, ids[0], []CatalogSet{catITStr("public_name", "%s", "IT name")}); err != nil {
		t.Fatalf("listing field edit refused: %v", err)
	}
}

func TestProductCatalogIntegration_ClearingACodePublishedVersionsRelyOn(t *testing.T) {
	r, _ := newProductCatalogIntegrationRepo(t)
	if _, err := catITCreate(t, r, catITProductCoded, "7.0.0", []string{"All In One"}, []CatalogSet{catITBool("is_public", true)}); err != nil {
		t.Fatalf("add: %v", err)
	}
	product := func(sets ...CatalogSet) error {
		_, err := r.ApplyChange(context.Background(), catITActor, CatalogApply{
			EntityType: domain.CatalogEntityProduct, Operation: CatalogOpUpdate, EntityID: catITProductCoded, Sets: sets,
		})
		return err
	}
	if err := product(catITStr("description", "%s", "edited")); err != nil {
		t.Fatalf("unrelated product edit refused: %v", err)
	}
	err := product(CatalogSet{Column: "code", Cast: "%s", Value: (*string)(nil)})
	catITWantRule(t, err, "would have no abbreviation")
}

func TestProductCatalogIntegration_RenamingAProfileToOneTakenIsRefused(t *testing.T) {
	r, _ := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "8.0.0", []string{"", "Gateway"}, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	gateway := ids[1]
	_, err = catITUpdate(r, gateway, nil, CatalogSet{Column: "deployment_profile", Cast: "%s", Value: (*string)(nil)})
	catITWantRule(t, err, "already has a row for no deployment profile")
	if _, err := catITUpdate(r, gateway, nil, catITStr("deployment_profile", "%s", "Gateway")); err != nil {
		t.Fatalf("keeping its own profile refused: %v", err)
	}
	if _, err := catITUpdate(r, gateway, nil, catITStr("deployment_profile", "%s", "Key Manager")); err != nil {
		t.Fatalf("free profile refused: %v", err)
	}
}

func TestProductCatalogIntegration_ConstraintErrorsNameTheField(t *testing.T) {
	r, _ := newProductCatalogIntegrationRepo(t)
	ids, err := catITCreate(t, r, catITProductCoded, "9.0.0", []string{"All In One"}, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	missing := "5fc00000-0000-4000-8000-0000000000ff"
	_, err = catITUpdate(r, ids[0], []CatalogSet{catITStr("release_line_id", "%s::uuid", missing)})
	catITWantRule(t, err, "releaseLineId: no such release line")
	var ve *apierror.ValidationError
	if errors.As(err, &ve) && (strings.Contains(ve.Msg, "release_line") || strings.Contains(ve.Msg, "Key (")) {
		t.Fatalf("database detail leaked: %q", ve.Msg)
	}
}
