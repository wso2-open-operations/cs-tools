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
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// githubIssueTemplateDir holds the repository's issue forms, whose field labels are the only ### headings that start a field.
const githubIssueTemplateDir = ".github/ISSUE_TEMPLATE"

// issueHeadings is the lower-cased field labels of a repository's issue forms; nil means every ### heading starts a field.
type issueHeadings map[string]bool

// standardIssueHeadings are the shared templates' labels, kept so an issue from an older template version still splits on them.
var standardIssueHeadings = []string{
	"Case Type", "ServiceNow Catalog Item", "Catalog Item", "Template Marker", "Change Request Type",
	"Issue Summary", "Detailed Description", "Severity", "Affected Environments", "Environment",
	"Request Details", "Description", "Priority", "Environment Details",
	"Issue Description", "Requested Environments", "System / User Impact", "Impact Description",
	"Implementation Steps", "Rollback Plan", "Emergency Approver",
	"Short Description", "Impact", "Impact Description (Overall)", "Impact Description (Customer)",
	"Affected Component", "Affected Services", "Service Outage/Downtime",
	"Is a maintenance window required or not", "Maintenance Window",
	"Implementation Plan", "Test Plan", "Monitoring Checks",
}

// fieldHeading returns the label a line starts a field with: a column-0 "### <label>" naming a template field not seen yet.
func (h issueHeadings) fieldHeading(line string, seen map[string]bool) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "###")
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	label := strings.TrimSpace(rest)
	key := strings.ToLower(label)
	if !h[key] || seen[key] {
		return "", false
	}
	seen[key] = true
	return label, true
}

// parseIssueFormLabels returns the field labels of one issue form; anything that is not an issue form yields none.
func parseIssueFormLabels(raw []byte) []string {
	var form struct {
		Body []struct {
			Attributes struct {
				Label string `yaml:"label"`
			} `yaml:"attributes"`
		} `yaml:"body"`
	}
	if err := yaml.Unmarshal(raw, &form); err != nil {
		return nil
	}
	var labels []string
	for _, f := range form.Body {
		if l := strings.TrimSpace(f.Attributes.Label); l != "" {
			labels = append(labels, l)
		}
	}
	return labels
}

type issueHeadingsEntry struct {
	headings issueHeadings
	at       time.Time
}

// githubIssueTemplates reads and caches each repository's issue form labels.
type githubIssueTemplates struct {
	files githubFileReader
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[string]issueHeadingsEntry
}

func newGithubIssueTemplates(files githubFileReader) *githubIssueTemplates {
	return &githubIssueTemplates{files: files, ttl: githubRepoConfigTTL, now: time.Now, entries: map[string]issueHeadingsEntry{}}
}

// get returns the repository's headings, nil when it has no issue forms; a failure to reach GitHub is returned and not cached.
func (c *githubIssueTemplates) get(ctx context.Context, owner, repo string) (issueHeadings, error) {
	key := strings.ToLower(owner + "/" + repo)
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && c.now().Sub(e.at) < c.ttl {
		return e.headings, nil
	}

	raw, err := c.files.FileContent(ctx, owner, repo, githubIssueTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("github: list %s in %s/%s: %w", githubIssueTemplateDir, owner, repo, err)
	}
	var listing []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	var labels []string
	if raw != nil && json.Unmarshal(raw, &listing) == nil {
		for _, f := range listing {
			if f.Type != "file" || !(strings.HasSuffix(f.Name, ".yml") || strings.HasSuffix(f.Name, ".yaml")) {
				continue
			}
			form, err := c.files.FileContent(ctx, owner, repo, f.Path)
			if err != nil {
				return nil, fmt.Errorf("github: read %s in %s/%s: %w", f.Path, owner, repo, err)
			}
			labels = append(labels, parseIssueFormLabels(form)...)
		}
	}

	var h issueHeadings
	if len(labels) > 0 {
		h = issueHeadings{}
		for _, l := range append(labels, standardIssueHeadings...) {
			h[strings.ToLower(l)] = true
		}
	}
	c.mu.Lock()
	c.entries[key] = issueHeadingsEntry{headings: h, at: c.now()}
	c.mu.Unlock()
	return h, nil
}

// issueHeadingsFor is the repository's template headings, or nil (every ### heading a field) when they cannot be read.
func (s *githubSyncService) issueHeadingsFor(ctx context.Context, owner, repo string) issueHeadings {
	if s.issueTemplates == nil {
		return nil
	}
	h, err := s.issueTemplates.get(ctx, owner, repo)
	if err != nil {
		slog.WarnContext(ctx, "github: issue templates unreadable; every ### heading is read as a field", "repository", owner+"/"+repo, "error", err)
		return nil
	}
	return h
}
