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
	"encoding/base64"
	"strings"
	"testing"
)

// TestSanitizeRichText_StructureAndFormatting is a regression test for a
// real bug: ServiceNow/the portal editors return case descriptions and
// comments as rich-text HTML (e.g.
// `<p><span style="white-space: pre-wrap;">some text</span></p>`), and this
// function must render that structure as safe HTML — never leak the
// source's own tags as literal, visible "<p>..." clutter, and never lose a
// paragraph break by running separate paragraphs into one line.
func TestSanitizeRichText_StructureAndFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "a span wrapper disappears, its text survives",
			input: `<p><span style="white-space: pre-wrap;">Test comment</span></p>`,
			want:  "Test comment",
		},
		{
			name:  "multiple paragraphs become line breaks, not run together",
			input: `<p>First paragraph.</p><p>Second paragraph.</p>`,
			want:  "First paragraph.<br>Second paragraph.",
		},
		{
			name:  "br becomes a line break",
			input: "Line one<br>Line two<br/>Line three",
			want:  "Line one<br>Line two<br>Line three",
		},
		{
			name:  "heading followed by a paragraph stays separated",
			input: "<h2>Summary</h2><p>Details</p>",
			want:  "Summary<br>Details",
		},
		{
			name:  "plain text with no markup passes through unchanged",
			input: "just plain text, no html here",
			want:  "just plain text, no html here",
		},
		{
			name:  "html entities are decoded then re-escaped safely",
			input: "<p>Salt &amp; pepper</p>",
			want:  "Salt &amp; pepper",
		},
		{
			name:  "bold, italic, and underline are preserved",
			input: "<p><b>bold</b> <i>italic</i> <u>underline</u></p>",
			want:  "<b>bold</b> <i>italic</i> <u>underline</u>",
		},
		{
			name:  "a bullet list renders as a real list",
			input: "<ul><li>one</li><li>two</li></ul>",
			want:  "<ul><li>one</li><li>two</li></ul>",
		},
		{
			name:  "an unrecognized tag is dropped, its text kept",
			input: `<table><tr><td>cell text</td></tr></table>`,
			want:  "cell text",
		},
		{
			name:  "a literal < a user actually typed is escaped, not stripped",
			input: "I <3 this",
			want:  "I &lt;3 this",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, images := sanitizeRichText(tt.input, &inlineImageBudget{}); got != tt.want {
				t.Errorf("sanitizeRichText(%q) = %q, want %q", tt.input, got, tt.want)
			} else if len(images) != 0 {
				t.Errorf("sanitizeRichText(%q) returned %d images, want 0", tt.input, len(images))
			}
		})
	}
}

// TestSanitizeRichText_Links verifies a hyperlink survives only when its
// scheme is one of the safe ones — an unsafe scheme (javascript:) must
// still render the link's own visible text, just not as a clickable tag,
// since a comment author could otherwise smuggle a script URL into an
// email a recipient's mail client might render as clickable.
func TestSanitizeRichText_Links(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "an https link is preserved",
			input: `<a href="https://example.com/case/1">View</a>`,
			want:  `<a href="https://example.com/case/1">View</a>`,
		},
		{
			name:  "a mailto link is preserved",
			input: `<a href="mailto:someone@example.com">Email</a>`,
			want:  `<a href="mailto:someone@example.com">Email</a>`,
		},
		{
			name:  "a javascript: href drops the tag but keeps the text",
			input: `<a href="javascript:alert(1)">click me</a>`,
			want:  "click me",
		},
		{
			name:  "a data: href on a link (not an image) drops the tag too",
			input: `<a href="data:text/html,evil">click me</a>`,
			want:  "click me",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, images := sanitizeRichText(tt.input, &inlineImageBudget{}); got != tt.want {
				t.Errorf("sanitizeRichText(%q) = %q, want %q", tt.input, got, tt.want)
			} else if len(images) != 0 {
				t.Errorf("sanitizeRichText(%q) returned %d images, want 0", tt.input, len(images))
			}
		})
	}
}

