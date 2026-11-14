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
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// internalQuerier answers the two internal-user lookups from fixed data and
// records what it was asked: by id (internalByID, lower-cased id -> internal)
// and by email (internalByEmail, lower-cased email -> internal), the answer for
// the one row userByEmailJoin would select.
type internalQuerier struct {
	internalByID    map[string]bool
	internalByEmail map[string]bool
	err             error
	askedIDs        [][]string
	askedEmails     [][]string
}

func (q *internalQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *internalQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if q.err != nil {
		return nil, q.err
	}
	asked := args[0].([]string)
	rows := &fakeRows{}
	if strings.Contains(sql, "DISTINCT ON") {
		q.askedEmails = append(q.askedEmails, asked)
		for _, e := range asked {
			if v, ok := q.internalByEmail[e]; ok {
				rows.rows = append(rows.rows, []any{e, v})
			}
		}
		return rows, nil
	}
	q.askedIDs = append(q.askedIDs, asked)
	for _, id := range asked {
		if v, ok := q.internalByID[id]; ok {
			rows.rows = append(rows.rows, []any{id, v})
		}
	}
	return rows, nil
}

func (q *internalQuerier) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

func uploader(email string) domain.Attachment {
	return domain.Attachment{CreatedBy: domain.NewUserReference("", email, "Jane Doe")}
}

func uploaderWithID(id, email string) domain.Attachment {
	return domain.Attachment{CreatedBy: domain.NewUserReference(id, email, "Jane Doe")}
}

func TestMarkInternalUploaders_FlagsOnlyInternalUsers(t *testing.T) {
	q := &internalQuerier{internalByEmail: map[string]bool{"staff@example.test": true, "customer@example.test": false}}
	list := []domain.Attachment{uploader("Staff@Example.test"), uploader("customer@example.test"), {}}

	markInternalUploaders(context.Background(), q, list)

	if !list[0].CreatedBy.IsInternal {
		t.Error("an INTERNAL user's upload was not flagged, whatever the email's case")
	}
	if list[1].CreatedBy.IsInternal {
		t.Error("a customer's upload was flagged as internal")
	}
	if len(q.askedEmails) != 1 || len(q.askedEmails[0]) != 2 || len(q.askedIDs) != 0 {
		t.Errorf("want one email lookup for the 2 distinct emails and none by id, got emails=%v ids=%v", q.askedEmails, q.askedIDs)
	}
}

// Two user rows can share an email. A reference that carries an id is judged by
// that user, never by another row with the same email.
func TestMarkInternalUploaders_AReferenceWithAnIDIsJudgedByThatUser(t *testing.T) {
	q := &internalQuerier{
		internalByID:    map[string]bool{"11111111-1111-1111-1111-111111111111": false},
		internalByEmail: map[string]bool{"shared@example.test": true},
	}
	list := []domain.Attachment{uploaderWithID("11111111-1111-1111-1111-111111111111", "shared@example.test")}

	markInternalUploaders(context.Background(), q, list)

	if list[0].CreatedBy.IsInternal {
		t.Error("an external user was flagged because another user row shares their email")
	}
	if len(q.askedEmails) != 0 {
		t.Errorf("a reference with an id must not be looked up by email, got %v", q.askedEmails)
	}
}

// With only an email, the answer is the one row the name lookup would select.
func TestMarkInternalUploaders_AnEmailOnlyReferenceTakesTheSelectedRow(t *testing.T) {
	q := &internalQuerier{internalByEmail: map[string]bool{"shared@example.test": false}}
	list := []domain.Attachment{uploader("shared@example.test")}

	markInternalUploaders(context.Background(), q, list)

	if list[0].CreatedBy.IsInternal {
		t.Error("the selected row is external, so the reference must not be flagged")
	}
}

func TestMarkInternalUploaders_LookupFailureLeavesFlagsUnset(t *testing.T) {
	q := &internalQuerier{err: errors.New("db down")}
	list := []domain.Attachment{uploader("staff@example.test")}

	markInternalUploaders(context.Background(), q, list)

	if list[0].CreatedBy.IsInternal {
		t.Error("a failed lookup must not flag anyone")
	}
}

func TestMarkInternalUploaders_NoPeopleNoQuery(t *testing.T) {
	q := &internalQuerier{}
	markInternalUploaders(context.Background(), q, []domain.Attachment{{}})
	if len(q.askedEmails) != 0 || len(q.askedIDs) != 0 {
		t.Errorf("an empty page must not query, got emails=%v ids=%v", q.askedEmails, q.askedIDs)
	}
}
