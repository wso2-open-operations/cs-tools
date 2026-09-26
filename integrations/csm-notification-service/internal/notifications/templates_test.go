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

// TestPlainTextFromHTML is a regression test for a real bug: ServiceNow
// returns case descriptions and comments as rich-text HTML (e.g.
// `<p><span style="white-space: pre-wrap;">some text</span></p>`), and
// escapeMultiline used to HTML-escape that HTML directly, so the source's
// own tags showed up as literal, visible "<p>..." clutter in the rendered
// email instead of just the text.
func TestPlainTextFromHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "servicenow's rich-text wrapper",
			input: `<p><span style="white-space: pre-wrap;">Test comment</span></p>`,
			want:  "Test comment",
		},
		{
			name:  "multiple paragraphs become newlines, not run together",
			input: `<p>First paragraph.</p><p>Second paragraph.</p>`,
			want:  "First paragraph.\nSecond paragraph.",
		},
		{
			name:  "br becomes a newline",
			input: "Line one<br>Line two<br/>Line three",
			want:  "Line one\nLine two\nLine three",
		},
		{
			name:  "heading followed by a paragraph stays separated",
			input: "<h2>Summary</h2><p>Details</p>",
			want:  "Summary\nDetails",
		},
		{
			name:  "plain text with no markup passes through unchanged",
			input: "just plain text, no html here",
			want:  "just plain text, no html here",
		},
		{
			name:  "html entities are decoded",
			input: "<p>Salt &amp; pepper</p>",
			want:  "Salt & pepper",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainTextFromHTML(tt.input); got != tt.want {
				t.Errorf("plainTextFromHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestEscapeMultiline_StripsSourceHTMLThenEscapes verifies the full pipeline
// escapeMultiline actually runs: strip the source's own HTML down to plain
// text first (plainTextFromHTML), then HTML-escape the result and convert
// newlines to <br> — so literal "<p>" tags from the source never survive
// into the rendered output, but something a user actually typed (e.g.
// "<3") still gets safely escaped rather than interpreted.
func TestEscapeMultiline_StripsSourceHTMLThenEscapes(t *testing.T) {
	got := escapeMultiline(`<p><span style="white-space: pre-wrap;">Test comment</span></p>`)
	if strings.Contains(got, "&lt;p&gt;") || strings.Contains(got, "<p>") {
		t.Errorf("escapeMultiline(...) = %q, source's own <p> tag leaked into the output one way or another", got)
	}
	if got != "Test comment" {
		t.Errorf("escapeMultiline(...) = %q, want %q", got, "Test comment")
	}

	// A literal "<" a user actually typed must still come out escaped, not
	// stripped as if it were a real tag with nothing after it.
	got = escapeMultiline("I <3 this")
	if !strings.Contains(got, "&lt;3") {
		t.Errorf("escapeMultiline(%q) = %q, want the literal \"<\" escaped, not stripped", "I <3 this", got)
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
	out := RenderCommentAddedEmail("Jane Doe", "CS0023001", "Something broke", "Working on it", "https://x/comment", "https://x/case")
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
	out := RenderInternalNoteEmail("Jane Doe", "WSO2-1000", "Something broke", "Internal only", "https://x/comment", "https://x/case")
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
			want: []string{"Welcome", "A WSO2 account has been created for you", "verification code"},
			deny: []string{"You already have a WSO2 account", "If you are signing in for the first time"},
		},
		{
			// Identity provisioning disabled: the email must not claim an
			// account was created, nor that one already exists.
			name:   "account state unknown",
			render: RenderProjectContactInvitedNewEmail,
			want:   []string{"Welcome", "Sign in with your email address", "If you are signing in for the first time"},
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
			want: []string{"Your invitation", "Here is your invitation to the project", "Sign in with your email address"},
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
