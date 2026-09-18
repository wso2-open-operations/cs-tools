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
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/crypto"
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
}

// ErrConsumptionStatusStale reports that the stored provisioning status is
// already at or beyond the status a caller tried to write.
var ErrConsumptionStatusStale = errors.New("project consumption: stored status is not older than the requested status")

type projectConsumptionRepo struct {
	db    *pgxpool.Pool
	codec crypto.SecretCodec
}

// NewProjectConsumptionRepository constructs a ProjectConsumptionRepository.
func NewProjectConsumptionRepository(db *pgxpool.Pool, codec crypto.SecretCodec) ProjectConsumptionRepository {
	return &projectConsumptionRepo{db: db, codec: codec}
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
		       p.client_id,
		       p.client_secret,
		       p.primary_secret_key,
		       p.secondary_secret_key,
		       p.created_on,
		       p.updated_on
		FROM project p
		WHERE p.id = $1`

	var (
		name, key       *string
		id              string
		appStatus       *string
		appID           *string
		clientID        *string
		clientSecretEnc *string
		primaryEnc      *string
		secondaryEnc    *string
		createdOn       time.Time
		updatedOn       time.Time
	)

	err := r.db.QueryRow(ctx, query, projectID).Scan(
		&name, &key, &id, &appStatus, &appID, &clientID,
		&clientSecretEnc, &primaryEnc, &secondaryEnc, &createdOn, &updatedOn,
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

	for _, f := range []struct {
		name   string
		stored *string
		dst    **string
	}{
		{"consumerSecret", clientSecretEnc, &state.ConsumerSecret},
		{"primarySecretKey", primaryEnc, &state.PrimarySecretKey},
		{"secondarySecretKey", secondaryEnc, &state.SecondarySecretKey},
	} {
		plain, err := r.decryptStored(f.stored)
		if err != nil {
			// The field name is safe to log; the value is not, and neither
			// the decode nor the decrypt error includes it.
			return domain.ProjectConsumption{}, "", "", fmt.Errorf("get project consumption: %s: %w", f.name, err)
		}
		*f.dst = plain
	}

	return state, projectName, projectKey, nil
}

// decryptStored unseals one base64-encoded ciphertext column, returning nil for
// a column that is NULL or empty.
//
// A value that is present but undecodable is an error, not a nil: these columns
// are also written by the ServiceNow sync, so silently reporting "no secret"
// for a value in an unexpected format would hide exactly the drift worth
// knowing about.
func (r *projectConsumptionRepo) decryptStored(stored *string) (*string, error) {
	if stored == nil || *stored == "" {
		return nil, nil
	}
	cipher, err := base64.StdEncoding.DecodeString(*stored)
	if err != nil {
		return nil, fmt.Errorf("stored value is not base64: %w", err)
	}
	if len(cipher) == 0 {
		return nil, nil
	}
	plain, err := r.codec.Decrypt(cipher)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return &plain, nil
}

// encryptOptional seals v and base64-encodes it for a TEXT column, passing nil
// through unchanged so that "field not supplied" stays distinguishable from
// "field set to empty" all the way down to the COALESCE in Upsert.
func (r *projectConsumptionRepo) encryptOptional(v *string) (*string, error) {
	if v == nil {
		return nil, nil
	}
	cipher, err := r.codec.Encrypt(*v)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(cipher)
	return &encoded, nil
}

// Upsert implements ProjectConsumptionRepository.
func (r *projectConsumptionRepo) Upsert(ctx context.Context, projectID string, next domain.ProjectConsumption) (domain.ProjectConsumption, error) {
	encryptedSecret, err := r.encryptOptional(next.ConsumerSecret)
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: encrypt consumerSecret: %w", err)
	}
	encryptedPrimary, err := r.encryptOptional(next.PrimarySecretKey)
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: encrypt primarySecretKey: %w", err)
	}
	encryptedSecondary, err := r.encryptOptional(next.SecondarySecretKey)
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: encrypt secondarySecretKey: %w", err)
	}

	statusEnum := statusToEnum(next.Status)

	// Every artefact column is COALESCEd against its stored value, so a step
	// that carries only its own output cannot erase an earlier step's: writing
	// the secret keys at step 5 must leave step 2's application id and step 4's
	// credentials exactly as they are.
	const updateQuery = `
		UPDATE project
		SET choreo_application_status = $2::choreo_application_status_enum,
		    choreo_application_id = COALESCE($3, choreo_application_id),
		    client_id = COALESCE($4, client_id),
		    client_secret = COALESCE($5, client_secret),
		    primary_secret_key = COALESCE($6, primary_secret_key),
		    secondary_secret_key = COALESCE($7, secondary_secret_key),
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
		RETURNING id, choreo_application_status, choreo_application_id, client_id, created_on, updated_on`

	var (
		outID     string
		retStatus *string
		appID     *string
		clientID  *string
		createdOn time.Time
		updatedOn time.Time
	)

	err = r.db.QueryRow(ctx, updateQuery,
		projectID, statusEnum, next.ChoreoApplicationID, next.ConsumerKey,
		encryptedSecret, encryptedPrimary, encryptedSecondary,
	).Scan(&outID, &retStatus, &appID, &clientID, &createdOn, &updatedOn)

	if errors.Is(err, pgx.ErrNoRows) {
		var existsID string
		checkErr := r.db.QueryRow(ctx, `SELECT id FROM project WHERE id = $1`, projectID).Scan(&existsID)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return domain.ProjectConsumption{}, &apierror.NotFoundError{Msg: "project not found"}
		}
		return domain.ProjectConsumption{}, ErrConsumptionStatusStale
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
