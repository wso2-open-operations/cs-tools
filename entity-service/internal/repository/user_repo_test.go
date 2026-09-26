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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeUserScanRow feeds scanUser fixed values without a real database
// connection, exercising the exact Scan destination order/types scanUser
// itself uses (userColumns/prefixUserColumns, both ending in
// user_type::TEXT).
type fakeUserScanRow struct {
	id, userName         string
	firstName, lastName  *string
	email, userType      *string
	createdOn, updatedOn time.Time
}

func (f fakeUserScanRow) Scan(dest ...any) error {
	*dest[0].(*string) = f.id
	*dest[1].(*string) = f.userName
	*dest[2].(**string) = f.firstName
	*dest[3].(**string) = f.lastName
	*dest[4].(**string) = f.email
	*dest[5].(**string) = f.userType
	*dest[6].(*time.Time) = f.createdOn
	*dest[7].(*time.Time) = f.updatedOn
	return nil
}

func strPtr(s string) *string { return &s }

func TestScanUser(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		row  fakeUserScanRow
		want domain.User
	}{
		{
			name: "all nullable columns present",
			row: fakeUserScanRow{
				id: "u1", userName: "jdoe",
				firstName: strPtr("Jane"), lastName: strPtr("Doe"),
				email: strPtr("jane@example.com"), userType: strPtr("INTERNAL"),
				createdOn: now, updatedOn: now,
			},
			want: domain.User{
				ID: "u1", UserName: "jdoe",
				FirstName: "Jane", LastName: "Doe", Email: "jane@example.com",
				UserType: domain.UserTypeInternal, CreatedOn: now, UpdatedOn: now,
			},
		},
		{
			name: "first_name/last_name/email/user_type all NULL",
			row:  fakeUserScanRow{id: "u2", userName: "no-profile", createdOn: now, updatedOn: now},
			want: domain.User{ID: "u2", UserName: "no-profile", CreatedOn: now, UpdatedOn: now},
		},
		{
			name: "user_type SYSTEM",
			row:  fakeUserScanRow{id: "u3", userName: "sys", userType: strPtr("SYSTEM"), createdOn: now, updatedOn: now},
			want: domain.User{ID: "u3", UserName: "sys", UserType: domain.UserTypeSystem, CreatedOn: now, UpdatedOn: now},
		},
		{
			name: "user_type EXTERNAL maps to UserTypeCustomer, not UserTypeExternal",
			row:  fakeUserScanRow{id: "u4", userName: "ext", userType: strPtr("EXTERNAL"), createdOn: now, updatedOn: now},
			want: domain.User{ID: "u4", UserName: "ext", UserType: domain.UserTypeCustomer, CreatedOn: now, UpdatedOn: now},
		},
		{
			name: "user_type NOT_AVAILABLE has no domain equivalent, left empty",
			row:  fakeUserScanRow{id: "u5", userName: "none", userType: strPtr("NOT_AVAILABLE"), createdOn: now, updatedOn: now},
			want: domain.User{ID: "u5", UserName: "none", CreatedOn: now, UpdatedOn: now},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scanUser(tt.row)
			if err != nil {
				t.Fatalf("scanUser() error = %v", err)
			}
			// DeepEqual, not ==: domain.User now holds a slice (Roles), which
			// scanUser must leave nil -- only user search fills it in.
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scanUser() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUserOrderBy(t *testing.T) {
	tests := []struct {
		name string
		sort domain.UserSortBy
		want string
	}{
		{"no sort keeps newest-first", domain.UserSortBy{}, "u.created_on DESC, u.id"},
		{"createdOn asc", domain.UserSortBy{Field: domain.UserSortFieldCreatedOn, Order: domain.UserSortOrderAsc}, "u.created_on ASC, u.id"},
		{"updatedOn desc", domain.UserSortBy{Field: domain.UserSortFieldUpdatedOn, Order: domain.UserSortOrderDesc}, "u.updated_on DESC, u.id"},
		{"order omitted defaults to ascending", domain.UserSortBy{Field: domain.UserSortFieldUpdatedOn}, "u.updated_on ASC, u.id"},
	}
	for _, tt := range tests {
		if got := userOrderBy(tt.sort); got != tt.want {
			t.Errorf("%s: userOrderBy = %q, want %q", tt.name, got, tt.want)
		}
	}
	// name sorts on the fallback expression, in the requested direction, with u.id last.
	got := userOrderBy(domain.UserSortBy{Field: domain.UserSortFieldName, Order: domain.UserSortOrderDesc})
	if !strings.HasPrefix(got, "LOWER(COALESCE(") || !strings.HasSuffix(got, " DESC, u.id") {
		t.Errorf("name sort = %q, want the COALESCE fallback ordered DESC then u.id", got)
	}
	// An unknown field must never reach SQL: it falls back to the default.
	if got := userOrderBy(domain.UserSortBy{Field: "email; DROP TABLE x"}); got != "u.created_on DESC, u.id" {
		t.Errorf("unknown field produced %q", got)
	}
}

func TestAssignRoles(t *testing.T) {
	users := []domain.User{{ID: "u1"}, {ID: "u2"}, {ID: "u3"}}
	assignRoles(users, map[string][]string{"u1": {"admin", "internal"}, "u3": {"agent"}})
	if got := strings.Join(users[0].Roles, ","); got != "admin,internal" {
		t.Errorf("u1 roles = %q", got)
	}
	if got := strings.Join(users[2].Roles, ","); got != "agent" {
		t.Errorf("u3 roles = %q", got)
	}
	// A user with no roles must be [] (serializes as []), never nil (null).
	if users[1].Roles == nil || len(users[1].Roles) != 0 {
		t.Errorf("u2 roles = %#v, want empty non-nil", users[1].Roles)
	}
}

func TestDisplayName(t *testing.T) {
	s := func(v string) *string { return &v }
	tests := []struct {
		name           string
		n, first, lst  *string
		userName, want string
	}{
		{"display name wins", s("Jane Q. Doe"), s("Jane"), s("Doe"), "jd", "Jane Q. Doe"},
		{"blank display name falls back to first + last", s("  "), s("Jane"), s("Doe"), "jd", "Jane Doe"},
		{"only a first name", nil, s("Jane"), nil, "jd", "Jane"},
		{"nothing set falls back to the user name", nil, nil, nil, "jane@example.com", "jane@example.com"},
	}
	for _, tt := range tests {
		if got := displayName(tt.n, tt.first, tt.lst, tt.userName); got != tt.want {
			t.Errorf("%s: displayName = %q, want %q", tt.name, got, tt.want)
		}
	}
}
