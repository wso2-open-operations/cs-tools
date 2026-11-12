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

package schedule

import (
	"testing"
	"time"
)

func TestLongestGap(t *testing.T) {
	from := time.Date(2026, 11, 12, 10, 7, 0, 0, time.UTC)
	for expr, want := range map[string]time.Duration{
		"*/15 * * * *":  15 * time.Minute,
		"20 * * * *":    time.Hour,
		"0 6 * * *":     24 * time.Hour,
		"0 9,17 * * *":  16 * time.Hour,
		"0 6 * * 1":     7 * 24 * time.Hour,
		"0 */6 * * 1-5": 6*time.Hour + 2*24*time.Hour, // Friday 18:00 to Monday 00:00
	} {
		got, err := LongestGap(expr, from)
		if err != nil || got != want {
			t.Errorf("%s: %v, %v; want %v", expr, got, err, want)
		}
	}
	if _, err := LongestGap("not a cron", from); err == nil {
		t.Error("an invalid expression should fail")
	}
}
