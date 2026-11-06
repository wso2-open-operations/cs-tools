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
	"testing"
)

// TestSplitTheBuiltInCatalogue checks that the catalogue splits into parts
// under MaxPartBytes, each but the first starting at a "## " section, that
// join back into it through NewDashboard.
func TestSplitTheBuiltInCatalogue(t *testing.T) {
	parts, err := Split(DashboardEmbedded(), MaxPartBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) < 2 {
		t.Fatalf("%d part, want the catalogue (%d bytes) in several", len(parts), len(DashboardEmbedded()))
	}
	dir := t.TempDir()
	for i, p := range parts {
		if len(p) > MaxPartBytes {
			t.Errorf("part %d is %d bytes, over %d", i+1, len(p), MaxPartBytes)
		}
		if i > 0 && !strings.HasPrefix(p, "## ") {
			t.Errorf("part %d starts %q, want a section", i+1, firstLine(p))
		}
		write(t, filepath.Join(dir, fmt.Sprintf("catalogue-%02d.md", i+1)), p)
	}
	s, err := NewDashboard(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Text() != DashboardEmbedded() {
		t.Error("the parts do not join back into the built-in catalogue")
	}
}

// TestSplitPacksWholeSections checks that sections are packed in order into
// as few parts as fit, and that a text under the limit is one part.
func TestSplitPacksWholeSections(t *testing.T) {
	a, b, c := "# T\n\n"+strings.Repeat("a", 40), "## B\n\n"+strings.Repeat("b", 40), "## C\n\n"+strings.Repeat("c", 40)
	text := a + "\n\n" + b + "\n\n" + c + "\n"

	for name, tc := range map[string]struct {
		max  int
		want []string
	}{
		"all in one":      {len(text), []string{text}},
		"two per part":    {len(a) + 2 + len(b) + 1, []string{a + "\n\n" + b + "\n", c + "\n"}},
		"one per section": {len(b) + 1, []string{a + "\n", b + "\n", c + "\n"}},
	} {
		got, err := Split(text, tc.max)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: parts = %q, want %q", name, got, tc.want)
		}
	}
}

// TestSplitCutsALargeSectionAtSubsectionsThenParagraphs checks the fallback
// cuts: a section too large for one part is cut before its "### "
// subsections, and one of those still too large at its blank lines.
func TestSplitCutsALargeSectionAtSubsectionsThenParagraphs(t *testing.T) {
	p := func(c string) string { return strings.Repeat(c, 30) }
	text := "## A\n\n" + p("a") + "\n\n### A1\n\n" + p("b") + "\n\n### A2\n\n" + p("c") + "\n\n" + p("d") + "\n"

	parts, err := Split(text, 45)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"## A\n\n" + p("a") + "\n",
		"### A1\n\n" + p("b") + "\n",
		"### A2\n\n" + p("c") + "\n",
		p("d") + "\n",
	}
	if strings.Join(parts, "|") != strings.Join(want, "|") {
		t.Errorf("parts = %q, want %q", parts, want)
	}
	for _, part := range parts {
		if len(part) > 45 {
			t.Errorf("part %q is %d bytes, over 45", firstLine(part), len(part))
		}
	}
}

// TestSplitRefusesAParagraphLargerThanAPart checks that a paragraph with no
// blank line to cut at, larger than a part, is an error naming its start.
func TestSplitRefusesAParagraphLargerThanAPart(t *testing.T) {
	_, err := Split("## A\n\nshort\n\n"+strings.Repeat("x", 100)+"\n", 50)
	if !errors.Is(err, errTooLarge) || !strings.Contains(err.Error(), "xxxx") {
		t.Errorf("err = %v, want errTooLarge naming the paragraph", err)
	}
}

// TestSplitRefusesTextThatWouldNotJoinBack checks that a text the parts would
// not reproduce (extra blank lines at a cut, no final newline) is an error.
func TestSplitRefusesTextThatWouldNotJoinBack(t *testing.T) {
	for name, text := range map[string]string{
		"three newlines at a cut": "## A\n\naaaa\n\n\n## B\n\nbbbb\n",
		"no final newline":        "## A\n\naaaa\n\n## B\n\nbbbb",
	} {
		if _, err := Split(text, 16); !errors.Is(err, errNoRoundTrip) {
			t.Errorf("%s: err = %v, want errNoRoundTrip", name, err)
		}
	}
}

// TestTheCommittedPartsAreUpToDate checks that catalogue-parts/, the files
// mounted in Choreo, are exactly what Split makes of catalogue.md now: an edit
// to catalogue.md without regenerating them fails here. Each part is also
// checked against MaxPartBytes, and the directory against NewDashboard.
func TestTheCommittedPartsAreUpToDate(t *testing.T) {
	const fix = "run `make catalogue-parts` in integrations/csm-ai-integration-service and commit catalogue-parts/"
	dir := filepath.Join("..", "..", PartsDir)
	want, err := Split(DashboardEmbedded(), MaxPartBytes)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewDashboard(dir)
	if err != nil {
		t.Fatalf("%v: %s", err, fix)
	}
	got := s.Parts()
	if len(got) != len(want) {
		t.Fatalf("catalogue-parts/ has %v, want %d parts: %s", got, len(want), fix)
	}
	for i, name := range got {
		if name != partName(i) {
			t.Errorf("part %d is %s, want %s: %s", i+1, name, partName(i), fix)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > MaxPartBytes {
			t.Errorf("%s is %d bytes, over %d: %s", name, len(raw), MaxPartBytes, fix)
		}
		if string(raw) != want[i] {
			t.Errorf("%s is out of date with catalogue.md: %s", name, fix)
		}
	}
	if s.Text() != DashboardEmbedded() {
		t.Errorf("catalogue-parts/ does not join back into catalogue.md: %s", fix)
	}
}

// TestWritePartsReplacesOnlyOldParts checks that WriteParts writes the parts
// in order, removes parts a longer catalogue left, and leaves other files.
func TestWritePartsReplacesOnlyOldParts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "parts")
	if _, err := WriteParts(dir, []string{"one\n", "two\n", "three\n"}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "README.md"), "kept")
	write(t, filepath.Join(dir, "catalogue-1.md"), "kept: not a two-digit part name")

	names, err := WriteParts(dir, []string{"uno\n", "dos\n"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "catalogue-01.md,catalogue-02.md" {
		t.Errorf("names = %v", names)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if strings.Join(left, ",") != "README.md,catalogue-01.md,catalogue-02.md,catalogue-1.md" {
		t.Errorf("directory holds %v, want the two new parts and the other files", left)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "catalogue-02.md")); string(raw) != "dos\n" {
		t.Errorf("catalogue-02.md = %q, want the new part", raw)
	}
}

// TestWritePartsFailures checks that too many parts, and a path that cannot
// be a directory, are errors.
func TestWritePartsFailures(t *testing.T) {
	if _, err := WriteParts(t.TempDir(), make([]string, 100)); err == nil {
		t.Error("100 parts: no error")
	}
	file := filepath.Join(t.TempDir(), "a-file")
	write(t, file, "x")
	if _, err := WriteParts(file, []string{"one\n"}); err == nil {
		t.Error("a file as the directory: no error")
	}
	if _, err := WriteParts(filepath.Join(file, "sub"), []string{"one\n"}); err == nil {
		t.Error("a directory under a file: no error")
	}
}
