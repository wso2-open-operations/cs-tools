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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// The proposer of a customer's proposed time is the change's last writer (work_item.updated_by),
// read before the act's own write, when that writer is a registered contact of the project -- and
// nothing else. These tests need no database: a recording querier shows exactly which statements the
// proposer path sends, and a source scan shows that no SQL of the proposal file reads a comment.

// proposerRow is a pgx.Row that hands back canned values.
type proposerRow struct {
	vals []any
	err  error
}

func (r proposerRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("fake row: %d destinations for %d values", len(dest), len(r.vals))
	}
	for i, d := range dest {
		switch d := d.(type) {
		case *bool:
			*d = r.vals[i].(bool)
		case **string:
			*d = r.vals[i].(*string)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		default:
			return fmt.Errorf("fake row: unsupported destination %T", d)
		}
	}
	return nil
}

// proposerQuerier answers the statements of the proposer path and records every one of them.
type proposerQuerier struct {
	statements []string
	// what the database would say
	project, lastWriter, userName *string
	updatedOn                     time.Time
	isContact                     bool
	contactChecks                 []string
}

func (q *proposerQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	q.statements = append(q.statements, sql)
	return nil, errors.New("fake querier: the proposer path reads one row at a time")
}

func (q *proposerQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.statements = append(q.statements, sql)
	switch {
	case strings.Contains(sql, "FROM work_item WHERE id"):
		return proposerRow{vals: []any{q.project, q.lastWriter, q.updatedOn}}
	case strings.Contains(sql, "FROM project_contact pc"):
		q.contactChecks = append(q.contactChecks, fmt.Sprint(args[1]))
		return proposerRow{vals: []any{q.isContact}}
	case strings.Contains(sql, `FROM "user" u`):
		return proposerRow{vals: []any{q.userName}}
	}
	return proposerRow{err: fmt.Errorf("fake querier: unexpected statement: %s", sql)}
}

// proposerPathTables are the only tables a statement of the proposer path may read: the change's own
// work item (its last writer and project) and the registered-contact join, and the user's name.
var proposerPathTables = map[string]bool{
	"work_item": true, "project_contact": true, "project_contact_group": true,
	"project_group_role": true, "project_role": true, `"user"`: true,
}

var tableRef = regexp.MustCompile(`(?is)\b(?:from|join)\s+("?[a-z_]+"?)`)

func proposerStr(s string) *string { return &s }

func (q *proposerQuerier) wantOnlyTheProposerTables(t *testing.T) {
	t.Helper()
	for _, sql := range q.statements {
		for _, m := range tableRef.FindAllStringSubmatch(sql, -1) {
			if !proposerPathTables[strings.ToLower(m[1])] {
				t.Fatalf("a statement of the proposer path reads %s, which is no part of the rule (the last writer and the contact register):\n%s", m[1], sql)
			}
		}
		if readsAComment(sql) {
			t.Fatalf("a statement of the proposer path reads the comment table:\n%s", sql)
		}
	}
}

func readsAComment(sql string) bool {
	return regexp.MustCompile(`(?is)\b(from|join)\s+"?comment"?\b`).MatchString(sql)
}

func TestNewLastWriter(t *testing.T) {
	for name, tc := range map[string]struct {
		in   *string
		want string
	}{
		"a NULL column is a writer who is nobody, but a writer that was read": {nil, ""},
		"a blank column the same":           {proposerStr("  "), ""},
		"an email, trimmed":                 {proposerStr(" alice@example.com "), "alice@example.com"},
		"the sync loader's own stamp as is": {proposerStr("sync-loader"), "sync-loader"},
	} {
		got := newLastWriter(tc.in)
		if !got.read || got.email != tc.want {
			t.Errorf("%s: newLastWriter = %+v, want read with %q", name, got, tc.want)
		}
	}
	if (lastWriter{}).read {
		t.Error("the zero lastWriter claims to have been read")
	}
}

