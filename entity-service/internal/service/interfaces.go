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

// Package service contains business logic that sits between the HTTP handlers
// and the repository layer.
package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// UserService defines the operations available on the user entity.
// Handlers depend on this interface rather than the concrete implementation,
// making it straightforward to substitute a test double in unit tests.
type UserService interface {
	// SearchUsers returns a paginated list of users that match the filters in
	// req. A ValidationError is returned for invalid input (e.g. limit > 100);
	// any other error indicates an infrastructure failure.
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchUsersResponse, error)
}
