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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package filestore

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// rootDir is the top folder in the SFTPGo user's home directory.
	rootDir = "csm"
	// projectDir holds files of known projects; defaultDir holds the rest.
	projectDir = "projects"
	defaultDir = "default"
	// stagingDir holds uploads that are not confirmed yet, one folder each.
	stagingDir = "staging"
	// maxNameBytes caps the filename part of a key.
	maxNameBytes = 150
)

const (
	workItemDir   = "workitems"
	deploymentDir = "deployments"
	// attachmentDir holds a work item's own files, inlineDir its comments' files.
	attachmentDir = "attachments"
	inlineDir     = "inline_attachments"
)

// KeyParts is what a file's final path is built from.
type KeyParts struct {
	// ProjectNumber is empty for a parent with no project.
	ProjectNumber string
	// WorkItemNumber is set for a work item or comment file, DeploymentNumber
	// for a deployment file.
	WorkItemNumber   string
	DeploymentNumber string
	// Inline marks an image embedded in a comment.
	Inline bool
	// AttachmentNumber (WIA…, CA…, DA…) makes the file name unique; Name is the
	// name the person gave the file.
	AttachmentNumber string
	Name             string
}

// Key builds the SFTPGo path of a stored file, without a leading slash. It is
// the layout csm-sync-service's attachment migrator used, so files uploaded now
// sit beside the migrated ones. Folder names are lowercase snake_case:
//
//	csm/projects/<project number>/workitems/<work item number>/attachments/<attachment number>_<name>
//	csm/projects/<project number>/workitems/<work item number>/inline_attachments/<attachment number>_<name>
//	csm/projects/<project number>/deployments/<deployment number>/<attachment number>_<name>
//
// A parent with no project goes under csm/default instead of
// csm/projects/<number>. The attachment number is unique, so two files with the
// same name never share a path.
//
// Keep this in step with the migrator's own Key: a difference would put new
// files in a different shape from migrated ones.
func Key(p KeyParts) string {
	base := path.Join(rootDir, defaultDir)
	if p.ProjectNumber != "" {
		base = path.Join(rootDir, projectDir, folderName(p.ProjectNumber))
	}
	file := strings.ToLower(SanitizeName(p.AttachmentNumber)) + "_" + SanitizeName(p.Name)
	if p.DeploymentNumber != "" {
		return path.Join(base, deploymentDir, folderName(p.DeploymentNumber), file)
	}
	kind := attachmentDir
	if p.Inline {
		kind = inlineDir
	}
	return path.Join(base, workItemDir, folderName(p.WorkItemNumber), kind, file)
}

// StagingDir is the folder one upload is received into, before confirm moves
// the file to its final key. One folder per attachment, so a share on it
// reaches nothing else.
func StagingDir(attachmentID string) string {
	return path.Join(rootDir, stagingDir, folderName(attachmentID))
}

// folderName turns a project or record number into one lowercase snake_case
// folder name: CS0439849 becomes cs0439849, "A-B/c" becomes a_b_c.
func folderName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
			b.WriteByte('_')
		}
	}
	name := strings.TrimRight(b.String(), "_")
	if name == "" {
		return "unknown"
	}
	return shorten(name, maxNameBytes)
}

// SanitizeName makes a file name safe as one path segment. Anything that could
// escape the folder or upset a filesystem becomes "_", and a long name is
// shortened with its extension kept.
func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)

	var b strings.Builder
	for _, r := range name {
		switch {
		case r == utf8.RuneError, unicode.IsControl(r):
			b.WriteRune('_')
		case strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	name = strings.ReplaceAll(b.String(), "..", "_")
	name = strings.Trim(name, " .")
	if name == "" || name == "_" {
		return "file"
	}
	return shorten(name, maxNameBytes)
}

// shorten cuts name to at most limit bytes on a rune boundary, keeping a short
// extension.
func shorten(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	ext := path.Ext(name)
	if len(ext) > 16 {
		ext = ""
	}
	stem := name[:len(name)-len(ext)]
	cut := limit - len(ext)
	for cut > 0 && !utf8.RuneStart(stem[cut]) {
		cut--
	}
	// Cutting can leave a trailing space or dot, which some systems reject.
	stem = strings.TrimRight(stem[:cut], " .")
	if stem == "" {
		stem = "file"
	}
	return stem + ext
}