// TestSanitizeRichText_Images verifies an inline image extracted from a
// self-contained base64 data URI is never re-embedded as a data: URI in the
// returned HTML — Gmail (and most major webmail clients) strip that on
// render regardless of encoding — but instead comes back as a short
// cid:<contentId> reference, with the decoded image bytes/content type
// returned separately as an InlineImage the caller must attach. An http(s)
// source is dropped entirely (see safeImageDataURI's own doc comment for
// why: it would let the recipient's mail client silently phone home to an
// external server the moment the email is opened).
func TestSanitizeRichText_Images(t *testing.T) {
	const dataURI = "data:image/png;base64,aGVsbG8="
	t.Run("a data:image src becomes a cid: reference, with the image extracted", func(t *testing.T) {
		html, images := sanitizeRichText(`<img src="`+dataURI+`" alt="screenshot">`, &inlineImageBudget{})
		if len(images) != 1 {
			t.Fatalf("got %d images, want 1", len(images))
		}
		img := images[0]
		if img.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want image/png", img.ContentType)
		}
		if string(img.Data) != "hello" {
			t.Errorf("Data = %q, want decoded %q", img.Data, "hello")
		}
		if img.ContentID == "" {
			t.Error("ContentID must not be empty")
		}
		// The surrounding <br>s trimBoundaryBreaks would otherwise leave in
		// place are stripped here because the image is the only content in
		// this input — see the next subtest for the case where they survive.
		want := `<img src="cid:` + img.ContentID + `" alt="screenshot" style="display:block;max-width:100%;height:auto;margin:8px 0;">`
		if html != want {
			t.Errorf("html = %q, want %q", html, want)
		}
		if strings.Contains(html, "data:image") {
			t.Error("returned HTML must never contain the original data: URI")
		}
	})

	// Regression test for a real reported bug: an image inserted after some
	// text rendered BEFORE that text in Outlook's web/desktop client, even
	// though the source order (and every other client) had it correctly
	// after. The image must sit inside its own <br>-bounded block so it can
	// never share an inline line box with adjacent text — see the img
	// branch's own comment in sanitizeRichText for the full reasoning.
	t.Run("an image between two text runs is wrapped in its own line, not left inline", func(t *testing.T) {
		html, images := sanitizeRichText(`<p>before<img src="`+dataURI+`">after</p>`, &inlineImageBudget{})
		if len(images) != 1 {
			t.Fatalf("got %d images, want 1", len(images))
		}
		want := `before<br><img src="cid:` + images[0].ContentID + `" alt="" style="display:block;max-width:100%;height:auto;margin:8px 0;"><br>after`
		if html != want {
			t.Errorf("html = %q, want %q", html, want)
		}
	})

	t.Run("an http(s) src is dropped entirely, no image extracted", func(t *testing.T) {
		html, images := sanitizeRichText(`<img src="https://evil.example.com/tracker.png">`, &inlineImageBudget{})
		if html != "" {
			t.Errorf("html = %q, want empty", html)
		}
		if len(images) != 0 {
			t.Errorf("got %d images, want 0", len(images))
		}
	})

	t.Run("two images in one comment each get their own distinct Content-ID", func(t *testing.T) {
		_, images := sanitizeRichText(`<img src="`+dataURI+`"><img src="`+dataURI+`">`, &inlineImageBudget{})
		if len(images) != 2 {
			t.Fatalf("got %d images, want 2", len(images))
		}
		if images[0].ContentID == images[1].ContentID {
			t.Errorf("both images share ContentID %q, want distinct ids", images[0].ContentID)
		}
	})

	// image/svg+xml is XML, not a raster format, and can carry a <script>
	// tag or an onload= handler some mail clients execute when rendering an
	// inline image — safeImageDataURI's allow-list must reject it even
	// though it's syntactically a well-formed data: URI.
	t.Run("an svg+xml data URI is rejected, not extracted", func(t *testing.T) {
		html, images := sanitizeRichText(`<img src="data:image/svg+xml;base64,aGVsbG8=">`, &inlineImageBudget{})
		if html != "" {
			t.Errorf("html = %q, want empty", html)
		}
		if len(images) != 0 {
			t.Errorf("got %d images, want 0 — image/svg+xml must never be extracted", len(images))
		}
	})

	// An image past maxInlineImageBytes is dropped, not truncated or sent
	// oversized — see that const's own doc comment for why.
	t.Run("an oversized image is dropped, not truncated", func(t *testing.T) {
		big := base64.StdEncoding.EncodeToString(make([]byte, maxInlineImageBytes+1))
		html, images := sanitizeRichText(`<img src="data:image/png;base64,`+big+`">`, &inlineImageBudget{})
		if html != "" {
			t.Errorf("html = %q, want empty", html)
		}
		if len(images) != 0 {
			t.Errorf("got %d images, want 0 — an oversized image must be dropped", len(images))
		}
	})

	// A comment embedding more images than maxInlineImagesPerComment must
	// only ever extract up to that cap, dropping the rest silently.
	t.Run("images past maxInlineImagesPerComment are dropped", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i < maxInlineImagesPerComment+3; i++ {
			sb.WriteString(`<img src="` + dataURI + `">`)
		}
		_, images := sanitizeRichText(sb.String(), &inlineImageBudget{})
		if len(images) != maxInlineImagesPerComment {
			t.Errorf("got %d images, want exactly %d (the cap)", len(images), maxInlineImagesPerComment)
		}
	})

	// maxInlineImageBytes alone doesn't stop several individually-small-enough
	// images from summing past email-service's own request-body limit once
	// base64-re-encoded — maxTotalInlineImageBytes bounds the running total
	// across one comment, not just each image on its own.
	t.Run("a second image that would push the running total past maxTotalInlineImageBytes is dropped", func(t *testing.T) {
		// firstSize is at maxInlineImageBytes itself (allowed on its own);
		// secondSize alone is also within maxInlineImageBytes, but the two
		// combined exceed maxTotalInlineImageBytes, so only the second must
		// be rejected — isolating the total-budget check from the
		// per-image one.
		const firstSize = maxInlineImageBytes
		const secondSize = maxTotalInlineImageBytes - maxInlineImageBytes + 1
		first := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, firstSize))
		second := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, secondSize))
		_, images := sanitizeRichText(`<img src="`+first+`"><img src="`+second+`">`, &inlineImageBudget{})
		if len(images) != 1 {
			t.Fatalf("got %d images, want exactly 1 (the second must be dropped for exceeding the total budget)", len(images))
		}
		if len(images[0].Data) != firstSize {
			t.Errorf("first image size = %d, want %d (must survive unchanged)", len(images[0].Data), firstSize)
		}
	})

	// A caller rendering more than one rich-text field into the same email
	// (RenderCaseCreatedEmail: Description + IncidentImpactDescription;
	// RenderCRPlanDateNoticeEmail: ShortDescription + Description) must
	// share one *inlineImageBudget across both sanitizeRichText calls, or
	// each field could independently max out the same limits, and both
	// fields' attachments still land on the same outgoing email/request.
	t.Run("a shared budget is enforced across two separate sanitizeRichText calls", func(t *testing.T) {
		const firstSize = maxInlineImageBytes
		const secondSize = maxTotalInlineImageBytes - maxInlineImageBytes + 1
		first := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, firstSize))
		second := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, secondSize))

		budget := &inlineImageBudget{}
		_, firstImages := sanitizeRichText(`<img src="`+first+`">`, budget)
		_, secondImages := sanitizeRichText(`<img src="`+second+`">`, budget)

		if len(firstImages) != 1 {
			t.Fatalf("first call: got %d images, want 1", len(firstImages))
		}
		if len(secondImages) != 0 {
			t.Errorf("second call: got %d images, want 0 — it must be rejected against the SHARED budget the first call already spent", len(secondImages))
		}
	})
}

