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

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// What a service request keeps of its catalog request, in
// service_request.json_data (migration 0024). The column is a free-form JSONB
// that records raised from GitHub already use for their template fields, as
// flat u_-prefixed keys (see github_mutation_repo.go); the keys below do not
// start with u_, so the two vocabularies share the object without colliding:
//
//	{
//	  "catalogId":     "<uuid of the catalog (sr_category.id)>",
//	  "catalogItemId": "<uuid of the catalog item (catalog_item.id)>",
//	  "variables":     { "<uuid of the catalog variable>": "<answer>", ... }
//	}
//
// Answers are plain strings, as the previous data source stores every answer.
// The object is keyed by variable id rather than held as a list because an
// answer belongs to exactly one question: a duplicate id is refused, never
// stored twice. A reader orders the answers by the question's own "order".
// New keys may be added later; a reader must ignore keys it does not know.
const (
	serviceRequestCatalogIDKey     = "catalogId"
	serviceRequestCatalogItemIDKey = "catalogItemId"
	serviceRequestVariablesKey     = "variables"
)

// Bounds on what a transfer may store in json_data. They are generous for a
// request form (a catalog item has a few dozen questions at most) and exist so
// one request cannot park an arbitrarily large document on a work item.
const (
	// MaxServiceRequestVariables is how many answers a service request may carry.
	MaxServiceRequestVariables = 200
	// MaxServiceRequestVariableValueBytes is the longest single answer.
	MaxServiceRequestVariableValueBytes = 8 * 1024
	// MaxServiceRequestVariablesBytes is the encoded size of all the answers.
	MaxServiceRequestVariablesBytes = 64 * 1024
)

// ServiceRequestCatalogData builds the json_data document for a service request
// raised from the given catalog item and answers, refusing what cannot be
// stored (see the bounds above). The service calls it while validating a
// transfer, before any other system is asked, and the repository calls it
// again when it writes, so the stored shape has exactly one author.
func ServiceRequestCatalogData(catalogID, catalogItemID string, variables []domain.Variable) ([]byte, error) {
	if !validate.IsUUID(catalogID) {
		return nil, &apierror.ValidationError{Msg: "catalogId must be a valid UUID"}
	}
	if !validate.IsUUID(catalogItemID) {
		return nil, &apierror.ValidationError{Msg: "catalogItemId must be a valid UUID"}
	}
	if len(variables) > MaxServiceRequestVariables {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("variables may hold at most %d entries", MaxServiceRequestVariables)}
	}
	answers := make(map[string]string, len(variables))
	for i, v := range variables {
		if !validate.IsUUID(v.ID) {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("variables[%d].id must be a valid UUID", i)}
		}
		key := strings.ToLower(v.ID)
		if _, dup := answers[key]; dup {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("variables[%d].id repeats a variable already answered", i)}
		}
		if len(v.Value) > MaxServiceRequestVariableValueBytes {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("variables[%d].value is longer than %d bytes", i, MaxServiceRequestVariableValueBytes)}
		}
		// jsonb cannot hold U+0000; left in, it would fail the insert as a 500.
		if strings.ContainsRune(v.Value, 0) {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("variables[%d].value must not contain a NUL character", i)}
		}
		answers[key] = v.Value
	}
	encodedAnswers, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("encode service request variables: %w", err)
	}
	if len(encodedAnswers) > MaxServiceRequestVariablesBytes {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("variables are larger than %d bytes in all", MaxServiceRequestVariablesBytes)}
	}
	doc, err := json.Marshal(map[string]any{
		serviceRequestCatalogIDKey:     strings.ToLower(catalogID),
		serviceRequestCatalogItemIDKey: strings.ToLower(catalogItemID),
		serviceRequestVariablesKey:     json.RawMessage(encodedAnswers),
	})
	if err != nil {
		return nil, fmt.Errorf("encode service request catalog data: %w", err)
	}
	return doc, nil
}

// serviceRequestCatalog is what a service request's json_data says about the
// catalog request it was raised from, resolved to names for the case view.
type serviceRequestCatalog struct {
	catalog     *domain.EntityRef
	catalogItem *domain.EntityRef
	variables   []domain.CaseVariable
}

// readServiceRequestCatalog reads the catalog request back out of
// service_request.json_data and names what it points at. Nothing there (a
// service request raised any other way, or a document in another shape) is not
// an error: the result is empty. An id that no longer resolves keeps the id as
// its name, so the answer is still shown.
func (r *caseRepo) readServiceRequestCatalog(ctx context.Context, id string) (serviceRequestCatalog, error) {
	var raw []byte
	if err := r.db.QueryRow(ctx, `SELECT json_data FROM service_request WHERE id = $1::uuid`, id).Scan(&raw); err != nil {
		return serviceRequestCatalog{}, fmt.Errorf("read service request data: %w", err)
	}
	if len(raw) == 0 {
		return serviceRequestCatalog{}, nil
	}
	var doc struct {
		CatalogID     string          `json:"catalogId"`
		CatalogItemID string          `json:"catalogItemId"`
		Variables     json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		// Not an object of the shape above (a field of the wrong type): there is
		// no catalog request to show, and the record is otherwise fine.
		slog.WarnContext(ctx, "service request data is not in the catalog shape", "caseId", id, "error", err)
		return serviceRequestCatalog{}, nil
	}
	var out serviceRequestCatalog
	if validate.IsUUID(doc.CatalogID) {
		out.catalog = &domain.EntityRef{ID: doc.CatalogID, Name: doc.CatalogID}
		var name string
		if err := r.db.QueryRow(ctx, `SELECT name FROM sr_category WHERE id = $1::uuid`, doc.CatalogID).Scan(&name); err == nil {
			out.catalog.Name = name
		}
	}
	if validate.IsUUID(doc.CatalogItemID) {
		out.catalogItem = &domain.EntityRef{ID: doc.CatalogItemID, Name: doc.CatalogItemID}
		var name string
		if err := r.db.QueryRow(ctx, `SELECT name FROM catalog_item WHERE id = $1::uuid`, doc.CatalogItemID).Scan(&name); err == nil {
			out.catalogItem.Name = name
		}
	}
	var answers map[string]string
	if len(doc.Variables) == 0 || json.Unmarshal(doc.Variables, &answers) != nil || len(answers) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(answers))
	for k := range answers {
		if validate.IsUUID(k) {
			ids = append(ids, k)
		}
	}
	sort.Strings(ids)
	type question struct {
		text  string
		order int
	}
	questions := make(map[string]question, len(ids))
	rows, err := r.db.Query(ctx,
		`SELECT id::TEXT, COALESCE(NULLIF(question_text, ''), NULLIF(name, ''), id::TEXT), COALESCE("order", 2147483647)
		   FROM catalog_variable WHERE id = ANY($1::text[]::uuid[])`, ids)
	if err != nil {
		return out, fmt.Errorf("read catalog variable names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var vid string
		var q question
		if err := rows.Scan(&vid, &q.text, &q.order); err != nil {
			return out, fmt.Errorf("scan catalog variable name: %w", err)
		}
		questions[vid] = q
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("read catalog variable names: %w", err)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		oi, oj := 2147483647, 2147483647
		if q, ok := questions[ids[i]]; ok {
			oi = q.order
		}
		if q, ok := questions[ids[j]]; ok {
			oj = q.order
		}
		return oi < oj
	})
	for _, vid := range ids {
		name := vid
		if q, ok := questions[vid]; ok {
			name = q.text
		}
		out.variables = append(out.variables, domain.CaseVariable{Name: name, Value: answers[vid]})
	}
	return out, nil
}
