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

package salesentity

import (
	"errors"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// ErrNotFound marks a by-id lookup that the upstream answered successfully
// with no matching record -- the record does not exist (or no longer does).
// Test with errors.Is. It is deliberately NOT set for an HTTP 404 from a
// search endpoint, which can equally mean a misrouted request: only a
// successful, empty answer counts as "gone".
var ErrNotFound = errors.New("salesentity: record not found")

// notFoundError is a ServiceUnavailableError (so every existing caller and
// the HTTP error mapping see exactly what they saw before) that also
// matches ErrNotFound.
type notFoundError struct {
	unavailable *apierror.ServiceUnavailableError
}

// NotFound returns an error carrying msg that unwraps to a
// ServiceUnavailableError and matches ErrNotFound.
func NotFound(msg string) error {
	return &notFoundError{unavailable: &apierror.ServiceUnavailableError{Msg: msg}}
}

func (e *notFoundError) Error() string { return e.unavailable.Error() }

func (e *notFoundError) Unwrap() error { return e.unavailable }

func (e *notFoundError) Is(target error) bool { return target == ErrNotFound }
