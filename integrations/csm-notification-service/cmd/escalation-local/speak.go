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
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Speaking the alert out loud, without a telephony account.
//
// What a person actually needs to check about a voice alert is whether it
// SOUNDS right: whether the case reference is intelligible read aloud, whether
// the pauses land, whether the closing instruction is unmistakable. None of
// that needs a phone call — it needs the document spoken.
//
// So --speak renders the very TwiML the stub received, which production code
// built, through the local speech synthesiser. It is not a substitute for a
// real call (it proves nothing about Twilio accepting the document, about the
// number, or about a phone ringing), but it is the only part of the voice path
// a laptop can genuinely exercise, and it catches the things a real call would
// only tell you after waking someone up.

// sayCommand is the macOS speech binary. Speaking is skipped with a note on
// any other platform rather than failing the run — the ladder is the point,
// the audio is a bonus.
const sayCommand = "say"

// speakTwiML speaks the <Say> content of a TwiML document.
func speakTwiML(ctx context.Context, twiml, voice string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("--speak needs macOS's %q; on this platform, read the message printed above instead", sayCommand)
	}
	script, err := sayScriptFromTwiML(twiml)
	if err != nil {
		return err
	}
	if strings.TrimSpace(script) == "" {
		return nil
	}
	args := []string{}
	if voice != "" {
		args = append(args, "-v", voice)
	}
	args = append(args, script)
	return exec.CommandContext(ctx, sayCommand, args...).Run()
}

// sayScriptFromTwiML converts the spoken half of a TwiML document into the
// inline command syntax macOS's speech synthesiser understands, so the
// pauses and the slowed case reference survive rather than being flattened
// into one hurried sentence.
//
// The mapping is deliberately narrow — it covers exactly the SSML this
// service emits:
//
//	<break time="500ms"/>              → [[slnc 500]]
//	<prosody rate="90%">…</prosody>    → [[rate N]]…[[rate R]]
//	<emphasis level="…">…</emphasis>   → [[emph +]]…[[emph -]]
//	<s>…</s>                           → a sentence boundary
//
// Anything else is spoken as its text, which is the safe direction to fail:
// a missed effect is a worse-sounding preview, never a wrong one.
func sayScriptFromTwiML(twiml string) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(twiml))
	var b strings.Builder
	inSay := false

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Say":
				inSay = true
			case "break":
				if inSay {
					b.WriteString(fmt.Sprintf(" [[slnc %d]] ", breakMillis(attr(t, "time"))))
				}
			case "prosody":
				if inSay {
					b.WriteString(fmt.Sprintf(" [[rate %d]] ", rateWords(attr(t, "rate"))))
				}
			case "emphasis":
				if inSay {
					b.WriteString(" [[emph +]] ")
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "Say":
				inSay = false
			case "prosody":
				if inSay {
					b.WriteString(fmt.Sprintf(" [[rate %d]] ", defaultRateWords))
				}
			case "emphasis":
				if inSay {
					b.WriteString(" [[emph -]] ")
				}
			case "s":
				if inSay {
					// A sentence boundary the synthesiser pauses at, standing
					// in for <s>'s own prosodic break — unless the text
					// already ended in one, which would double the pause.
					if !strings.HasSuffix(strings.TrimSpace(b.String()), ".") {
						b.WriteString(".")
					}
					b.WriteString(" ")
				}
			}
		case xml.CharData:
			if inSay {
				b.WriteString(string(t))
			}
		}
	}
	return collapseSpaces(b.String()), nil
}

// defaultRateWords is the synthesiser's normal pace, in words per minute —
// what a <prosody> region is restored to when it closes.
const defaultRateWords = 180

// rateWords converts an SSML rate ("90%") into words per minute. An
// unparsable or absent rate falls back to the normal pace.
func rateWords(rate string) int {
	pct := strings.TrimSuffix(strings.TrimSpace(rate), "%")
	n, err := strconv.Atoi(pct)
	if err != nil || n <= 0 {
		return defaultRateWords
	}
	return defaultRateWords * n / 100
}

