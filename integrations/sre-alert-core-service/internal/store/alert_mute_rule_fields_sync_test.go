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

package store

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// alertMuteRuleContractFields reads the alert fields a mute rule condition can name from the contract, the
// AlertMuteRuleCondition.field enum in entity-service/openapi.yaml (path relative to this test's directory).
func alertMuteRuleContractFields(t *testing.T, openapiPath string) []string {
	t.Helper()
	src, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("read %s: %v", openapiPath, err)
	}
	m := regexp.MustCompile(`(?s)\n    AlertMuteRuleCondition:\n.*?field:\n.*?enum: \[([^\]]*)\]`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("AlertMuteRuleCondition.field enum not found in %s", openapiPath)
	}
	var out []string
	for _, f := range strings.Split(string(m[1]), ",") {
		out = append(out, strings.TrimSpace(f))
	}
	sort.Strings(out)
	return out
}


// TestFieldValueColumnsMatchTheContract: every field a condition can name (entity-service's contract) has a
// column to search, and no other.
func TestFieldValueColumnsMatchTheContract(t *testing.T) {
	want := alertMuteRuleContractFields(t, "../../../../entity-service/openapi.yaml")
	var got []string
	for f := range fieldValueColumns {
		got = append(got, f)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("fieldValueColumns = %v, entity-service AlertMuteRuleCondition.field = %v", got, want)
	}
}
