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
)

// This file owns the "Deviation from the implementation plan"
// (change_request.implementation_deviation, migration 0245): what the
// implementation or the review actually did differently from the approved
// implementation plan. The plan itself is under the approved-content lock
// (change_request_approved_lock.go); the deviation is deliberately not one of
// its fields, because it is written exactly when the plan can no longer
// change. It stays editable until the change request reaches a final state
// (closed, canceled, rollback), after which the record is final: a change is
// refused and resending the stored value is the accepted no-op.

// normalizeImplementationDeviation is the value a request's deviation is
// stored as: nil (NULL) for a blank one, the value as sent otherwise.
func normalizeImplementationDeviation(v *string) *string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return v
}

// deviationFinalMsg is the 400 for a PATCH that changes the deviation of a
// change request in a final state.
func deviationFinalMsg(state string) string {
	word := lockStateName(state)
	if state == crStateRollback {
		word = "rolled back"
	}
	return fmt.Sprintf("implementationDeviation can no longer be changed: the change request is %s, so its record is final", word)
}

// checkDeviationEditable refuses a change of the deviation once the stored
// state is final. deviation is the request's value (nil when the request does
// not carry the field, which is never judged). The comparison is with the
// stored column, so a resend is accepted; a blank value means NULL.
func checkDeviationEditable(ctx context.Context, q crQuerier, id string, snap changeRequestGateSnapshot, deviation *string) error {
	if deviation == nil || !terminalChangeRequestState(snap.state) {
		return nil
	}
	var changed bool
	err := q.QueryRow(ctx,
		`SELECT cr.implementation_deviation IS DISTINCT FROM $2::text FROM change_request cr WHERE cr.id = $1`,
		id, normalizeImplementationDeviation(deviation)).Scan(&changed)
	if errors.Is(err, pgx.ErrNoRows) {
		return &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return fmt.Errorf("patch change request: compare deviation: %w", err)
	}
	if !changed {
		return nil
	}
	return &apierror.ValidationError{Msg: deviationFinalMsg(snap.state), Code: apierror.CodeChangeRequestDeviationFinal}
}
