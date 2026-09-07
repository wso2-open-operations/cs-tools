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

// Port of v3's src/server/lib/issue-titles.ts. Runtime issue-title
// resolution — PRIVACY: titles are never persisted; internal/handler's
// titles.go is the only caller and the only endpoint allowed to return one
// (SPEC's non-negotiable #1).
//
// Flow: ids -> (owner, name, number) from OUR DB (handler's job) -> batched
// GitHub GraphQL (aliased repository/issue lookups, <=100 issues per
// request, this file's job) -> in-memory TTL cache (handler's job) ->
// {id: title | null}.
//
// Deliberately has NO retry, unlike client.go's gql: a failed batch degrades
// every id in it to null for this request only and is never cached, so a
// later request can retry — retrying here would just make an already
// best-effort, latency-sensitive endpoint slower.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TitleRef is enough identity to ask GitHub for one issue's title: never an
// id trusted from the client — the handler resolves ids to these from our
// own DB first.
type TitleRef struct {
	ID     int32
	Owner  string
	Name   string
	Number int
}

func RefKey(r TitleRef) string {
	return fmt.Sprintf("%s/%s#%d", r.Owner, r.Name, r.Number)
}

type refGroup struct {
	Owner, Name string
	Refs        []TitleRef
}

// groupRefsByRepo groups refs by (owner, name), preserving first-seen order
// — both BuildTitlesQuery and the response reader in FetchTitles must
// derive the exact same r<i> alias layout independently.
func groupRefsByRepo(refs []TitleRef) []refGroup {
	var groups []refGroup
	index := make(map[string]int, len(refs))
	for _, r := range refs {
		key := r.Owner + "/" + r.Name
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, refGroup{Owner: r.Owner, Name: r.Name})
		}
		groups[i].Refs = append(groups[i].Refs, r)
	}
	return groups
}

// BuildTitlesQuery builds one GraphQL query aliasing each repository r<i>
// and each issue n<number>, so a single request covers every ref regardless
// of how many distinct repos it spans. Aliases must match
// [A-Za-z_][A-Za-z0-9_]*, which n<number> and r<i> always satisfy.
func BuildTitlesQuery(refs []TitleRef) string {
	groups := groupRefsByRepo(refs)
	parts := make([]string, len(groups))
	for i, g := range groups {
		issueLines := make([]string, len(g.Refs))
		for j, r := range g.Refs {
			issueLines[j] = fmt.Sprintf("n%d: issue(number: %d) { title }", r.Number, r.Number)
		}
		// owner/name come from our DB config; jsonString still escapes defensively.
		parts[i] = fmt.Sprintf("r%d: repository(owner: %s, name: %s) {\n      %s\n    }",
			i, jsonString(g.Owner), jsonString(g.Name), strings.Join(issueLines, "\n      "))
	}
	return fmt.Sprintf("query IssueTitles {\n    %s\n  }", strings.Join(parts, "\n    "))
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

type issueTitleNode struct {
	Title string `json:"title"`
}

type titlesGraphQLResponse struct {
	Data   map[string]map[string]*issueTitleNode `json:"data"`
	Errors []graphQLError                        `json:"errors"`
}

var titlesHTTPClient = &http.Client{}

// FetchTitles resolves every ref's title in one batched GraphQL request
// (callers are responsible for keeping each call to <=100 refs). Returns an
// empty map with no error when refs is empty or token is blank — the
// caller's null-per-id default already handles that. A per-ref key present
// with a nil value means GitHub confirmed no title (issue deleted, or the
// repo/issue lookup itself came back null) — a whole-batch failure (network
// error, non-200, or `data: null`) returns an error instead, so the caller
// does not cache nulls for a transient failure.
func FetchTitles(ctx context.Context, token string, refs []TitleRef) (map[string]*string, error) {
	out := make(map[string]*string)
	token = strings.TrimSpace(token)
	if len(refs) == 0 || token == "" {
		return out, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	body, err := json.Marshal(map[string]any{"query": BuildTitlesQuery(refs)})
	if err != nil {
		return nil, fmt.Errorf("github: encode titles request: %w", err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, graphQLPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("github: build titles request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "git-internals-dashboard")

	resp, err := titlesHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: titles request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("github graphql titles http %d: %s", resp.StatusCode, snippet)
	}

	var parsed titlesGraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("github: decode titles response: %w", err)
	}

	// GraphQL can return HTTP 200 with data: null (missing scope, secondary
	// rate limit, query too complex). That's a whole-batch failure, not a
	// per-issue null.
	if parsed.Data == nil {
		if len(parsed.Errors) > 0 {
			messages := make([]string, len(parsed.Errors))
			for i, e := range parsed.Errors {
				messages[i] = e.Message
			}
			return nil, fmt.Errorf("github graphql: empty response (no data): %s", strings.Join(messages, "; "))
		}
		return nil, fmt.Errorf("github graphql: empty response (no data)")
	}

	for i, g := range groupRefsByRepo(refs) {
		repoNode := parsed.Data[fmt.Sprintf("r%d", i)]
		for _, r := range g.Refs {
			var title *string
			if node := repoNode[fmt.Sprintf("n%d", r.Number)]; node != nil {
				t := node.Title
				title = &t
			}
			out[RefKey(r)] = title
		}
	}
	return out, nil
}
