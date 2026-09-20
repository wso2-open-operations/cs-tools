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

package repository

import "testing"

func TestFormatDurationSeconds(t *testing.T) {
	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"zero", 0, "0 Minutes"},
		{"single minute", 60, "1 Minute"},
		{"under an hour rounds to minutes", 45*60 + 15, "45 Minutes"},
		{"hours and minutes", 2*3600 + 15*60, "2 Hours 15 Minutes"},
		{"exact hour, no minutes shown", 3600, "1 Hour"},
		{"days hours minutes", 9*86400 + 22*3600 + 11*60 + 6, "9 Days 22 Hours 11 Minutes"},
		{"real sample: 238:11:06", 238*3600 + 11*60 + 6, "9 Days 22 Hours 11 Minutes"},
		{"negative clamps to zero", -30, "0 Minutes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDurationSeconds(tt.seconds); got != tt.want {
				t.Errorf("formatDurationSeconds(%v) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestFormatDurationSecondsPtr_Nil(t *testing.T) {
	if got := formatDurationSecondsPtr(nil); got != nil {
		t.Errorf("formatDurationSecondsPtr(nil) = %v, want nil", got)
	}
}
