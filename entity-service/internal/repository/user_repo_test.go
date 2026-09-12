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

package repository

import (
	"context"
	"testing"
)

// TestUserRepo_GetUsersByIDs_EmptyIDsSkipsQuery proves the documented
// (nil, nil) short-circuit for an empty ids slice, without needing a real
// Postgres connection — this package has no DB-backed test harness, so
// GetUsersByIDs' actual `WHERE id = ANY($1::uuid[])` query path is
// exercised only indirectly, through internal/service's
// TestCaseService_CreateCaseComment_PublishesCaseMentioned (via
// stubUserRepo standing in for this repository).
func TestUserRepo_GetUsersByIDs_EmptyIDsSkipsQuery(t *testing.T) {
	r := NewUserRepository(nil)
	users, err := r.GetUsersByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if users != nil {
		t.Errorf("users = %v, want nil", users)
	}
}
