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
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProjectConsumptionRepository defines the persistence operations for a
// project's provisioning state.
type ProjectConsumptionRepository interface {
	// Get returns the project's provisioning state along with the project's own
	// name and key, which the caller needs to name the Choreo application.
	//
	// A project that has never started the flow returns ConsumptionStatusPending
	// rather than a not-found error. A project ID that does not exist at all
	// returns NotFoundError.
	Get(ctx context.Context, projectID string) (state domain.ProjectConsumption, name, key string, err error)

	// Upsert advances the project's provisioning state.
	//
	// Only non-nil fields of next are updated; nil fields preserve stored values.
	// The status guard is applied in SQL: status may only move forward.
	// ErrConsumptionStatusStale is returned if stored status is already at or
	// beyond the requested status.
	Upsert(ctx context.Context, projectID string, next domain.ProjectConsumption) (domain.ProjectConsumption, error)

	// GetSigningContext returns the credentials and metadata needed to sign a deployment
	// licence for projectID and deploymentID.
	GetSigningContext(ctx context.Context, projectID, deploymentID string) (*domain.SigningContext, error)
}

// ErrConsumptionStatusStale reports that the stored provisioning status is
// already at or beyond the status a caller tried to write.
var ErrConsumptionStatusStale = errors.New("project consumption: stored status is not older than the requested status")

// projectConsumptionRepo stores the provisioning artefacts as they are given.
//
// These columns are shared with the ServiceNow sync and the project row is a
// mirror of the ServiceNow record, where ProductConsumptionUtils.updateProject
// assigns each of them straight from its payload -- so the record this mirrors
// holds them in the clear, and matching that is what "mirror" means here.
//
// Encrypting only this service's writes would put two indistinguishable
// formats in one column (a stored value cannot be classified after the fact,
// since a hex key is also valid base64) and would make a row written here stop
// matching the record it mirrors. It would also have to be undone before the
// licence path could move here: ServiceNow signs a licence by reading these
// four values back in the clear, and refuses to sign unless all of them are
// present.
//
// So they are stored as supplied. Encrypting them at rest is worth doing
// across every writer, which is a platform change rather than this service's
// to make.
type projectConsumptionRepo struct {
	db *pgxpool.Pool
}

// NewProjectConsumptionRepository constructs a ProjectConsumptionRepository.
func NewProjectConsumptionRepository(db *pgxpool.Pool) ProjectConsumptionRepository {
	return &projectConsumptionRepo{db: db}
}

func statusToEnum(status domain.ConsumptionStatus) string {
	switch status {
	case domain.ConsumptionStatusCreated:
		return "CREATED_APPLICATION"
	case domain.ConsumptionStatusSubscribed:
		return "SUBSCRIBED_APPLICATION"
	case domain.ConsumptionStatusGeneratedCredentials:
		return "GENERATED_CREDENTIALS"
	case domain.ConsumptionStatusGeneratedSecretKeys:
		return "COMPLETED"
	default:
		return "PENDING"
	}
}

func enumToStatus(enumVal *string) domain.ConsumptionStatus {
	if enumVal == nil {
		return domain.ConsumptionStatusPending
	}
	switch *enumVal {
	case "CREATED_APPLICATION":
		return domain.ConsumptionStatusCreated
	case "SUBSCRIBED_APPLICATION":
		return domain.ConsumptionStatusSubscribed
	case "GENERATED_CREDENTIALS":
		return domain.ConsumptionStatusGeneratedCredentials
	case "COMPLETED":
		return domain.ConsumptionStatusGeneratedSecretKeys
	default:
		return domain.ConsumptionStatusPending
	}
}

// Get implements ProjectConsumptionRepository.
func (r *projectConsumptionRepo) Get(ctx context.Context, projectID string) (domain.ProjectConsumption, string, string, error) {
	const query = `
		SELECT p.name,
		       p.key,
		       p.id,
		       p.choreo_application_status,
		       p.choreo_application_id,
		       p.product_consumption_client_id,
		       p.product_consumption_client_secret,
		       p.product_consumption_primary_secret_key,
		       p.product_consumption_secondary_secret_key,
		       p.created_on,
		       p.updated_on
		FROM project p
		WHERE p.id = $1`

	var (
		name, key    *string
		id           string
		appStatus    *string
		appID        *string
		clientID     *string
		clientSecret *string
		primary      *string
		secondary    *string
		createdOn    time.Time
		updatedOn    time.Time
	)

	err := r.db.QueryRow(ctx, query, projectID).Scan(
		&name, &key, &id, &appStatus, &appID, &clientID,
		&clientSecret, &primary, &secondary, &createdOn, &updatedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectConsumption{}, "", "", &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.ProjectConsumption{}, "", "", fmt.Errorf("get project consumption: %w", err)
	}

	projectName := ""
	if name != nil {
		projectName = *name
	}
	projectKey := ""
	if key != nil {
		projectKey = *key
	}

	state := domain.ProjectConsumption{
		ProjectID:           projectID,
		Status:              enumToStatus(appStatus),
		ChoreoApplicationID: appID,
		ConsumerKey:         clientID,
		CreatedOn:           createdOn,
		UpdatedOn:           updatedOn,
	}

	state.ConsumerSecret = presentOrNil(clientSecret)
	state.PrimarySecretKey = presentOrNil(primary)
	state.SecondarySecretKey = presentOrNil(secondary)

	return state, projectName, projectKey, nil
}

