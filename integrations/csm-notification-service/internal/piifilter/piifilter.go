// Package piifilter scrubs personally identifying data from case comment
// text before it reaches Azure OpenAI. Ported from Sasmitha's Python
// pii_filter.py, which itself is a port of the ServiceNow PIIFilter
// script include.
//
// Usage:
//
//	f := piifilter.New()
//	clean := f.Filter(rawText)   // emails, phones, timestamps, names replaced
package piifilter

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Compiled patterns -- allocated once, safe for concurrent use.
var (
	emailRe     = regexp.MustCompile(`[\w.%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	timestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
	phoneRe     = regexp.MustCompile(`\+?\d[\d\s\-().]{6,}\d`)
	// ServiceNow journal: agent names trail a circled-W marker (U+24CC or U+24E6).
	agentNameRe = regexp.MustCompile(`(\b\w+(?:\s\w+){0,2})\s([\x{24cc}\x{24e6}])`)
	// Customer names precede "(Additional comments)" in ServiceNow journals.
	customerNameRe = regexp.MustCompile(`(\b\w+(?:\s\w+){0,2})(?:\s\(Additional comments\))`)
)

const (
	emailPlaceholder     = "[email@example.com]"
	timestampPlaceholder = "[-Date Time-]"
	phonePlaceholder     = "[-Phone Number-]"
)

// Filter holds per-case pseudonym state so the same person gets the same
// placeholder across all comments of one case. Create one per case via New().
type Filter struct {
	mu              sync.Mutex
	agentNames      map[string]string
	customerNames   map[string]string
	agentCounter    int
	customerCounter int
	compiledParts   map[string]*regexp.Regexp
}

// New returns a fresh Filter with empty pseudonym maps.
func New() *Filter {
	return &Filter{
		agentNames:      make(map[string]string),
		customerNames:   make(map[string]string),
		agentCounter:    1,
		customerCounter: 1,
		compiledParts:   make(map[string]*regexp.Regexp),
	}
}

// Filter scrubs PII from text: timestamps, emails, phone numbers, and
// ServiceNow-style agent/customer names. Safe for concurrent use.
func (f *Filter) Filter(text string) string {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 1. Timestamps
	out := timestampRe.ReplaceAllString(text, timestampPlaceholder)
	// 2. Emails
	out = emailRe.ReplaceAllString(out, emailPlaceholder)
	// 3. Phone numbers
	out = phoneRe.ReplaceAllString(out, phonePlaceholder)
	// 4. Agent names (ServiceNow journal header pattern)
	out = agentNameRe.ReplaceAllStringFunc(out, func(match string) string {
		parts := agentNameRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		name := strings.TrimSpace(parts[1])
		marker := parts[2]
		if name == "" {
			return match
		}
		if _, ok := f.agentNames[name]; !ok {
			f.agentNames[name] = fmt.Sprintf("WSO2 Agent %d", f.agentCounter)
			f.agentCounter++
		}
		return f.agentNames[name] + " " + marker
	})
	// 5. Customer names (ServiceNow journal header pattern)
	out = customerNameRe.ReplaceAllStringFunc(out, func(match string) string {
		parts := customerNameRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		name := strings.TrimSpace(parts[1])
		if name == "" {
			return match
		}
		if _, ok := f.customerNames[name]; !ok {
			f.customerNames[name] = fmt.Sprintf("Customer %d", f.customerCounter)
			f.customerCounter++
		}
		// Keep the "(Additional comments)" suffix intact
		return f.customerNames[name] + match[len(parts[1]):]
	})
	// 6. Scrub every name-part (>2 chars) from discovered names everywhere
	allNames := make(map[string]string)
	for k, v := range f.agentNames {
		allNames[k] = v
	}
	for k, v := range f.customerNames {
		allNames[k] = v
	}
	for fullName, placeholder := range allNames {
		for _, part := range strings.Fields(fullName) {
			if len(part) > 2 {
				re, ok := f.compiledParts[part]
				if !ok {
					re = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(part) + `\b`)
					f.compiledParts[part] = re
				}
				out = re.ReplaceAllString(out, placeholder)
			}
		}
	}
	return out
}

// FilterAll applies Filter to every string in the slice, returning a new slice.
func (f *Filter) FilterAll(texts []string) []string {
	result := make([]string, len(texts))
	for i, t := range texts {
		result[i] = f.Filter(t)
	}
	return result
}
