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
	"reflect"
	"testing"
)

func TestParseClientActorEmails(t *testing.T) {
	t.Run("empty input returns nil", func(t *testing.T) {
		if got := parseClientActorEmails(""); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("parses a JSON object of client id to email", func(t *testing.T) {
		got := parseClientActorEmails(`{"client-a":"team-a@example.com","client-b":"team-b@example.com"}`)
		want := map[string]string{"client-a": "team-a@example.com", "client-b": "team-b@example.com"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("trims whitespace around client id and email", func(t *testing.T) {
		got := parseClientActorEmails(`{" client-a ":" team-a@example.com "}`)
		want := map[string]string{"client-a": "team-a@example.com"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("drops an entry with a blank client id or email, keeps the rest", func(t *testing.T) {
		got := parseClientActorEmails(`{"client-a":"team-a@example.com","":"ignored@example.com","client-c":"   "}`)
		want := map[string]string{"client-a": "team-a@example.com"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("not a JSON object returns nil rather than failing startup", func(t *testing.T) {
		for _, s := range []string{"not-json", `["client-a","team-a@example.com"]`, `{"client-a":123}`} {
			if got := parseClientActorEmails(s); got != nil {
				t.Errorf("input %q: got %v, want nil", s, got)
			}
		}
	})

	t.Run("an object with every entry blank returns nil, not an empty map", func(t *testing.T) {
		if got := parseClientActorEmails(`{"":""}`); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}
