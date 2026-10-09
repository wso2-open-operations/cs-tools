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

package service

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestAttributedWritesRefuseAMissingActor pins the rule validateActor exists for.
//
// An omitted actorId is not a loud failure on its own: it reaches uuidArg as "",
// which becomes a NULL column, so the write SUCCEEDS and records nobody. That is
// the worst possible outcome for an audit column — it looks like working
// attribution until someone asks who did something and the answer is blank.
//
// The cases below are constructed by hand, so they only cover writes someone
// remembered to add. That is not enough on its own: DetachRun went unguarded
// while this test passed, because nothing here knew DetachRun existed. The
// source-derived test underneath is what actually holds the rule; these cases
// prove the behaviour the rule is for.
func TestAttributedWritesRefuseAMissingActor(t *testing.T) {
	const notAUUID = "jane.doe@example.com"

	cases := []struct {
		name string
		call func(actor string) error
	}{
		{"playbook create", func(a string) error {
			_, err := (&playbookService{}).Create(context.Background(), domain.CreatePlaybookRequest{
				ProductCode: "IAM", Name: "x",
				LifecycleStage: domain.StageActivated, PlaybookType: domain.PlaybookRecovery,
			}, a)
			return err
		}},
		{"playbook patch", func(a string) error {
			_, err := (&playbookService{}).Patch(context.Background(), domain.PatchPlaybookRequest{
				ID: "11111111-1111-1111-1111-111111111111",
			}, a)
			return err
		}},
		{"playbook replace tasks", func(a string) error {
			_, err := (&playbookService{}).ReplaceTasks(context.Background(), domain.ReplacePlaybookTasksRequest{
				PlaybookID: "11111111-1111-1111-1111-111111111111",
			}, a)
			return err
		}},
	}

	for _, c := range cases {
		for _, actor := range []string{"", notAUUID} {
			label := "empty"
			if actor != "" {
				label = "not a uuid"
			}
			t.Run(c.name+"/"+label, func(t *testing.T) {
				err := c.call(actor)
				if err == nil {
					t.Fatalf("an attributed write accepted actorId %q — it would record nobody", actor)
				}
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want a ValidationError the caller can act on, got %T: %v", err, err)
				}
			})
		}
	}
}

// TestEveryAttributedWriteValidatesItsActor enumerates the writes from the
// source instead of from memory.
//
// The table above is hand-maintained, and a hand-maintained list cannot fail
// for a write nobody added to it — which is exactly how DetachRun shipped
// taking an actorID, writing it to detached_by, and never checking it. The
// request returned 200 and the row recorded nobody.
//
// So: anything whose signature ends in `actorID string` is an attributed write
// by definition, because it has an actor to record. Each one must call
// validateActor before it reaches the repository.
func TestEveryAttributedWriteValidatesItsActor(t *testing.T) {
	// Signature first, then the body up to the closing brace at column 0.
	fn := regexp.MustCompile(`(?m)^func \((\w+) \*(\w+)\) (\w+)\(ctx context\.Context[^)]*actorID string\)`)

	found := 0
	for _, path := range []string{
		"plg_pairing_writes.go", "plg_playbook_writes.go", "plg_organization_service.go",
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		for _, loc := range fn.FindAllStringSubmatchIndex(text, -1) {
			name := text[loc[6]:loc[7]]
			end := strings.Index(text[loc[1]:], "\n}\n")
			if end < 0 {
				t.Fatalf("%s: could not find the end of %s", path, name)
			}
			body := text[loc[1] : loc[1]+end]
			found++

			if !strings.Contains(body, "validateActor(actorID)") {
				t.Errorf("%s.%s takes an actorID and never validates it — an omitted "+
					"actorId becomes a NULL column and the write still returns 200, "+
					"recording nobody", path, name)
				continue
			}
			// Validation has to happen before the repository call, or the row is
			// already written by the time the request is refused.
			if v, r := strings.Index(body, "validateActor(actorID)"), strings.Index(body, "s.repo."); r >= 0 && v > r {
				t.Errorf("%s.%s validates the actor AFTER calling the repository — "+
					"the write has already happened", path, name)
			}
		}
	}

	// Without this the test passes silently if the regex ever stops matching.
	if found < 10 {
		t.Fatalf("only %d attributed writes found; the scan is matching too little to "+
			"be holding anything", found)
	}
	t.Logf("checked %d attributed writes", found)
}
