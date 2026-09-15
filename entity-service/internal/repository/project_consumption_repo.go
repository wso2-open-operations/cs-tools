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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/crypto"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProjectConsumptionRepository defines the persistence operations for the
// project_consumption table.
type ProjectConsumptionRepository interface {
	// Get returns the project's provisioning state along with the project's own
	// name and key, which the caller needs to name the Choreo application.
	//
	// A project that has never started the flow has no row, and Get returns a
	// synthesised ConsumptionStatusPending state rather than a not-found error —
	// "not provisioned yet" is step 1 of the state machine, not an absent
	// resource. A project ID that does not exist at all is still not found.
	Get(ctx context.Context, projectID string) (state domain.ProjectConsumption, name, key string, err error)

	// Upsert advances the project's provisioning state, creating the row on
	// first use.
	//
	// Only the non-nil fields of next are written; a nil field leaves whatever
	// is already stored untouched, so recording step 4's credentials cannot
	// erase step 2's application ID. The status guard is applied in SQL rather
	// than read-then-write: the update only lands if the stored status is still
	// below the incoming one, and Upsert reports staleness rather than silently
	// rewinding a state machine that another caller has already advanced.
	Upsert(ctx context.Context, projectID string, next domain.ProjectConsumption) (domain.ProjectConsumption, error)
}

// ErrConsumptionStatusStale reports that the stored provisioning status is
// already at or beyond the status a caller tried to write. It is not an error
// condition in the usual sense — the service turns it into a no-op read — but
// it must be distinguishable from a successful write.
var ErrConsumptionStatusStale = errors.New("project consumption: stored status is not older than the requested status")

type projectConsumptionRepo struct {
	db    *pgxpool.Pool
	codec crypto.SecretCodec
}

// NewProjectConsumptionRepository constructs a ProjectConsumptionRepository.
// codec must not be nil — the credential columns are never written in the
// clear.
func NewProjectConsumptionRepository(db *pgxpool.Pool, codec crypto.SecretCodec) ProjectConsumptionRepository {
	return &projectConsumptionRepo{db: db, codec: codec}
}

// Get implements ProjectConsumptionRepository.
func (r *projectConsumptionRepo) Get(ctx context.Context, projectID string) (domain.ProjectConsumption, string, string, error) {
	const query = `
		SELECT p.name,
		       p.key,
		       pc.project_id,
		       pc.status,
		       pc.choreo_application_id,
		       pc.consumer_key,
		       pc.consumer_secret,
		       pc.primary_secret_key,
		       pc.secondary_secret_key,
		       pc.created_at,
		       pc.updated_at
		FROM projects p
		LEFT JOIN project_consumption pc ON pc.project_id = p.id
		WHERE p.id = $1`

	var (
		name, key      string
		storedID       *string
		status         *int16
		appID          *string
		consumerKey    *string
		consumerSecret []byte
		primaryKey     []byte
		secondaryKey   []byte
		createdOn      *time.Time
		updatedOn      *time.Time
	)

	err := r.db.QueryRow(ctx, query, projectID).Scan(
		&name, &key, &storedID, &status, &appID, &consumerKey,
		&consumerSecret, &primaryKey, &secondaryKey, &createdOn, &updatedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectConsumption{}, "", "", &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.ProjectConsumption{}, "", "", fmt.Errorf("get project consumption: %w", err)
	}

	state := domain.ProjectConsumption{
		ProjectID: projectID,
		Status:    domain.ConsumptionStatusPending,
	}
	// storedID is NULL when the LEFT JOIN found no consumption row — the
	// project exists but has never entered the flow.
	if storedID == nil {
		return state, name, key, nil
	}

	state.Status = domain.ConsumptionStatus(*status)
	state.ChoreoApplicationID = appID
	state.ConsumerKey = consumerKey
	if createdOn != nil {
		state.CreatedOn = *createdOn
	}
	if updatedOn != nil {
		state.UpdatedOn = *updatedOn
	}

	for _, f := range []struct {
		name   string
		cipher []byte
		dst    **string
	}{
		{"consumerSecret", consumerSecret, &state.ConsumerSecret},
		{"primarySecretKey", primaryKey, &state.PrimarySecretKey},
		{"secondarySecretKey", secondaryKey, &state.SecondarySecretKey},
	} {
		if len(f.cipher) == 0 {
			continue
		}
		plain, decErr := r.codec.Decrypt(f.cipher)
		if decErr != nil {
			// The field name is safe to log; the value is not, and Decrypt
			// never includes it.
			return domain.ProjectConsumption{}, "", "", fmt.Errorf("get project consumption: decrypt %s: %w", f.name, decErr)
		}
		*f.dst = &plain
	}

	return state, name, key, nil
}

