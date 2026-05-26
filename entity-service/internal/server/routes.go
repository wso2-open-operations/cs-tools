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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// NewRouter builds the dependency graph (repository → service → handler),
// registers all routes, and wraps the mux with the middleware chain:
// Recovery → Logger → Timeout.
func NewRouter(db *pgxpool.Pool) http.Handler {
	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)
	userHandler := handler.NewUserHandler(svc)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.HealthCheck)
	mux.HandleFunc("POST /users/search", userHandler.SearchUsers)

	return middleware.Recovery(
		middleware.Logger(
			middleware.Timeout(10 * time.Second)(mux),
		),
	)
}
