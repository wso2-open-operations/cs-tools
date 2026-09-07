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

package github

import (
	"fmt"
	"strings"
	"time"
)

// errKind classifies an APIError so callers can both apply a status-aware
// retry policy (AUDIT-FINDINGS A5) and produce a message safe to persist or
// return to API clients (AUDIT-FINDINGS A2) without a status-code check.
type errKind int

const (
	errKindHTTPStatus errKind = iota // non-2xx HTTP response
	errKindGraphQL                   // 200 OK but a GraphQL-level errors[] array
	errKindTransport                 // request build/send/decode failure
)

// APIError is returned by gql/doGQL for every GitHub GraphQL failure. Error()
// carries full detail (including up to 500 bytes of raw response body) for
// server-side logging; Public() never does — it is the only form of this
// error that may be persisted to sync_runs or returned by an API response.
type APIError struct {
	Kind       errKind
	StatusCode int           // set only when Kind == errKindHTTPStatus
	RetryAfter time.Duration // GitHub's Retry-After, when present (403/429)
	detail     string        // raw message; logged only, never public
	wrapped    error         // original error (network/decode), nil for HTTP-status/GraphQL kinds
}

func (e *APIError) Error() string {
	if e.detail == "" {
		return e.Public()
	}
	return e.detail
}

// Unwrap exposes the original network/decode error so errors.Is/errors.As
// (e.g. a caller checking for context.DeadlineExceeded) can still see
// through an APIError instead of the chain ending here.
func (e *APIError) Unwrap() error {
	return e.wrapped
}

// Public returns a short, stable classification with no response-body or
// GraphQL-message detail — safe for sync_runs.error and any API response.
func (e *APIError) Public() string {
	switch e.Kind {
	case errKindHTTPStatus:
		return fmt.Sprintf("github http %d", e.StatusCode)
	case errKindGraphQL:
		return "github graphql error"
	default:
		return "github request failed"
	}
}

// NewHTTPStatusError builds an APIError for a non-2xx GraphQL HTTP response.
// Exported so tests outside this package (e.g. internal/sync's sanitization
// tests) can construct a realistic error without a network round trip.
func NewHTTPStatusError(status int, retryAfter time.Duration, body string) *APIError {
	return &APIError{
		Kind:       errKindHTTPStatus,
		StatusCode: status,
		RetryAfter: retryAfter,
		detail:     fmt.Sprintf("github graphql http %d: %s", status, body),
	}
}

// NewGraphQLError builds an APIError for a 200 OK response carrying a
// GraphQL-level errors[] array.
func NewGraphQLError(messages []string) *APIError {
	return &APIError{
		Kind:   errKindGraphQL,
		detail: fmt.Sprintf("github graphql error: %s", strings.Join(messages, "; ")),
	}
}

// NewTransportError builds an APIError for a request build/send/decode
// failure (no HTTP status to classify against). wrapped is the original
// error (nil when there isn't one, e.g. an empty-data response) — kept
// reachable via Unwrap so errors.Is/errors.As still see through this
// wrapper to things like context.DeadlineExceeded.
func NewTransportError(detail string, wrapped error) *APIError {
	return &APIError{Kind: errKindTransport, detail: detail, wrapped: wrapped}
}
