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

// This is a plain (non-DB) static check, not an integration test: it needs
// no database and runs as part of every ordinary `go test ./...`, exactly
// the "compile-time-adjacent backstop" the RLS migration series' own plan
// called for -- "a lint rule banning pgxpool imports outside
// internal/db/cmd".
//
// That rule, taken completely literally, does not match this codebase:
// dozens of repository files legitimately still hold a raw *pgxpool.Pool
// (access_repo.go, catalog_repo.go, the plg_* files, and so on) because
// their tables were never in scope for RLS at all -- they carry no
// customer-project data to scope, so forcing them through Scoped would add
// runtime overhead (an identity stamp on every call) for no security
// benefit, and this test would be flagging dozens of unrelated files
// forever. rls.go/scoped.go themselves also legitimately hold the pool --
// they ARE the mechanism.
//
// What the plan's rule actually protects against is a file that touches
// one of the RLS-protected tables reaching Postgres through a raw pool
// instead of Scoped -- exactly the "someone reached the raw pool, bypassing
// the mechanism" gap FORCE ROW LEVEL SECURITY's own fail-closed default
// already backstops at the database layer, but which this test catches
// earlier, at build/test time, without needing a live database at all. So
// the rule this test actually enforces is narrower and more precise than
// the plan's literal wording: any internal/repository/*.go file (other
// than rls.go/scoped.go themselves) that imports pgxpool directly AND
// contains a string literal (or an identifier resolving to a string
// declared anywhere else in the package -- see packageStringConstants)
// referencing one of rlsProtectedTables (see rls_schema_integration_test.go)
// is a violation -- it is either a not-yet-converted file that needs to
// move to Scoped, or a regression in an already-converted one. A separate,
// pgxpool-import-independent rule flags any file other than scoped.go
// itself that accesses a selector named "pool" at all, closing the gap
// where Scoped's own unexported field could be reached from within this
// same package without ever needing a pgxpool import in the accessing file.
//
// Three known blind spots were identified in review (issue #2127). Two are
// closed by the checks just described (a shared package-level constant
// referenced only by identifier -- packageStringConstants; a raw .pool
// field access needing no pgxpool import of its own -- the selector check).
// The third remains open, deliberately: a table name assembled at runtime
// via fmt.Sprintf (e.g. fmt.Sprintf("...%s", "case_escalation")) has no
// single string literal containing both a SQL keyword and the table name
// for the AST walk to match, and reliably distinguishing that shape from
// unrelated Sprintf calls without a real parser tracking value flow across
// variables would add real complexity for a pattern that does not exist
// anywhere in this codebase today (confirmed by grep) -- FORCE ROW LEVEL
// SECURITY remains the actual, database-level backstop under all three.
package repository_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// rlsBypassLintExemptFiles are the only internal/repository/*.go files
// allowed to import pgxpool directly regardless of what their string
// literals mention -- the mechanism itself, not a table-specific
// repository. Every other file's exemption comes from simply not
// mentioning a protected table, not from being listed here.
var rlsBypassLintExemptFiles = map[string]bool{
	"rls.go":    true,
	"scoped.go": true,
}

// rlsTableNameMatchers builds one regexp per protected table name from
// rls_schema_integration_test.go's own rlsProtectedTables list, so the two
// can never quietly drift apart. Each requires an actual SQL keyword
// (FROM/JOIN/INTO/UPDATE) immediately before the table name, not a bare
// word-boundary match on the name alone: several of these names (case,
// incident, comment, problem) are also common English words, and a bare
// match flagged human-readable strings like an error message mentioning
// "an incident" or a reference-data label {"announcement", "Announcement"}
// -- neither is a real SQL reference. "case" additionally requires its
// double-quoted SQL form (`"case"`), since every real query in this
// codebase quotes it that way (case is a reserved word).
func rlsTableNameMatchers() map[string]*regexp.Regexp {
	out := make(map[string]*regexp.Regexp, len(rlsProtectedTables))
	for _, table := range rlsProtectedTables {
		// A trailing \b keeps "comment" from matching "commented"; it is
		// omitted for the quoted "case" form because the closing quote is a
		// non-word character, so \b after it never matches real SQL
		// (`FROM "case" c`).
		name := regexp.QuoteMeta(table) + `\b`
		if table == "case" {
			name = `"case"`
		}
		out[table] = regexp.MustCompile(`(?i)\b(FROM|JOIN|INTO|UPDATE)\s+` + name)
	}
	return out
}

// packageStringConstants parses every non-test .go file in repoDir and
// returns a name -> value map of every top-level `const NAME = "literal"` /
// `var NAME = "literal"` single-string-literal declaration in the whole
// package (every file, not just the one currently being checked for a
// violation) -- closes blind spot #2 (see this file's own package doc
// comment): a query built from a shared constant like case_repo.go's
// caseLikeJoins, referenced only by identifier in the file that actually
// runs the query, previously had no matching string literal for the AST
// walk below to find IN THAT FILE at all. Deliberately narrow: only a
// declaration whose entire initializer is one BasicLit string is resolved
// (exactly the shape every real shared query-fragment constant in this
// package already uses -- see e.g. caseLikeJoins/taskSlaViewJoins/
// accountFromJoins). A declaration built from concatenation or a function
// call is silently skipped rather than guessed at; that's a strictly
// narrower net than before this function existed, never a wider one.
func packageStringConstants(t *testing.T, repoDir string, fset *token.FileSet) map[string]string {
	t.Helper()
	out := map[string]string{}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		t.Fatalf("read %s: %v", repoDir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(repoDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
					continue
				}
				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				out[vs.Names[0].Name] = stringLitValue(lit.Value)
			}
		}
	}
	return out
}

