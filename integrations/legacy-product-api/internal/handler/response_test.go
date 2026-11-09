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

package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/legacy-product-api/internal/apierror"
)

func TestSummarizeErrNeverLogsTheUpstreamBody(t *testing.T) {
	got := summarizeErr(&apierror.Error{StatusCode: 502, Body: `{"secret":"x"}`})
	if got != "upstream status 502" {
		t.Errorf("summarizeErr = %q", got)
	}
}

func TestSummarizeErrBoundsOtherErrors(t *testing.T) {
	if got := summarizeErr(errors.New(strings.Repeat("x", 500))); len(got) != 200 {
		t.Errorf("len = %d, want 200", len(got))
	}
}
