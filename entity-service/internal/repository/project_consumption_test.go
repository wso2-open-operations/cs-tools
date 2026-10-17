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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"testing"
)

func TestConsumptionStatusEnumMapping(t *testing.T) {
	cases := []struct {
		status  int
		want    *string
	}{
		{status: 0, want: nil},
		{status: 1, want: stringPtr("PENDING")},
		{status: 2, want: stringPtr("CREATED_APPLICATION")},
		{status: 3, want: stringPtr("SUBSCRIBED_APPLICATION")},
		{status: 4, want: stringPtr("GENERATED_CREDENTIALS")},
		{status: 5, want: stringPtr("COMPLETED")},
		{status: 6, want: stringPtr("COMPLETED")},
		{status: -1, want: nil},
	}

	for _, tc := range cases {
		got := consumptionStatusToEnum(tc.status)
		if tc.want == nil {
			if got != nil {
				t.Errorf("consumptionStatusToEnum(%d) = %v, want nil", tc.status, *got)
			}
		} else {
			if got == nil || *got != *tc.want {
				t.Errorf("consumptionStatusToEnum(%d) = %v, want %q", tc.status, got, *tc.want)
			}
		}
	}
}

func TestConsumptionEnumToStatus(t *testing.T) {
	cases := []struct {
		enumVal      *string
		hasSecretKey bool
		wantStatus   int
	}{
		{enumVal: nil, hasSecretKey: false, wantStatus: 1},
		{enumVal: stringPtr("PENDING"), hasSecretKey: false, wantStatus: 1},
		{enumVal: stringPtr("CREATED_APPLICATION"), hasSecretKey: false, wantStatus: 2},
		{enumVal: stringPtr("SUBSCRIBED_APPLICATION"), hasSecretKey: false, wantStatus: 3},
		{enumVal: stringPtr("GENERATED_CREDENTIALS"), hasSecretKey: false, wantStatus: 4},
		{enumVal: stringPtr("GENERATED_CREDENTIALS"), hasSecretKey: true, wantStatus: 5},
		{enumVal: stringPtr("COMPLETED"), hasSecretKey: false, wantStatus: 5},
		{enumVal: stringPtr("COMPLETED"), hasSecretKey: true, wantStatus: 5},
		{enumVal: stringPtr("UNKNOWN"), hasSecretKey: false, wantStatus: 1},
	}

	for _, tc := range cases {
		got := consumptionEnumToStatus(tc.enumVal, tc.hasSecretKey)
		if got != tc.wantStatus {
			t.Errorf("consumptionEnumToStatus(%v, %v) = %d, want %d", tc.enumVal, tc.hasSecretKey, got, tc.wantStatus)
		}
	}
}

func stringPtr(s string) *string {
	return &s
}
