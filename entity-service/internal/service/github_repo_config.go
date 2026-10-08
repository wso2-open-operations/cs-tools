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
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A repository declares which account and project its issues belong to, in a
// file of its own -- the file ServiceNow's integration already reads.
//
// servicenow_create_case.yml loads case.account and case.project from
// .github/servicenow-config.yml, refuses to run without both, and posts them
// to ServiceNow's "GitHub Integration :: Create Case" API, which sets them on
// the case as given (discovery scripts 75/76). So every GitHub-raised case in
// ServiceNow has a project (31 of 31 over 90 days), and onboarding a
// repository needs nothing on the ServiceNow side. Reading the same file keeps
// both: the repository owns its mapping, and a record raised from it has the
// project its comment emails and visibility depend on.
//
// The values are ServiceNow sys_ids today. Migrated accounts and projects
// keep theirs as their UUID (csm-sync-service's sysid_to_uuid), so a sys_id is
// accepted as is and a CSM UUID works too.

// DefaultGithubRepoConfigPath is where a repository declares its mapping.
const DefaultGithubRepoConfigPath = ".github/servicenow-config.yml"

// githubRepoConfigTTL is how long a repository's file is trusted before it is
// read again. A file is edited rarely; an issue's several events (opened,
// labeled, commented) arrive within seconds and should not each fetch it.
const githubRepoConfigTTL = 5 * time.Minute

// githubFileReader is the slice of *github.Client that reads the file.
type githubFileReader interface {
	FileContent(ctx context.Context, owner, repository, path string) ([]byte, error)
}

// repoCaseConfig is what a repository's file declares.
type repoCaseConfig struct {
	AccountID string
	ProjectID string
}

// parseRepoCaseConfig reads case.account and case.project. Both are
// required, as servicenow_create_case.yml requires them.
func parseRepoCaseConfig(raw []byte) (repoCaseConfig, error) {
	var doc struct {
		Case struct {
			Account string `yaml:"account"`
			Project string `yaml:"project"`
		} `yaml:"case"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return repoCaseConfig{}, fmt.Errorf("not valid YAML: %v", err)
	}
	account, err := recordIDFromConfig("case.account", doc.Case.Account)
	if err != nil {
		return repoCaseConfig{}, err
	}
	project, err := recordIDFromConfig("case.project", doc.Case.Project)
	if err != nil {
		return repoCaseConfig{}, err
	}
	return repoCaseConfig{AccountID: account, ProjectID: project}, nil
}

// recordIDFromConfig accepts a ServiceNow sys_id (32 hex) or a UUID and
// returns the UUID, lower-cased.
func recordIDFromConfig(key, v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "", fmt.Errorf("%s is missing", key)
	}
	v = sysidToUUID(v)
	if !isCanonicalUUID(v) {
		return "", fmt.Errorf("%s is neither a sys_id nor a UUID", key)
	}
	return v, nil
}

// repoConfigEntry is one cached read. Present false means the repository has
// no file; invalid is why a file that exists cannot be used.
type repoConfigEntry struct {
	present bool
	cfg     repoCaseConfig
	invalid string
	at      time.Time
}

// githubRepoConfigs reads and caches each repository's file.
type githubRepoConfigs struct {
	files githubFileReader
	path  string
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[string]repoConfigEntry
}

func newGithubRepoConfigs(files githubFileReader, path string) *githubRepoConfigs {
	if strings.TrimSpace(path) == "" {
		path = DefaultGithubRepoConfigPath
	}
	return &githubRepoConfigs{
		files: files, path: path, ttl: githubRepoConfigTTL, now: time.Now,
		entries: map[string]repoConfigEntry{},
	}
}

// get returns the repository's entry, reading the file when the cached one
// has expired. A failure to reach GitHub is returned and not cached, so the
// next delivery tries again.
func (c *githubRepoConfigs) get(ctx context.Context, owner, repo string) (repoConfigEntry, error) {
	key := strings.ToLower(owner + "/" + repo)
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && c.now().Sub(e.at) < c.ttl {
		return e, nil
	}

	raw, err := c.files.FileContent(ctx, owner, repo, c.path)
	if err != nil {
		return repoConfigEntry{}, fmt.Errorf("github: read %s from %s/%s: %w", c.path, owner, repo, err)
	}
	e = repoConfigEntry{present: raw != nil, at: c.now()}
	if e.present {
		cfg, perr := parseRepoCaseConfig(raw)
		if perr != nil {
			e.invalid = perr.Error()
		} else {
			e.cfg = cfg
		}
	}
	c.mu.Lock()
	c.entries[key] = e
	c.mu.Unlock()
	return e, nil
}

// WithGithubRepoConfig makes the sync read each repository's own mapping from
// path (DefaultGithubRepoConfigPath when empty), falling back to
// account_github_repo for a repository without the file. svc is returned
// unchanged when it is not the service this package builds.
func WithGithubRepoConfig(svc GithubSyncService, files githubFileReader, path string) GithubSyncService {
	if s, ok := svc.(*githubSyncService); ok && files != nil {
		s.repoConfigs = newGithubRepoConfigs(files, path)
	}
	return svc
}

// resolveMapping decides which account (and project) a repository's issues
// belong to. A non-empty skip reason means the repository is not one to act
// on; an error means the answer could not be had and the delivery should be
// retried.
//
// The repository's own file wins: it is what the repository says about
// itself. Without one, account_github_repo answers as it always has -- that
// table is still the allow-list for a repository that has no file.
func (s *githubSyncService) resolveMapping(ctx context.Context, owner, repo string) (*repository.RepoMapping, string, error) {
	if s.repoConfigs != nil {
		e, err := s.repoConfigs.get(ctx, owner, repo)
		if err != nil {
			return nil, "", err
		}
		if e.present {
			if e.invalid != "" {
				return nil, fmt.Sprintf("%s in %s/%s: %s", s.repoConfigs.path, owner, repo, e.invalid), nil
			}
			name, ok, err := s.repo.AccountProject(ctx, e.cfg.AccountID, e.cfg.ProjectID)
			if err != nil {
				return nil, "", err
			}
			if !ok {
				return nil, fmt.Sprintf("%s in %s/%s: project %s is not a project of account %s",
					s.repoConfigs.path, owner, repo, e.cfg.ProjectID, e.cfg.AccountID), nil
			}
			return &repository.RepoMapping{
				AccountID: e.cfg.AccountID, AccountName: name, ProjectID: e.cfg.ProjectID,
				Owner: owner, Repository: repo,
			}, "", nil
		}
	}

	m, err := s.repo.RepoMapping(ctx, owner, repo)
	if err != nil {
		return nil, "", err
	}
	if m == nil {
		return nil, fmt.Sprintf("repository %s/%s is not mapped to an account", owner, repo), nil
	}
	return m, "", nil
}
