package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// IngestRepository writes what the registration source delivers.
//
// Everything here obeys one rule: a later payload never replaces a known value
// with nothing. Each upsert coalesces the incoming value against the stored one,
// so NULL keeps its single meaning — "the source has never told us" — rather
// than also meaning "the source told us, then a sparse payload wiped it".
type IngestRepository interface {
	Register(ctx context.Context, in domain.Registration, attrs []domain.OrganizationAttribute) (*domain.IngestResult, error)
}

type ingestRepository struct{ db *pgxpool.Pool }

// NewIngestRepository builds an IngestRepository over the given pool.
func NewIngestRepository(db *pgxpool.Pool) IngestRepository {
	return &ingestRepository{db: db}
}

// Register lands one payload: person, then organisation, then pairing.
//
// One transaction, because a pairing with no organisation is not a partial
// result — it is a corrupt one.
func (r *ingestRepository) Register(ctx context.Context, in domain.Registration,
	attrs []domain.OrganizationAttribute) (*domain.IngestResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ingest: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	registeredOn := time.Now()
	if in.CreatedOn != nil && !in.CreatedOn.IsZero() {
		registeredOn = in.CreatedOn.Time
	}

	personID, err := upsertPerson(ctx, tx, in)
	if err != nil {
		return nil, err
	}

	orgID, orgCreated, err := upsertOrganization(ctx, tx, in, personID, registeredOn)
	if err != nil {
		return nil, err
	}

	res := &domain.IngestResult{
		OrganizationID:   orgID,
		OrganizationName: strings.TrimSpace(in.OrganizationName),
		PersonEmail:      strings.ToLower(strings.TrimSpace(in.RegisteredEmail)),
	}

	// A payload with no platform is legitimate — it just registers the customer,
	// and the pairing arrives with a later event.
	platform := strings.ToUpper(strings.TrimSpace(in.InitiatedPlatform))
	var pairingCreated bool
	if platform != "" {
		pairID, created, err := upsertOrgPlatform(ctx, tx, in, orgID, platform, registeredOn)
		if err != nil {
			return nil, err
		}
		res.OrgPlatformID, pairingCreated = &pairID, created
	}

	// Attributes land last, because a platform-scoped one needs the pairing that
	// the step above may just have created.
	if err := upsertAttributes(ctx, tx, orgID, res.OrgPlatformID, attrs); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ingest: %w", err)
	}

	res.Status, res.Message = describeIngest(orgCreated, pairingCreated, platform)
	return res, nil
}

// describeIngest words the outcome so an operator reading the webhook log can
// tell a genuinely new customer from a repeat delivery without opening the portal.
func describeIngest(orgCreated, pairingCreated bool, platform string) (string, string) {
	switch {
	case orgCreated && pairingCreated:
		return "created", "registered organisation on " + platform
	case orgCreated:
		return "created", "registered organisation with no platform"
	case pairingCreated:
		return "updated", "added " + platform + " to an existing organisation"
	default:
		return "unchanged", "organisation and platform were already registered"
	}
}

// upsertPerson keys on email — the one field every payload carries, and the
// field the sparse second registration relies on to find its way home.
func upsertPerson(ctx context.Context, tx pgx.Tx, in domain.Registration) (string, error) {
	email := strings.ToLower(strings.TrimSpace(in.RegisteredEmail))
	if email == "" {
		return "", &apierror.ValidationError{Msg: "Email is required"}
	}

	const q = `
		INSERT INTO plg_person (email, first_name, last_name)
		VALUES ($1, NULLIF(BTRIM($2), ''), NULLIF(BTRIM($3), ''))
		ON CONFLICT (email) DO UPDATE
		SET first_name = COALESCE(EXCLUDED.first_name, plg_person.first_name),
		    last_name  = COALESCE(EXCLUDED.last_name,  plg_person.last_name),
		    updated_at = NOW()
		RETURNING id::TEXT`

	var id string
	if err := tx.QueryRow(ctx, q, email, in.FirstName, in.LastName).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert person: %w", err)
	}
	return id, nil
}

