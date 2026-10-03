// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package auth checks source webhooks against alerts-core's integration_users table (AUTH_ENABLED).
package auth

import (
	"errors"
	"log/slog"
	"net/http"
)

// ErrUnauthorized is returned by an Authenticator that rejects a request; the router answers 401.
var ErrUnauthorized = errors.New("unauthorized")

// Authenticator decides whether a source webhook may proceed, before the body is read.
type Authenticator interface {
	Authenticate(r *http.Request, source string) error
}

// None accepts every request.
type None struct{}

// Authenticate always succeeds.
func (None) Authenticate(*http.Request, string) error { return nil }

// Audit logs what it would reject but never rejects, so enabling auth can't drop alerts.
type Audit struct {
	inner  Authenticator
	logger *slog.Logger
}

// NewAudit wraps inner so it logs what it would reject without rejecting anything.
func NewAudit(inner Authenticator, logger *slog.Logger) Authenticator {
	return Audit{inner: inner, logger: logger}
}

// Authenticate always returns nil, logging what the wrapped Authenticator would reject.
func (a Audit) Authenticate(r *http.Request, source string) error {
	if err := a.inner.Authenticate(r, source); err != nil {
		a.logger.Warn("auth would reject request",
			"source", source,
			"path", r.URL.Path,
			"reason", err.Error())
	}
	return nil
}
