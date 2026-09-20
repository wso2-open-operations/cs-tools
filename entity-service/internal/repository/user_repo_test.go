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
			if got != tt.want {
				t.Errorf("scanUser() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
