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

package dispatch

import (
	"testing"
	"time"
)

func TestIncidentEventTime(t *testing.T) {
	want := time.Date(2026, 11, 3, 7, 19, 53, 0, time.UTC)
	if got := incidentEventTime("2026-11-03T07:19:53Z"); !got.Equal(want) {
		t.Errorf("incidentEventTime(RFC3339) = %v, want %v", got, want)
	}
	// A missing or unreadable time falls back to now; registration still
	// takes the clock's real start from entity-service.
	for _, s := range []string{"", "03/11/2026 07:19", "2026-11-03 07:19:53"} {
		before := time.Now()
		got := incidentEventTime(s)
		if got.Before(before) || got.After(time.Now()) {
			t.Errorf("incidentEventTime(%q) = %v, want the current time", s, got)
		}
	}
}