// breakMillis converts an SSML break time ("500ms", "1s") to milliseconds.
func breakMillis(t string) int {
	t = strings.TrimSpace(t)
	switch {
	case strings.HasSuffix(t, "ms"):
		if n, err := strconv.Atoi(strings.TrimSuffix(t, "ms")); err == nil {
			return n
		}
	case strings.HasSuffix(t, "s"):
		if n, err := strconv.Atoi(strings.TrimSuffix(t, "s")); err == nil {
			return n * 1000
		}
	}
	return 500
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// collapseSpaces keeps the script readable when printed alongside the audio.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// A run places a dozen calls, and on a compressed clock they land seconds
// apart — speaking every one produces a wall of overlapping audio nobody can
// follow, which is the opposite of the point.
//
// So the speaker plays a given message ONCE, and blocks while it does, so two
// alerts never talk over each other. Once, not once per rung, because every
// rung of a ladder speaks the identical words: section 10.0's message is built
// from the incident's trigger, priority, account, case reference and team, and
// none of those changes as the ladder climbs — only who is dialled, which the
// document does not carry. Hearing it a second time would tell a listener
// nothing the printed rung lines do not already say.
//
// A message that genuinely differs is heard again, which is what makes the
// interesting case audible: an elevation replacing a running ladder speaks a
// different trigger type and priority, and a listener should hear that change.
type speaker struct {
	// Two locks, deliberately. said guards the "have we heard this" map and
	// is held for microseconds on the request path; audio serialises playback
	// and is held for the length of a spoken alert. One lock for both meant
	// the second call's dedup check waited on the first call's audio — inside
	// the stub's HTTP handler — which timed out the engine's client and made
	// it retry a call that had been accepted. Exactly the bug moving the
	// speaking into a goroutine was supposed to fix, reintroduced by the lock
	// it still shared.
	said    map[string]bool
	saidMu  sync.Mutex
	audioMu sync.Mutex
	wg      sync.WaitGroup
}

func newSpeaker() *speaker {
	return &speaker{said: map[string]bool{}}
}

// play speaks a call's document once, in the background.
//
// In the background because the caller is the stub's own HTTP handler, and Go
// buffers a response until the handler returns — holding here to speak made
// the engine's client time out mid-call and retry a call that had in fact been
// accepted. The mutex still serialises the audio itself, so alerts never
// overlap; only the waiting moved off the request path.
func (s *speaker) play(twiml, voice string) {
	label, script := describe(twiml)

	s.saidMu.Lock()
	heard := s.said[script]
	s.said[script] = true
	s.saidMu.Unlock()
	if heard {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.audioMu.Lock()
		defer s.audioMu.Unlock()
		fmt.Printf("\n  ♪ ALERT TRIGGERED — speaking it now: %s\n", label)
		if err := speakTwiML(context.Background(), twiml, voice); err != nil {
			fmt.Printf("  (could not speak: %v)\n", err)
		}
	}()
}

// wait lets any alert still being spoken finish before the process exits.
func (s *speaker) wait() { s.wg.Wait() }

// describe returns a short label for the announcement line and the full script
// used to tell one alert from another.
//
// Every rung of a ladder speaks identical words — section 10.0's message is
// built from the incident's trigger, priority, account, case reference and
// team, none of which changes as the ladder climbs, only who is dialled, which
// the document does not carry. So the script is the right identity: an alert
// is heard once, and heard again only when it genuinely differs, which is what
// makes an elevation replacing a running ladder audible.
func describe(twiml string) (label, script string) {
	script, err := sayScriptFromTwiML(twiml)
	if err != nil || script == "" {
		return "alert", twiml
	}
	// The trigger type and the priority are what distinguish one alert from
	// another to a listener; the rest is the same every time.
	var wanted []string
	for _, sentence := range strings.Split(script, ".") {
		sentence = strings.TrimSpace(sentence)
		if strings.HasPrefix(sentence, "Trigger Type") || strings.HasPrefix(sentence, "Priority") {
			wanted = append(wanted, sentence)
		}
	}
	if len(wanted) == 0 {
		return "alert", script
	}
	return strings.Join(wanted, ", "), script
}
