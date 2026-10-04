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

package apierror

import (
	"strings"
	"testing"
)

// TestErrorOmitsBody: the error string ends up in logs, so it carries the
// status only; the body stays available on the field for response mapping.
func TestErrorOmitsBody(t *testing.T) {
	e := &Error{StatusCode: 400, Body: `{"message":"jane.doe@example.com is invalid"}`}
	if got := e.Error(); got != "upstream returned 400" {
		t.Fatalf("Error() = %q", got)
	}
	if strings.Contains(e.Error(), "jane.doe") {
		t.Fatal("body leaked into Error()")
	}
}
