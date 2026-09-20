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
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
)

// This file adds the SSML half of the voice channel. MakeCall (twilio.go)
// speaks one flat string; MakeSSMLCall speaks a structured document, which is
// what the incident escalation alert needs for its pauses and its slowed-down
// case reference.
//
// The safety property from sayTwiML's doc comment is preserved by
// construction, not by escaping a pre-built string: a caller cannot hand this
// package SSML *markup* at all. It hands it a tree, and only this file decides
// which element names and attributes exist. Caller-supplied text only ever
// reaches the document through xml.CharData, which the encoder escapes — so a
// value containing "</Say><Redirect>" is spoken aloud, exactly as it is today.
//
// Building the same document from an SSML *string* is what this deliberately
// avoids: it would mean either trusting the caller's markup (a TwiML injection
// hole) or escaping it (which is what makes escalation.Trigger.VoiceMessage
// undeliverable — Twilio reads the escaped tags aloud instead of interpreting
// them). See internal/escalation/plan.go's VoiceSpeech.
//
// Note there is no <speak> root here. A standalone SSML document has one;
// inside TwiML the <Say> verb is itself the root, and SSML elements are its
// children. The specification's section 10.0 template shows <speak> because it
// is ServiceNow flow script, not TwiML.

// Speech is an ordered SSML document: a sequence of sentences, spoken in
// order. It is the whole vocabulary this package supports, which is
// deliberately just what section 10.0's template uses.
type Speech struct {
	Sentences []Sentence
}

// Sentence is one <s> element — a sentence boundary the voice engine pauses
// at naturally. Its parts are spoken in order.
type Sentence struct {
	Parts []SpeechPart
}

// SpeechPart is one piece of a sentence: plain text, or one of the three
// markup elements section 10.0 uses. Exactly one field is meaningful per part
// — the first non-nil element wins, and Text is used when all are nil. Build
// them with Say/Pause/Spell/Stress rather than by hand.
type SpeechPart struct {
	Text     string
	Break    *SpeechBreak
	Prosody  *SpeechProsody
	Emphasis *SpeechEmphasis
}

// SpeechBreak is <break time="..."/> — a fixed pause.
type SpeechBreak struct {
	XMLName xml.Name `xml:"break"`
	Time    string   `xml:"time,attr"`
}

// SpeechProsody is <prosody rate="...">text</prosody> — text spoken at an
// adjusted rate.
type SpeechProsody struct {
	XMLName xml.Name `xml:"prosody"`
	Rate    string   `xml:"rate,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// SpeechEmphasis is <emphasis level="...">text</emphasis>.
type SpeechEmphasis struct {
	XMLName xml.Name `xml:"emphasis"`
	Level   string   `xml:"level,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// Say returns a plain-text part.
func Say(text string) SpeechPart { return SpeechPart{Text: text} }

// Pause returns a <break> of the given duration (Twilio accepts e.g. "500ms",
// "1s").
func Pause(d string) SpeechPart { return SpeechPart{Break: &SpeechBreak{Time: d}} }

// Spell returns text slowed down via <prosody rate>, which is how section
// 10.0 reads a case reference out so it can be written down.
func Spell(rate, text string) SpeechPart {
	return SpeechPart{Prosody: &SpeechProsody{Rate: rate, Text: text}}
}

// Stress returns text wrapped in <emphasis>.
func Stress(level, text string) SpeechPart {
	return SpeechPart{Emphasis: &SpeechEmphasis{Level: level, Text: text}}
}

// IsEmpty reports whether this document would say nothing at all — used by
// MakeSSMLCall to reject a silent call the same way MakeCall rejects an empty
// message.
func (s Speech) IsEmpty() bool {
	for _, sentence := range s.Sentences {
		for _, p := range sentence.Parts {
			if p.Break != nil || p.Prosody != nil || p.Emphasis != nil {
				return false
			}
			if strings.TrimSpace(p.Text) != "" {
				return false
			}
		}
	}
	return true
}

// MarshalXML writes the sentence as <s>…</s> with its parts in order.
//
// encoding/xml cannot express ordered mixed content (text interleaved with
// child elements) declaratively — a []SpeechPart field would be marshaled as
// a repeated wrapper element named after the field. Driving the encoder token
// by token is what keeps the order, and it keeps the escaping guarantee too:
// text goes through xml.CharData, and every element below is a typed struct
// marshaled by the encoder itself.
func (s Sentence) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "s"}
	start.Attr = nil
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, p := range s.Parts {
		if err := p.encode(e); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

func (p SpeechPart) encode(e *xml.Encoder) error {
	switch {
	case p.Break != nil:
		return e.Encode(p.Break)
	case p.Prosody != nil:
		return e.Encode(p.Prosody)
	case p.Emphasis != nil:
		return e.Encode(p.Emphasis)
	default:
		return e.EncodeToken(xml.CharData(p.Text))
	}
}

// twimlSSMLResponse is the <Response><Say …><s>…</s></Say></Response>
// document MakeSSMLCall sends, the SSML counterpart of twimlResponse.
type twimlSSMLResponse struct {
	XMLName xml.Name     `xml:"Response"`
	Say     twimlSSMLSay `xml:"Say"`
}

type twimlSSMLSay struct {
	Voice     string     `xml:"voice,attr,omitempty"`
	Language  string     `xml:"language,attr,omitempty"`
	Sentences []Sentence `xml:"s"`
}

// ssmlTwiML builds the TwiML document for a Speech, in the given
// voice/language (either may be empty to use Twilio's account defaults).
func ssmlTwiML(s Speech, voice, language string) (string, error) {
	doc, err := xml.Marshal(twimlSSMLResponse{
		Say: twimlSSMLSay{Voice: voice, Language: language, Sentences: s.Sentences},
	})
	if err != nil {
		return "", fmt.Errorf("notifications: encode call message as SSML TwiML: %w", err)
	}
	return xml.Header + string(doc), nil
}

// MakeSSMLCall places a single voice call to `to` (E.164) that speaks a
// structured SSML document. Identical to MakeCall in every other respect —
// same account, same FromNumber requirement, same Voice/Language config, same
// Calls.json resource — differing only in what it puts inside <Say>.
func (c *TwilioClient) MakeSSMLCall(ctx context.Context, to string, speech Speech) (Call, error) {
	if strings.TrimSpace(to) == "" {
		return Call{}, fmt.Errorf("notifications: to is required")
	}
	if speech.IsEmpty() {
		return Call{}, fmt.Errorf("notifications: message is required")
	}
	if c.cfg.AccountSID == "" || c.cfg.AuthToken == "" || c.cfg.FromNumber == "" {
		return Call{}, fmt.Errorf("notifications: twilio is not configured")
	}

	twiml, err := ssmlTwiML(speech, c.cfg.Voice, c.cfg.Language)
	if err != nil {
		return Call{}, err
	}
	form := url.Values{
		"To":    {to},
		"From":  {c.cfg.FromNumber},
		"Twiml": {twiml},
	}
	c.applyRingTimeout(form)

	return c.do(ctx, "Calls.json", form)
}