func TestResolveProposer_TheLastWriterAndNothingElse(t *testing.T) {
	ctx := context.Background()
	project := proposerStr("00000000-0000-4000-8000-0000000000a1")

	t.Run("a writer that was never read is a bug of the caller, refused before any statement", func(t *testing.T) {
		q := &proposerQuerier{}
		_, err := resolveProposer(ctx, q, project, lastWriter{})
		if err == nil || !strings.Contains(err.Error(), "was not read") {
			t.Fatalf("err = %v, want the refusal of an unread writer", err)
		}
		if len(q.statements) != 0 {
			t.Fatalf("statements = %v, want none", q.statements)
		}
	})

	t.Run("nobody as the last writer (NULL, blank) names nobody and sends no statement", func(t *testing.T) {
		for _, w := range []*string{nil, proposerStr(""), proposerStr("   ")} {
			q := &proposerQuerier{}
			p, err := resolveProposer(ctx, q, project, newLastWriter(w))
			if err != nil || p.known {
				t.Fatalf("writer %v: proposer = %+v, err = %v, want nobody", w, p, err)
			}
			if len(q.statements) != 0 {
				t.Fatalf("writer %v: statements = %v, want none", w, q.statements)
			}
		}
	})

	t.Run("a registered contact of the project is the proposer, by one contact check and nothing more", func(t *testing.T) {
		q := &proposerQuerier{isContact: true}
		p, err := resolveProposer(ctx, q, project, newLastWriter(proposerStr(" Alice@Example.com ")))
		if err != nil || !p.known || p.email != "Alice@Example.com" {
			t.Fatalf("proposer = %+v, err = %v, want the trimmed writer", p, err)
		}
		if len(q.statements) != 1 || len(q.contactChecks) != 1 || q.contactChecks[0] != "Alice@Example.com" {
			t.Fatalf("statements = %d, contact checks = %v, want exactly one check of the writer", len(q.statements), q.contactChecks)
		}
		q.wantOnlyTheProposerTables(t)
	})

	t.Run("a writer who is not a registered contact of the project is nobody", func(t *testing.T) {
		q := &proposerQuerier{isContact: false}
		p, err := resolveProposer(ctx, q, project, newLastWriter(proposerStr("wso2.engineer@example.com")))
		if err != nil || p.known || p.email != "" {
			t.Fatalf("proposer = %+v, err = %v, want nobody", p, err)
		}
		q.wantOnlyTheProposerTables(t)
	})

	t.Run("a change with no project has no contact to be", func(t *testing.T) {
		q := &proposerQuerier{isContact: true}
		p, err := resolveProposer(ctx, q, nil, newLastWriter(proposerStr("alice@example.com")))
		if err != nil || p.known {
			t.Fatalf("proposer = %+v, err = %v, want nobody on a change with no project", p, err)
		}
	})
}

func TestReadProposer_ReadsTheLastWriterAndNothingElse(t *testing.T) {
	ctx := context.Background()
	on := time.Date(2030, 3, 8, 9, 0, 0, 0, time.UTC)
	q := &proposerQuerier{
		project: proposerStr("00000000-0000-4000-8000-0000000000a1"), lastWriter: proposerStr("alice@example.com"),
		userName: proposerStr("Alice Aaron"), updatedOn: on, isContact: true}
	p, err := readProposer(ctx, q, "00000000-0000-4000-8000-000000000001")
	if err != nil || !p.known || p.email != "alice@example.com" || p.name != "Alice Aaron" || !p.proposedOn.Equal(on) {
		t.Fatalf("proposer = %+v, err = %v, want the last writer, their name and when they wrote", p, err)
	}
	q.wantOnlyTheProposerTables(t)
	if len(q.statements) != 3 {
		t.Fatalf("statements = %d, want the last writer, the contact check and the name", len(q.statements))
	}

	// Not a contact: nobody, and not even the name is looked up.
	q = &proposerQuerier{project: proposerStr("p"), lastWriter: proposerStr("wso2.engineer@example.com"), isContact: false}
	p, err = readProposer(ctx, q, "x")
	if err != nil || p.known || !p.proposedOn.IsZero() || p.name != "" {
		t.Fatalf("proposer = %+v, err = %v, want nobody", p, err)
	}
	if len(q.statements) != 2 {
		t.Fatalf("statements = %d, want the last writer and the contact check", len(q.statements))
	}
	q.wantOnlyTheProposerTables(t)
}

// The mutation check of the rule "nothing but the last writer names a proposer": no SQL literal of
// the proposal file may read the comment table, whatever function it sits in. Putting the lookup of
// the comment the existing trigger writes back (in resolveProposer, readProposer or anywhere else
// in the file) fails here and in the traced integration test.
func TestProposalFile_NoSQLReadsTheCommentTable(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "change_request_customer_proposal.go", nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	scanned := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		sql, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if regexp.MustCompile(`(?is)\b(select|update|insert)\b`).MatchString(sql) {
			scanned++
			if readsAComment(sql) || regexp.MustCompile(`(?i)\binto\s+"?comment"?\b`).MatchString(sql) {
				t.Errorf("%s: a statement touches the comment table:\n%s", fset.Position(lit.Pos()), sql)
			}
		}
		return true
	})
	if scanned < 5 {
		t.Fatalf("scanned %d SQL literals, want the proposal file's own statements: the scan is not working", scanned)
	}
}
