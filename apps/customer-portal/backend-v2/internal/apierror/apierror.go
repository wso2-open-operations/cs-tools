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

// Package apierror defines the error type used to carry upstream (entity-service)
// HTTP failures back through the client layer to the handler layer.
package apierror

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Error wraps a non-2xx response from an upstream service call.
type Error struct {
	StatusCode int
	Body       string
	// Code is the upstream's machine-readable name for the refusal (its error
	// body's "errorCode", see entity-service's apierror/codes.go), kept only when
	// it is a plain lower-case snake_case name; empty when the upstream sent none
	// (an older entity-service, or a refusal that has no name). It is what a
	// client may branch on, where Body is wording for people.
	Code string
}

func (e *Error) Error() string {
	return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, e.Body)
}

// upstreamErrorBody is the {"message": "..."} shape every upstream service
// this backend calls uses for its own error responses (entity-service's is a
// superset, {"code":...,"message":"..."}, which unmarshals the same way).
type upstreamErrorBody struct {
	Message string `json:"message"`
	// ErrorCode is read as any so a value of the wrong type (not a string) is
	// merely not a code, instead of failing the whole body and losing Message.
	ErrorCode any `json:"errorCode"`
}

// errorCodeRe is the shape of a machine-readable error code: lower-case words
// joined by underscores, at most 64 characters. A value of any other shape is
// not passed on, so nothing but a plain name can reach a client through it.
var errorCodeRe = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// ValidErrorCode reports whether code is a well-formed machine-readable error
// code (see Error.Code).
func ValidErrorCode(code string) bool {
	return len(code) <= 64 && errorCodeRe.MatchString(code)
}

// NewUpstreamError builds an *Error from a non-2xx upstream HTTP response.
// Body is set to the upstream's own "message" field when the response is the
// expected {"message": "..."} shape, and left empty otherwise; Code is set to its
// "errorCode" when that is a well-formed code. Every upstream
// client in this backend must construct its errors through this function
// rather than falling back to a raw response excerpt: callers already treat
// an empty Body as "no specific message available" (both mapUpstreamError's
// 400 case and writeUpstreamMessage fall back to a fixed message), so a raw
// excerpt is never necessary — and logging or returning one to the frontend
// risks leaking unbounded, non-message upstream content (e.g. a gateway HTML
// error page).
func NewUpstreamError(statusCode int, rawBody []byte) *Error {
	var body upstreamErrorBody
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return &Error{StatusCode: statusCode}
	}
	e := &Error{StatusCode: statusCode, Body: body.Message}
	if code, ok := body.ErrorCode.(string); ok && ValidErrorCode(code) {
		e.Code = code
	}
	return e
}

// DataError is a failure where the upstream call SUCCEEDED but returned
// something this backend cannot act on — a record missing a field the next
// step needs, a status outside the known set.
//
// Distinct from Error, which carries an upstream HTTP status. These have no
// status to carry: nothing failed at the transport or protocol level. Without
// a type of their own they are plain errors, and summarizeErr logs every plain
// error as "upstream request failed" — which is precisely wrong here and sends
// whoever is debugging to look for an outage that never happened.
//
// It deliberately does NOT satisfy the *Error mapping, so the client still
// receives the handler's generic message rather than upstream internals. The
// gain is in the log, which is where diagnosis actually happens.
type DataError struct {
	Reason string
}

func (e *DataError) Error() string {
	return "upstream data unusable: " + e.Reason
}

// NewDataError builds a DataError with a formatted reason.
func NewDataError(format string, args ...any) *DataError {
	return &DataError{Reason: fmt.Sprintf(format, args...)}
}
