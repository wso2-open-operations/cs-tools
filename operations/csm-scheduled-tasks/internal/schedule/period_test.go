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

func TestMinInterval(t *testing.T) {
	// A Friday, so the weekday case has to look past the weekend.
	from := time.Date(2026, 10, 2, 10, 2, 0, 0, time.UTC)
	cases := []struct {
		expr string
		want time.Duration
	}{
		{"*/5 * * * *", 5 * time.Minute},
		{"*/15 * * * *", 15 * time.Minute},
		{"0 3 * * *", 24 * time.Hour},
		{"0 9 * * 1-5", 24 * time.Hour},
		{"*/30 * * * * *", 30 * time.Second},
	}
	for _, tc := range cases {
		got, err := MinInterval(tc.expr, from)
		if err != nil {
			t.Errorf("%q: unexpected error %v", tc.expr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %s, want %s", tc.expr, got, tc.want)
		}
	}
	if _, err := MinInterval("not a cron", from); err == nil {
		t.Error("an invalid expression must return an error")
	}
}

func TestPeriodKeyIsStableWithinAPeriod(t *testing.T) {
	a, err := PeriodKey("0 3 * * *", time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := PeriodKey("0 3 * * *", time.Date(2026, 10, 2, 23, 59, 0, 0, time.UTC))
	if !a.Equal(b) || !a.Equal(time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("both ticks should map to 03:00 that day, got %s and %s", a, b)
	}
}
