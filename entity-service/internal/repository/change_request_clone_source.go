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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// The change request a change request was cloned from
// (change_request.cloned_from_change_request_id, migration 0285): written once,
// by the create's own INSERT (after checkCloneSource, in the same transaction),
// from POST /change-requests clonedFromChangeRequestId, and never changed
// afterwards. Clone lineage is a fact of this database only:
// the previous system has no field for it (the dual-write mirror leaves it out,
// the legacy data source refuses it).

const cloneSourceNotFoundMsg = "clonedFromChangeRequestId does not refer to an existing change request"

// NormalizeCloneSource trims the requested clone source and refuses one that is
// not a UUID (400). nil stays nil; blank is none.
func NormalizeCloneSource(id *string) (*string, error) {
	if id == nil {
		return nil, nil
	}
	v := strings.ToLower(strings.TrimSpace(*id))
	if v == "" {
		return nil, nil
	}
	if !validate.IsUUID(v) {
		return nil, &apierror.ValidationError{Msg: "clonedFromChangeRequestId must be a UUID"}
	}
	return &v, nil
}

// checkCloneSource normalises the clone source and refuses one that is not an
// existing change request (400: a work item of another type is no source
// either). Read under the caller's own identity, so a source the caller may not
// see is refused like a missing one. Returns the normalised id, nil for none.
func checkCloneSource(ctx context.Context, q crQuerier, sourceID *string) (*string, error) {
	src, err := NormalizeCloneSource(sourceID)
	if err != nil || src == nil {
		return nil, err
	}
	var exists bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM change_request cr JOIN work_item wi ON wi.id = cr.id
		                 WHERE cr.id = $1::uuid AND wi.type = 'CHANGE_REQUEST')`, *src).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check clone source: %w", err)
	}
	if !exists {
		return nil, &apierror.ValidationError{Msg: cloneSourceNotFoundMsg}
	}
	return src, nil
}

// readChangeRequestClonedFrom is the detail's clonedFrom: the source's id and
// number, nil when the change request records none (or its source was deleted).
func readChangeRequestClonedFrom(ctx context.Context, q crQuerier, id string) (*domain.EntityRef, error) {
	var ref domain.EntityRef
	err := q.QueryRow(ctx, `
		SELECT src.id::text, COALESCE(src.number, '')
		  FROM change_request cr
		  JOIN work_item src ON src.id = cr.cloned_from_change_request_id
		 WHERE cr.id = $1`, id).Scan(&ref.ID, &ref.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ref, nil
}
