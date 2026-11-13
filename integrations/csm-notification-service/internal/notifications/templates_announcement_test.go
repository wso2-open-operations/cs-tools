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

package notifications

import (
	"strings"
	"testing"
)

const announcementBulletin = `<p>Kindly refer to the <strong>Security Advisories</strong> section below.</p>
<p>Please note that only versions affected according to our<a style="color: #ff7300 !important; text-decoration: none;" href="https://example.test/policy" target="_blank" rel="noopener noreferrer"> back porting policy</a> are included.</p>
<div style="width: 100%;box-sizing: border-box;padding: 15px;border-radius: 12px;border: 1px solid rgba(128,128,128,0.25);background: linear-gradient(160deg, rgba(255,255,255,0.05) 0%, rgba(255,255,255,0) 60%), rgba(128,128,128,0.06);box-shadow: 0 2px 8px rgba(0,0,0,0.12);">
  <div style="margin: 0;">
    <div><span><strong>WSO2-2026-0001: Advisory</strong></span></div>
    <div>Severity: Medium<br />Note</div>
    <div><a style="color: #1a8038 !important; text-decoration: underline;" href="https://example.test/a.pdf">Download here</a></div>
  </div>
</div>`

func announcementHTML(t *testing.T, in string) string {
	t.Helper()
	out, images := sanitizeAnnouncementRichText(in, &inlineImageBudget{})
	if len(images) != 0 {
		t.Fatalf("unexpected inline images: %d", len(images))
	}
	return out
}

func TestSanitizeAnnouncementRichText_KeepsTheStyledBoxAndSpacing(t *testing.T) {
	out := announcementHTML(t, announcementBulletin)

	for _, want := range []string{
		`<p style="margin:0 0 12px;">Kindly refer to the <strong>Security Advisories</strong> section below.</p>`,
		// the link keeps its colour, and the source's target/rel are still dropped
		`<a href="https://example.test/policy" style="color:#ff7300 !important;text-decoration:none;"> back porting policy</a>`,
		// the box keeps its border, radius, padding and shading
		`border-radius:12px;`,
		`border:1px solid rgba(128,128,128,0.25);`,
		`padding:15px;`,
		`background:linear-gradient(160deg, rgba(255,255,255,0.05) 0%, rgba(255,255,255,0) 60%), rgba(128,128,128,0.06);`,
		`box-shadow:0 2px 8px rgba(0,0,0,0.12);`,
		// a line break the author typed survives
		`Severity: Medium<br>Note`,
		`<strong>WSO2-2026-0001: Advisory</strong>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q\n got: %s", want, out)
		}
	}
	for _, unwanted := range []string{"target=", "rel=", "box-sizing", "<script"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output must not contain %q\n got: %s", unwanted, out)
		}
	}
}

func TestSanitizeAnnouncementRichText_BlocksCloseAsRealBlocks(t *testing.T) {
	got := announcementHTML(t, `<div style="padding:4px"><p>one</p><p>two</p></div>`)
	want := `<div style="padding:4px;"><p style="margin:0 0 12px;">one</p><p style="margin:0 0 12px;">two</p></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizeAnnouncementRichText_StyleAllowList(t *testing.T) {
	tests := []struct {
		name  string
		style string
		want  string
	}{
		{"allowed properties pass", "color: red; padding: 4px 8px", "color:red;padding:4px 8px;"},
		{"unknown properties are dropped", "position: fixed; top: 0; color: red", "color:red;"},
		{"url() is refused", "background: url(https://evil.test/x.png)", ""},
		{"url() with a different case is refused", "background: URL(https://evil.test/x.png)", ""},
		{"a relative url() is refused", "background: url(logo.png)", ""},
		{"image-set() is refused", "background: image-set(logo 1x)", ""},
		{"the word javascript is refused even without a colon", "color: javascript", ""},
		{"behavior is refused", "width: behavior", ""},
		{"expression() is refused", "width: expression(alert(1))", ""},
		{"javascript is refused", "color: javascript:alert(1)", ""},
		{"quotes cannot break out of the attribute", `color: red" onmouseover="alert(1)`, ""},
		{"angle brackets are refused", "color: red<script>", ""},
		{"a backslash escape is refused", `color: \72 ed`, ""},
		{"var() is refused", "color: var(--x)", ""},
		{"a declaration without a colon is ignored", "color red; padding: 2px", "padding:2px;"},
		{"an over-long value is refused", "color: " + strings.Repeat("a", 300), ""},
		{"a gradient background passes", "background: linear-gradient(90deg, #fff 0%, #000 100%)", "background:linear-gradient(90deg, #fff 0%, #000 100%);"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeInlineStyle(tt.style); got != tt.want {
				t.Errorf("safeInlineStyle(%q) = %q, want %q", tt.style, got, tt.want)
			}
		})
	}
}

func TestSanitizeAnnouncementRichText_KeepsEverySecurityRule(t *testing.T) {
	out := announcementHTML(t, `<a style="color:red" href="javascript:alert(1)">click</a>`+
		`<a href="https://example.test/ok" onclick="alert(1)" class="x">ok</a>`+
		`<script>alert(1)</script><img src="https://evil.test/p.png"><iframe src="https://evil.test"></iframe>`+
		`<div onclick="alert(1)" style="color:blue">x</div>`)

	if strings.Contains(out, "javascript:") || strings.Contains(out, "onclick") || strings.Contains(out, "class=") {
		t.Errorf("unsafe attribute survived: %s", out)
	}
	if strings.Contains(out, "<script") || strings.Contains(out, "<img") || strings.Contains(out, "<iframe") {
		t.Errorf("unsafe element survived: %s", out)
	}
	if !strings.Contains(out, `<a href="https://example.test/ok">ok</a>`) {
		t.Errorf("a safe link lost: %s", out)
	}
	if !strings.Contains(out, `<div style="color:blue;">x</div>`) {
		t.Errorf("an allowed style was lost: %s", out)
	}
}

func TestSanitizeAnnouncementRichText_UnclosedTagsAreStillClosed(t *testing.T) {
	out := announcementHTML(t, `<div style="padding:4px"><p>open`)
	if !strings.HasSuffix(out, "</p></div>") {
		t.Errorf("unclosed tags must be closed so they cannot swallow the template: %s", out)
	}
}

// The comment, incident and every other email keep the plain rules: no inline
// styles, and paragraphs close as a line break.
func TestSanitizeRichText_StillDropsStyles(t *testing.T) {
	out, _ := sanitizeRichText(announcementBulletin, &inlineImageBudget{})
	for _, unwanted := range []string{"style=", "border-radius", "<p", "<div", "<span"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("plain sanitiser must not emit %q: %s", unwanted, out)
		}
	}
}

