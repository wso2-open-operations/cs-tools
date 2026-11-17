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

package repository

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// IsBlankRichText reports whether rich text (the HTML the CSM webapp's editor
// writes, or plain text) has no text a reader would see. It is the webapp's
// isBlankHtml (apps/csm-portal/webapp/src/utils/sanitizeHtml.ts), mirrored so
// that the two agree on what is blank. isBlankHtml parses the value as the
// browser does (DOMPurify, through DOMParser), removes every element but keeps
// the text inside it, except where DOMPurify drops the content with the element
// (script, style, svg, ...; richTextDroppedContent), turns U+00A0 into a space
// and trims what is left as JavaScript's trim does. So markup with no text is
// blank: "<p><br></p>", "<p>&nbsp;</p>", an empty list. So is markup that is
// only images (`<p><img src="..."></p>`): an image is no text. A "<" that opens
// no tag ("a < b") is text, as it is to the browser.
//
// The value is parsed with the HTML5 parser (golang.org/x/net/html), with
// scripting off as DOMParser has it, and only the body is read: what the parser
// puts in the head (a title or noscript before any body content) is not text to
// isBlankHtml either. It also follows DOMPurify's guard against markup smuggled
// through a comment (richTextSmuggled): "<p><!-- c -->&lt;b&gt;</p>" is blank.
//
// Where the two parsers follow different versions of the specification (the
// contents of a select, mostly) the answer can differ; the editor never writes
// such markup, and the webapp sends what DOMPurify has already sanitised.
// TestIsBlankRichText's inputs were checked against isBlankHtml itself. A value
// the parser refuses (elements nested more than 512 deep) is not blank: it is
// kept as written rather than judged.
//
// It takes time linear in the length of s: it runs under a change request's row
// locks, on input up to the 1 MiB request body cap.
func IsBlankRichText(s string) bool {
	doc, err := html.ParseWithOptions(strings.NewReader(s), html.ParseOptionEnableScripting(false))
	if err != nil {
		return false
	}
	body := richTextBody(doc)
	if body == nil {
		return true
	}
	blank := true
	var visit func(n *html.Node)
	visit = func(n *html.Node) {
		for c := n.FirstChild; c != nil && blank; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				blank = blankRichTextRun(c.Data)
			case html.ElementNode:
				if !richTextDroppedContent[strings.ToLower(c.Data)] && !richTextSmuggled(c) {
					visit(c)
				}
			}
		}
	}
	visit(body)
	return blank
}

// richTextBody is the document's body element, nil when there is none.
func richTextBody(doc *html.Node) *html.Node {
	for root := doc.FirstChild; root != nil; root = root.NextSibling {
		if root.Type != html.ElementNode || root.Data != "html" {
			continue
		}
		for c := root.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "body" {
				return c
			}
		}
	}
	return nil
}

// richTextDroppedContent are the elements DOMPurify removes together with their
// content: its default FORBID_CONTENTS, as of the 3.4.16 the webapp pins.
var richTextDroppedContent = map[string]bool{
	"annotation-xml": true, "audio": true, "colgroup": true, "desc": true, "foreignobject": true,
	"head": true, "iframe": true, "math": true, "mi": true, "mn": true, "mo": true, "ms": true,
	"mtext": true, "noembed": true, "noframes": true, "noscript": true, "plaintext": true,
	"script": true, "selectedcontent": true, "style": true, "svg": true, "template": true,
	"thead": true, "title": true, "video": true, "xmp": true,
}

// richTextSmuggled is DOMPurify's guard against markup smuggled through a
// comment: it removes, with its content, an element whose children are only
// text and comments, at least one of them a comment, when the text contains a
// "<" followed by a letter, a digit, "_", "/" or "!" (/<[/\w!]/).
func richTextSmuggled(n *html.Node) bool {
	comment := false
	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode:
			return false
		case html.CommentNode:
			comment = true
		case html.TextNode:
			text.WriteString(c.Data)
		}
	}
	if !comment {
		return false
	}
	t := text.String()
	for k := strings.IndexByte(t, '<'); k >= 0 && k+1 < len(t); {
		c := t[k+1]
		if c == '/' || c == '!' || c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' {
			return true
		}
		next := strings.IndexByte(t[k+1:], '<')
		if next < 0 {
			return false
		}
		k += 1 + next
	}
	return false
}

// blankRichTextRun reports whether a run of text, its character references
// already decoded, is blank: nothing but U+00A0 and what JavaScript's trim
// removes.
func blankRichTextRun(run string) bool {
	for _, r := range run {
		if r != '\u00A0' && !isJSWhiteSpace(r) {
			return false
		}
	}
	return true
}

// isJSWhiteSpace reports whether JavaScript's String.prototype.trim removes r
// (WhiteSpace and LineTerminator in ECMA-262). It is not unicode.IsSpace, which
// has U+0085 and lacks U+FEFF.
func isJSWhiteSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', '\uFEFF', '\u2028', '\u2029':
		return true
	}
	return unicode.Is(unicode.Zs, r)
}
