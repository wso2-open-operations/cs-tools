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

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/githubissue"
)

func TestMetadataHandler_GetMetadata(t *testing.T) {
	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewMetadataHandler()
		w := httptest.NewRecorder()
		h.GetMetadata(w, httptest.NewRequest(http.MethodGet, "/metadata", nil))
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("serves the configured githubIssueRepoOptions catalogue verbatim", func(t *testing.T) {
		original := githubissue.Active()
		defer githubissue.SetActive(original)

		githubissue.SetActive([]githubissue.RepoOption{
			{Value: "choreo", DisplayLabel: "WSO2 Developer Platform (Choreo)", Owner: "wso2-enterprise", Repo: "choreo", GithubLabel: "Choreo"},
		})

		h := NewMetadataHandler()
		w := httptest.NewRecorder()
		h.GetMetadata(w, withUser(httptest.NewRequest(http.MethodGet, "/metadata", nil)))

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		got := decodeJSON[MetadataResponse](t, w)
		if len(got.GithubIssueRepoOptions) != 1 {
			t.Fatalf("githubIssueRepoOptions = %+v, want exactly the one configured option", got.GithubIssueRepoOptions)
		}
		opt := got.GithubIssueRepoOptions[0]
		if opt.Value != "choreo" || opt.DisplayLabel != "WSO2 Developer Platform (Choreo)" || opt.Owner != "wso2-enterprise" || opt.Repo != "choreo" || opt.GithubLabel != "Choreo" {
			t.Errorf("option = %+v, want it to match the configured entry verbatim", opt)
		}
	})

	t.Run("serves an empty githubIssueRepoOptions array, not an error, when unconfigured", func(t *testing.T) {
		original := githubissue.Active()
		defer githubissue.SetActive(original)
		githubissue.SetActive(nil)

		h := NewMetadataHandler()
		w := httptest.NewRecorder()
		h.GetMetadata(w, withUser(httptest.NewRequest(http.MethodGet, "/metadata", nil)))

		assertStatus(t, w, http.StatusOK)
		got := decodeJSON[MetadataResponse](t, w)
		if got.GithubIssueRepoOptions == nil || len(got.GithubIssueRepoOptions) != 0 {
			t.Errorf("githubIssueRepoOptions = %+v, want an empty (non-nil) array", got.GithubIssueRepoOptions)
		}
	})
}
