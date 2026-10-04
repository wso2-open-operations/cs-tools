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

// Package apierror defines a typed error returned by upstream service clients
// when a non-2xx response is received, allowing handlers to map upstream status
// codes to appropriate HTTP responses.
package apierror

import "fmt"

// Error is returned when an upstream service responds with a non-2xx status.
type Error struct {
	StatusCode int
	Body       string
	// RetryAfter is the upstream's Retry-After header value, if it sent one
	// (typically with a 429 or 503). Handlers pass it through to the caller
	// after validating its shape; it is never interpreted here.
	RetryAfter string
}

// Error reports the upstream status only. The response body is deliberately
// left out: it can carry upstream data that must not reach logs or callers,
// and any code that needs it reads the Body field explicitly (as the
// handler's client-safety filter does).
func (e *Error) Error() string {
	return fmt.Sprintf("upstream returned %d", e.StatusCode)
}
