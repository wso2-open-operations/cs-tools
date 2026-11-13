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

package service

import (
	"fmt"
	"html"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The tokenizer was ten regular-expression passes; it is now one combined pass,
// each run only when it can match, with letters split out directly. This is the
// old implementation, kept to prove the new one gives the same words.
var (
	refTicket  = regexp.MustCompile(`\b(chg|inc|cs|prb|scp|std|sr|ritm|req|task|ctask)[- ]?\d+\b`)
	refDate    = regexp.MustCompile(`\b\d{4}[-/.]\d{1,2}[-/.]\d{1,2}\b|\b\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4}\b`)
	refTime    = regexp.MustCompile(`\b\d{1,2}:\d{2}(:\d{2})?\s*(am|pm)?\b`)
	refVersion = regexp.MustCompile(`\bv?\d+(\.\d+)+[a-z0-9-]*\b|\bv\d+\b`)
	refOrdinal = regexp.MustCompile(`\b\d+(st|nd|rd|th)\b`)
	refDigits  = regexp.MustCompile(`\d+`)
	refNonWord = regexp.MustCompile(`[^a-z]+`)
)

func referenceSuggestionWords(text string) []string {
	s := suggestionHTMLTag.ReplaceAllString(text, " ")
	s = html.UnescapeString(s)
	s = suggestionPlaceholder.ReplaceAllString(s, " ")
	s = strings.ToLower(s)
	s = suggestionTagged.ReplaceAllString(s, " ")
	for _, re := range []*regexp.Regexp{refTicket, refDate, refTime, refVersion, refOrdinal, refDigits} {
		s = re.ReplaceAllString(s, " ")
	}
	s = refNonWord.ReplaceAllString(s, " ")
	var out []string
	for _, w := range strings.Fields(s) {
		if len(w) < 2 || suggestionFillerWords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

func TestSuggestionWords_SameAsTheRegexPasses(t *testing.T) {
	inputs := []string{
		"[ACME] Deploy CApps v1.2.3 to Pekin prod on 2026-03-01 (CHG0012345)",
		"Deploy capps to PEKIN prod - 14/03/2026 10:30 PM", "Deploy CApps to Pekin prod, March 3rd",
		"<b>Rotate</b> the TLS certificate &amp; restart gw-01", "WSO2 API Manager 4.2.0 update level 57", "12345",
		"<p>Run the rotation on {{cluster}} at 09:15:00am, then INC-4411 and task 12</p>",
		"Patch 1.2.3beta-rc1 on node-7 &lt;prod&gt; [urgent] 2nd time v3", "café naïve résumé 1st 22nd 103rd",
		"", "   ", "<p>&nbsp;</p>", "a b c d e", "chg 12 chg-13 chg14 xchg15", "10:30pm 9:5 1.2 1.2.x v12 v1x",
	}
	rnd := rand.New(rand.NewSource(1))
	parts := []string{"stop", "the", "gateway", "CHG0099", "v4.1.2", "2026-11-06", "12:30", "3rd", "&amp;", "<br/>", "{{env}}",
		"[x]", "node-12", "API", "Ünïcode", "-", ".", "/", ":", "42", "am", "release-7b"}
	for i := 0; i < 2000; i++ {
		var b strings.Builder
		for j := 0; j < 1+rnd.Intn(40); j++ {
			b.WriteString(parts[rnd.Intn(len(parts))])
			b.WriteString([]string{" ", "", "\n", ", "}[rnd.Intn(4)])
		}
		inputs = append(inputs, b.String())
	}
	for _, in := range inputs {
		if got, want := suggestionWords(in), referenceSuggestionWords(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("suggestionWords(%q)\n got %v\nwant %v", in, got, want)
		}
	}
}

func BenchmarkSuggestionWords(b *testing.B) {
	plan := "<p>" + strings.Repeat("stop the gateway copy the capp archive to deployment folder check health ", 25) + "</p>"
	withNumbers := plan + "<p>CHG0012345 v4.1.2 on 2026-11-06 12:30</p>"
	for _, c := range []struct{ name, text string }{{"plain", plan}, {"numbers", withNumbers}} {
		b.Run("new/"+c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suggestionWords(c.text)
			}
		})
		b.Run("reference/"+c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				referenceSuggestionWords(c.text)
			}
		})
	}
	_ = fmt.Sprint
}
