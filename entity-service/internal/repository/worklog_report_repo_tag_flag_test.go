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
	"strings"
	"testing"
)

// Which allocation tags take somebody off the timesheet is the tag's own
// excludes_from_timesheet (migration 0281), so a tag a lead adds can do it
// too. A list of codes here would silently ignore every such tag.
func TestWorklogLeaveQueryReadsTheTagsTimesheetFlag(t *testing.T) {
	if !strings.Contains(worklogLeaveQuery, "k.excludes_from_timesheet") {
		t.Fatal("worklogLeaveQuery does not read excludes_from_timesheet")
	}
	for _, code := range []string{"'ENG'", "'ALLO_EXT'", "'ALLO_INT'"} {
		if strings.Contains(worklogLeaveQuery, code) {
			t.Fatalf("worklogLeaveQuery names %s by code; read the tag's flag instead", code)
		}
	}
}
