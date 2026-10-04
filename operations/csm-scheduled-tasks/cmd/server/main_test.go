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

import (
	"strings"
	"testing"
	"time"
)

func TestParseSubCronSchedulesAndScheduleFor(t *testing.T) {
	o := parseSubCronSchedules(`{"a":"*/10 * * * *","b":""}`)
	if got := scheduleFor(o, "a", "0 3 * * *"); got != "*/10 * * * *" {
		t.Errorf("override not applied: %q", got)
	}
	if got := scheduleFor(o, "b", "0 3 * * *"); got != "0 3 * * *" {
		t.Errorf("an empty override must keep the default: %q", got)
	}
	if got := scheduleFor(o, "c", "0 3 * * *"); got != "0 3 * * *" {
		t.Errorf("an unmentioned task must keep the default: %q", got)
	}
	if parseSubCronSchedules(`{bad`) != nil || parseSubCronSchedules("") != nil {
		t.Error("malformed or empty input must yield no overrides")
	}
}

func TestParseSubCronRecipients(t *testing.T) {
	o := parseSubCronRecipients(`{"a":{"to":["x@example.com"],"cc":["y@example.com"]}}`)
	to, cc := recipientsFor(o, "a")
	if strings.Join(to, ",") != "x@example.com" || strings.Join(cc, ",") != "y@example.com" {
		t.Errorf("got to=%v cc=%v", to, cc)
	}
	if to, cc := recipientsFor(o, "missing"); to != nil || cc != nil {
		t.Error("an unmentioned task must get no recipients")
	}
	if parseSubCronRecipients(`[1,2]`) != nil {
		t.Error("malformed input must yield no recipients")
	}
}

func TestEnvDays(t *testing.T) {
	cases := map[string]time.Duration{
		"":                     30 * 24 * time.Hour,
		"7":                    7 * 24 * time.Hour,
		"0":                    30 * 24 * time.Hour,
		"-1":                   30 * 24 * time.Hour,
		"abc":                  30 * 24 * time.Hour,
		"99999999999999999999": 30 * 24 * time.Hour,
		"106751":               time.Duration(maxRetentionDays) * 24 * time.Hour,
		"106752":               30 * 24 * time.Hour, // would overflow time.Duration
	}
	for v, want := range cases {
		t.Setenv("TEST_DAYS", v)
		if got := envDays("TEST_DAYS", 30); got != want {
			t.Errorf("%q: got %s, want %s", v, got, want)
		}
	}
}

func TestEnvDurationBoolInt(t *testing.T) {
	t.Setenv("TEST_D", "5m")
	if envDuration("TEST_D", time.Hour) != 5*time.Minute {
		t.Error("valid duration not parsed")
	}
	t.Setenv("TEST_D", "-5m")
	if envDuration("TEST_D", time.Hour) != time.Hour {
		t.Error("non-positive duration must fall back")
	}
	t.Setenv("TEST_B", "nope")
	if !envBool("TEST_B", true) {
		t.Error("malformed bool must fall back")
	}
	t.Setenv("TEST_I", "0")
	if envInt("TEST_I", 2) != 2 {
		t.Error("an int below 1 must fall back")
	}
	t.Setenv("TEST_I", "3")
	if envInt("TEST_I", 2) != 3 {
		t.Error("valid int not parsed")
	}
}

func TestSplitComma(t *testing.T) {
	if got := splitComma(" a@example.com, ,b@example.com "); strings.Join(got, "|") != "a@example.com|b@example.com" {
		t.Errorf("got %v", got)
	}
	if splitComma("") != nil {
		t.Error("empty input must be nil")
	}
}
