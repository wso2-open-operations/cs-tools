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

// A plain (non-DB) static check, run by every `go test ./...`, modelled on
// rls_bypass_lint_test.go: it pins that no repository entry point a customer can
// reach touches a change request without asking the visibility rule
// (change_request_visibility.go) -- the same lesson the row-level-security
// series learned, that a rule enforced by remembering to apply it in each query
// is only as good as the next query someone writes.
//
// Two rules, both on the package's non-test source:
//
//  1. Every EXPORTED function or method whose body (string literals, and package
//     string constants it names) reads or writes a change request or its
//     approvals -- FROM / JOIN / UPDATE / INTO change_request, approval_stage,
//     approval_stage_approver, the 'CHANGE_REQUEST' work item type, or the
//     change_request comment reference type -- must either call one of the
//     visibility helpers (crvisGuardCalls) or carry a `// crvis:<reason>` comment
//     in its doc comment or body saying why it need not (internal-only,
//     caller-guarded, system worker). The reason is the review.
//  2. Every method of the interfaces that carry customer reads and writes of
//     change requests (ChangeRequestRepository, CommentRepository,
//     CRNoticeRepository, and the change request methods of
//     ProjectStatsRepository) is classified the same way, whether or not its own
//     body names a table: a method that only delegates to a helper is the shape
//     rule 1 cannot see.
//
// The known blind spot, as in the RLS lint: a table name assembled at run time
// from two literals has no single literal to match. No code here does that, and
// the integration tests (TestChangeRequestVisibilityIntegration_*) are what
// proves the rule holds.
package repository_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// crvisGuardCalls are the calls that apply the visibility rule: the SQL fragment
// builder and the by-id guard (and the two thin wrappers that apply them for the
// comment repository).
var crvisGuardCalls = map[string]bool{
	"requireVisibleChangeRequest": true,
	"andClause":                   true,
	"clause":                      true,
	"requireVisibleReference":     true,
	"hiddenChangeRequestComment":  true,
}

var crvisTouchesChangeRequest = regexp.MustCompile(
	`(?i)\b(FROM|JOIN|UPDATE|INTO)\s+(change_request|approval_stage_approver|approval_stage)\b|'CHANGE_REQUEST'`)

// crvisInterfaces maps an interface to the method-name filter that makes a method
// part of rule 2 ("" = every method).
var crvisInterfaces = map[string]*regexp.Regexp{
	"ChangeRequestRepository": nil,
	"CommentRepository":       nil,
	"CRNoticeRepository":      nil,
	"ProjectStatsRepository":  regexp.MustCompile(`ChangeRequest|OutstandingCounts`),
}

type crvisFunc struct {
	file     string
	recv     string // receiver type name, "" for a plain function
	name     string
	exported bool
	guarded  bool   // calls a visibility helper
	reason   string // the text after `crvis:`, "" when absent
	touches  bool   // names a change request table / type in a literal or constant
}

func (f crvisFunc) id() string {
	if f.recv != "" {
		return f.file + ":" + f.recv + "." + f.name
	}
	return f.file + ":" + f.name
}

// crvisScan parses the package's non-test files into crvisFunc records, and
// returns the method names of every interface in crvisInterfaces.
func crvisScan(t *testing.T) ([]crvisFunc, map[string][]string) {
	t.Helper()
	const repoDir = "."
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		t.Fatalf("read internal/repository: %v", err)
	}
	fset := token.NewFileSet()
	constants := packageStringConstants(t, repoDir, fset)

	var funcs []crvisFunc
	interfaces := map[string][]string{}
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
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					it, ok := ts.Type.(*ast.InterfaceType)
					if !ok {
						continue
					}
					if _, wanted := crvisInterfaces[ts.Name.Name]; !wanted {
						continue
					}
					for _, m := range it.Methods.List {
						for _, n := range m.Names {
							interfaces[ts.Name.Name] = append(interfaces[ts.Name.Name], n.Name)
						}
					}
				}
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				fn := crvisFunc{file: name, name: d.Name.Name, exported: d.Name.IsExported()}
				if d.Recv != nil && len(d.Recv.List) == 1 {
					fn.recv = receiverTypeName(d.Recv.List[0].Type)
				}
				// Comments inside the function and its doc comment.
				var comments []string
				if d.Doc != nil {
					comments = append(comments, d.Doc.Text())
				}
				for _, cg := range file.Comments {
					if cg.Pos() >= d.Pos() && cg.End() <= d.End() {
						comments = append(comments, cg.Text())
					}
				}
				for _, c := range comments {
					if i := strings.Index(c, "crvis:"); i >= 0 {
						reason := strings.TrimSpace(strings.SplitN(c[i+len("crvis:"):], "\n", 2)[0])
						fn.reason = reason
					}
				}
				ast.Inspect(d.Body, func(n ast.Node) bool {
					switch v := n.(type) {
					case *ast.CallExpr:
						switch fun := v.Fun.(type) {
						case *ast.SelectorExpr:
							if crvisGuardCalls[fun.Sel.Name] {
								fn.guarded = true
							}
						case *ast.Ident:
							if crvisGuardCalls[fun.Name] {
								fn.guarded = true
							}
						}
					case *ast.BasicLit:
						if v.Kind == token.STRING && crvisTouchesChangeRequest.MatchString(stringLitValue(v.Value)) {
							fn.touches = true
						}
					case *ast.Ident:
						if resolved, ok := constants[v.Name]; ok && crvisTouchesChangeRequest.MatchString(resolved) {
							fn.touches = true
						}
						if v.Name == "ReferenceTypeChangeRequest" {
							fn.touches = true
						}
					case *ast.SelectorExpr:
						if v.Sel.Name == "ReferenceTypeChangeRequest" {
							fn.touches = true
						}
					}
					return true
				})
				funcs = append(funcs, fn)
			}
		}
	}
	return funcs, interfaces
}

func receiverTypeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StarExpr:
		return receiverTypeName(v.X)
	case *ast.Ident:
		return v.Name
	case *ast.IndexExpr:
		return receiverTypeName(v.X)
	}
	return ""
}

// Rule 1: an exported function that names a change request table applies the
// visibility rule or says, in a `crvis:` comment, why it need not.
func TestChangeRequestVisibilityLint_ExportedEntryPointsApplyTheRule(t *testing.T) {
	funcs, _ := crvisScan(t)
	var unclassified []string
	touching := 0
	for _, fn := range funcs {
		if !fn.exported || !fn.touches {
			continue
		}
		touching++
		if fn.guarded || fn.reason != "" {
			continue
		}
		unclassified = append(unclassified, fn.id())
	}
	if touching == 0 {
		t.Fatal("the lint found no exported function that touches a change request: its matcher is broken")
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Errorf("these exported functions read or write change requests (or their approvals) without applying the customer-visibility rule "+
			"(change_request_visibility.go: andClause / clause / requireVisibleChangeRequest) and without a `// crvis:<reason>` comment saying why they need not:\n  %s\n"+
			"A customer must never reach a change request that was not designated to them: apply the rule, or document why this path cannot be reached by one.",
			strings.Join(unclassified, "\n  "))
	}
}

// Rule 2: every method of the customer-facing interfaces is classified, whether
// or not its own body names a table.
func TestChangeRequestVisibilityLint_EveryInterfaceMethodIsClassified(t *testing.T) {
	funcs, interfaces := crvisScan(t)
	byRecv := map[string]map[string]crvisFunc{}
	for _, fn := range funcs {
		if fn.recv == "" {
			continue
		}
		if byRecv[fn.recv] == nil {
			byRecv[fn.recv] = map[string]crvisFunc{}
		}
		byRecv[fn.recv][fn.name] = fn
	}
	impl := map[string]string{
		"ChangeRequestRepository": "changeRequestRepo",
		"CommentRepository":       "commentRepo",
		"CRNoticeRepository":      "crNoticeRepository",
		"ProjectStatsRepository":  "projectStatsRepo",
	}
	var unclassified []string
	for iface, methods := range interfaces {
		filter := crvisInterfaces[iface]
		if len(methods) == 0 {
			t.Errorf("interface %s has no methods: the lint could not read it", iface)
		}
		for _, m := range methods {
			if filter != nil && !filter.MatchString(m) {
				continue
			}
			fn, ok := byRecv[impl[iface]][m]
			if !ok {
				t.Errorf("%s.%s has no implementation on %s: the lint cannot classify it", iface, m, impl[iface])
				continue
			}
			if !fn.guarded && fn.reason == "" {
				unclassified = append(unclassified, iface+"."+m+" ("+fn.id()+")")
			}
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Errorf("these methods of the change request repositories neither apply the customer-visibility rule nor carry a `// crvis:<reason>` comment:\n  %s",
			strings.Join(unclassified, "\n  "))
	}
	for _, iface := range []string{"ChangeRequestRepository", "CommentRepository", "CRNoticeRepository", "ProjectStatsRepository"} {
		if len(interfaces[iface]) == 0 {
			t.Errorf("interface %s was not found", iface)
		}
	}
}

// The lint's own matcher: the shapes it exists to catch.
func TestChangeRequestVisibilityLintMatchers(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1 FROM change_request cr", "JOIN change_request cr ON cr.id = wi.id", "UPDATE change_request SET state = 'X'",
		"INSERT INTO change_request (id)", "FROM approval_stage ast", "JOIN approval_stage_approver asa", "WHERE wi.type = 'CHANGE_REQUEST'",
	} {
		if !crvisTouchesChangeRequest.MatchString(sql) {
			t.Errorf("the lint missed %q", sql)
		}
	}
	for _, sql := range []string{"SELECT 1 FROM case_escalation", "a change request cannot be found", "FROM change_request_deployment d"} {
		if crvisTouchesChangeRequest.MatchString(sql) {
			t.Errorf("the lint flagged %q", sql)
		}
	}
}