// TestSanitizeRichText_ScriptContentNeverExecutes verifies a <script> tag's
// own body is dropped as a normal unrecognized tag — the tokenizer hands
// its raw-text content back as an ordinary text token, which this function
// HTML-escapes like any other text, so it can only ever render as inert,
// visible text, never as executable markup.
func TestSanitizeRichText_ScriptContentNeverExecutes(t *testing.T) {
	got, _ := sanitizeRichText(`<p>before</p><script>alert(1)</script><p>after</p>`, &inlineImageBudget{})
	if strings.Contains(got, "<script>") {
		t.Errorf("sanitizeRichText(...) = %q, <script> tag survived", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("sanitizeRichText(...) = %q, surrounding text was lost", got)
	}
}

// TestEscapeHTML_EncodesNonASCIIAsNumericEntity verifies the fix for a real
// reported bug: entity-service sometimes returns a display name with a
// trailing marker like "Jane Doe Ⓦ" — a browser renders that raw UTF-8
// rune fine, but the external email-sending service this client calls
// doesn't reliably preserve non-ASCII bytes through its own send path, so it
// arrived as a literal "?" in the recipient's inbox. escapeHTML must convert
// it to a numeric HTML character reference instead, which survives as plain
// ASCII regardless and still renders as "Ⓦ" in any HTML-capable mail client.
func TestEscapeHTML_EncodesNonASCIIAsNumericEntity(t *testing.T) {
	got := escapeHTML("Jane Doe Ⓦ")
	want := "Jane Doe &#9420;"
	if got != want {
		t.Errorf("escapeHTML(...) = %q, want %q", got, want)
	}

	// Ordinary ASCII and the standard HTML-escaped characters must still
	// come out exactly as html.EscapeString alone would produce them.
	got = escapeHTML(`Tom & Jerry <script>`)
	want = "Tom &amp; Jerry &lt;script&gt;"
	if got != want {
		t.Errorf("escapeHTML(...) = %q, want %q", got, want)
	}
}

// TestRenderCommentAddedEmail_UsesCaseNumberNotRawID verifies the "commented
// on" strap line renders caseNumber (a human-readable reference like
// "CS0023001"), not some other, meaningless identifier — a real bug this
// caught: the line used to substitute a raw UUID (project id) there
// instead.
func TestRenderCommentAddedEmail_UsesCaseNumberNotRawID(t *testing.T) {
	out, _ := RenderCommentAddedEmail("Jane Doe", "CS0023001", "Something broke", "Working on it", "https://x/comment", "https://x/case")
	if !strings.Contains(out, "CS0023001") {
		t.Error("rendered email doesn't contain the case number")
	}
}

// TestRenderInternalNoteEmail_NoReplyStrapAndUsesWorkNoteWording verifies
// the internal-note layout doesn't carry RenderCommentAddedEmail's
// "Re: <title>" strap (an internal note isn't "about" the case title the
// way a reply is) and uses "added work note" wording instead of
// "commented on case" — matching an existing internal WSO2-support email
// format recipients (always wso2.com staff) are already used to.
func TestRenderInternalNoteEmail_NoReplyStrapAndUsesWorkNoteWording(t *testing.T) {
	out, _ := RenderInternalNoteEmail("Jane Doe", "WSO2-1000", "Something broke", "Internal only", "https://x/comment", "https://x/case")
	if !strings.Contains(out, "added work note") {
		t.Error("rendered email doesn't use the internal-note wording")
	}
	if strings.Contains(out, "Re: Something broke") {
		t.Error("rendered email carries a \"Re: <title>\" strap, which the internal-note layout must not have")
	}
	if !strings.Contains(out, "WSO2-1000") {
		t.Error("rendered email doesn't contain the case reference")
	}
	if !strings.Contains(out, "Internal only") {
		t.Error("rendered email doesn't contain the note's own content")
	}
}

// TestRenderCaseCreatedEmail_OmitsPriorityAndProductRowsWhenEmpty verifies
// the Priority/Product rows are dropped entirely (not just rendered blank)
// when their data is empty — the real state for every case type but "case"
// (Priority) and for "announcement" specifically (Product, since neither
// concept applies there — see entity-service's own validateCreateCaseRequest).
// A populated case (the "case" type's own shape) must still show both.
func TestRenderCaseCreatedEmail_OmitsPriorityAndProductRowsWhenEmpty(t *testing.T) {
	base := CaseCreatedEmailData{
		ReporterName: "Jane Doe",
		ProjectName:  "Project Zeta",
		CaseNumber:   "CS0023001",
		CaseTitle:    "Something broke",
		CaseType:     "ANNOUNCEMENT",
		CreatedAt:    "2026-01-02",
		Description:  "d",
		CaseLink:     "https://x/case",
		CommentLink:  "https://x/comment",
	}

	t.Run("both empty: neither row renders", func(t *testing.T) {
		out, _ := RenderCaseCreatedEmail(base)
		if strings.Contains(out, ">Priority<") {
			t.Error("rendered email still has a Priority row with no priority data")
		}
		if strings.Contains(out, ">Product<") {
			t.Error("rendered email still has a Product row with no product data")
		}
	})

	t.Run("both set: both rows render", func(t *testing.T) {
		withData := base
		withData.CaseType = "CASE"
		withData.Priority = "High (S2)"
		withData.Product = "WSO2 API Manager"
		out, _ := RenderCaseCreatedEmail(withData)
		if !strings.Contains(out, ">Priority<") || !strings.Contains(out, "High (S2)") {
			t.Error("rendered email is missing the Priority row/value")
		}
		if !strings.Contains(out, ">Product<") || !strings.Contains(out, "WSO2 API Manager") {
			t.Error("rendered email is missing the Product row/value")
		}
	})
}

// TestRenderSeverityChangedEmail_ContainsOldAndNewSeverity verifies both
// severities render, distinctly, in the output — a real bug the analogous
// RenderCommentAddedEmail test above caught for a different placeholder,
// so this checks the same class of mistake can't happen here (e.g. the
// old severity accidentally substituted into the new severity's slot).
func TestRenderSeverityChangedEmail_ContainsOldAndNewSeverity(t *testing.T) {
	out := RenderSeverityChangedEmail("CS0023001", "High (P2)", "Low (P4)", "https://x/case", "https://x/comment")
	if !strings.Contains(out, "CS0023001") {
		t.Error("rendered email doesn't contain the case number")
	}
	if !strings.Contains(out, "High (P2)") {
		t.Error("rendered email doesn't contain the old severity")
	}
	if !strings.Contains(out, "Low (P4)") {
		t.Error("rendered email doesn't contain the new severity")
	}
}

// TestRenderCRApprovalRequestedEmail_IsOneWellFormedDocument is a regression
// test for a real bug: the template was assembled by splicing fragments of
// status_changed.html together and ended up holding the document twice, with a
// truncated "OCTYPE html>" where the second copy began. Every recipient would
// have received two concatenated <html> documents -- invalid markup, and the
// same failure this repo already hit once when debug mode merged two rendered
// bodies into one email.
//
// It also pins the wording, since the spliced copy still said "Updated status
// of ... to ...", offered an "Add Comment" link an email cannot action, and
// called a change request a Case.
func TestRenderCRApprovalRequestedEmail_IsOneWellFormedDocument(t *testing.T) {
	got := RenderCRApprovalRequestedEmail(CRApprovalEmailData{
		Number:        "CHG0031234",
		State:         "ASSESS",
		Audience:      "internal",
		Team:          "Choreo",
		GroupName:     "Devops Approval",
		RequesterName: "Sasmitha",
		ProjectName:   "Acme Cloud",
		Link:          "https://csm.example/operations/change-requests/cr-1",
	})

	if n := strings.Count(got, "<!DOCTYPE"); n != 1 {
		t.Errorf("rendered %d documents, want exactly 1 — a mail client shows only the first", n)
	}
	if n := strings.Count(got, "</html>"); n != 1 {
		t.Errorf("found %d </html>, want exactly 1", n)
	}
	if strings.Contains(got, "OCTYPE html>\n<html") && !strings.Contains(got, "<!DOCTYPE html>\n<html") {
		t.Error("found a truncated doctype — the template was spliced mid-tag")
	}

	// Every placeholder must be substituted. An unresolved one renders as an
	// HTML comment, so it is invisible in a mail client: the recipient just
	// sees a missing word, and no test catches it unless one looks here.
	for _, slot := range []string{
		"[CR_NUMBER]", "[STATE_LABEL]", "[AUDIENCE_LABEL]",
		"[REQUESTER]", "[CR_LINK]", "[CONTEXT_LINE]", "[LOGO_SRC]",
	} {
		if strings.Contains(got, slot) {
			t.Errorf("placeholder %s was never substituted", slot)
		}
	}

	// Wording inherited from status_changed.html, all wrong here.
	for _, wrong := range []string{"status update", "Updated status", "Add Comment", "View Case"} {
		if strings.Contains(got, wrong) {
			t.Errorf("found %q — leftover from the template this was derived from", wrong)
		}
	}

	for _, want := range []string{
		"CHG0031234", "Assess", "Devops Approval", "Sasmitha",
		"View change request",
		"https://csm.example/operations/change-requests/cr-1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered email does not mention %q", want)
		}
	}
}

