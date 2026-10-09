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
	"os"
	"strings"
	"testing"
)

// TestDetachedRunTasksAreNotReachable guards a regression the soft detach
// introduced and nothing else would catch.
//
// While detaching was a DELETE, a run's task rows went with it —
// plg_playbook_run_task's foreign key is ON DELETE CASCADE — so a task id from a
// detached run resolved to nothing and both paths below answered "not found".
// Keeping the run as history removes that accident: the task rows survive, and a
// caller holding an id can still read and complete them, writing completed_by
// onto work that is no longer attached to anything.
//
// Asserted against the SQL rather than through a database because the schema
// harness cannot run (the legacy 000NNN migrations fail against an empty
// database, see TestMain), and an un-runnable behavioural test would guard
// nothing at all. Both statements must scope themselves to attached runs.
func TestDetachedRunTasksAreNotReachable(t *testing.T) {
	src, err := os.ReadFile("plg_orgplatform_repo.go")
	if err != nil {
		t.Fatalf("read repository source: %v", err)
	}
	text := string(src)

	for _, fn := range []string{"RunTaskShape", "PatchRunTask"} {
		start := strings.Index(text, "func (r *orgPlatformRepository) "+fn+"(")
		if start < 0 {
			t.Fatalf("%s not found — this test is pinned to a function that no longer exists", fn)
		}
		// Bound the search at the next top-level func so a filter belonging to a
		// later method cannot satisfy this one.
		end := strings.Index(text[start+1:], "\nfunc ")
		body := text[start:]
		if end > 0 {
			body = text[start : start+1+end]
		}
		if !strings.Contains(body, "detached_on IS NULL") {
			t.Errorf("%s does not scope to attached runs: a task of a DETACHED run is still "+
				"readable/writable by id. Join plg_playbook_run and require detached_on IS NULL.", fn)
		}

		// Every UPDATE in the function must carry the guard itself, not just the
		// lookup that precedes it. A read-then-write pair leaves a window: a
		// concurrent detach between the two lands the write on history, and a
		// lock cannot span that gap. The predicate has to travel inside each
		// statement, the same way the detach's own guard does.
		for _, stmt := range updateStatements(body) {
			if !strings.Contains(stmt, "detached_on IS NULL") {
				t.Errorf("%s has an UPDATE that does not re-check the parent run:\n%s\n"+
					"add AND EXISTS (SELECT 1 FROM plg_playbook_run r WHERE r.id = "+
					"plg_playbook_run_task.playbook_run_id AND r.detached_on IS NULL)", fn, stmt)
			}
		}
	}
}

// updateStatements returns each UPDATE … backtick-quoted SQL literal in src,
// from the UPDATE keyword to the end of that literal.
func updateStatements(src string) []string {
	var out []string
	for i := 0; ; {
		u := strings.Index(src[i:], "UPDATE plg_playbook_run_task")
		if u < 0 {
			return out
		}
		u += i
		end := strings.Index(src[u:], "`")
		if end < 0 {
			return append(out, src[u:])
		}
		out = append(out, src[u:u+end])
		i = u + end + 1
	}
}
