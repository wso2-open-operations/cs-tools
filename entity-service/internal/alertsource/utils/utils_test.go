// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"encoding/json"
	"testing"
)

func TestStr(t *testing.T) {
	m := map[string]any{"s": "  x ", "n": json.Number("7010000000000000123"), "b": true, "null": nil}
	for key, want := range map[string]string{"s": "x", "n": "7010000000000000123", "b": "true", "null": "", "missing": ""} {
		if got := Str(m, key); got != want {
			t.Errorf("Str(%q) = %q, want %q", key, got, want)
		}
	}
	if got := Str(nil, "s"); got != "" {
		t.Errorf("Str(nil) = %q, want empty", got)
	}
}