// presentOrNil collapses a NULL or empty column to nil, so that the caller's
// "is this artefact set" check cannot be satisfied by a blank string.
//
// The value itself is never returned to a client -- the read response carries
// only whether each secret is present -- so it is passed through untouched.
func presentOrNil(stored *string) *string {
	if stored == nil || *stored == "" {
		return nil
	}
	return stored
}

// Upsert implements ProjectConsumptionRepository.
func (r *projectConsumptionRepo) Upsert(ctx context.Context, projectID string, next domain.ProjectConsumption) (domain.ProjectConsumption, error) {
	statusEnum := statusToEnum(next.Status)

	// Every artefact column is COALESCEd against its stored value, so a step
	// that carries only its own output cannot erase an earlier step's: writing
	// the secret keys at step 5 must leave step 2's application id and step 4's
	// credentials exactly as they are.
	const updateQuery = `
		UPDATE project
		SET choreo_application_status = $2::choreo_application_status_enum,
		    choreo_application_id = COALESCE($3, choreo_application_id),
		    product_consumption_client_id = COALESCE($4, product_consumption_client_id),
		    product_consumption_client_secret = COALESCE($5, product_consumption_client_secret),
		    product_consumption_primary_secret_key = COALESCE($6, product_consumption_primary_secret_key),
		    product_consumption_secondary_secret_key = COALESCE($7, product_consumption_secondary_secret_key),
		    consumption_tracking_file_generated_on = CASE WHEN $2 = 'COMPLETED' THEN NOW() ELSE consumption_tracking_file_generated_on END,
		    updated_on = NOW()
		WHERE id = $1
		  AND (
		      choreo_application_status IS NULL
		      OR choreo_application_status = 'PENDING'
		      OR ($2 IN ('SUBSCRIBED_APPLICATION', 'GENERATED_CREDENTIALS', 'COMPLETED') AND choreo_application_status = 'CREATED_APPLICATION')
		      OR ($2 IN ('GENERATED_CREDENTIALS', 'COMPLETED') AND choreo_application_status = 'SUBSCRIBED_APPLICATION')
		      OR ($2 = 'COMPLETED' AND choreo_application_status = 'GENERATED_CREDENTIALS')
		  )
		RETURNING id, choreo_application_status, choreo_application_id, product_consumption_client_id, created_on, updated_on`

	var (
		outID     string
		retStatus *string
		appID     *string
		clientID  *string
		createdOn time.Time
		updatedOn time.Time
	)

	err := r.db.QueryRow(ctx, updateQuery,
		projectID, statusEnum, next.ChoreoApplicationID, next.ConsumerKey,
		next.ConsumerSecret, next.PrimarySecretKey, next.SecondarySecretKey,
	).Scan(&outID, &retStatus, &appID, &clientID, &createdOn, &updatedOn)

	if errors.Is(err, pgx.ErrNoRows) {
		// Zero rows is ambiguous: either the project does not exist, or the
		// forward-only guard refused the transition. Distinguish them.
		var existsID string
		checkErr := r.db.QueryRow(ctx, `SELECT id FROM project WHERE id = $1`, projectID).Scan(&existsID)
		switch {
		case errors.Is(checkErr, pgx.ErrNoRows):
			return domain.ProjectConsumption{}, &apierror.NotFoundError{Msg: "project not found"}
		case checkErr != nil:
			// Anything else -- a dropped connection, a timeout, a cancelled
			// context -- says nothing about the status, and must not be
			// reported as staleness: UpdateProjectConsumption treats
			// ErrConsumptionStatusStale as success and answers "already at or
			// beyond the requested status", so a transient database failure
			// would surface to the caller as a successful no-op.
			return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: check project exists: %w", checkErr)
		default:
			return domain.ProjectConsumption{}, ErrConsumptionStatusStale
		}
	}
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: %w", err)
	}

	return domain.ProjectConsumption{
		ProjectID:           outID,
		Status:              enumToStatus(retStatus),
		ChoreoApplicationID: appID,
		ConsumerKey:         clientID,
		CreatedOn:           createdOn,
		UpdatedOn:           updatedOn,
	}, nil
}

