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

package catalogue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxPartBytes is the largest catalogue part Split writes, in bytes. Choreo
// limits the size of a mounted file: about 20 KB, the limit the backend's
// DASHBOARDS_DIR part files are cut to (apps/csm-portal/backend
// internal/dashboard, loadDir). 20,000 is under 20 KB whether that means
// 20,000 or 20,480 bytes.
const MaxPartBytes = 20000

// partSeparators are where Split may cut, in the order it tries them: before
// a "## " section, before a "### " subsection, then at any blank line. Each
// starts with a blank line, so every cut is one the loader's join (one blank
// line between parts) puts back.
var partSeparators = []string{"\n\n## ", "\n\n### ", "\n\n"}

// Split cuts a catalogue into parts of at most maxBytes each, for mounting as
// a CATALOGUE_DIR: the contents of catalogue-01.md, catalogue-02.md, ... in
// order. It cuts before "## " sections and packs whole sections into as few
// parts as fit; a section too large for one part is cut before its "### "
// subsections, then at blank lines. A paragraph larger than maxBytes on its
// own is an error.
//
// The parts join back, as NewDashboard joins them, into text; a text that
// would not (say, one with three newlines before a cut) is an error rather
// than parts that would change the catalogue.
func Split(text string, maxBytes int) ([]string, error) {
	// Every part gets its own final newline, so the text's is not in a piece.
	pieces, err := splitPieces(strings.TrimSuffix(text, "\n"), maxBytes, partSeparators)
	if err != nil {
		return nil, err
	}

	var parts []string
	var current []string
	size := 0
	for _, p := range pieces {
		// Each part ends with one newline; pieces in it are a blank line apart.
		if len(current) > 0 && size+2+len(p) > maxBytes {
			parts = append(parts, strings.Join(current, "\n\n")+"\n")
			current, size = nil, 0
		}
		if len(current) == 0 {
			size = len(p) + 1
		} else {
			size += 2 + len(p)
		}
		current = append(current, p)
	}
	if len(current) > 0 {
		parts = append(parts, strings.Join(current, "\n\n")+"\n")
	}

	if joinParts(parts) != text {
		return nil, errNoRoundTrip
	}
	return parts, nil
}

// splitPieces cuts text at the first of seps it contains, then each piece
// that is still too large (with its trailing newline) at the next ones, and
// returns the pieces in order. A piece no separator can cut is an error.
func splitPieces(text string, maxBytes int, seps []string) ([]string, error) {
	if len(text)+1 <= maxBytes {
		return []string{text}, nil
	}
	if len(seps) == 0 {
		return nil, fmt.Errorf("a paragraph of %d bytes starting %q: %w", len(text), firstLine(text), errTooLarge)
	}
	sep := seps[0]
	if !strings.Contains(text, sep) {
		return splitPieces(text, maxBytes, seps[1:])
	}
	// The heading marker after the blank line belongs to the next piece.
	marker := strings.TrimPrefix(sep, "\n\n")
	var pieces []string
	for i, p := range strings.Split(text, sep) {
		if i > 0 {
			p = marker + p
		}
		sub, err := splitPieces(p, maxBytes, seps[1:])
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, sub...)
	}
	return pieces, nil
}

// joinParts joins part contents the way newSource does.
func joinParts(parts []string) string {
	texts := make([]string, len(parts))
	for i, p := range parts {
		texts[i] = strings.TrimRight(p, partTrim)
	}
	return strings.Join(texts, "\n\n") + "\n"
}

// firstLine is s up to its first newline, for an error message.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

var (
	errTooLarge    = errors.New("larger than one part, with no blank line to cut at")
	errNoRoundTrip = errors.New("the parts would not join back into the catalogue: keep exactly one blank line between paragraphs and end the file with one newline")
)

// PartsDir is where the catalogue's parts are kept, relative to the service's
// root: the files mounted in Choreo, written by `make catalogue-parts`.
const PartsDir = "catalogue-parts"

// partName is the file name of the i-th part (from 0): catalogue-01.md, ...
// Two digits, so the names sort in order.
func partName(i int) string { return fmt.Sprintf("catalogue-%02d%s", i+1, partExt) }

// isPartName reports whether name is one WriteParts writes, so only those are
// replaced and anything else in the directory is left alone.
func isPartName(name string) bool {
	var n int
	_, err := fmt.Sscanf(name, "catalogue-%02d.md", &n)
	return err == nil && n > 0 && name == partName(n-1)
}

// WriteParts writes parts to dir as catalogue-01.md, catalogue-02.md, ...,
// creating dir if needed, and removes any other catalogue-NN.md left in it by
// a longer catalogue, so the directory holds exactly these parts. It returns
// the names written, in order.
func WriteParts(dir string, parts []string) ([]string, error) {
	if len(parts) > 99 {
		return nil, fmt.Errorf("%d catalogue parts: more than two-digit names sort in order", len(parts))
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create %q: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && isPartName(e.Name()) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return nil, fmt.Errorf("remove the old part %q: %w", e.Name(), err)
			}
		}
	}
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = partName(i)
		if err := os.WriteFile(filepath.Join(dir, names[i]), []byte(p), 0o600); err != nil {
			return nil, fmt.Errorf("write %q: %w", names[i], err)
		}
	}
	return names, nil
}
