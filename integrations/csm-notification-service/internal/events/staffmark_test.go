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

package events

import (
	"encoding/json"
	"testing"
)

func TestStripStaffMark(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"raw lowercase", `{"name":"Jane Doe \u24e6"}`, `{"name":"Jane Doe"}`},
		{"raw capital", "{\"name\":\"Jane Doe \u24CC\"}", `{"name":"Jane Doe"}`},
		{"raw UTF-8 lowercase", "{\"name\":\"Jane Doe \u24E6\"}", `{"name":"Jane Doe"}`},
		{"no space before it", "{\"name\":\"Jane Doe\u24E6\"}", `{"name":"Jane Doe"}`},
		{"escaped, no space", `{"name":"Doe\u24e6"}`, `{"name":"Doe"}`},
		{"every string in the payload", "{\"a\":\"A \u24E6\",\"b\":\"B \u24E6\"}", `{"a":"A","b":"B"}`},
		{"not at the end of a string", "{\"name\":\"Jane \u24E6 Doe\"}", "{\"name\":\"Jane \u24E6 Doe\"}"},
		{"no mark", `{"name":"Jane Doe"}`, `{"name":"Jane Doe"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StripStaffMark([]byte(c.in))
			if string(got) != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
			if !json.Valid(got) {
				t.Errorf("result is not valid JSON: %q", got)
			}
		})
	}
}
