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

package repository_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Clone lineage (migration 0282): a create that names clonedFromChangeRequestId
// records it and the detail reads it back as clonedFrom; a source that is not a
// UUID or not a change request is a 400 that creates nothing (and is refused by
// the dual-write pre-flight too); deleting the source clears the link. Same
// harness as the other crFlow tests (CHANGE_REQUEST_TEST_DSN, superuser and csm_app).
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestCloneSourceIntegration -p 1

func (f *crFlow) createCloneOf(source *string) (string, error) {
	typ := domain.ChangeRequestTypeNormal
	g := crFlowGroupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &g, ClonedFromChangeRequestID: source,
	}, crFlowEmail(crFlowCreatorID))
	return resp.ChangeRequest.ID, err
}

func TestChangeRequestCloneSourceIntegration(t *testing.T) {
	f := newCustomerGroupFlow(t)
	source := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	sourceNumber := f.get(source).Number

	t.Run("recorded and read back", func(t *testing.T) {
		upper := strings.ToUpper(" " + source + " ")
		clone, err := f.createCloneOf(&upper)
		if err != nil {
			t.Fatalf("create clone: %v", err)
		}
		got := f.get(clone).ClonedFrom
		if got == nil || got.ID != source || got.Name != sourceNumber {
			t.Fatalf("clonedFrom = %+v, want {%s %s}", got, source, sourceNumber)
		}
		if f.get(source).ClonedFrom != nil {
			t.Fatal("the source reads a clonedFrom of its own")
		}
	})
	t.Run("blank is none", func(t *testing.T) {
		clone, err := f.createCloneOf(sp("  "))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if f.get(clone).ClonedFrom != nil {
			t.Fatal("a blank source recorded a clonedFrom")
		}
	})
	for name, src := range map[string]string{
		"not a uuid":                  "CHG0001234",
		"no such change request":      "00000000-0000-4000-8000-0000000c1001",
		"a work item of another type": crScopeProjectA, // a project id: no change request
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			before := f.crCount()
			_, err := f.createCloneOf(sp(src))
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "clonedFromChangeRequestId") {
				t.Fatalf("err = %v (%T), want a 400 naming clonedFromChangeRequestId", err, err)
			}
			if after := f.crCount(); after != before {
				t.Fatalf("a refused create left %d change requests behind", after-before)
			}
			// The dual-write pre-flight refuses it before the previous system is called.
			if _, err := f.repo.ValidateChangeRequestLinks(f.sys, domain.ChangeRequestLinkSelection{CloneSourceID: sp(src)}); !errors.As(err, &ve) {
				t.Fatalf("ValidateChangeRequestLinks: err = %v, want a 400", err)
			}
		})
	}
	t.Run("deleting the source clears the link", func(t *testing.T) {
		src := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		clone, err := f.createCloneOf(&src)
		if err != nil {
			t.Fatalf("create clone: %v", err)
		}
		f.execSQL(`DELETE FROM work_item WHERE id = $1`, src)
		if got := f.get(clone).ClonedFrom; got != nil {
			t.Fatalf("clonedFrom after the source was deleted = %+v, want none", got)
		}
	})
}
