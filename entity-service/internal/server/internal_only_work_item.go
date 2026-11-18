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

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// newWorkItemTypeGuard builds the lookup the guard runs; a variable so a test
// can stand in for the database.
var newWorkItemTypeGuard = func(db *pgxpool.Pool) internalOnlyWorkItemRejecter {
	return repository.NewWorkItemTypeGuard(repository.NewScoped(db))
}

// newCaseIDGuard returns guardCaseID: refuseInternalOnlyWorkItem when the
// git-internals feature is active and there is a database (the same conditions
// as registering its routes), else the identity wrapper, so with the flag off
// the wrapped routes are exactly the routes they were.
func newCaseIDGuard(cfg *config.Config, db *pgxpool.Pool) func(http.HandlerFunc) http.HandlerFunc {
	if !cfg.GitInternalsActive() || db == nil || cfg.DataSource == config.DataSourceServiceNow {
		return func(h http.HandlerFunc) http.HandlerFunc { return h }
	}
	g := newWorkItemTypeGuard(db)
	return func(h http.HandlerFunc) http.HandlerFunc { return refuseInternalOnlyWorkItem(g, h) }
}

// internalOnlyWorkItemRejecter answers a NotFoundError for an id that names a
// work item of an internal-only type (repository.WorkItemTypeGuard).
type internalOnlyWorkItemRejecter interface {
	RejectInternalOnly(ctx context.Context, id string) error
}

// refuseInternalOnlyWorkItem wraps a /cases/{id}/... handler so an id that names
// an internal-only work item (a product issue) gets the route's normal 404
// before the handler runs. Those routes' repository methods key on work_item.id
// with no type check (the Phase 0 audit, row A4), so without this a product
// issue's id would be accepted as a case's. Any other id passes straight
// through, so no existing type is affected.
//
// It FAILS OPEN: if the lookup itself errors (database trouble) the request goes
// on to the route, whose own behaviour then decides, and a WARN naming the work
// item id is logged. A guard that answered 500 would take comments and tags down
// with the database check it adds; the route's own checks fail on their own.
func refuseInternalOnlyWorkItem(g internalOnlyWorkItemRejecter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := g.RejectInternalOnly(r.Context(), r.PathValue("id")); err != nil {
			var nfe *apierror.NotFoundError
			if errors.As(err, &nfe) {
				apierror.WriteJSON(w, http.StatusNotFound, nfe.Msg)
				return
			}
			// Ids only: a database error can embed hosts and addresses.
			slog.WarnContext(r.Context(), "internal-only work item guard: the type lookup failed; passing the request through",
				"workItemId", r.PathValue("id"), "method", r.Method)
			next(w, r)
			return
		}
		next(w, r)
	}
}
