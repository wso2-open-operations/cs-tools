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

package ingest

import (
	"regexp"
	"strings"
)

// IssueMeta is the metadata derived from an issue body that is safe to
// persist: an ABT team name and an opened-by address. The body itself is
// never persisted, logged, or returned.
type IssueMeta struct {
	ABTTeam  *string
	OpenedBy *string
}

const (
	// maxABTTeamLen should stay at least as large as appconfig.API's
	// AbtTeamParamMaxLength: a stored team name longer than that config
	// value could never be matched by the abtTeam filter param.
	maxABTTeamLen       = 100
	allowedOpenerDomain = "@wso2.com"
)

// metaLineRe matches an "ABT Team" or "Opened by" key/value line while
// tolerating leading markdown/quote/bullet noise and bold/italic wrappers
// around the key, e.g. "ABT Team : Atlas", "- **ABT Team:** Atlas",
// "> Opened by: x@wso2.com". It intentionally does not allow arbitrary extra
// words before or after the key (so "Previously opened by:" does not match),
// only the punctuation/whitespace a markdown renderer might have introduced.
//
// emailRe extracts the first email-shaped substring from a value so an
// address wrapped in angle brackets or a markdown mailto link can still be
// read out.
//
// wso2Re is the same shape as the database's issues_opened_by_wso2_chk
// check constraint, so a value that fails this regex would also be rejected
// by the database if it were ever sent there.
var (
	metaLineRe = regexp.MustCompile(`(?i)^[\s>*_\-]*(abt\s+team|opened\s+by)[\s*_]*:[\s*_]*(.*)$`)
	emailRe    = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+`)
	wso2Re     = regexp.MustCompile(`^[a-z0-9._%+\-]+@wso2\.com$`)
)

// placeholderValues are the case-insensitive strings issue templates use to
// mean "not filled in"; any of these is treated the same as an absent value
// rather than being stored literally.
var placeholderValues = map[string]bool{
	"": true, "undefined": true, "null": true, "none": true,
	"n/a": true, "na": true, "-": true, "tbd": true,
}

// ExtractIssueMeta scans body line by line for "ABT Team" and "Opened by"
// key/value lines. The last occurrence of a key wins, because an automated
// footer appended at the bottom of the body should take precedence over any
// earlier free-text mention of the same key elsewhere in the body.
func ExtractIssueMeta(body string) IssueMeta {
	var meta IssueMeta
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		m := metaLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := strings.Trim(strings.TrimSpace(m[2]), "*_` ")
		switch strings.ToLower(strings.Join(strings.Fields(m[1]), " ")) {
		case "abt team":
			meta.ABTTeam = normalizeABTTeam(value)
		case "opened by":
			meta.OpenedBy = normalizeOpenedBy(value)
		}
	}
	return meta
}

// normalizeABTTeam collapses internal whitespace and rejects a placeholder
// value or one longer than maxABTTeamLen, since a team name that long is
// almost certainly stray text rather than an actual team name.
func normalizeABTTeam(v string) *string {
	v = strings.Join(strings.Fields(v), " ")
	if placeholderValues[strings.ToLower(v)] || len(v) > maxABTTeamLen {
		return nil
	}
	return &v
}

// normalizeOpenedBy accepts only a @wso2.com address (handles a bare
// address, "<a@wso2.com>", or a markdown mailto link), lower-cased so
// callers never see mixed-case duplicates of the same address. Any other
// domain, including a look-alike such as "@wso2.com.evil.io" or a
// subdomain such as "@sub.wso2.com", is rejected rather than stored.
func normalizeOpenedBy(v string) *string {
	if placeholderValues[strings.ToLower(v)] {
		return nil
	}
	email := strings.ToLower(emailRe.FindString(v))
	if !strings.HasSuffix(email, allowedOpenerDomain) || !wso2Re.MatchString(email) {
		return nil
	}
	return &email
}

// normalizeTitle trims whitespace; an empty result becomes nil rather than
// an empty string, matching how the other two fields represent "no value".
func normalizeTitle(t string) *string {
	t = strings.TrimSpace(t)
	if t == "" {
		return nil
	}
	return &t
}
