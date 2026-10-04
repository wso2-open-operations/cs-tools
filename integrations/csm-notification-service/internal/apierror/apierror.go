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
// codes to appropriate HTTP responses — mirroring the Ballerina getStatusCode pattern.
package apierror

import (
	"errors"
	"fmt"
	"strings"
)

// Error is returned when an upstream service responds with a non-2xx status.
type Error struct {
	StatusCode int
	Body       string
}

func (e *Error) Error() string {
	if e.Body == "" {
		return e.summary()
	}
	return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, e.Body)
}

// summary is the body-free form of Error(): status code only.
func (e *Error) summary() string {
	return fmt.Sprintf("upstream returned %d", e.StatusCode)
}

// Summary returns err's message with any upstream response excerpt removed.
// An *Error anywhere in err's chain carries up to a few hundred bytes of the
// upstream response body, which can echo recipient addresses or message
// content this service must not log; Summary keeps the surrounding context
// ("dispatch: send invitation for membership X: upstream returned 500") and
// drops only the excerpt. For an error with no *Error in its chain it is
// plain err.Error(). Nil yields "".
func Summary(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Body == "" {
		return msg
	}
	msg = strings.Replace(msg, apiErr.Error(), apiErr.summary(), 1)
	if strings.Contains(msg, apiErr.Body) {
		// A wrapper that reworded the inner message rather than embedding
		// it verbatim: the replacement above missed, so fall back to the
		// status alone rather than risk the excerpt getting through.
		return apiErr.summary()
	}
	return msg
}
