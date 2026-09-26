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

package github

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseIssueURL turns an issue's browser URL into the parts the API needs.
//
//	https://github.com/wso2/choreo/issues/42  ->  {wso2, choreo, 42}
//
// This is the join between a change request and its issue: the stored
// reference is the html_url, and every API call needs owner/repo/number.
//
// Parsed as a URL rather than pattern-matched. ServiceNow used
// /([^\/]+)\/([^\/]+)\/issues\/(\d+)$/ against the raw string, which takes
// whatever two path segments happen to precede "/issues/" -- so a pull-request
// URL, a query string, or a trailing slash all produce a confidently wrong
// answer instead of an error.
func ParseIssueURL(raw string) (Issue, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Issue{}, fmt.Errorf("github: empty issue URL")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Issue{}, fmt.Errorf("github: parse issue URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Issue{}, fmt.Errorf("github: issue URL %q is not http(s)", raw)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// owner / repo / issues / number
	if len(parts) != 4 || parts[2] != "issues" {
		return Issue{}, fmt.Errorf("github: %q is not an issue URL", raw)
	}
	owner, repo := parts[0], parts[1]
	if owner == "" || repo == "" {
		return Issue{}, fmt.Errorf("github: %q has no owner or repository", raw)
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return Issue{}, fmt.Errorf("github: %q has no issue number", raw)
	}
	return Issue{Owner: owner, Repository: repo, Number: number}, nil
}
