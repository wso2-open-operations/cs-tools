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
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// newGithubDeliveryService is the service behind POST /github/deliveries.
//
// With git internals off (or no database, or the ServiceNow data source: the same
// conditions as its other routes) it returns inner untouched, nil included, so
// the route exists exactly when it did before and runs exactly the code it ran.
// With it on, product issue deliveries are taken first; inner (nil when the
// customer repository sync is off) still gets everything else, so the old path
// runs only when its own gate is on.
func newGithubDeliveryService(cfg *config.Config, db *pgxpool.Pool, inner service.GithubSyncService) service.GithubSyncService {
	if !cfg.GitInternalsActive() || db == nil || cfg.DataSource == config.DataSourceServiceNow {
		return inner
	}
	if len(cfg.GitInternalsBotLogins) == 0 {
		slog.Warn("git internals: GIT_INTERNALS_BOT_LOGINS is empty; comments the portal posts on a product issue will be mirrored back as new comments until it lists the account(s) the engineering entity writes as")
	}
	front := service.NewProductIssueDeliveries(
		repository.NewProductIssueSyncRepository(repository.NewScoped(db)), cfg.GitInternalsBotLogins)
	return service.WithProductIssueDeliveries(inner, front)
}
