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

import "testing"

// TestError_MessageLeavesOutTheBody: the error text is what ends up in the
// logs ("project failed ... err=..."), and an upstream error body can echo
// personal data (e.g. "invalid recipient bob@customer.com"). The message
// carries only the status code; Body stays available to code, never logged.
func TestError_MessageLeavesOutTheBody(t *testing.T) {
	err := &Error{StatusCode: 400, Body: `{"error":"invalid recipient bob@customer.com"}`}

	if got, want := err.Error(), "upstream returned 400"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
