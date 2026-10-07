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
	"context"
	"fmt"
	"strings"

	xhtml "golang.org/x/net/html"
)

// The three service-request (SR) cards, posted by dispatch to the SR's SRE
// team space (the sr.* payload's sreTeamName as the audience). The first two
// reproduce ServiceNow's "SR New Request - Acknowledge & Chat Alert" flow's
// cards (built there by SRChatCardUtils); the customer-comment card has no
// ServiceNow original. All three share srThreadKey(caseID), so an SR's
// created -> acknowledged -> comment alerts read as one thread in the space.

// srChatTextLimit is how much of a description or comment an SR card shows:
// ServiceNow's card cut both to 300 characters, and the portal link is there
// for the rest.
const srChatTextLimit = 300

// SRCreatedAlert is SendSRCreatedAlert's card content.
type SRCreatedAlert struct {
	// CaseID keys the card's thread (srThreadKey).
	CaseID     string
	Number     string
	WSO2CaseID string
	Subject    string
	// AssignmentGroupName fills the "Assigned to" subtitle; empty omits it.
	AssignmentGroupName string
	State               string
	// Description is the SR's description as stored, HTML or not -- the card
	// strips it to plain text (srChatPlainText).
	Description string
	CaseLink    string
}

// SRAcknowledgedAlert is SendSRAcknowledgedAlert's card content.
type SRAcknowledgedAlert struct {
	// CaseID keys the card's thread (srThreadKey).
	CaseID     string
	Number     string
	WSO2CaseID string
	// AssignmentGroupName is who the SR awaits review by; SRETeamName stands
	// in when it is empty.
	AssignmentGroupName string
	SRETeamName         string
	CaseLink            string
}

// SRCustomerCommentAlert is SendSRCustomerCommentAlert's card content.
type SRCustomerCommentAlert struct {
	// CaseID keys the card's thread (srThreadKey).
	CaseID     string
	Number     string
	WSO2CaseID string
	Subject    string
	// AuthorName is shown; AuthorEmail only when there is no name.
	AuthorName  string
	AuthorEmail string
	// Content is the comment as stored, stripped to plain text like
	// SRCreatedAlert.Description.
	Content     string
	CommentLink string
}

// SendSRCreatedAlert posts ServiceNow's new-SR card: header "New Service
// Request | <number>" with "Assigned to <group>" under it, then "Short
// Description", "Details" (WSO2 Case ID, State) and a collapsed
// "Description", and a "View Case" button.
//
// The one card in this file with a button rather than a "View case" link:
// ServiceNow's card had one, and this card reproduces that card rather than
// following the case.* cards' own link-only convention.
func (c *GoogleChatClient) SendSRCreatedAlert(ctx context.Context, audience string, a SRCreatedAlert) error {
	if a.Number == "" {
		return fmt.Errorf("notifications: number is required")
	}
	header := &chatCardHeader{Title: "New Service Request | " + a.Number}
	if a.AssignmentGroupName != "" {
		header.Subtitle = "Assigned to " + a.AssignmentGroupName
	}

	sections := []chatCardSection{{
		Header:  "Short Description",
		Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: caseAlertLine(`%s`, orEmDash(a.Subject))}}},
	}}
	var details []string
	if a.WSO2CaseID != "" {
		details = append(details, caseAlertLine(`<b>WSO2 Case ID:</b> %s`, a.WSO2CaseID))
	}
	if a.State != "" {
		details = append(details, caseAlertLine(`<b>State:</b> %s`, a.State))
	}
	if len(details) > 0 {
		sections = append(sections, chatCardSection{
			Header:  "Details",
			Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: strings.Join(details, "<br>")}}},
		})
	}
	if desc := srChatPlainText(a.Description, srChatTextLimit); desc != "" {
		sections = append(sections, chatCardSection{
			Header:      "Description",
			Collapsible: true,
			Widgets:     []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: caseAlertLine(`%s`, desc)}}},
		})
	}
	sections = append(sections, chatCardSection{Widgets: []chatCardWidget{{ButtonList: &chatButtonList{
		Buttons: []chatButton{{Text: "View Case", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: a.CaseLink}}}},
	}}}})

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{{CardID: "sr-created-alert", Card: chatCard{Header: header, Sections: sections}}},
		Thread:  &chatThread{ThreadKey: srThreadKey(a.CaseID, a.Number)},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// SendSRAcknowledgedAlert posts ServiceNow's SR acknowledgement card, as a