// upsertOrganization keys on the Asgardeo org name, which is what the customer
// is in both systems.
//
// registered_user is deliberately absent from DO UPDATE SET: when a colleague
// adds a second platform to an organisation someone else registered, the record
// of who registered it must not move to the newcomer.
func upsertOrganization(ctx context.Context, tx pgx.Tx, in domain.Registration,
	personID string, registeredOn time.Time) (string, bool, error) {

	name := strings.TrimSpace(in.OrganizationName)
	if name == "" {
		return "", false, &apierror.ValidationError{Msg: "Asgardeo_Org_name__c is required"}
	}

	const q = `
		INSERT INTO plg_organization (
		    organization_name, created_on, registered_user,
		    country_name, company_name_from_domain, moesif_company_id)
		VALUES ($1, $2, $3::UUID,
		        NULLIF(BTRIM($4), ''), NULLIF(BTRIM($5), ''), NULLIF(BTRIM($6), ''))
		ON CONFLICT (organization_name) DO UPDATE
		SET country_name             = COALESCE(EXCLUDED.country_name,             plg_organization.country_name),
		    company_name_from_domain = COALESCE(EXCLUDED.company_name_from_domain, plg_organization.company_name_from_domain),
		    moesif_company_id        = COALESCE(EXCLUDED.moesif_company_id,        plg_organization.moesif_company_id),
		    created_on               = LEAST(plg_organization.created_on, EXCLUDED.created_on),
		    updated_at               = NOW()
		RETURNING id::TEXT, (xmax = 0) AS inserted`

	var (
		id       string
		inserted bool
	)
	err := tx.QueryRow(ctx, q, name, registeredOn, personID,
		in.CountryName, in.CompanyNameFromDomain, in.CompanyID).Scan(&id, &inserted)
	if err != nil {
		if isUniqueViolation(err) {
			// The only other unique key here is the company id: two organisation
			// names claiming one company means the source contradicted itself.
			return "", false, &apierror.ValidationError{Msg: "company id " + in.CompanyID + " is already registered under a different organisation name"}
		}
		return "", false, fmt.Errorf("upsert organization: %w", err)
	}
	return id, inserted, nil
}

// upsertOrgPlatform lands the pairing — the unit of work.
//
// A repeat delivery of the same platform must not reset the lifecycle stage or
// clear the acknowledgement, so neither is touched on conflict. Only the two
// payload-owned fields are coalesced forward.
func upsertOrgPlatform(ctx context.Context, tx pgx.Tx, in domain.Registration,
	orgID, platform string, registeredOn time.Time) (string, bool, error) {

	const q = `
		INSERT INTO plg_org_platform (organization_id, product_id, registered_on)
		SELECT $1::UUID, p.id, $3
		FROM   plg_product p
		WHERE  p.code = $2
		ON CONFLICT (organization_id, product_id) DO UPDATE
		SET registered_on = LEAST(plg_org_platform.registered_on, EXCLUDED.registered_on),
		    updated_at    = NOW()
		RETURNING id::TEXT, (xmax = 0) AS inserted`

	var (
		id       string
		inserted bool
	)
	err := tx.QueryRow(ctx, q, orgID, platform, registeredOn).Scan(&id, &inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		// The SELECT matched no product, so the platform name is one we do not
		// carry. Better a 400 naming it than a silent drop.
		return "", false, &apierror.ValidationError{Msg: "unknown platform: " + platform}
	}
	if err != nil {
		return "", false, fmt.Errorf("upsert org platform: %w", err)
	}

	// The pairing's first stage is recorded like any other, so the history reads
	// from REGISTRATION onward rather than starting mid-journey.
	if inserted {
		const h = `
			INSERT INTO plg_lifecycle_history (org_platform_id, from_stage, to_stage, reason, changed_on)
			VALUES ($1::UUID, NULL, 'REGISTRATION', 'Registered via the source feed', $2)`
		if _, err := tx.Exec(ctx, h, id, registeredOn); err != nil {
			return "", false, fmt.Errorf("record initial stage: %w", err)
		}
	}
	return id, inserted, nil
}

// upsertAttributes stores the extra source fields the source map named.
//
// A platform-scoped attribute on a payload that carried no platform is skipped
// rather than demoted to the organisation: it describes something about a
// pairing that does not exist yet, and it will arrive again with the delivery
// that creates one.
//
// Like every other write in this file, a value is only ever replaced by another
// value — an attribute missing from a later payload means "not sent this time",
// never "delete it".
func upsertAttributes(ctx context.Context, tx pgx.Tx, orgID string, orgPlatformID *string,
	attrs []domain.OrganizationAttribute) error {

	const q = `
		INSERT INTO plg_organization_attribute (
		    organization_id, org_platform_id, attribute_name, attribute_value, source_field)
		VALUES ($1::UUID, $2::UUID, $3, $4, $5)
		ON CONFLICT (organization_id, org_platform_id, attribute_name) DO UPDATE
		SET attribute_value = EXCLUDED.attribute_value,
		    source_field    = EXCLUDED.source_field,
		    updated_at      = NOW()`

	for _, attr := range attrs {
		var pairing *string
		if attr.Scope == domain.ScopePlatform {
			if orgPlatformID == nil {
				continue
			}
			pairing = orgPlatformID
		}
		if _, err := tx.Exec(ctx, q, orgID, pairing, attr.Name, attr.Value, attr.SourceField); err != nil {
			return fmt.Errorf("upsert attribute %s: %w", attr.Name, err)
		}
	}
	return nil
}
