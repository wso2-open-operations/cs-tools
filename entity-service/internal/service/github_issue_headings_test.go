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
	"sort"
	"strings"
	"testing"
)

// nestedHeadingsBody is a change request whose Description, Implementation Plan and Test Plan carry their own ### headings, one indented.
const nestedHeadingsBody = `### Change Request Type

Normal Change

### Priority

High

### Short Description

Add the trace publisher secret

### Description

### Task

Add the property to vault.

### Notes

Never in Git.

### Implementation Plan

### 1. Generate the secret

  openssl rand -hex 32

  ### 2. Add it to vault

  Alongside the existing properties.

### Test Plan

### Priority

Check the synced keys.

### Customer Reference

ACME-42`

// nestedHeadings is the shared templates' labels plus one custom field.
func nestedHeadings() issueHeadings {
	h := issueHeadings{}
	for _, l := range append([]string{"Customer Reference"}, standardIssueHeadings...) {
		h[strings.ToLower(l)] = true
	}
	return h
}

// fieldKeys lists a field map's keys in order.
func fieldKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestExtractTemplateFields_OnlyTemplateHeadingsSplit(t *testing.T) {
	f := extractTemplateFields(nestedHeadingsBody, nestedHeadings())
	want := "u_change_request_type,u_customer_reference,u_description,u_implementation_plan,u_priority,u_short_description,u_test_plan"
	if got := strings.Join(fieldKeys(f), ","); got != want {
		t.Fatalf("fields = %s\nwant     %s", got, want)
	}
	for key, want := range map[string]string{
		"u_description":         "### Task\n\nAdd the property to vault.\n\n### Notes\n\nNever in Git.",
		"u_implementation_plan": "### 1. Generate the secret\n\n  openssl rand -hex 32\n\n  ### 2. Add it to vault\n\n  Alongside the existing properties.",
		"u_test_plan":           "### Priority\n\nCheck the synced keys.",
		"u_priority":            "High",
		"u_customer_reference":  "ACME-42",
	} {
		if got := f[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestExtractTemplateFields_WithoutTemplatesSplitsOnEveryHeading(t *testing.T) {
	f := ExtractTemplateFields(nestedHeadingsBody)
	if _, ok := f["u_task"]; !ok {
		t.Errorf("without templates a nested heading should still be a field, as before: %v", fieldKeys(f))
	}
}

func TestExtractSNIssueFields_OnlyTemplateHeadingsSplit(t *testing.T) {
	f := extractSNIssueFields(nestedHeadingsBody, nestedHeadings())
	var keys []string
	for _, e := range f.order {
		keys = append(keys, e.key)
	}
	if got, want := strings.Join(keys, ","), "u_change_request_type,priority,u_short_description,description,u_implementation_plan,u_test_plan,u_customer_reference"; got != want {
		t.Fatalf("keys = %s\nwant   %s", got, want)
	}
	if got, _ := f.get("description"); !strings.Contains(got, "Add the property to vault.") || !strings.Contains(got, "Never in Git.") {
		t.Errorf("description = %q, want the text under its nested headings", got)
	}
	if got, _ := f.get("priority"); got != "2 - High" {
		t.Errorf("priority = %q, want the field's own value, not the Test Plan's repeat", got)
	}
}

const sampleIssueForm = `name: SR Change
body:
  - type: markdown
    attributes:
      value: "Fill this in"
  - type: textarea
    id: description
    attributes:
      label: Description
  - type: input
    id: ref
    attributes:
      label: "Customer Reference"
`

func TestGithubIssueTemplates_ReadsTheRepositorysForms(t *testing.T) {
	files := &fakeFileReader{files: map[string]string{
		"wso2/choreo:.github/ISSUE_TEMPLATE":               `[{"name":"sr-change.yml","path":".github/ISSUE_TEMPLATE/sr-change.yml","type":"file"},{"name":"README.md","path":".github/ISSUE_TEMPLATE/README.md","type":"file"}]`,
		"wso2/choreo:.github/ISSUE_TEMPLATE/sr-change.yml": sampleIssueForm,
	}}
	c := newGithubIssueTemplates(files)
	h, err := c.get(context.Background(), "wso2", "choreo")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, l := range []string{"customer reference", "description", "priority"} {
		if !h[l] {
			t.Errorf("headings lack %q", l)
		}
	}
	reads := files.reads
	if _, err := c.get(context.Background(), "WSO2", "Choreo"); err != nil || files.reads != reads {
		t.Errorf("second get read GitHub again (reads %d -> %d, err %v)", reads, files.reads, err)
	}
}

func TestGithubIssueTemplates_NoFormsMeansEveryHeading(t *testing.T) {
	h, err := newGithubIssueTemplates(&fakeFileReader{}).get(context.Background(), "wso2", "choreo")
	if err != nil || h != nil {
		t.Errorf("headings = %v, err = %v, want nil and nil", h, err)
	}
}

func TestGithubIssueTemplates_MalformedListingIsAnErrorNotCached(t *testing.T) {
	files := &fakeFileReader{files: map[string]string{"wso2/choreo:.github/ISSUE_TEMPLATE": `{"type":"file"}`}}
	c := newGithubIssueTemplates(files)
	if _, err := c.get(context.Background(), "wso2", "choreo"); err == nil {
		t.Fatal("want an error for a listing that is not an array")
	}
	files.files["wso2/choreo:.github/ISSUE_TEMPLATE"] = `[{"name":"sr-change.yml","path":".github/ISSUE_TEMPLATE/sr-change.yml","type":"file"}]`
	files.files["wso2/choreo:.github/ISSUE_TEMPLATE/sr-change.yml"] = sampleIssueForm
	if h, err := c.get(context.Background(), "wso2", "choreo"); err != nil || !h["customer reference"] {
		t.Errorf("after a good listing headings = %v, err = %v; the bad one must not have been cached", h, err)
	}
}

func TestGithubIssueTemplates_UnreachableIsNotCached(t *testing.T) {
	files := &fakeFileReader{err: errors.New("github down")}
	c := newGithubIssueTemplates(files)
	if _, err := c.get(context.Background(), "wso2", "choreo"); err == nil {
		t.Fatal("want the error")
	}
	files.err = nil
	files.files = map[string]string{
		"wso2/choreo:.github/ISSUE_TEMPLATE":               `[{"name":"sr-change.yml","path":".github/ISSUE_TEMPLATE/sr-change.yml","type":"file"}]`,
		"wso2/choreo:.github/ISSUE_TEMPLATE/sr-change.yml": sampleIssueForm,
	}
	if h, err := c.get(context.Background(), "wso2", "choreo"); err != nil || !h["customer reference"] {
		t.Errorf("after recovery headings = %v, err = %v", h, err)
	}
}

func TestHandleWebhook_NestedHeadingsStayInTheirField(t *testing.T) {
	files := &fakeFileReader{files: map[string]string{
		"wso2/choreo:.github/ISSUE_TEMPLATE":               `[{"name":"sr-change.yml","path":".github/ISSUE_TEMPLATE/sr-change.yml","type":"file"}]`,
		"wso2/choreo:.github/ISSUE_TEMPLATE/sr-change.yml": sampleIssueForm,
	}}
	m := &fakeGhMutations{}
	svc := WithGithubRepoConfig(writingSvc(&fakeGhRepo{mapping: mapped()}, m, &fakeGhClient{}), files, "")
	d := ghDelivery("issues", "labeled", "[CR]: add the secret", []string{"CR/NormalChange"})
	d.Payload.Issue.Body = nestedHeadingsBody
	out, err := svc.HandleWebhook(context.Background(), d)
	if err != nil || out.Action != "created" {
		t.Fatalf("outcome = %+v, err = %v", out, err)
	}
	if _, ok := m.lastSR.Fields["u_task"]; ok {
		t.Errorf("a nested heading became a field: %v", fieldKeys(m.lastSR.Fields))
	}
	if !strings.Contains(m.lastSR.Fields["u_description"], "Add the property to vault.") {
		t.Errorf("u_description = %q", m.lastSR.Fields["u_description"])
	}
}