// reply under the SR's created card: header "Acknowledged" over
// "<number> / <wso2CaseId>", the fixed acknowledgement line, a "Status"
// section reading "Awaiting review by <group>", then a "View case" link.
func (c *GoogleChatClient) SendSRAcknowledgedAlert(ctx context.Context, audience string, a SRAcknowledgedAlert) error {
	if a.Number == "" {
		return fmt.Errorf("notifications: number is required")
	}
	subtitle := a.Number
	if a.WSO2CaseID != "" {
		subtitle += " / " + a.WSO2CaseID
	}
	sections := []chatCardSection{{
		Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: "An automated acknowledgement has been posted to this case."}}},
	}}
	reviewer := a.AssignmentGroupName
	if reviewer == "" {
		reviewer = a.SRETeamName
	}
	if reviewer != "" {
		sections = append(sections, chatCardSection{
			Header:  "Status",
			Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: caseAlertLine(`Awaiting review by %s`, reviewer)}}},
		})
	}
	sections = append(sections, chatCardSection{
		Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: caseAlertLine(`<a href="%s">View case</a>`, a.CaseLink)}}},
	})

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{{CardID: "sr-acknowledged-alert", Card: chatCard{
			Header:   &chatCardHeader{Title: "Acknowledged", Subtitle: subtitle},
			Sections: sections,
		}}},
		Thread: &chatThread{ThreadKey: srThreadKey(a.CaseID, a.Number)},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// SendSRCustomerCommentAlert posts a customer's comment on a devops-sm SR to
// the SRE team: header "💬 Customer comment · <number> · <wso2CaseId>" over
// the SR subject, then the author (bold), the comment as plain text, and a
// "View comment" link.
//
// Threaded under the SR's created card, unlike SendFrustrationAlert (which
// stays top-level to stand out): this alert is the next turn of the same SR
// conversation the team is already following in that thread.
func (c *GoogleChatClient) SendSRCustomerCommentAlert(ctx context.Context, audience string, a SRCustomerCommentAlert) error {
	if a.Number == "" {
		return fmt.Errorf("notifications: number is required")
	}
	author := a.AuthorName
	if author == "" {
		author = a.AuthorEmail
	}
	var lines []string
	if author != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, author))
	}
	if content := srChatPlainText(a.Content, srChatTextLimit); content != "" {
		lines = append(lines, caseAlertLine(`%s`, content))
	}
	lines = append(lines, caseAlertLine(`<a href="%s">View comment</a>`, a.CommentLink))

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{{CardID: "sr-customer-comment-alert", Card: chatCard{
			Header:   &chatCardHeader{Title: "💬 Customer comment · " + chatHeaderCaseRef(a.Number, a.WSO2CaseID), Subtitle: a.Subject},
			Sections: []chatCardSection{{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: strings.Join(lines, "<br>")}}}}},
		}}},
		Thread: &chatThread{ThreadKey: srThreadKey(a.CaseID, a.Number)},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// srBlockTags are the tags that end a run of text: each becomes a space, so
// "<p>a</p><p>b</p>" reads "a b" rather than "ab".
var srBlockTags = map[string]bool{
	"br": true, "p": true, "div": true, "li": true, "tr": true, "td": true, "th": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"blockquote": true, "pre": true, "hr": true, "ul": true, "ol": true, "table": true,
}

// srChatPlainText reduces s -- a description or comment, rich-text HTML or
// plain -- to one line of plain text of at most max runes ("..." appended
// when cut), the way ServiceNow's card cleaned it. A real tokenizer, not a
// regex, so entities decode and a "<" in prose survives; script/style
// contents are dropped. ServiceNow's journal "[code]" markers are removed
// too, since SN-sourced comments wrap their HTML in them. The result is
// unescaped text: callers still pass it through caseAlertLine.
func srChatPlainText(s string, max int) string {
	s = strings.NewReplacer("[code]", " ", "[/code]", " ").Replace(s)
	var b strings.Builder
	z := xhtml.NewTokenizer(strings.NewReader(s))
	skip := 0
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			break
		}
		switch tt {
		case xhtml.TextToken:
			if skip == 0 {
				b.Write(z.Text())
			}
		case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "script" || tag == "style" {
				if tt == xhtml.StartTagToken {
					skip++
				} else if tt == xhtml.EndTagToken && skip > 0 {
					skip--
				}
				continue
			}
			if srBlockTags[tag] {
				b.WriteByte(' ')
			}
		}
	}
	text := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(text); len(r) > max {
		return string(r[:max]) + "..."
	}
	return text
}

// orEmDash is ServiceNow's SRChatCardUtils convention for an empty card value:
// "—" rather than a blank line.
func orEmDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "\u2014"
	}
	return s
}

// srThreadKey is an SR's Chat thread: "case-" + the SR's id, as ServiceNow's
// flow keys it ('case-' + sys_id). So each new SR starts its own thread and
// every later card about it replies there. Not chatThreadKey(number):
// numbers are only unique within one environment, and two environments
// posting to one space -- or a database reseeded under the same numbers --
// would reply into another SR's thread. The number is only a fallback for a
// card built without an id.
func srThreadKey(caseID, number string) string {
	if caseID != "" {
		return "case-" + caseID
	}
	return chatThreadKey(number)
}
