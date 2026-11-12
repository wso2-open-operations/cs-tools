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

package emaildomains

import (
	"bufio"
	_ "embed"
	"strings"
)

// disposableList is a committed copy of the curated disposable-email-domains
// blocklist (CC0), updated by `make disposable-list` and reviewed in a PR: a
// DISPOSABLE answer abandons the signup, so every change is seen first.
//
//go:embed disposable_domains.txt
var disposableList string

// internalDomains are WSO2's own; their subdomains count too.
var internalDomains = []string{"wso2.com"}

// freeProviders are consumer mail providers.
var freeProviders = setOf(
	"gmail.com", "googlemail.com", "yahoo.com", "ymail.com", "outlook.com",
	"hotmail.com", "live.com", "msn.com", "icloud.com", "me.com", "aol.com",
	"proton.me", "protonmail.com", "gmx.com", "zoho.com", "yandex.com", "mail.com",
)

func setOf(domains ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		m[d] = struct{}{}
	}
	return m
}

// Lists holds the disposable list. It is read-only once built.
type Lists struct {
	disposable map[string]struct{}
}

// NewLists loads the committed disposable list.
func NewLists() *Lists {
	return &Lists{disposable: parseDomains(disposableList)}
}

// Disposable reports whether domain is on the disposable list.
func (l *Lists) Disposable(domain string) bool {
	_, ok := l.disposable[domain]
	return ok
}

// Size is how many disposable domains are loaded.
func (l *Lists) Size() int { return len(l.disposable) }

func isFreeProvider(domain string) bool {
	_, ok := freeProviders[domain]
	return ok
}

func isInternal(domain string) bool {
	for _, d := range internalDomains {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

// parseDomains reads one domain per line; blank lines and # comments are skipped.
func parseDomains(text string) map[string]struct{} {
	set := map[string]struct{}{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.ToLower(strings.TrimSpace(sc.Text()))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[line] = struct{}{}
	}
	return set
}
