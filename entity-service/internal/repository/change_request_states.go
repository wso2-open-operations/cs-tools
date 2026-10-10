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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// ChangeRequestStates is what a write that may move a change request reports
// about change_request.state: the state the transaction FOUND, read under the
// row lock before anything was written, and the state it COMMITTED, read again
// under that same lock after every write of the transaction and before the
// commit. Both are the column's own upper-case label ("ASSESS"), nil for NULL
// (a row migrated from the previous system can carry none).
//
// The pair exists for the dual-write mirror (service.changeRequestService):
// the previous system is kept in step with every state move PostgreSQL commits,
// whatever caused it -- a staff PATCH, the approval cascade, the customer's
// own answer, Accept proposed time -- and the only sound way to know a move
// happened is to compare the two inside the transaction that made it. A read
// before the call is racy (another transaction can move the change in between)
// and the detail read after the commit runs in another transaction and can
// already see a later move.
type ChangeRequestStates struct {
	Prior     *string
	Committed *string
}

// Moved reports whether the transaction changed the state (NULL and "" are
// the same absence).
func (s ChangeRequestStates) Moved() bool {
	return stringOrEmpty(s.Prior) != stringOrEmpty(s.Committed)
}

// lockChangeRequestStateForPatch reads change_request.state under the locks a
// PATCH takes, in the order every PATCH takes them: the work_item row first
// (FOR NO KEY UPDATE, the strength the PATCH's own UPDATE of work_item takes --
// see lockChangeRequestForPatch for why not FOR UPDATE), the change_request row
// second (FOR UPDATE). Taking them here, as the first statements after the
// visibility check, means every later lock of the same transaction is a
// re-lock of a row it already holds, so nothing about the PATCH's own locking
// changes; and no other transaction can move the state between this read and
// the PATCH's writes. Under row-level security a caller who may not update the
// row locks none, which is reported as not found like everywhere else.
//
// crvis: called by PatchChangeRequestStates after its requireVisibleChangeRequest
func lockChangeRequestStateForPatch(ctx context.Context, tx pgx.Tx, id string) (*string, error) {
	var locked string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM work_item WHERE id = $1::uuid AND type = 'CHANGE_REQUEST' FOR NO KEY UPDATE`, id).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return nil, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("patch change request: lock work item: %w", err)
	}
	return lockChangeRequestState(ctx, tx, id)
}

// lockChangeRequestState reads change_request.state FOR UPDATE -- the lock an
// approval decision takes as its first statement (decideChangeRequestApprovalTx)
// and a PATCH takes after its work_item lock. Not found when the change_request
// row does not exist for this caller.
//
// crvis: callers run requireVisibleChangeRequest first (PatchChangeRequestStates, DecideChangeRequestApprovalStates)
func lockChangeRequestState(ctx context.Context, tx pgx.Tx, id string) (*string, error) {
	var state *string
	err := tx.QueryRow(ctx, `SELECT state::text FROM change_request WHERE id = $1::uuid FOR UPDATE`, id).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return nil, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("lock change request state: %w", err)
	}
	return state, nil
}

// readCommittedChangeRequestState reads change_request.state after the
// transaction's writes, while it still holds the row lock the two lock helpers
// above took, so what it returns is exactly what the commit will publish.
//
// crvis: runs inside a transaction that passed requireVisibleChangeRequest and holds the row lock
func readCommittedChangeRequestState(ctx context.Context, tx pgx.Tx, id string) (*string, error) {
	var state *string
	err := tx.QueryRow(ctx, `SELECT state::text FROM change_request WHERE id = $1::uuid`, id).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return nil, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("read committed change request state: %w", err)
	}
	return state, nil
}
