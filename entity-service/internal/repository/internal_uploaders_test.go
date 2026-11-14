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
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// internalQuerier answers the internal-user lookup with a fixed set of
// lower-cased emails and records what it was asked.
type internalQuerier struct {
	internal []string
	err      error
	asked    [][]string
}

func (q *internalQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *internalQuerier) Query(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
	if q.err != nil {
		return nil, q.err
	}
	q.asked = append(q.asked, args[0].([]string))
	rows := &fakeRows{}
	for _, e := range q.internal {
		rows.rows = append(rows.rows, []any{e})
	}
	return rows, nil
}

func (q *internalQuerier) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

func uploader(email string) domain.Attachment {
	return domain.Attachment{CreatedBy: domain.NewUserReference("", email, "Jane Doe")}
}

func TestMarkInternalUploaders_FlagsOnlyInternalUsers(t *testing.T) {
	q := &internalQuerier{internal: []string{"staff@example.test"}}
	list := []domain.Attachment{uploader("Staff@Example.test"), uploader("customer@example.test"), {}}

	markInternalUploaders(context.Background(), q, list)

	if !list[0].CreatedBy.IsInternal {
		t.Error("an INTERNAL user's upload was not flagged, whatever the email's case")
	}
	if list[1].CreatedBy.IsInternal {
		t.Error("a customer's upload was flagged as internal")
	}
	if len(q.asked) != 1 || len(q.asked[0]) != 2 {
		t.Errorf("want one lookup for the 2 distinct emails, got %v", q.asked)
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

func TestMarkInternalUploaders_NoEmailsNoQuery(t *testing.T) {
	q := &internalQuerier{}
	markInternalUploaders(context.Background(), q, []domain.Attachment{{}})
	if len(q.asked) != 0 {
		t.Errorf("an empty page must not query, got %v", q.asked)
	}
}
