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

package rotareminder

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// Invite is one calendar event: one rotation window, with everybody on it as
// a guest, so they can start a group chat from it. It is written as an
// iCalendar (RFC 5545) REQUEST -- the form calendar clients show as an
// invitation -- or, for somebody taken off the window, a CANCEL.
type Invite struct {
	// UID identifies the event: built from the rota day and the shift
	// (inviteUID), never from the run, so sending it again -- a retry, the
	// Friday reminder -- updates the event instead of adding a copy.
	UID string
	// Cancel writes a CANCEL instead of a REQUEST: the event is withdrawn from
	// the attendees listed.
	Cancel bool
	// Summary is the event title ("CRE LK Weekend Rotation").
	Summary string
	// Description is the event's notes, plain text.
	Description string
	// Start and End are the window's instants, which entity-service resolved
	// from the shift's own zone (a window past midnight included). They are
	// written in UTC, so every guest's calendar shows them in their own zone.
	Start, End time.Time
	// OrganizerName and OrganizerEmail are the sender (EMAIL_FROM_ADDRESS).
	OrganizerName, OrganizerEmail string
	// Attendees are the people on the window (or, for a CANCEL, the people
	// taken off it).
	Attendees []Attendee
	// URL, when set, is the Team Schedule page.
	URL string
	// Sequence orders the versions of one event: a later send carries a higher
	// one, so calendars take it as the newer version.
	Sequence int
	// Stamp is when the invite was made.
	Stamp time.Time
}

// Attendee is one person invited to an event.
type Attendee struct {
	Name, Email string
}

// inviteContentType is how the invitation travels: text/calendar with the
// method, the form mail clients show as an invitation.
func (inv Invite) contentType() string {
	return "text/calendar; method=" + inv.method() + "; charset=UTF-8"
}

func (inv Invite) method() string {
	if inv.Cancel {
		return "CANCEL"
	}
	return "REQUEST"
}

// sequenceAt is the sequence of an invite made at t: minutes since the epoch,
// so every later send of an event carries a higher sequence without this job
// remembering the last one.
func sequenceAt(t time.Time) int {
	return int(t.Unix() / 60)
}

// inviteUID is the event's stable identity: family, rota day and shift code,
// e.g. "cre-rota-2026-10-24-cre_weekend@csm-team-schedule". Only letters,
// digits, '-' and '_' are kept, so a family name with a space cannot put one
// in the identity.
func inviteUID(family, rotaDate, shiftCode string) string {
	raw := strings.ToLower(fmt.Sprintf("%s-rota-%s-%s", family, rotaDate, shiftCode))
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String() + "@csm-team-schedule"
}

// ICS writes the invite as an iCalendar file. The organizer is a no-reply
// address, so every attendee is marked RSVP=FALSE: a Yes/No would go nowhere.
// Nothing taken from the rota or the configuration can start a line of its
// own: text is escaped and stripped of control characters, an address that
// is not a plain address is left out, and so is a URL that is not http(s).
func (inv Invite) ICS() []byte {
	var b strings.Builder
	line := func(s string) { b.WriteString(foldLine(s)) }
	utc := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

	line("BEGIN:VCALENDAR")
	line("PRODID:-//WSO2//CSM Team Schedule//EN")
	line("VERSION:2.0")
	line("CALSCALE:GREGORIAN")
	line("METHOD:" + inv.method())
	line("BEGIN:VEVENT")
	line("UID:" + inv.UID)
	line("DTSTAMP:" + utc(inv.Stamp))
	line(fmt.Sprintf("SEQUENCE:%d", inv.Sequence))
	line("DTSTART:" + utc(inv.Start))
	line("DTEND:" + utc(inv.End))
	line("SUMMARY:" + escapeText(inv.Summary))
	if inv.Description != "" {
		line("DESCRIPTION:" + escapeText(inv.Description))
	}
	if ValidScheduleURL(inv.URL) {
		line("URL:" + inv.URL)
	}
	if ValidInviteAddress(inv.OrganizerEmail) {
		line(fmt.Sprintf("ORGANIZER;CN=%s:%s", paramValue(inv.OrganizerName), mailtoURI(inv.OrganizerEmail)))
	}
	for _, a := range inv.Attendees {
		if !ValidInviteAddress(a.Email) {
			continue
		}
		line(fmt.Sprintf("ATTENDEE;CN=%s;CUTYPE=INDIVIDUAL;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=FALSE:%s",
			paramValue(a.Name), mailtoURI(a.Email)))
	}
	if inv.Cancel {
		line("STATUS:CANCELLED")
	} else {
		line("STATUS:CONFIRMED")
	}
	line("TRANSP:OPAQUE")
	line("END:VEVENT")
	line("END:VCALENDAR")
	return []byte(b.String())
}

// escapeText escapes a TEXT value (RFC 5545 3.3.11): backslash, semicolon,
// comma and newline. A lone carriage return is a newline too, and any other
// control character is dropped: none may appear in a TEXT value.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", `\n`)
	return withoutControls(s)
}

// paramValue quotes a parameter value (a CN) when it holds a character that
// would end it, and drops double quotes and control characters, which a
// parameter value cannot hold.
func paramValue(s string) string {
	s = withoutControls(strings.ReplaceAll(s, `"`, ""))
	if strings.ContainsAny(s, ",;:") {
		return `"` + s + `"`
	}
	return s
}

// mailtoURI is a plain address as a mailto URI (RFC 6068): a character an
// address may hold but a URI may not, such as "?" (which would start a query),
// "#" or "%", is percent-encoded. An everyday address is unchanged.
func mailtoURI(address string) string {
	var b strings.Builder
	b.WriteString("mailto:")
	for i := 0; i < len(address); i++ {
		c := address[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("-._~!$'()*+,;:@", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// withoutControls drops every control character (a line break included).
func withoutControls(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// ValidInviteAddress reports whether s is a plain email address an
// invitation can name after "mailto:": one address, nothing around it, no
// control character, quote or space.
func ValidInviteAddress(s string) bool {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) || r == '"' }) {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s
}

// InviteOrganizer is the address an invitation names as its organizer, read
// from the "From" the emails go out with (EMAIL_FROM_ADDRESS): a plain address
// as it is, or the address inside "Name <address>", whose name the emails keep.
// ok is false when there is no plain address to name.
func InviteOrganizer(from string) (address string, ok bool) {
	// A control character anywhere, ends included, refuses the sender: the
	// emails go out with it as given, so trimming one away would hide it.
	if strings.ContainsFunc(from, unicode.IsControl) {
		return "", false
	}
	from = strings.TrimSpace(from)
	if ValidInviteAddress(from) {
		return from, true
	}
	if from == "" {
		return "", false
	}
	a, err := mail.ParseAddress(from)
	if err != nil || !ValidInviteAddress(a.Address) {
		return "", false
	}
	return a.Address, true
}

// ValidScheduleURL reports whether s is an http or https URL with a host and
// no control character or space, safe to put in an email and an invitation.
func ValidScheduleURL(s string) bool {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// foldLine ends a content line with CRLF, folding it at 75 octets as RFC 5545
// 3.1 requires: each continuation starts with one space. It never splits a
// UTF-8 character.
func foldLine(s string) string {
	const limit = 75
	var b strings.Builder
	n := 0
	for _, r := range s {
		size := len(string(r))
		if n+size > limit {
			b.WriteString("\r\n ")
			n = 1
		}
		b.WriteRune(r)
		n += size
	}
	b.WriteString("\r\n")
	return b.String()
}
