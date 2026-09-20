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

package githubissue

import "testing"

func TestParseRepoOptions(t *testing.T) {
	t.Run("empty string yields no options and no error", func(t *testing.T) {
		got, err := ParseRepoOptions("")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got != nil {
			t.Errorf("options = %+v, want nil", got)
		}
	})

	t.Run("JSON null is rejected, not treated as unset", func(t *testing.T) {
		if _, err := ParseRepoOptions("null"); err == nil {
			t.Fatal("err = nil, want an error for a JSON null value")
		}
	})

	t.Run("JSON empty array yields no options and no error", func(t *testing.T) {
		got, err := ParseRepoOptions("[]")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if len(got) != 0 {
			t.Errorf("options = %+v, want empty", got)
		}
	})

	t.Run("valid options parse verbatim", func(t *testing.T) {
		got, err := ParseRepoOptions(`[{"value":"asgardeo","displayLabel":"Asgardeo","owner":"wso2-enterprise","repo":"wso2-iam-internal","githubLabel":"Asgardeo"}]`)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if len(got) != 1 || got[0] != (RepoOption{Value: "asgardeo", DisplayLabel: "Asgardeo", Owner: "wso2-enterprise", Repo: "wso2-iam-internal", GithubLabel: "Asgardeo"}) {
			t.Errorf("options = %+v, want the single parsed entry", got)
		}
	})

	t.Run("duplicate value is rejected", func(t *testing.T) {
		_, err := ParseRepoOptions(`[{"value":"a","displayLabel":"A","owner":"o","repo":"r","githubLabel":"L"},{"value":"a","displayLabel":"B","owner":"o","repo":"r","githubLabel":"L"}]`)
		if err == nil {
			t.Fatal("err = nil, want an error for a duplicate value")
		}
	})

	t.Run("blank field is rejected", func(t *testing.T) {
		_, err := ParseRepoOptions(`[{"value":"a","displayLabel":"","owner":"o","repo":"r","githubLabel":"L"}]`)
		if err == nil {
			t.Fatal("err = nil, want an error for a blank displayLabel")
		}
	})

	t.Run("blank githubLabel is rejected", func(t *testing.T) {
		_, err := ParseRepoOptions(`[{"value":"a","displayLabel":"A","owner":"o","repo":"r","githubLabel":""}]`)
		if err == nil {
			t.Fatal("err = nil, want an error for a blank githubLabel")
		}
	})
}