// TestRenderCRApprovalRequestedEmail_MissingDetail: a change request with no
// opener and no project still renders a sendable notice rather than a sentence
// with a hole in it. This is the common case for a CR created by a sync rather
// than a person.
func TestRenderCRApprovalRequestedEmail_MissingDetail(t *testing.T) {
	got := RenderCRApprovalRequestedEmail(CRApprovalEmailData{
		Number:    "CHG-TEST-0001",
		State:     "ASSESS",
		Audience:  "internal",
		GroupName: "Devops Approval",
		Link:      "https://csm.example/operations/change-requests/cr-1",
	})

	if !strings.Contains(got, "Someone") {
		t.Error("want a neutral subject for the sentence when no requester is known")
	}
	if strings.Contains(got, "Project .") || strings.Contains(got, "owned by .") {
		t.Error("rendered a context line with an empty value in it")
	}
	if !strings.Contains(got, "Open the change request") {
		t.Error("want the fallback context line when neither project nor team is known")
	}
}

// The greeting and the project cell must match what ServiceNow sends, since
// these emails land next to years of the originals.
func TestRenderQueryHourThresholdEmail_GreetingAndProjectCell(t *testing.T) {
	body := RenderQueryHourThresholdEmail(QueryHourThresholdEmailData{
		Subject:         "Query Hour Exceeded in Intrepid Travel",
		OwnerName:       "Ivan Saverus",
		AccountName:     "Intrepid Travel",
		ProjectName:     "Intrepidsub - Subscription",
		ProjectKey:      "INTREPIDSUBSUB",
		State:           3,
		TotalQueryHours: "100h 0m",
		ConsumedHours:   "101h 35m",
		RemainingHours:  "-1h 35m",
		PercentConsumed: 101.58,
	})
	for _, want := range []string{
		"Hi Ivan Saverus,",
		"Intrepidsub - Subscription",
		"Allocated query support hours are exceeded in Intrepid Travel",
		"100h 0m", "101h 35m", "-1h 35m",
	} {
		if !contains(body, want) {
			t.Errorf("rendered email is missing %q", want)
		}
	}
	// No owner on file must not produce an empty greeting.
	fallback := RenderQueryHourThresholdEmail(QueryHourThresholdEmailData{State: 1})
	if !contains(fallback, "Hi Account Manager,") {
		t.Error("with no owner name, the greeting should fall back to \"Hi Account Manager,\"")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && stringsIndex(haystack, needle) >= 0
}

func stringsIndex(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// The confirmed target format, from a real 75% notice. Five columns in this
// exact order, the greeting, the message, and the sign-off — and nothing the
// original does not have.
func TestRenderQueryHourThresholdEmail_MatchesTheConfirmedFormat(t *testing.T) {
	body := RenderQueryHourThresholdEmail(QueryHourThresholdEmailData{
		Subject:         "75% of Query Hours Utilized",
		OwnerName:       "Tissaka Senarath",
		AccountName:     "CHUV (Lausanne University Hospital)",
		ProjectName:     "CHUV - Lausanne University Hospital - Evaluation Subscription",
		ProjectKey:      "CHUVEVAL",
		State:           1,
		TotalQueryHours: "10h 0m",
		ConsumedHours:   "7h 55m",
		RemainingHours:  "2h 5m",
		PercentConsumed: 79.17,
	})
	for _, want := range []string{
		"Hi Tissaka Senarath,",
		"Kindly note that, Allocated 75% of query support hours are utilized in CHUV (Lausanne University Hospital). Query support will be disabled on 100% usage. Therefore, It is advised to notify customers to repurchase additional subscription hours.",
		"CHUV - Lausanne University Hospital - Evaluation Subscription",
		"10h 0m", "7h 55m", "2h 5m",
		"WSO2 Support Administrator",
	} {
		if !contains(body, want) {
			t.Errorf("missing from the rendered email: %q", want)
		}
	}
	// Things the original does NOT contain must not appear.
	for _, unwanted := range []string{
		"Project Key :", // the account-level variant's suffix, not this one
		"% of the allocated query hours have been consumed", // an invention
		"Opportunities",  // seven-column variant only
		"Total Consumed", // seven-column variant only
	} {
		if contains(body, unwanted) {
			t.Errorf("rendered email contains %q, which the ServiceNow original does not", unwanted)
		}
	}
}

// TestRenderProjectContactInvitedEmail_Variants pins what distinguishes the
// three invitation wordings — the "new" one welcomes a just-created account
// and explains the first-sign-in email code, the "existing" one says the
// project was added to an account the reader already has, the "reminder"
// (a deliberate resend) just repeats the invitation — and that all
// carry every value a reader needs (name, project, key, email, sign-in
// link, roles) with no placeholder left unsubstituted.
func TestRenderProjectContactInvitedEmail_Variants(t *testing.T) {
	data := ProjectContactInvitedEmailData{
		DisplayName: "Jane Doe",
		Email:       "jane@acme.com",
		ProjectName: "Acme Cloud",
		ProjectKey:  "ACMECLOUD",
		Roles:       []string{"Admin", "Portal user"},
		PortalURL:   "https://support.wso2.com",
	}
	tests := []struct {
		name       string
		render     func(ProjectContactInvitedEmailData) string
		want, deny []string
	}{
		{
			name: "new account",
			render: func(d ProjectContactInvitedEmailData) string {
				d.AccountCreated = true
				return RenderProjectContactInvitedNewEmail(d)
			},
			want: []string{"We're delighted to inform you that you have been given access to", "A WSO2 account has been created for you", "verification code"},
			deny: []string{"You already have a WSO2 account", "If you are signing in for the first time"},
		},
		{
			// Identity provisioning disabled: the email must not claim an
			// account was created, nor that one already exists.
			name:   "account state unknown",
			render: RenderProjectContactInvitedNewEmail,
			want:   []string{"you have been given access to", "Sign in with your email address", "If you are signing in for the first time"},
			deny:   []string{"A WSO2 account has been created for you", "You already have a WSO2 account"},
		},
		{
			name:   "existing account",
			render: RenderProjectContactInvitedExistingEmail,
			want:   []string{"has been added to your WSO2 Support Portal account", "You already have a WSO2 account"},
			deny:   []string{"A WSO2 account has been created for you", "verification code"},
		},
		{
			// A resend: it must claim neither that an account was just
			// created nor that the reader already has one — they may
			// never have seen the first invitation.
			name: "reminder",
			render: func(d ProjectContactInvitedEmailData) string {
				d.AccountCreated = true
				return RenderProjectContactInvitedReminderEmail(d)
			},
			want: []string{"This is a reminder of your invitation to", "Sign in with your email address"},
			deny: []string{"A WSO2 account has been created for you", "You already have a WSO2 account", "Welcome"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.render(data)
			if n := strings.Count(got, "<!DOCTYPE"); n != 1 {
				t.Errorf("rendered %d documents, want exactly 1", n)
			}
			for _, want := range append(tt.want, "Jane Doe", "Acme Cloud", "ACMECLOUD", "jane@acme.com", "Admin, Portal user", `href="https://support.wso2.com"`) {
				if !strings.Contains(got, want) {
					t.Errorf("rendered email does not contain %q", want)
				}
			}
			for _, deny := range tt.deny {
				if strings.Contains(got, deny) {
					t.Errorf("rendered email contains %q, which belongs to the other variant", deny)
				}
			}
			for _, slot := range []string{"[DISPLAY_NAME]", "[EMAIL]", "[PROJECT_NAME]", "[PROJECT_KEY]", "[ROLES]", "[PORTAL_URL]", "[LOGO_SRC]", "[BLOCK:"} {
				if strings.Contains(got, slot) {
					t.Errorf("placeholder %s was never substituted", slot)
				}
			}
		})
	}
}

// TestRenderProjectContactInvitedEmail_OmitsRolesLineWhenEmpty: a
// membership with no Salesforce roles must not render "Your role: " with
// nothing after it.
func TestRenderProjectContactInvitedEmail_OmitsRolesLineWhenEmpty(t *testing.T) {
	for name, render := range map[string]func(ProjectContactInvitedEmailData) string{
		"new":      RenderProjectContactInvitedNewEmail,
		"existing": RenderProjectContactInvitedExistingEmail,
		"reminder": RenderProjectContactInvitedReminderEmail,
	} {
		got := render(ProjectContactInvitedEmailData{DisplayName: "jane", Email: "jane@acme.com", ProjectName: "Acme Cloud", ProjectKey: "ACMECLOUD", PortalURL: "https://support.wso2.com"})
		if strings.Contains(got, "Your role") {
			t.Errorf("%s: rendered a roles line for a membership with no roles", name)
		}
		if strings.Contains(got, "[BLOCK:") {
			t.Errorf("%s: optional-block markers leaked into the output", name)
		}
	}
}

// TestRenderProjectContactInvitedEmail_SignInButton: every invitation
// variant ends with the orange button (black text) and a copyable fallback link,
// and the portal URL is filled into all three places it appears (the
// button, the fallback href and the fallback's visible text).
func TestRenderProjectContactInvitedEmail_SignInButton(t *testing.T) {
	const portal = "https://portal.example.com/sign-in?next=%2Fprojects"
	for name, render := range map[string]func(ProjectContactInvitedEmailData) string{
		"new":      RenderProjectContactInvitedNewEmail,
		"existing": RenderProjectContactInvitedExistingEmail,
		"reminder": RenderProjectContactInvitedReminderEmail,
	} {
		got := render(ProjectContactInvitedEmailData{DisplayName: "jane", Email: "jane@acme.com", ProjectName: "Acme Cloud", ProjectKey: "ACMECLOUD", PortalURL: portal})
		for _, want := range []string{`bgcolor="#FF6700"`, "background-color:#FF6700", "color:#000000", ">Sign in to Support Portal</a>", "Button not working? Use this link:", "Sign-in email", "mailto:support@wso2.com", "Cheers!<br>The WSO2 Team", "WSO2-Logo-White.png", "WSO2-Pulse-Orange.png"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: rendered email does not contain %q", name, want)
			}
		}
		if n := strings.Count(got, portal); n != 3 {
			t.Errorf("%s: portal URL appears %d times, want 3 (button, fallback href, fallback text)", name, n)
		}
		// Only the grey fallback link is underlined; the button is not.
		if n := strings.Count(got, "text-decoration:underline"); n != 1 {
			t.Errorf("%s: %d underlined links, want 1 (the fallback)", name, n)
		}
	}
}

// TestRenderProjectContactRegisteredEmail: the Welcome email carries the
// greeting, project, video link and portal button, with no placeholder left.
func TestRenderProjectContactRegisteredEmail(t *testing.T) {
	got := RenderProjectContactRegisteredEmail(ProjectContactRegisteredEmailData{
		DisplayName: "Jane <Doe>", ProjectName: "Acme Cloud", ProjectKey: "ACMECLOUD", PortalURL: "https://support.wso2.com",
	})
	for _, want := range []string{
		"Hi Jane &lt;Doe&gt;,", "Welcome to the WSO2 Customer Support Portal", "<b>Acme Cloud</b>", ">ACMECLOUD</td>", "create and manage cases",
		`href="https://youtu.be/1v5SqP6qRLc"`, "Watch the getting-started video",
		`href="https://support.wso2.com"`, ">Go to Support Portal</a>", `bgcolor="#FF6700"`, "color:#000000", "mailto:support@wso2.com", "Cheers!",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("welcome email does not contain %q", want)
		}
	}
	if strings.Contains(got, "Button not working?") {
		t.Error("welcome email should not carry the fallback link line")
	}
	if strings.Contains(got, "<!-- [") || strings.Count(got, "<!DOCTYPE") != 1 {
		t.Error("welcome email has an unsubstituted placeholder or is not one document")
	}
}
