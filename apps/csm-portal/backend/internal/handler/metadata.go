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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/githubissue"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// metadataGithubIssueRepoOptionView is one entry of the "repository" catalogue
// the "Open Git issue" dialog offers — see githubissue.RepoOption.
type metadataGithubIssueRepoOptionView struct {
	Value        string `json:"value"`
	DisplayLabel string `json:"displayLabel"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	GithubLabel  string `json:"githubLabel"`
}

// MetadataResponse is the portal's response for GET /metadata: a single
// growable bag of reference/config data the webapp fetches once, rather than
// a dedicated endpoint per field. GithubIssueRepoOptions is the first field;
// more are expected to be added here over time as new frontend needs come up
// — this struct should read as "the metadata bag", not as any one field
// wrapped in an envelope.
type MetadataResponse struct {
	GithubIssueRepoOptions []metadataGithubIssueRepoOptionView `json:"githubIssueRepoOptions"`
}

// MetadataHandler serves GET /metadata, the portal's single config-driven
// metadata bag.
//
// It has no upstream dependency: every field it currently serves is resolved
// once at process startup and read straight from memory. A future field
// backed by an upstream call should still assemble its part of the response
// here rather than growing a second endpoint.
type MetadataHandler struct{}

// NewMetadataHandler creates a MetadataHandler.
func NewMetadataHandler() *MetadataHandler {
	return &MetadataHandler{}
}

// GetMetadata handles GET /metadata.
//
// A deployment with GITHUB_ISSUE_REPO_OPTIONS unset serves an empty
// githubIssueRepoOptions array, not an error — same "must still start"
// contract as the dashboard registry.
func (h *MetadataHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	options := githubissue.Active()
	views := make([]metadataGithubIssueRepoOptionView, 0, len(options))
	for _, o := range options {
		views = append(views, metadataGithubIssueRepoOptionView{
			Value:        o.Value,
			DisplayLabel: o.DisplayLabel,
			Owner:        o.Owner,
			Repo:         o.Repo,
			GithubLabel:  o.GithubLabel,
		})
	}

	writeJSONValue(w, http.StatusOK, MetadataResponse{GithubIssueRepoOptions: views})
}
