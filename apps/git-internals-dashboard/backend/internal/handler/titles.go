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
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	ghclient "github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	titlesCacheTTL  = 15 * time.Minute
	titlesCacheCap  = 5000
	titlesBatchSize = 100 // issues per GraphQL request

	// titlesMaxBodyBytes bounds POST /issues/titles' request body before it is
	// decoded. 200 ids as JSON ints is well under 4KB; 64KB is generous
	// headroom without letting a client force a multi-hundred-MB allocation
	// before the 1-200 length check ever runs (AUDIT-FINDINGS A1).
	titlesMaxBodyBytes = 1 << 16
)

// TitlesHandler serves POST /issues/titles (SPEC §6.5). PRIVACY: titles are
// resolved live from GitHub on every cache miss and cached in memory only —
// never persisted to the database (SPEC's non-negotiable #1).
type TitlesHandler struct {
	pool        *pgxpool.Pool
	githubToken string
	cache       *metrics.TTLCache[string, *string]
}

// NewTitlesHandler creates a TitlesHandler. githubToken may be empty — every
// title then resolves to null (no live GitHub calls are attempted).
func NewTitlesHandler(pool *pgxpool.Pool, githubToken string) *TitlesHandler {
	return &TitlesHandler{
		pool:        pool,
		githubToken: strings.TrimSpace(githubToken),
		cache:       metrics.NewTTLCache[string, *string](titlesCacheTTL, titlesCacheCap),
	}
}

type titlesRequestBody struct {
	IDs []int `json:"ids"`
}

type titlesResponseBody struct {
	Titles map[string]*string `json:"titles"`
}

// PostTitles handles POST /issues/titles: body {"ids": [1,2,3]}, 1-200
// positive ints. Resolution flow (port of issue-titles.ts): ids -> (owner,
// name, number) from OUR DB -> batched GitHub GraphQL -> in-memory TTL
// cache -> map. null per id when: no GITHUB_TOKEN, unknown id, synthetic
// fixture, deleted issue, or GitHub failure.
func (h *TitlesHandler) PostTitles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, titlesMaxBodyBytes)

	var body titlesRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			apierror.ValidationFailed(w, "request body too large")
			return
		}
		apierror.ValidationFailed(w, "invalid JSON body")
		return
	}
	if len(body.IDs) < 1 || len(body.IDs) > 200 {
		apierror.ValidationFailed(w, "ids must contain between 1 and 200 entries")
		return
	}
	for _, id := range body.IDs {
		if id <= 0 || id > math.MaxInt32 {
			apierror.ValidationFailed(w, "ids must all be positive 32-bit integers")
			return
		}
	}

	result := make(map[string]*string, len(body.IDs))
	for _, id := range body.IDs {
		result[strconv.Itoa(id)] = nil
	}

	// Resolve ids -> refs from OUR DB (never trust client-supplied repo/number).
	refs, err := h.resolveRefs(r.Context(), body.IDs)
	if err != nil {
		apierror.Internal(w, r, "resolve issue titles refs failed", err)
		return
	}

	// Cache pass.
	var misses []ghclient.TitleRef
	for _, ref := range refs {
		if hit, ok := h.cache.Get(ghclient.RefKey(ref)); ok {
			result[strconv.Itoa(int(ref.ID))] = hit
		} else {
			misses = append(misses, ref)
		}
	}
	if len(misses) == 0 || h.githubToken == "" {
		writeJSON(w, http.StatusOK, titlesResponseBody{Titles: result})
		return
	}

	// Fetch misses in batches; a failed batch degrades to nulls, never a 500.
	for start := 0; start < len(misses); start += titlesBatchSize {
		end := min(start+titlesBatchSize, len(misses))
		chunk := misses[start:end]
		fetched, err := ghclient.FetchTitles(r.Context(), h.githubToken, chunk)
		if err != nil {
			continue // leave chunk as null; do not cache failures so a later request can retry
		}
		for _, ref := range chunk {
			title := fetched[ghclient.RefKey(ref)]
			h.cache.Set(ghclient.RefKey(ref), title)
			result[strconv.Itoa(int(ref.ID))] = title
		}
	}

	writeJSON(w, http.StatusOK, titlesResponseBody{Titles: result})
}

func (h *TitlesHandler) resolveRefs(ctx context.Context, ids []int) ([]ghclient.TitleRef, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT i.id, i.github_number, r.owner, r.name
		FROM issues i
		JOIN repositories r ON r.id = i.repository_id
		WHERE i.id = ANY($1) AND r.enabled = true
	`, toInt32Slice(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ghclient.TitleRef
	for rows.Next() {
		var ref ghclient.TitleRef
		if err := rows.Scan(&ref.ID, &ref.Number, &ref.Owner, &ref.Name); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// toInt32Slice converts already-validated ids (PostTitles rejects anything
// outside 1..math.MaxInt32 before this is ever called) to the issues.id
// column's int32 representation.
func toInt32Slice(ids []int) []int32 {
	out := make([]int32, len(ids))
	for i, id := range ids {
		out[i] = int32(id) // #nosec G115 -- bounds-checked by PostTitles (1..math.MaxInt32) before this runs
	}
	return out
}