// GetSigningContext returns the credentials and metadata needed to sign a deployment
// licence for projectID and deploymentID.
//
// Both ids are bound as uuid rather than compared against a ::text cast of the
// column, so the primary-key indexes are usable; casting the column instead
// made every call scan project, deployment and deployed_product in full. The
// consequence is that a malformed id is now a driver error rather than simply
// matching nothing, which is why callers validate first -- ProcessLicenseDownload
// runs validateUUIDs on both before reaching this.
func (r *projectConsumptionRepo) GetSigningContext(ctx context.Context, projectID, deploymentID string) (*domain.SigningContext, error) {
	const query = `
		SELECT
			COALESCE(p.product_consumption_client_id, ''),
			COALESCE(p.product_consumption_client_secret, ''),
			COALESCE(p.product_consumption_primary_secret_key, ''),
			COALESCE(p.product_consumption_secondary_secret_key, ''),
			COALESCE(p.product_consumption_license_secrets::text, ''),
			p.key,
			COALESCE(d.name, dp.name, '') AS deployment_name,
			COALESCE(d.number, dp.number, '') AS deployment_number,
			(d.id IS NOT NULL OR dp.id IS NOT NULL) AS deployment_matched
		FROM project p
		LEFT JOIN deployment d ON d.id = $2::uuid AND d.project_id = p.id
		LEFT JOIN deployed_product dp ON dp.id = $2::uuid AND dp.project_id = p.id
		WHERE p.id = $1::uuid`

	var (
		clientID          string
		clientSecret      string
		primaryKey        string
		secondaryKey      string
		licenseSecrets    string
		subscriptionKey   string
		deploymentName    string
		deploymentNumber  string
		deploymentMatched bool
	)

	err := r.db.QueryRow(ctx, query, projectID, deploymentID).Scan(
		&clientID,
		&clientSecret,
		&primaryKey,
		&secondaryKey,
		&licenseSecrets,
		&subscriptionKey,
		&deploymentName,
		&deploymentNumber,
		&deploymentMatched,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("get signing context: %w", err)
	}

	// Trim any enclosing quotes if license_secrets was stored in a JSONB column as a JSON string literal.
	licenseSecrets = strings.Trim(licenseSecrets, "\"")

	// Refuse rather than return a partial context. Both joins are LEFT joins
	// scoped to this project, so a deployment that does not exist -- or that
	// belongs to a DIFFERENT project -- simply produces no match, and the
	// COALESCEs above would turn that into empty strings.
	//
	// That matters because deploymentName and deploymentNumber are part of the
	// signed payload. Handing the signer a blank for either does not fail: it
	// produces a valid signature over the wrong payload, which the customer's
	// product then rejects, with nothing anywhere reporting a problem.
	// ServiceNow refuses this case outright ("Deployment does not belong to
	// project"), and so does this.
	if !deploymentMatched {
		return nil, &apierror.NotFoundError{Msg: "deployment not found for this project"}
	}

	// Same reasoning for the credentials. ServiceNow will not sign unless every
	// one of them is present, so an incomplete set is never signable -- and a
	// project can genuinely be in this state, e.g. one marked complete whose
	// step-5 write never landed. Reported as an error so the caller falls back
	// to the direct download instead of signing with blanks.
	for _, f := range []struct {
		name  string
		value string
	}{
		{"clientId", clientID},
		{"clientSecret", clientSecret},
		{"primarySecretKey", primaryKey},
		{"deploymentName", deploymentName},
		{"deploymentNumber", deploymentNumber},
	} {
		if f.value == "" {
			// The field name is safe to report; no value is included.
			return nil, fmt.Errorf("get signing context: %s is not set for this project", f.name)
		}
	}

	return &domain.SigningContext{
		ClientID:           clientID,
		ClientSecret:       clientSecret,
		PrimarySecretKey:   primaryKey,
		SecondarySecretKey: secondaryKey,
		LicenseSecrets:     licenseSecrets,
		DeploymentName:     deploymentName,
		DeploymentNumber:   deploymentNumber,
		SubscriptionKey:    subscriptionKey,
	}, nil
}
