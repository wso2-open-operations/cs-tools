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
	"os"
	"regexp"
	"strings"
	"testing"
)

// piiFields are the PLG values that identify a customer or a person, and so
// must never reach a log line.
//
// Ids are deliberately absent: an id names a row, not a person, and is the
// whole point of this logging. Names, emails and free text are what a reader
// outside the portal must not be handed.
var piiFields = []string{
	"OrganizationName", "organizationName",
	"RegisteredEmail", "registeredEmail",
	"Email", "email",
	"FirstName", "LastName", "DisplayName", "displayName",
	"Body", "body", // note text
	"TextValue", "textValue", // free-text task answers
	"Reason", "reason", // an engineer's own words on an axis change
}

// auditCall matches one auditWrite(...) call and captures its arguments.
var auditCall = regexp.MustCompile(`(?s)auditWrite\(r,\s*"[^"]*",\s*(?:nil|err),(.*?)\)\n`)

// TestAuditLogsCarryNoPII is the enforcement for a rule that otherwise survives
// only as long as the next person remembers it.
//
// These lines are written on every PLG write and read by anyone with access to
// the service's output, which is a far wider audience than the database. A
// customer's name or an engineer's email in that stream is a disclosure that no
// amount of care at review time reliably prevents — so the rule is a test.
//
// It reads the real call sites rather than exercising them: the point is to fail
// when somebody ADDS a field, and a behavioural test only covers the paths it
// happens to drive.
func TestAuditLogsCarryNoPII(t *testing.T) {
	src, err := os.ReadFile("handlers.go")
	if err != nil {
		t.Fatalf("read handlers.go: %v", err)
	}

	calls := auditCall.FindAllStringSubmatch(string(src), -1)
	if len(calls) == 0 {
		t.Fatal("found no auditWrite calls — the pattern is wrong, not the code")
	}

	for _, c := range calls {
		args := c[1]
		for _, field := range piiFields {
			// Match the field as a whole word so "reason" does not fire on
			// "reasonCode" and "body" does not fire on "bodyCount".
			if regexp.MustCompile(`\b` + regexp.QuoteMeta(field) + `\b`).MatchString(args) {
				t.Errorf("an audit log carries %q, which identifies a customer or a person:\n  %s\n"+
					"log the id instead — these lines are far more widely readable than the database",
					field, strings.TrimSpace(args))
			}
		}
	}
	t.Logf("checked %d audit call sites", len(calls))
}

// writeHandlers maps each audited operation to the handler that must contain it.
var writeHandlers = map[string]string{
	"set owner": "PatchOrganization", "patch pairing": "PatchProduct",
	"attach playbook": "AttachPlaybook", "create note": "CreateNote",
	"detach run": "DetachRun", "patch run task": "PatchRunTask",
	"patch note": "PatchNote", "acknowledge": "Acknowledge",
	"create playbook": "CreatePlaybook", "patch playbook": "PatchPlaybook",
	"replace tasks": "ReplacePlaybookTasks", "delete playbook": "DeletePlaybook",
}

// TestAuditLogsSitInTheRightHandler pins which handler each operation is logged
// from, and that every write has one.
//
// This exists because the first version of this instrumentation put both
// "delete playbook" calls inside Dashboard() — a READ handler — and everything
// still compiled, the counts still came to 24, and the PII check still passed.
// The only symptom was a missing line for one operation and two spurious lines
// on an unrelated route, which is the kind of thing that is noticed months later
// when somebody asks who deleted a playbook and the answer is nothing.
//
// A count is not enough. The operation has to be logged from the handler that
// performs it.
func TestAuditLogsSitInTheRightHandler(t *testing.T) {
	src, err := os.ReadFile("handlers.go")
	if err != nil {
		t.Fatalf("read handlers.go: %v", err)
	}
	text := string(src)

	type fn struct {
		at   int
		name string
	}
	var funcs []fn
	for _, m := range regexp.MustCompile(`func \(h \*Handlers\) (\w+)\(`).FindAllStringSubmatchIndex(text, -1) {
		funcs = append(funcs, fn{at: m[0], name: text[m[2]:m[3]]})
	}

	seen := map[string]int{}
	for _, m := range regexp.MustCompile(`auditWrite\(r, "([^"]+)"`).FindAllStringSubmatchIndex(text, -1) {
		op := text[m[2]:m[3]]
		owner := ""
		for _, f := range funcs {
			if f.at < m[0] {
				owner = f.name
			}
		}
		want, known := writeHandlers[op]
		switch {
		case !known:
			t.Errorf("audit log for unknown operation %q in %s() — add it to writeHandlers or fix the name", op, owner)
		case owner != want:
			t.Errorf("audit log for %q sits in %s(), but that operation is performed by %s()", op, owner, want)
		}
		seen[op]++
	}

	for op, want := range writeHandlers {
		switch seen[op] {
		case 2: // the success and the failure path
		case 0:
			t.Errorf("%s() has no audit log: %s is a write and leaves no record", want, op)
		default:
			t.Errorf("%q is logged %d times, want 2 (one success, one failure)", op, seen[op])
		}
	}
}