func TestRenderCaseCreatedEmail_OnlyAnnouncementsKeepTheirStyles(t *testing.T) {
	data := CaseCreatedEmailData{
		ReporterName: "Reporter", ProjectName: "Project", CaseNumber: "CS0000001", CaseTitle: "Title",
		CreatedAt: "2026-10-13T09:04:16Z", Description: announcementBulletin, CaseLink: "https://x.test/c", CommentLink: "https://x.test/m",
	}

	data.CaseType = "Announcement"
	announcement, _ := RenderCaseCreatedEmail(data)
	if !strings.Contains(announcement, "border-radius:12px;") {
		t.Errorf("an announcement email lost its box styling")
	}

	for _, other := range []string{"Case", "Service Request", "Engagement", "Security Report Analysis"} {
		data.CaseType = other
		got, _ := RenderCaseCreatedEmail(data)
		if strings.Contains(got, "border-radius:12px;") {
			t.Errorf("a %q email must not keep announcement styles", other)
		}
	}
}

func TestSanitizeAnnouncementRichText_TableCellsStayTrimmed(t *testing.T) {
	got := announcementHTML(t, `<table><tr><td><p style="padding:2px">13</p></td></tr></table>`)
	if !strings.Contains(got, ">13</td>") {
		t.Errorf("a cell's paragraph must not leave a block or break inside the cell: %s", got)
	}
	if strings.Contains(got, "<p") {
		t.Errorf("no paragraph element inside a cell: %s", got)
	}
}

func TestHasPresentationStyles(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"plain text", "just text", false},
		{"editor paragraph", `<p dir="ltr"><span style="white-space: pre-wrap;">text</span></p>`, false},
		{"editor aligned paragraph", `<p style="text-align: center;">text</p>`, false},
		{"editor list indent", `<ul style="padding-inline-start: 24px;"><li>x</li></ul>`, false},
		{"a styled box", `<div style="padding:4px">x</div>`, true},
		{"a coloured link", `<a style="color:red" href="https://x.test">x</a>`, true},
		{"only disallowed properties", `<div style="position:fixed;box-sizing:border-box">x</div>`, false},
		{"a style on an unsupported tag", `<section style="padding:4px">x</section>`, false},
		{"unsafe values do not count", `<div style="background:url(x.png)">x</div>`, false},
		{"the bulletin", announcementBulletin, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasPresentationStyles(tt.in); got != tt.want {
				t.Errorf("hasPresentationStyles(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// An announcement typed in the rich editor has no real styling, so its email
// must be exactly what it was before announcement styling existed: identical to
// any other case type's.
func TestRenderCaseCreatedEmail_UnstyledAnnouncementIsUnchanged(t *testing.T) {
	data := CaseCreatedEmailData{
		ReporterName: "Reporter", ProjectName: "Project", CaseNumber: "CS0000001", CaseTitle: "Title",
		CreatedAt: "2026-10-13T09:04:16Z", CaseLink: "https://x.test/c", CommentLink: "https://x.test/m",
		Description: `<p dir="ltr"><span style="white-space: pre-wrap;">One</span></p><p><br></p>` +
			`<p style="text-align: center;"><b><strong style="white-space: pre-wrap;">Two</strong></b></p>` +
			`<ul style="padding-inline-start: 24px;"><li><span>a</span></li></ul>`,
	}
	data.CaseType = "Announcement"
	announcement, _ := RenderCaseCreatedEmail(data)
	data.CaseType = "Case"
	plain, _ := RenderCaseCreatedEmail(data)

	// The only intended differences are the type label and the author-free
	// headline and missing Reported row (see RenderCaseCreatedEmail); compare
	// the rest with whitespace collapsed.
	reportedRow := `<tr> <td style="color:#71717a; width:110px; padding:2px 0; vertical-align:top;">Reported</td> <td style="padding:2px 0;">Reporter</td> </tr>`
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	want := norm(plain)
	want = strings.ReplaceAll(want, reportedRow+" ", "")
	want = strings.ReplaceAll(want, "Reporter <b>created</b> a case", "A new <b>announcement</b> was published")
	want = strings.ReplaceAll(want, ">Case<", ">Announcement<")
	if norm(announcement) != want {
		t.Errorf("an unstyled announcement's email changed beyond the author-free headline:\n announcement: %s\n plain: %s", announcement, plain)
	}
}
