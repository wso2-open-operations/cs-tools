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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// InternalOnlyWorkItemTypes are the work item types customers never see and the
// id-addressed case routes must refuse (404): today PRODUCT_ISSUE.
//
// It is a DENYLIST, deliberately not the case-like allowlist: the case comment
// and tag routes also serve other existing types (a change request's tags, a
// conversation's or incident's comments), and nothing that works today may
// stop working.
//
// KEEP THIS IN STEP WITH work_item_type_is_internal_only() in migration
// 100029_product_issue_rls.sql, the SQL list the row-level security policy uses
// (a later internal-only type redefines that function in a new migration and is
// added here too). TestInternalOnlyWorkItemTypesMatchTheSQLFunction fails when
// the two disagree.
var InternalOnlyWorkItemTypes = []string{"PRODUCT_ISSUE"}

// internalOnlyWorkItemTypesSQL is InternalOnlyWorkItemTypes as a SQL array
// literal, for the statements that exclude them (like caseLikeWorkItemTypes).
var internalOnlyWorkItemTypesSQL = "'{" + strings.Join(InternalOnlyWorkItemTypes, ",") + "}'::work_item_type_enum[]"

// rejectInternalOnlyWorkItem answers a 404 notFound for an id that names a work
// item of an internal-only type, whoever the caller is. Any other work item, or
// an id that exists nowhere (or is not a UUID), passes untouched and is left to
// the operation's own checks and to row-level security. Call it next to
// rejectHiddenChangeRequest in an operation that takes a "case" id but runs
// against any work item the id names.
func rejectInternalOnlyWorkItem(ctx context.Context, q crQuerier, id, notFound string) error {
	if !validate.IsUUID(id) {
		return nil
	}
	var internalOnly bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM work_item wi WHERE wi.id = $1::uuid AND wi.type = ANY(`+internalOnlyWorkItemTypesSQL+`))`,
		id).Scan(&internalOnly)
	if err != nil {
		return fmt.Errorf("check work item type: %w", err)
	}
	if internalOnly {
		return &apierror.NotFoundError{Msg: notFound}
	}
	return nil
}

// WorkItemTypeGuard is rejectInternalOnlyWorkItem for the HTTP layer: one
// check in front of every id-addressed /cases/{id}/... route, instead of one in
// each repository method.
type WorkItemTypeGuard struct {
	db *Scoped
}

// NewWorkItemTypeGuard returns a guard bound to db.
func NewWorkItemTypeGuard(db *Scoped) *WorkItemTypeGuard {
	return &WorkItemTypeGuard{db: db}
}

// RejectInternalOnly returns a NotFoundError when id names an internal-only
// work item. The type is read under the system identity so the answer does not
// depend on what row-level security shows the caller (a customer's and an
// internal caller's answer are the same 404).
func (g *WorkItemTypeGuard) RejectInternalOnly(ctx context.Context, id string) error {
	return rejectInternalOnlyWorkItem(WithSystemIdentity(ctx), g.db, id, "case not found")
}
