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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltInCatalogue(t *testing.T) {
	if text := DashboardEmbedded(); !strings.HasPrefix(text, "# CSM Portal dashboard catalogue") ||
		!strings.Contains(text, "## 1. Output contract") || !strings.Contains(text, "## 2. Dashboard object") {
		t.Errorf("built-in dashboard catalogue starts with %q, want the catalogue", firstLine(text))
	}
}

// TestNoDirIsTheBuiltInCopy checks that an empty or blank directory setting
// uses the built-in catalogue.
func TestNoDirIsTheBuiltInCopy(t *testing.T) {
	for _, dir := range []string{"", "   "} {
		d, err := NewDashboard(dir)
		if err != nil {
			t.Fatalf("NewDashboard(%q): %v", dir, err)
		}
		if d.Origin() != "built-in" || len(d.Parts()) != 0 {
			t.Errorf("origin = %q, parts = %v, want built-in and no parts", d.Origin(), d.Parts())
		}
		if d.Text() != DashboardEmbedded() {
			t.Error("dashboard Text() is not the built-in catalogue")
		}
	}
}

// TestPartsAreJoinedInNameOrder checks that the *.md parts are joined by name,
// with one blank line between them, and that everything else is skipped:
// other files, subdirectories, and names starting with "." or "_".
func TestPartsAreJoinedInNameOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "catalogue-02.md"), "## 2. Second\n\n\n")
	write(t, filepath.Join(dir, "catalogue-01.md"), "# Catalogue\n\n## 1. First\n")
	write(t, filepath.Join(dir, "catalogue-03.MD"), "## 3. Third")
	write(t, filepath.Join(dir, "notes.txt"), "not a part")
	write(t, filepath.Join(dir, "_draft.md"), "not a part")
	write(t, filepath.Join(dir, ".hidden.md"), "not a part")
	if err := os.Mkdir(filepath.Join(dir, "..data"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "..data", "catalogue-00.md"), "not a part")

	s, err := NewDashboard("  " + dir + "  ")
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
	}
	if want := "# Catalogue\n\n## 1. First\n\n## 2. Second\n\n## 3. Third\n"; s.Text() != want {
		t.Errorf("Text() = %q, want %q", s.Text(), want)
	}
	if s.Origin() != dir {
		t.Errorf("Origin() = %q, want the trimmed directory %q", s.Origin(), dir)
	}
	if got := strings.Join(s.Parts(), ","); got != "catalogue-01.md,catalogue-02.md,catalogue-03.MD" {
		t.Errorf("Parts() = %s, want the three parts in name order", got)
	}
}

// TestTheBuiltInCatalogueSplitAtHeadingsJoinsBackUnchanged checks that the
// catalogue split into parts at its "## " headings, as it would be for Choreo,
// joins back into the built-in text.
func TestTheBuiltInCatalogueSplitAtHeadingsJoinsBackUnchanged(t *testing.T) {
	dir := t.TempDir()
	sections := strings.Split(DashboardEmbedded(), "\n\n## ")
	for i, sec := range sections {
		if i > 0 {
			sec = "## " + sec
		}
		write(t, filepath.Join(dir, fmt.Sprintf("catalogue-%02d.md", i+1)), sec)
	}
	s, err := NewDashboard(dir)
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
	}
	if len(s.Parts()) < 2 {
		t.Fatalf("parts = %v, want the catalogue split into several", s.Parts())
	}
	if s.Text() != DashboardEmbedded() {
		t.Error("the joined parts differ from the built-in catalogue")
	}
}

// TestPartsAreReadOnceAtStart checks that an edit or a removal after start is
// not seen: requests use the text read at start.
func TestPartsAreReadOnceAtStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalogue-01.md")
	write(t, path, "first version")
	s, err := NewDashboard(dir)
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
	}
	write(t, path, "edited version")
	if s.Text() != "first version\n" {
		t.Errorf("after an edit Text() = %q, want the text read at start", s.Text())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "first version\n" {
		t.Errorf("after a removal Text() = %q, want the text read at start, not the built-in one", firstLine(s.Text()))
	}
}

// TestABadDirFailsTheStart checks that a missing directory, one with no parts,
// a file in place of a directory, and an empty or unreadable part each fail
// with an error naming the catalogue and the path.
func TestABadDirFailsTheStart(t *testing.T) {
	root := t.TempDir()
	noParts := filepath.Join(root, "no-parts")
	emptyPart := filepath.Join(root, "empty-part")
	badPart := filepath.Join(root, "bad-part")
	for _, d := range []string{noParts, emptyPart, badPart} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(noParts, "readme.txt"), "not a part")
	write(t, filepath.Join(emptyPart, "catalogue-01.md"), "## 1. First")
	write(t, filepath.Join(emptyPart, "catalogue-02.md"), " \n\t")
	// A link to a missing file is listed as a part but cannot be read.
	if err := os.Symlink(filepath.Join(root, "gone.md"), filepath.Join(badPart, "catalogue-01.md")); err != nil {
		t.Fatal(err)
	}
	aFile := filepath.Join(root, "a-file.md")
	write(t, aFile, "a file, not a directory")

	for name, tc := range map[string]struct{ dir, names string }{
		"missing":    {filepath.Join(root, "nope"), filepath.Join(root, "nope")},
		"no parts":   {noParts, noParts},
		"a file":     {aFile, aFile},
		"empty part": {emptyPart, filepath.Join(emptyPart, "catalogue-02.md")},
		"bad part":   {badPart, filepath.Join(badPart, "catalogue-01.md")},
	} {
		if _, err := NewDashboard(tc.dir); err == nil || !strings.Contains(err.Error(), tc.names) || !strings.Contains(err.Error(), "dashboard catalogue") {
			t.Errorf("%s: err = %v, want an error naming the catalogue and %s", name, err, tc.names)
		}
	}
}

// write writes content to path, failing the test on an error.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
