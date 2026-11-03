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

package main

import "testing"

func TestLoadGenerateRateLimits(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	got, err := loadGenerateRateLimits(env(nil))
	if err != nil || got != (generateRateLimits{PerMinute: 10, PerDay: 100}) {
		t.Errorf("unset: %+v, %v; want the defaults 10 / 100", got, err)
	}
	got, err = loadGenerateRateLimits(env(map[string]string{
		"DASHBOARD_GENERATE_LIMIT_PER_MINUTE": " 5 ", "DASHBOARD_GENERATE_LIMIT_PER_DAY": "40",
	}))
	if err != nil || got != (generateRateLimits{PerMinute: 5, PerDay: 40}) {
		t.Errorf("set: %+v, %v; want 5 / 40", got, err)
	}
	for name, m := range map[string]map[string]string{
		"zero":         {"DASHBOARD_GENERATE_LIMIT_PER_MINUTE": "0"},
		"negative":     {"DASHBOARD_GENERATE_LIMIT_PER_DAY": "-1"},
		"not a number": {"DASHBOARD_GENERATE_LIMIT_PER_MINUTE": "ten"},
		"day < minute": {"DASHBOARD_GENERATE_LIMIT_PER_MINUTE": "20", "DASHBOARD_GENERATE_LIMIT_PER_DAY": "10"},
	} {
		if _, err := loadGenerateRateLimits(env(m)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
