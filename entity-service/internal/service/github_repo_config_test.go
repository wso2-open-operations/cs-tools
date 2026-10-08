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
	"strings"
	"testing"
	"time"
)

// choreoConfig is wso2-enterprise/choreo's .github/servicenow-config.yml as
// it stands, trimmed to what is read.
const choreoConfig = `
case:
  category: "Issue"                                    # Category (choice) display value
  account: "3587f161c3ba8f10af2f404599013160"          # Account reference sys_id
  announcement_type: "General"
  project: "01b7f961c3ba8f10af2f40459901313d"          # Project reference sys_id
labels:
  validation_passed: "validation-passed"
`

const (
	choreoAccountUUID = "3587f161-c3ba-8f10-af2f-404599013160"
	choreoProjectUUID = "01b7f961-c3ba-8f10-af2f-40459901313d"
)

type fakeFileReader struct {
	files map[string]string // "owner/repo:path" -> content; absent = 404
	err   error
	reads int
}

func (f *fakeFileReader) FileContent(_ context.Context, owner, repo, path string) ([]byte, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	v, ok := f.files[owner+"/"+repo+":"+path]
	if !ok {
		return nil, nil
	}
	return []byte(v), nil
}

func choreoFiles() *fakeFileReader {
	return &fakeFileReader{files: map[string]string{
		"wso2/choreo:" + DefaultGithubRepoConfigPath: choreoConfig,
	}}
}

func TestParseRepoCaseConfig(t *testing.T) {
	cfg, err := parseRepoCaseConfig([]byte(choreoConfig))
	if err != nil {
		t.Fatalf("choreo's own file must parse: %v", err)
	}
	if cfg.AccountID != choreoAccountUUID || cfg.ProjectID != choreoProjectUUID {
		t.Errorf("sys_ids must become the migrated UUIDs, got %+v", cfg)
	}

	cfg, err = parseRepoCaseConfig([]byte("case:\n  account: 3587F161-C3BA-8F10-AF2F-404599013160\n  project: " + choreoProjectUUID + "\n"))
	if err != nil || cfg.AccountID != choreoAccountUUID {
		t.Errorf("a UUID must be accepted and lower-cased, got %+v, %v", cfg, err)
	}

	for name, raw := range map[string]string{
		"no project":    "case:\n  account: 3587f161c3ba8f10af2f404599013160\n",
		"no account":    "case:\n  project: 01b7f961c3ba8f10af2f40459901313d\n",
		"not an id":     "case:\n  account: Choreo\n  project: 01b7f961c3ba8f10af2f40459901313d\n",
		"not yaml":      "case: [unclosed\n",
		"no case block": "labels:\n  x: y\n",
	} {
		if _, err := parseRepoCaseConfig([]byte(raw)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// The repository's own file decides the account and project; the SR is
// created with both, and remembers its repository so outbound can reach it.
// No account_github_repo row is needed.
func TestHandleWebhook_RepoFileMapsAccountAndProject(t *testing.T) {
	repo := &fakeGhRepo{accountProjects: map[string]string{
		choreoAccountUUID + "/" + choreoProjectUUID: "Choreo",
	}}
	mut := &fakeGhMutations{}
	svc := WithGithubRepoConfig(writingSvc(repo, mut, &fakeGhClient{}), choreoFiles(), "")

	out, err := svc.HandleWebhook(context.Background(), crDelivery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Action != "created" {
		t.Fatalf("want created, got %+v", out)
	}
	sr := mut.lastSR
	if sr.AccountID != choreoAccountUUID || sr.ProjectID != choreoProjectUUID {
		t.Errorf("SR must carry the file's account and project, got account %q project %q", sr.AccountID, sr.ProjectID)
	}
	if sr.Owner != "wso2" || sr.Repository != "choreo" {
		t.Errorf("SR must remember its repository, got %q/%q", sr.Owner, sr.Repository)
	}
}

// A repository names both ids itself, so a project that is not the account's
// must not be accepted -- nothing else stops a file pointing an SR at
// another customer's project.
func TestHandleWebhook_RepoFileProjectOfAnotherAccountIsSkipped(t *testing.T) {
	repo := &fakeGhRepo{} // AccountProject confirms nothing
	mut := &fakeGhMutations{}
	svc := WithGithubRepoConfig(writingSvc(repo, mut, &fakeGhClient{}), choreoFiles(), "")

	out, err := svc.HandleWebhook(context.Background(), crDelivery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mut.created != 0 || !strings.Contains(out.Skipped, "is not a project of account") {
		t.Errorf("want skipped for a mismatched project, got created=%d %+v", mut.created, out)
	}
}

func TestHandleWebhook_InvalidRepoFileIsSkipped(t *testing.T) {
	files := &fakeFileReader{files: map[string]string{
		"wso2/choreo:" + DefaultGithubRepoConfigPath: "case:\n  account: 3587f161c3ba8f10af2f404599013160\n",
	}}
	mut := &fakeGhMutations{}
	// A table row exists, but a file that is present yet broken is reported,
	// not silently overridden by it.
	svc := WithGithubRepoConfig(writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{}), files, "")

	out, err := svc.HandleWebhook(context.Background(), crDelivery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mut.created != 0 || !strings.Contains(out.Skipped, "case.project is missing") {
		t.Errorf("want skipped naming the missing key, got created=%d %+v", mut.created, out)
	}
}

// A repository without the file behaves exactly as before: account_github_repo
// answers, and there is no project.
func TestHandleWebhook_NoRepoFileFallsBackToTable(t *testing.T) {
	mut := &fakeGhMutations{}
	svc := WithGithubRepoConfig(writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{}), &fakeFileReader{}, "")

	out, err := svc.HandleWebhook(context.Background(), crDelivery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Action != "created" || mut.lastSR.AccountID != "a1" || mut.lastSR.ProjectID != "" {
		t.Errorf("want the table's account and no project, got %+v / %+v", out, mut.lastSR)
	}
}

// GitHub being unreachable is not "no file": falling back to the table then
// could create the SR under a different account than the file names.
func TestHandleWebhook_RepoFileReadFailureReleasesDelivery(t *testing.T) {
	repo := &fakeGhRepo{mapping: mapped()}
	mut := &fakeGhMutations{}
	svc := WithGithubRepoConfig(writingSvc(repo, mut, &fakeGhClient{}), &fakeFileReader{err: errors.New("502")}, "")

	if _, err := svc.HandleWebhook(context.Background(), crDelivery()); err == nil {
		t.Fatal("want the read failure returned so the delivery can be retried")
	}
	if mut.created != 0 || repo.claimed["d1"] {
		t.Errorf("nothing may be created and the claim must be released, created=%d claimed=%v", mut.created, repo.claimed["d1"])
	}
}

// An issue arrives as several events within seconds; the file is read once.
func TestGithubRepoConfigs_CachesWithinTTL(t *testing.T) {
	files := choreoFiles()
	c := newGithubRepoConfigs(files, "")
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if _, err := c.get(context.Background(), "WSO2", "Choreo"); err != nil {
			t.Fatal(err)
		}
	}
	if files.reads != 1 {
		t.Errorf("want one read within the TTL, got %d", files.reads)
	}
	now = now.Add(githubRepoConfigTTL + time.Second)
	if _, err := c.get(context.Background(), "wso2", "choreo"); err != nil {
		t.Fatal(err)
	}
	if files.reads != 2 {
		t.Errorf("want a re-read after the TTL, got %d reads", files.reads)
	}
}