// Upsert implements ProjectConsumptionRepository.
func (r *projectConsumptionRepo) Upsert(ctx context.Context, projectID string, next domain.ProjectConsumption) (domain.ProjectConsumption, error) {
	consumerSecret, err := r.encryptOptional(next.ConsumerSecret)
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: encrypt consumerSecret: %w", err)
	}
	primaryKey, err := r.encryptOptional(next.PrimarySecretKey)
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: encrypt primarySecretKey: %w", err)
	}
	secondaryKey, err := r.encryptOptional(next.SecondarySecretKey)
	if err != nil {
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: encrypt secondarySecretKey: %w", err)
	}

	// The COALESCEs are in the SELECT that builds the proposed row, not in the
	// DO UPDATE SET, and that placement is load-bearing.
	//
	// PostgreSQL evaluates CHECK constraints against the *proposed* insert
	// tuple, before ON CONFLICT resolves anything. Merging in the DO UPDATE SET
	// is therefore too late: advancing a project to status 3 while supplying
	// only the status proposes a row whose choreo_application_id is NULL, and
	// chk_project_consumption_application_id rejects it — even though the
	// update that would have followed preserves the stored value. Merging here
	// means the row that reaches the constraints is already the final one.
	//
	// It also makes EXCLUDED carry the merged values, so DO UPDATE SET is a
	// plain assignment rather than a second copy of the same COALESCE list.
	//
	// The WHERE clause is the concurrency guard. Two license downloads racing
	// for the same project both read status 1 and both try to write 2; the
	// second one's update is filtered out here and it learns it lost, instead
	// of overwriting the winner's application ID with its own.
	const query = `
		INSERT INTO project_consumption (
			project_id, status, choreo_application_id, consumer_key,
			consumer_secret, primary_secret_key, secondary_secret_key
		)
		SELECT next.project_id,
		       next.status,
		       COALESCE(next.choreo_application_id, stored.choreo_application_id),
		       COALESCE(next.consumer_key, stored.consumer_key),
		       COALESCE(next.consumer_secret, stored.consumer_secret),
		       COALESCE(next.primary_secret_key, stored.primary_secret_key),
		       COALESCE(next.secondary_secret_key, stored.secondary_secret_key)
		FROM (
			SELECT $1::TEXT     AS project_id,
			       $2::SMALLINT AS status,
			       $3::TEXT     AS choreo_application_id,
			       $4::TEXT     AS consumer_key,
			       $5::BYTEA    AS consumer_secret,
			       $6::BYTEA    AS primary_secret_key,
			       $7::BYTEA    AS secondary_secret_key
		) AS next
		LEFT JOIN project_consumption AS stored ON stored.project_id = next.project_id
		ON CONFLICT (project_id) DO UPDATE SET
			status                = EXCLUDED.status,
			choreo_application_id = EXCLUDED.choreo_application_id,
			consumer_key          = EXCLUDED.consumer_key,
			consumer_secret       = EXCLUDED.consumer_secret,
			primary_secret_key    = EXCLUDED.primary_secret_key,
			secondary_secret_key  = EXCLUDED.secondary_secret_key,
			updated_at            = NOW()
		WHERE project_consumption.status < EXCLUDED.status
		RETURNING project_id, status, choreo_application_id, consumer_key, created_at, updated_at`

	var (
		out    domain.ProjectConsumption
		status int16
	)
	err = r.db.QueryRow(ctx, query,
		projectID, int16(next.Status), next.ChoreoApplicationID, next.ConsumerKey,
		consumerSecret, primaryKey, secondaryKey,
	).Scan(&out.ProjectID, &status, &out.ChoreoApplicationID, &out.ConsumerKey, &out.CreatedOn, &out.UpdatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		// The WHERE clause filtered the update out — the stored status is
		// already at or past this one.
		return domain.ProjectConsumption{}, ErrConsumptionStatusStale
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation — the project does not exist
				return domain.ProjectConsumption{}, &apierror.NotFoundError{Msg: "project not found"}
			case "23514": // check_violation — a step's artefacts are missing for its status
				return domain.ProjectConsumption{}, &apierror.ValidationError{
					Msg: "provisioning state is incomplete for the requested status",
				}
			}
		}
		return domain.ProjectConsumption{}, fmt.Errorf("upsert project consumption: %w", err)
	}

	out.Status = domain.ConsumptionStatus(status)
	return out, nil
}

// encryptOptional seals v, passing nil through unchanged so that "field not
// supplied" stays distinguishable from "field set to empty" all the way down
// to the COALESCE in Upsert.
func (r *projectConsumptionRepo) encryptOptional(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return r.codec.Encrypt(*v)
}
