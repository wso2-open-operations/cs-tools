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
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// kbArticleFakeRow feeds scanKBArticle fixed values without a database, and
// -- unlike the looser fakes elsewhere in this package, which leave a
// destination at its zero value for a nil input -- rejects a NULL (nil)
// value for any destination that is not a pointer, the same way pgx does
// ("cannot scan NULL into *string"). That strictness is the point: a scan that
// goes back to plain string/bool destinations for a nullable column fails
// these tests instead of passing quietly.
type kbArticleFakeRow struct {
	vals []any
	err  error
}

func (r kbArticleFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("kbArticleFakeRow: %d scan destinations, %d values", len(dest), len(r.vals))
	}
	for i, d := range dest {
		elem := reflect.ValueOf(d).Elem()
		v := r.vals[i]
		if v == nil {
			if elem.Kind() != reflect.Pointer {
				return fmt.Errorf("can't scan into dest[%d]: cannot scan NULL into %T", i, d)
			}
			elem.SetZero()
			continue
		}
		val := reflect.ValueOf(v)
		if elem.Kind() == reflect.Pointer {
			p := reflect.New(elem.Type().Elem())
			p.Elem().Set(val.Convert(elem.Type().Elem()))
			elem.Set(p)
			continue
		}
		elem.Set(val.Convert(elem.Type()))
	}
	return nil
}

// kbArticleRowVals returns the 16 column values in kbArticleColumns order:
// id, knowledge_base_id, title, body, state, author_id, revised_by_id,
// source_case_id, rejection_comment, updated_by, base_version_id, latest,
// created_on, updated_on, published_on, retired_on. A nil argument is a SQL
// NULL.
func kbArticleRowVals(knowledgeBaseID, body, state, authorID, latest any, createdOn, updatedOn time.Time) []any {
	return []any{
		"a1", knowledgeBaseID, "How to reset a password", body, state, authorID,
		nil, nil, nil, nil, nil, latest,
		createdOn, updatedOn, nil, nil,
	}
}

func TestScanKBArticle_NullableColumns(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		vals []any
		want domain.KBArticle
	}{
		{
			name: "NULL body, author_id and state",
			vals: kbArticleRowVals("kb1", nil, nil, nil, true, now, now),
			want: domain.KBArticle{
				ID: "a1", KnowledgeBaseID: "kb1", Title: "How to reset a password",
				Body: "", State: "", AuthorID: "", Latest: true,
				CreatedOn: now, UpdatedOn: now,
			},
		},
		{
			name: "NULL knowledge_base_id and latest",
			vals: kbArticleRowVals(nil, "body text", "published", "u1", nil, now, now),
			want: domain.KBArticle{
				ID: "a1", KnowledgeBaseID: "", Title: "How to reset a password",
				Body: "body text", State: domain.KBArticleStatePublished, AuthorID: "u1", Latest: false,
				CreatedOn: now, UpdatedOn: now,
			},
		},
		{
			name: "every nullable column NULL",
			vals: kbArticleRowVals(nil, nil, nil, nil, nil, now, now),
			want: domain.KBArticle{
				ID: "a1", Title: "How to reset a password",
				CreatedOn: now, UpdatedOn: now,
			},
		},
		{
			name: "every column populated",
			vals: kbArticleRowVals("kb1", "body text", "draft", "u1", true, now, now),
			want: domain.KBArticle{
				ID: "a1", KnowledgeBaseID: "kb1", Title: "How to reset a password",
				Body: "body text", State: domain.KBArticleStateDraft, AuthorID: "u1", Latest: true,
				CreatedOn: now, UpdatedOn: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got domain.KBArticle
			if err := scanKBArticle(kbArticleFakeRow{vals: tt.vals}, &got); err != nil {
				t.Fatalf("scanKBArticle() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scanKBArticle() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Callers branch on the raw scan error (pgx.ErrNoRows -> conflict in
// UpdateKBArticleState, *pgconn.PgError -> validation), so it must come back
// unwrapped.
func TestScanKBArticle_PropagatesScanError(t *testing.T) {
	var got domain.KBArticle
	err := scanKBArticle(kbArticleFakeRow{err: pgx.ErrNoRows}, &got)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("scanKBArticle() error = %v, want pgx.ErrNoRows", err)
	}
}

// Guards the fake itself: if it ever stopped rejecting NULL into a plain
// destination, the tests above would no longer prove anything.
func TestKBArticleFakeRow_RejectsNullIntoPlainDestination(t *testing.T) {
	var s string
	if err := (kbArticleFakeRow{vals: []any{nil}}).Scan(&s); err == nil {
		t.Fatal("Scan(NULL into *string) succeeded, want an error")
	}
	var p *string
	if err := (kbArticleFakeRow{vals: []any{nil}}).Scan(&p); err != nil || p != nil {
		t.Fatalf("Scan(NULL into **string) = %v, p = %v, want nil error and nil pointer", err, p)
	}
}
