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

// Package catalogue holds the catalogue the model is given as its system
// prompt: catalogue.md, for a dashboard definition (POST
// /ai/dashboards/generate). It tells the model what it may use, and is the
// only thing about the portal the model sees besides the user's prompt, the
// draft being edited and the team list.
//
// It ships inside the binary (embedded at build time), so it is changed and
// reviewed like code and a deployment without a mount still works. It can be
// overridden by a directory of Markdown files, named by CATALOGUE_DIR: in
// Choreo, one file mount per part (Configs & Secrets), all under the same
// directory, since Choreo limits the size of each mounted file. The parts are
// read in name order and joined into one catalogue, so name them to sort
// (catalogue-01.md, catalogue-02.md, ...). The parts to mount are generated
// from catalogue.md into catalogue-parts/ (Split, `make catalogue-parts`):
// after editing catalogue.md, regenerate and commit them, and update the
// mounts, which nothing updates for you (CLAUDE.md, "Catalogue").
//
// The directory is read once, when the service starts (a missing directory, a
// directory with no parts and an empty part each fail the start, so a
// mistyped path is caught there), and requests use that text with no file
// access. A changed mount applies on the next start; in Choreo, changing a
// mount restarts the pod anyway.
package catalogue

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed catalogue.md
var dashboardEmbedded string

// DashboardEmbedded returns the dashboard catalogue built into the binary.
func DashboardEmbedded() string { return dashboardEmbedded }

// partExt is the extension of a catalogue part; other files are skipped.
const partExt = ".md"

// partTrim is the trailing whitespace trimmed from each part before joining.
const partTrim = " \t\r\n"

// Source is the catalogue's text, from the built-in copy or from the parts
// read from a directory when it was made.
type Source struct {
	text  string
	dir   string   // "" for the built-in copy
	parts []string // the part file names, in the order they were joined
}

// NewDashboard is the dashboard catalogue: the built-in copy when dir is
// empty, else the parts in dir, read now. A directory that cannot be read,
// holds no parts, or holds an empty part is an error.
func NewDashboard(dir string) (Source, error) {
	return newSource("dashboard catalogue", dashboardEmbedded, dir)
}

// newSource is a catalogue source for dir (trimmed; empty means the built-in
// copy). The directory is read here, once: an empty system prompt, or one
// with a part missing, would let the model invent fields, so an empty
// directory or part is an error rather than a fall back to the built-in copy,
// which would hide the mounted one.
//
// The parts are the directory's *.md files, sorted by name, so a part's place
// in the catalogue is its name's. Directories, other files, and names starting
// with "." or "_" are skipped: Kubernetes keeps hidden "..data" entries next
// to mounted files. Each part's trailing whitespace is trimmed and the parts
// are joined with one blank line, so a catalogue split at a blank line joins
// back into the same text.
func newSource(name, embedded, dir string) (Source, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return Source{text: embedded}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Source{}, fmt.Errorf("read %s directory %q: %w", name, dir, err)
	}
	var parts []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_") || !strings.EqualFold(filepath.Ext(n), partExt) {
			continue
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return Source{}, fmt.Errorf("%s directory %q: %w", name, dir, errNoParts)
	}
	sort.Strings(parts)

	texts := make([]string, 0, len(parts))
	for _, n := range parts {
		path := filepath.Join(dir, n)
		raw, err := os.ReadFile(path) // #nosec G304 -- path is deployment configuration (CATALOGUE_DIR), not user input
		if err != nil {
			return Source{}, fmt.Errorf("read %s part %q: %w", name, path, err)
		}
		text := string(raw)
		if strings.TrimSpace(text) == "" {
			return Source{}, fmt.Errorf("%s part %q: %w", name, path, errEmpty)
		}
		texts = append(texts, text)
	}
	return Source{text: joinParts(texts), dir: dir, parts: parts}, nil
}

// Text returns the catalogue: the text read when the source was made.
func (s Source) Text() string { return s.text }

// Origin names where the catalogue comes from, for the startup log line:
// "built-in" or the directory.
func (s Source) Origin() string {
	if s.dir == "" {
		return "built-in"
	}
	return s.dir
}

// Parts are the part file names joined into the catalogue, in order; none for
// the built-in copy. For the startup log line, so the order can be checked.
func (s Source) Parts() []string { return append([]string(nil), s.parts...) }

var (
	errEmpty   = errors.New("file is empty")
	errNoParts = errors.New("no " + partExt + " files")
)
