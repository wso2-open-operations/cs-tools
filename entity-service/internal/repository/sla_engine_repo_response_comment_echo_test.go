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

// TestHasQualifyingResponseComment_IgnoresDescriptionEcho is the regression
// test for a real, reported bug: the external sync job's own business rules
// auto-generate a case's "first comment" as a verbatim echo of its
// description (confirmed live -- a case created and "responded to" by the
// same account, a second apart, where that echo alone satisfied the
// CSEngineerResponseComment check). An echo must never count as a genuine
// response, regardless of who it's attributed to.
func TestHasQualifyingResponseComment_IgnoresDescriptionEcho(t *testing.T) {
	description := "<p>test</p>"

	t.Run("a lone description echo does not qualify", func(t *testing.T) {
		if hasQualifyingResponseComment(description, []string{"<p>test</p>"}) {
			t.Error("want false: the only candidate is just the description echoed back")
		}
	})

	t.Run("echo is case/whitespace/markup insensitive", func(t *testing.T) {
		if hasQualifyingResponseComment(description, []string{"  <P>Test</P>  "}) {
			t.Error("want false: still an echo once normalized")
		}
	})

	t.Run("a genuine reply alongside the echo still qualifies", func(t *testing.T) {
		if !hasQualifyingResponseComment(description, []string{"<p>test</p>", "Looking into this now."}) {
			t.Error("want true: a real reply exists alongside the echo")
		}
	})

	t.Run("a reply that quotes the description plus adds its own text qualifies", func(t *testing.T) {
		if !hasQualifyingResponseComment(description, []string{"test -- confirmed reproducible, investigating"}) {
			t.Error("want true: this is not an exact echo, it has its own content")
		}
	})

	t.Run("no candidates at all never qualifies", func(t *testing.T) {
		if hasQualifyingResponseComment(description, nil) {
			t.Error("want false: no candidate comments exist")
		}
	})

	t.Run("blank description means nothing to echo, so any candidate qualifies", func(t *testing.T) {
		if !hasQualifyingResponseComment("", []string{"anything"}) {
			t.Error("want true: a blank description has nothing to repeat")
		}
	})

	t.Run("blank description with no candidates still does not qualify", func(t *testing.T) {
		if hasQualifyingResponseComment("", nil) {
			t.Error("want false: no candidates at all")
		}
	})
}
