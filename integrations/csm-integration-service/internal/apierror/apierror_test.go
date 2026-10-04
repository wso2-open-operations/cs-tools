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

func TestErrorOmitsBody(t *testing.T) {
	e := &Error{StatusCode: 409, Body: `{"message":"secret upstream detail"}`, RetryAfter: "5"}
	got := e.Error()
	if got != "upstream returned 409" {
		t.Errorf("Error() = %q, want %q", got, "upstream returned 409")
	}
	if strings.Contains(got, "secret") {
		t.Errorf("Error() leaks the body: %q", got)
	}
}
