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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import "strings"

const (
	codeBlockOpen  = "[code]"
	codeBlockClose = "[/code]"
)

// ServiceNow HTML-escapes anything written to a work_notes/comments journal
// field by default, unless the value is wrapped in "[code]"/"[/code]" -- its
// documented opt-in signal to render the content as HTML instead of showing
// the tags literally. apps/csm-portal/backend's direct ServiceNow client
// already relies on this for case work notes (see
// internal/servicenow/worknotes.go); incident work notes/comments need the
// same wrap, since they go HTML-sourced but through this service's Choreo
// ServiceNow integration instead.
//
// wrapCodeBlock/trimCodeBlock only add/remove this service's own outermost
// wrapper (added exactly once, by wrapCodeBlock itself, around the whole
// value) -- they must never touch a "[code]"/"[/code]" substring the caller's
// own content legitimately contains, e.g. a note that mentions "[code]" in
// its text.
//
// Both only ever see content this portal's own rich-text editor produced
// (every caller threads it through as "bodyHtml" -- see e.g.
// useCsmIncidentComments.ts/useCsmChangeRequestComments.ts), so it is already
// HTML; this does not escape plain text before wrapping it.

// wrapCodeBlock marks an HTML-sourced work note/comment so ServiceNow renders
// it instead of displaying the tags literally. nil is passed through as nil.
func wrapCodeBlock(value *string) *string {
	if value == nil {
		return nil
	}
	wrapped := codeBlockOpen + *value + codeBlockClose
	return &wrapped
}

// trimCodeBlock strips wrapCodeBlock's own leading "[code]"/trailing
// "[/code]" markers from a work note/comment read back from ServiceNow --
// only when both are present at the respective boundary (i.e. this is really
// wrapCodeBlock's own wrapper), so an unpaired or interior "[code]"/"[/code]"
// elsewhere in the content is left untouched. nil is passed through as nil.
func trimCodeBlock(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := *value
	if strings.HasPrefix(trimmed, codeBlockOpen) && strings.HasSuffix(trimmed, codeBlockClose) {
		trimmed = strings.TrimSuffix(strings.TrimPrefix(trimmed, codeBlockOpen), codeBlockClose)
	}
	return &trimmed
}
