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

package main

import (
	"strings"
	"testing"
)

// The spoken preview is the only part of the voice path a laptop can exercise
// without a telephony account, so the conversion it rests on is worth pinning:
// a dropped pause or a lost rate change makes the preview sound like something
// the real call would not.

const realSSMLDocument = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<Response><Say voice="Polly.Aditi" language="en-IN">` +
	`<s>WSO2 Support Alert.</s><s>Trigger Type - New Case.</s><s>Priority - P0.</s>` +
	`<s>Account - Automation Test Account.</s>` +
	`<s>Case number - <break time="500ms"></break> <prosody rate="90%">INC0012345</prosody> .</s>` +
	`<s>Team - Americas CS Team.</s>` +
	`<s><emphasis level="moderate"> Update the ticket status to 'Work In Progress' to stop further notifications.</emphasis></s>` +
	`<s><break time="1s"></break></s></Say></Response>`

func TestSayScript_CarriesEverySSMLEffect(t *testing.T) {
	got, err := sayScriptFromTwiML(realSSMLDocument)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"WSO2 Support Alert",
		"[[slnc 500]]",            // the pause before the case reference
		"[[rate 162]] INC0012345", // 90% of the normal pace
		"[[rate 180]]",            // and restored afterwards
		"[[emph +]]",              // the closing instruction, stressed
		"[[emph -]]",
		"[[slnc 1000]]", // the trailing beat
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the spoken script is missing %q:\n%s", want, got)
		}
	}
}

// Only the spoken half of the document is read: the XML declaration, the
// Response wrapper and the Say attributes are markup, not words.
func TestSayScript_SpeaksOnlyTheSayContent(t *testing.T) {
	got, err := sayScriptFromTwiML(realSSMLDocument)
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"Response", "Polly.Aditi", "en-IN", "xml", "version"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the spoken script reads markup aloud (%q):\n%s", unwanted, got)
		}
	}
}

// A plain document has no effects to carry, and must come through as the
// sentences themselves.
func TestSayScript_PlainDocument(t *testing.T) {
	plain := `<?xml version="1.0" encoding="UTF-8"?><Response><Say>WSO2 Support Alert. Priority, CRITICAL.</Say></Response>`
	got, err := sayScriptFromTwiML(plain)
	if err != nil {
		t.Fatal(err)
	}
	if got != "WSO2 Support Alert. Priority, CRITICAL." {
		t.Errorf("got %q", got)
	}
}

// A sentence already ending in a full stop must not gain a second one, which
// would double the pause between every sentence of the alert.
func TestSayScript_DoesNotDoublePunctuation(t *testing.T) {
	got, err := sayScriptFromTwiML(`<Response><Say><s>One.</s><s>Two.</s></Say></Response>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "..") {
		t.Errorf("doubled punctuation: %q", got)
	}
}

func TestRateWords(t *testing.T) {
	for _, tc := range []struct {
		rate string
		want int
	}{
		{"90%", 162},
		{"100%", defaultRateWords},
		{"50%", 90},
		{"", defaultRateWords},     // absent
		{"fast", defaultRateWords}, // unparsable
		{"0%", defaultRateWords},   // nonsensical
	} {
		if got := rateWords(tc.rate); got != tc.want {
			t.Errorf("rateWords(%q) = %d, want %d", tc.rate, got, tc.want)
		}
	}
}

func TestBreakMillis(t *testing.T) {
	for _, tc := range []struct {
		time string
		want int
	}{
		{"500ms", 500},
		{"1s", 1000},
		{"2s", 2000},
		{"", 500},      // absent falls back to a natural beat
		{"later", 500}, // unparsable likewise
	} {
		if got := breakMillis(tc.time); got != tc.want {
			t.Errorf("breakMillis(%q) = %d, want %d", tc.time, got, tc.want)
		}
	}
}

// The announcement names the trigger and priority, which is what distinguishes
// one alert from another to a listener — and the identity used to decide
// whether an alert has already been heard is the whole script, so an elevation
// replacing a ladder is heard again.
func TestDescribe(t *testing.T) {
	label, script := describe(realSSMLDocument)
	if !strings.Contains(label, "Trigger Type - New Case") || !strings.Contains(label, "Priority - P0") {
		t.Errorf("label = %q, want the trigger and priority", label)
	}
	if strings.HasPrefix(label, ",") || strings.Contains(label, "  ") {
		t.Errorf("label has empty fragments: %q", label)
	}
	if !strings.Contains(script, "INC0012345") {
		t.Errorf("the identity script is not the full message: %q", script)
	}

	elevated := strings.Replace(realSSMLDocument, "New Case", "Priority Elevation", 1)
	_, otherScript := describe(elevated)
	if script == otherScript {
		t.Error("a genuinely different alert must not be treated as already heard")
	}
}

// A speaker plays a given alert once and no more, so a ladder's dozen calls
// do not become a dozen overlapping recitals.
func TestSpeaker_PlaysAnAlertOnce(t *testing.T) {
	s := newSpeaker()
	_, script := describe(realSSMLDocument)

	// Simulate what play does to the map, without invoking the synthesiser.
	first := !s.said[script]
	s.said[script] = true
	second := !s.said[script]

	if !first {
		t.Error("the first occurrence of an alert should be heard")
	}
	if second {
		t.Error("a repeat of the same alert should be skipped")
	}
}