// TestRLSBypassLint_NoRawPoolAgainstAProtectedTable walks every
// internal/repository/*.go source file (not _test.go: test fixtures
// legitimately seed through a raw pool for tables that have no RLS-aware
// helper of their own, e.g. project/account/user in this session's own
// integration test fixtures) and fails if:
//
//   - a non-exempt file both imports pgxpool directly and contains a string
//     literal (or an identifier resolving, via packageStringConstants, to a
//     string declared anywhere else in the package) mentioning one of
//     rlsProtectedTables, or
//   - ANY non-exempt file (regardless of pgxpool import -- this specific
//     check needs none) accesses a selector literally named "pool". Scoped's
//     own pool field (scoped.go) is unexported, so the only way another file
//     in this same package could ever reach it directly is by writing
//     someValue.pool -- confirmed by grep to occur nowhere in this package
//     outside scoped.go itself today, so this rule has no known
//     false-positive risk against the current codebase. Closes blind spot
//     #1: unlike a raw *pgxpool.Pool, reaching this field needs no pgxpool
//     import in the accessing file at all (Go only requires the import in
//     whichever file first names the type; a value already typed via
//     another file's struct definition needs no import to call methods on
//     it), so the file-level importsPgxpool gate alone could never catch it.
func TestRLSBypassLint_NoRawPoolAgainstAProtectedTable(t *testing.T) {
	const repoDir = "."
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		t.Fatalf("read internal/repository: %v", err)
	}

	matchers := rlsTableNameMatchers()
	fset := token.NewFileSet()
	sharedConstants := packageStringConstants(t, repoDir, fset)

	matchesProtectedTable := func(text string) []string {
		var hits []string
		for table, re := range matchers {
			if re.MatchString(text) {
				hits = append(hits, table)
			}
		}
		return hits
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if rlsBypassLintExemptFiles[name] {
			continue
		}

		path := filepath.Join(repoDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		// The .pool selector check applies to every non-exempt file, with
		// or without a pgxpool import -- see this test's own doc comment.
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "pool" {
				t.Errorf(
					"%s accesses a selector named %q -- Scoped's own pool field is unexported specifically so "+
						"nothing outside scoped.go can reach Postgres without going through Scoped's identity-setting "+
						"methods; this file must not reference it directly",
					name, sel.Sel.Name,
				)
			}
			return true
		})

		if !importsPgxpool(file) {
			continue
		}

		var violations []string
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					violations = append(violations, matchesProtectedTable(stringLitValue(v.Value))...)
				}
			case *ast.Ident:
				if resolved, ok := sharedConstants[v.Name]; ok {
					violations = append(violations, matchesProtectedTable(resolved)...)
				}
			}
			return true
		})

		if len(violations) > 0 {
			t.Errorf(
				"%s imports pgxpool directly AND references RLS-protected table(s) %v (directly or via a shared "+
					"package-level string constant) -- this file must take a *repository.Scoped instead of a raw "+
					"*pgxpool.Pool, the same conversion already done for every other repository backing one of "+
					"these tables",
				name, uniqueSorted(violations),
			)
		}
	}
}

func importsPgxpool(file *ast.File) bool {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		// A repository now holds the db.Pool interface rather than a concrete
		// *pgxpool.Pool, so importing internal/db is the same raw-pool reach
		// that importing pgxpool was.
		if path == "github.com/jackc/pgx/v5/pgxpool" ||
			path == "github.com/wso2-open-operations/cs-tools/entity-service/internal/db" {
			return true
		}
	}
	return false
}

// stringLitValue best-effort decodes a Go string literal's source text
// (raw backtick or interpreted double-quoted) into its actual value. A raw
// string never needs escape processing; strconv.Unquote handles the
// interpreted case. Falls back to the literal source text (still good
// enough for a plain substring/word-boundary search) if neither applies.
func stringLitValue(src string) string {
	if strings.HasPrefix(src, "`") {
		return strings.Trim(src, "`")
	}
	if v, err := strconv.Unquote(src); err == nil {
		return v
	}
	return src
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// TestRLSBypassLintMatchers pins the matchers themselves: the quoted "case"
// form must match real SQL, and a plain table must still respect a word
// boundary so ordinary words do not trip it.
func TestRLSBypassLintMatchers(t *testing.T) {
	m := rlsTableNameMatchers()
	for _, sql := range []string{`SELECT 1 FROM "case" c`, "JOIN \"case\"\n ON x", `UPDATE "case" SET a = 1`} {
		if !m["case"].MatchString(sql) {
			t.Errorf(`case matcher missed %q`, sql)
		}
	}
	if m["comment"].MatchString("FROM commented_out") {
		t.Error("comment matcher must not match a longer identifier")
	}
	if !m["comment"].MatchString("SELECT 1 FROM comment c") {
		t.Error("comment matcher missed a real reference")
	}
}
