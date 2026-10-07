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

func TestResolveCaseFeedbackRating(t *testing.T) {
	cases := []struct {
		metricName string
		wantRating int
		wantLabel  string
		wantOK     bool
	}{
		{"Very Dissatisfied - Reasons", 1, "Very Dissatisfied", true},
		{"Dissatisfied - Reasons", 2, "Dissatisfied", true},
		{"Neutral - Reasons", 3, "Neutral", true},
		{"Satisfied - Reasons", 4, "Satisfied", true},
		{"Very Satisfied - Reasons", 5, "Very Satisfied", true},
		{"Very Satisfied", 0, "", false},                       // missing the suffix
		{"Unknown Label - Reasons", 0, "Unknown Label", false}, // not one of the five known labels
		{"", 0, "", false},
	}
	for _, c := range cases {
		rating, label, ok := resolveCaseFeedbackRating(c.metricName)
		if rating != c.wantRating || label != c.wantLabel || ok != c.wantOK {
			t.Errorf("resolveCaseFeedbackRating(%q) = (%d, %q, %v), want (%d, %q, %v)",
				c.metricName, rating, label, ok, c.wantRating, c.wantLabel, c.wantOK)
		}
	}
}
