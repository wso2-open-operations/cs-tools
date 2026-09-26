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

// Package server assembles the HTTP server, router, and middleware chain.
package server

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

const (
	serverReadTimeout  = 15 * time.Second
	serverWriteTimeout = 15 * time.Second
	serverIdleTimeout  = 60 * time.Second
)

// New creates an http.Server listening on addr with production-safe timeouts
// and the full middleware/router chain wired up via NewRouter. Also returns
// NewRouter's shutdown function, which closes every Kafka producer it built
// (the shared-topic publisher and the onboarding-topic one), so
// cmd/api/main.go can release them gracefully on shutdown. Never nil.
func New(addr string, db *pgxpool.Pool, cfg *config.Config) (*http.Server, func()) {
	handler, closePublishers := NewRouter(db, cfg)
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}, closePublishers
}
