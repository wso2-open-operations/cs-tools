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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The markup must reach Twilio as real nested XML — this is the whole reason
// this path exists. Handing MakeCall a pre-built SSML string produced escaped
// tags, which Twilio reads aloud instead of interpreting.
func TestSSMLTwiML_EmitsRealNestedMarkup(t *testing.T) {
	speech := Speech{Sentences: []Sentence{
		{Parts: []SpeechPart{Say("WSO2 Support Alert.")}},
		{Parts: []SpeechPart{
			Say("Case number - "), Pause("500ms"), Say(" "), Spell("90%", "WSO2-1000"), Say(" ."),
		}},
		{Parts: []SpeechPart{Stress("moderate", " Update the ticket status.")}},
	}}

	got, err := ssmlTwiML(speech, "Polly.Aditi", "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<Say voice="Polly.Aditi" language="en-IN">`,
		`<s>WSO2 Support Alert.</s>`,
		`<break time="500ms">`,
		`<prosody rate="90%">WSO2-1000</prosody>`,
		`<emphasis level="moderate"> Update the ticket status.</emphasis>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("document is missing %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "&lt;s&gt;") {
		t.Errorf("the markup was escaped, which is the bug this path fixes:\n%s", got)
	}
	// TwiML has no <speak> root — the <Say> verb is the root, and SSML
	// elements are its children.
	if strings.Contains(got, "<speak>") {
		t.Errorf("TwiML must not carry a <speak> root:\n%s", got)
	}
}

// The safety property from sayTwiML must survive: caller-supplied text can
// never become markup, however it is shaped.
func TestSSMLTwiML_TextCannotInjectTwiML(t *testing.T) {
	speech := Speech{Sentences: []Sentence{
		{Parts: []SpeechPart{Say(`</Say><Redirect>http://evil.example</Redirect><Say>`)}},
		{Parts: []SpeechPart{Spell("90%", `</prosody></Say><Dial>+15550000000</Dial>`)}},
	}}

	got, err := ssmlTwiML(speech, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<Redirect>", "<Dial>"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("caller text injected %s:\n%s", forbidden, got)
		}
	}
	if !strings.Contains(got, "&lt;/Say&gt;&lt;Redirect&gt;") {
		t.Errorf("expected the injection attempt to be escaped into spoken text:\n%s", got)
	}
}

// An empty voice/language omits the attributes entirely, matching sayTwiML's
// own behaviour with Twilio's account defaults.
func TestSSMLTwiML_OmitsEmptyVoiceAndLanguage(t *testing.T) {
	got, err := ssmlTwiML(Speech{Sentences: []Sentence{{Parts: []SpeechPart{Say("Hello.")}}}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<Say>") {
		t.Errorf("expected a bare <Say>, got:\n%s", got)
	}
}

func TestSpeech_IsEmpty(t *testing.T) {
	if !(Speech{}).IsEmpty() {
		t.Error("a speech with no sentences must report empty")
	}
	blank := Speech{Sentences: []Sentence{{Parts: []SpeechPart{Say("   ")}}}}
	if !blank.IsEmpty() {
		t.Error("a speech of only whitespace must report empty")
	}
	pauseOnly := Speech{Sentences: []Sentence{{Parts: []SpeechPart{Pause("1s")}}}}
	if pauseOnly.IsEmpty() {
		t.Error("a speech carrying markup must not report empty")
	}
}

func TestMakeSSMLCall(t *testing.T) {
	var gotTwiML, gotTo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		gotTwiML = r.PostFormValue("Twiml")
		gotTo = r.PostFormValue("To")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewTwilioClient(TwilioConfig{
		AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000",
		APIBaseURL: srv.URL,
	})
	speech := Speech{Sentences: []Sentence{{Parts: []SpeechPart{Say("WSO2 Support Alert.")}}}}
	if _, err := client.MakeSSMLCall(context.Background(), "+94770000001", speech); err != nil {
		t.Fatal(err)
	}
	if gotTo != "+94770000001" {
		t.Errorf("To = %q", gotTo)
	}
	if !strings.Contains(gotTwiML, "<s>WSO2 Support Alert.</s>") {
		t.Errorf("Twiml = %q", gotTwiML)
	}
}

func TestMakeSSMLCall_Validation(t *testing.T) {
	client := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000"})
	speech := Speech{Sentences: []Sentence{{Parts: []SpeechPart{Say("Alert.")}}}}

	if _, err := client.MakeSSMLCall(context.Background(), "  ", speech); err == nil {
		t.Error("expected an error for an empty destination")
	}
	if _, err := client.MakeSSMLCall(context.Background(), "+94770000001", Speech{}); err == nil {
		t.Error("expected an error for a silent call")
	}
	unconfigured := NewTwilioClient(TwilioConfig{})
	if _, err := unconfigured.MakeSSMLCall(context.Background(), "+94770000001", speech); err == nil {
		t.Error("expected an error when Twilio is not configured")
	}
}
