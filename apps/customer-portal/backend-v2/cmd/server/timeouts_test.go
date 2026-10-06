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
	"testing"
	"time"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadTimeouts_Defaults(t *testing.T) {
	got, err := loadTimeouts(envMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := timeouts{read: 60 * time.Second, write: 60 * time.Second, entity: 60 * time.Second}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadTimeouts_Override(t *testing.T) {
	got, err := loadTimeouts(envMap(map[string]string{
		envReadTimeout:   "2m",
		envWriteTimeout:  "90s",
		envEntityTimeout: "1m15s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := timeouts{read: 2 * time.Minute, write: 90 * time.Second, entity: 75 * time.Second}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadTimeouts_Invalid(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"read not a duration":  {envReadTimeout: "abc"},
		"read bare number":     {envReadTimeout: "60"},
		"write not a duration": {envWriteTimeout: "1 minute"},
		"entity garbage":       {envEntityTimeout: "x"},
		"read zero":            {envReadTimeout: "0s"},
		"read negative":        {envReadTimeout: "-5s"},
		"write zero":           {envWriteTimeout: "0"},
		"write negative":       {envWriteTimeout: "-1m"},
		"entity zero":          {envEntityTimeout: "0s"},
		"entity negative":      {envEntityTimeout: "-1s"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadTimeouts(envMap(env)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
