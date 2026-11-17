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
	"testing"
	"time"
)

// Every expectation below is what the CSM webapp's isBlankHtml
// (apps/csm-portal/webapp/src/utils/sanitizeHtml.ts) returned for the input,
// run in jsdom with the DOMPurify 3.4.16 the webapp pins: the backend and the
// webapp must agree on what is blank, so these are not hand-written answers. A
// case added here should be checked against isBlankHtml the same way.
func TestIsBlankRichText(t *testing.T) {
	for _, c := range []struct {
		in    string
		blank bool
	}{
		{"", true},
		{" ", true},
		{"\n\t \r\n", true},
		{"\u00A0", true},
		{"\u3000\uFEFF", true},
		{"\u0085", false},
		{"\u200B", false},
		{"\u0000", true},
		{" \u0000 ", true},
		{"x", false},
		{"<p>Restarted</p>", false},
		{" <p> a </p> ", false},
		{"<p><br></p>", true},
		{"<p><br/></p>", true},
		{"<p></p>", true},
		{"<P><BR></P>", true},
		{"<p>&nbsp;</p>", true},
		{"<p> &nbsp; </p>", true},
		{"<p>&#160;</p>", true},
		{"<p>&#xA0;</p>", true},
		{"&nbsp;", true},
		{"&nbsp", true},
		{"&nbspx", false},
		{"<p>&#65279;</p>", true},
		{"<p>&#32;</p>", true},
		{"<p>&#0;</p>", false},
		{"<ul><li><br></li></ul>", true},
		{"<ol><li></li><li> </li></ol>", true},
		{"<div><span>  </span></div>", true},
		{"<p><img src=\"https://example.com/a.png\"></p>", true},
		{"<img src=x alt=\"words\">", true},
		{"<p><img src=x>caption</p>", false},
		{"<!-- a comment -->", true},
		{"<p><!-- x --></p>", true},
		{"<!---->", true},
		{"<!-->", true},
		{"<!--->", true},
		{"<!-- open", true},
		{"<!--x--!>", true},
		{"<!--a-->b", false},
		{"<script>alert(1)</script>", true},
		{"<script>if (a</b) {}</script>", true},
		{"<STYLE>p{}</STYLE>", true},
		{"<style>p{color:red}</style>after", false},
		{"<script>x</scriptx>y", true},
		{"<p><svg><text>drawn</text></svg></p>", true},
		{"<svg/>after", false},
		{"<math><mi>x</mi></math>", true},
		{"<desc>d</desc>", true},
		{"<template>hidden</template>", true},
		{"<title>T</title>", true},
		{"<noscript>n</noscript>", false},
		{"<p>a</p><noscript>n</noscript>", false},
		{"<video>fallback</video>", true},
		{"<audio>fallback</audio>", true},
		{"<iframe>x</iframe>", true},
		{"<xmp><b>x</b></xmp>", true},
		{"<plaintext>everything", true},
		{"<p></p><plaintext>everything", true},
		{"<textarea>typed</textarea>", false},
		{"<textarea>&nbsp;</textarea>", true},
		{"<p>a</p><textarea>typed</textarea>", false},
		{"<table><thead><tr><th>Header</th></tr></thead></table>", true},
		{"<thead><tr><th>Header</th></tr></thead>", false},
		{"<table><thead><tr><th>H</th></tr></thead><tbody><tr><td>x</td></tr></tbody></table>", false},
		{"<table><thead><tr><th>H</th></tr><tbody><tr><td>x</td></tr></tbody></table>", false},
		{"<table><thead><tr><th>H</th></tr></table>after", false},
		{"<p title=\"a > b\"></p>", true},
		{"<p title=\"a > b\">t</p>", false},
		{"<p title='x>y'></p>", true},
		{"<p", true},
		{"<p>hello <b", false},
		{"</p>", true},
		{"</>", true},
		{"</ p>", true},
		{"</", false},
		{"<", false},
		{"< p>", false},
		{"<>", false},
		{"<3", false},
		{"a < b", false},
		{"x &lt; y", false},
		{"<!DOCTYPE html>", true},
		{"<?xml version=\"1.0\"?>", true},
		{"<p>&amp;</p>", false},
		{"<p>&lt;br&gt;</p>", false},
		{"<p>&#x41;</p>", false},
		{"<p><br></p><p>Used the standby.</p>", false},
		{"<p>\u200B</p>", false},
		{"<p>\u0085</p>", false},
		{"<p>\u2028</p>", true},
		{"<p>\u1680\u2000\u205F</p>", true},
		{"<p><strong></strong><em> </em></p>", true},
		{"<h1></h1><blockquote><p><br></p></blockquote>", true},
		{"<pre>  </pre>", true},
		{"<code>x</code>", false},
		{"<a href=\"https://example.com\"></a>", true},
		{"<a href=\"https://example.com\">link</a>", false},
		{"<p></p><noscript>n</noscript>", true},
		{"<noscript><p>n</p></noscript>", false},
		{"<meta charset=utf-8><noscript>n</noscript>", false},
		{"</head><noscript>n</noscript>", true},
		{"<template>x</template><noscript>n</noscript>", false},
		{"<head></head><noscript>n</noscript>", true},
		{"<html><noscript>n</noscript>", false},
		{"<body><noscript>n</noscript>", true},
		{" \n<noscript>n</noscript>", false},
		{"<!-- c --><noscript>n</noscript>", false},
		{"<noscript><style>x</style></noscript>", true},
		{"&nbsp;<noscript>n</noscript>", true},
		{"\u0000<noscript>n</noscript>", true},
		{"<br><noscript>n</noscript>", true},
		{"</br><noscript>n</noscript>", true},
		{"</div><noscript>n</noscript>", false},
		{"<title>t</title><noscript>n</noscript>", false},
		{"<svg><p>x</p></svg>", false},
		{"<math><p>x</p></math>", false},
		{"<svg><g><p>x</p></g></svg>", false},
		{"<video><svg><p>x</p></svg></video>", true},
		{"<svg><foreignObject><div>x</div></foreignObject></svg>", true},
		{"<svg><desc><p>x</p></desc></svg>", true},
		{"<math><mi><p>x</p></mi></math>", true},
		{"<svg><style>a</svg>b</style>", false},
		{"<svg><title>t</title></svg>after", false},
		{"<svg><font color=red>x</font></svg>", false},
		{"<svg><font>x</font></svg>", true},
		{"<svg></p>x</svg>", false},
		{"<svg><span>x</span></svg>", false},
		{"<svg><a>x</a></svg>", true},
		{"<svg><video><p>x</p></video></svg>", false},
		{"<svg><svg><p>x</p></svg></svg>", false},
		{"<a =\">\">x", false},
		{"<svg x=y/>after", true},
		{"<svg x=\"y\"/>after", false},
		{"<svg / >after", true},
		{"<p class=a>t</p>", false},
		{"<p data-x=\"<b>\"></p>", true},
		{"<font color=red>x</font>", false},
		{"<svg><font face=\"x\">t</font></svg>", false},
		{"<svg><font title=\" color=x\">t</font></svg>", true},
		// A tag the end of the text cuts off is no tag at all.
		{"<u><p>&lt;1<?pi?><text a ", true},
		// DOMPurify drops an element whose children are only text and comments,
		// a comment among them, when the text has a "<" followed by a letter, a
		// digit, "_", "/" or "!": only once its children are known (its end, an
		// ancestor's end, a tag that ends it, the end of the text), and never the
		// body itself.
		{"<h1><a href='x>y'><!-- c -->&lt;b&gt;</h1>", true},
		{"<thead><p>&lt;b&gt;<!-- x -- y --></p>&emsp;<code></code></thead>\v", true},
		{"<p><!-- c -->&lt;b&gt;</p>", true},
		{"<p>&lt;b&gt;<!-- c --></p>", true},
		{"<p><!-- c -->&lt;b&gt;", true},
		{"<p>&lt;<!-- c -->b</p>", true},
		{"<p><!-- c -->&lt;/x</p>", true},
		{"<p><!-- c -->&lt;!</p>", true},
		{"<p><!-- c -->&lt;1</p>", true},
		{"<p><!-- c -->&lt;_</p>", true},
		{"<p><!-- c -->&lt;\x00b</p>", true},
		{"<p><!-- c -->a &lt; b</p>", false},
		{"<p><!-- c -->&lt;\u00E9</p>", false},
		{"<p><?pi?>&lt;b</p>", true},
		{"<p><!doctype html>&lt;b<!-- c --></p>", true},
		{"<p><!-- c -->&lt;b&gt;<br></p>", false},
		{"<p><!-- c -->&lt;b&gt;</br></p>", false},
		{"<p><!-- c -->&lt;b&gt;<div>after</div>", false},
		{"<p><!-- c -->&lt;b&gt;<div></div>", true},
		{"<ul><li><!-- c -->&lt;b&gt;<li>next</ul>", false},
		{"<ul><li><!-- c -->&lt;b&gt;<li></ul>", true},
		{"<li><p><!-- c -->&lt;b<li>", true},
		{"<span><!-- c -->&lt;b</p>", false},
		{"<p><span><!-- c -->&lt;b</p>", true},
		{"<div><!-- c -->&lt;b</span>", true},
		{"<span><p><!-- c -->&lt;b</span>", true},
		{"<b><!-- c -->&lt;b</i>", true},
		{"<b><!-- c -->&lt;b</i><br>", false},
		{"<p><b>x</p><p><!-- c -->&lt;b</p>", false},
		{"&lt;b<!-- c -->", false},
		{"<!-- c -->&lt;b", false},
		{"<h3><!-- c -->&amp;</h2>&lt;b&gt;", false},
		{"<h2><!-- c -->&lt;b</h3>", true},
		{"<p><!-- c -->&lt;b</body><!-- d -->", true},
		{"<p>&lt;b</body><!-- d -->", false},
		{"<table><tr><td><!-- c -->&lt;b<td>z</table>", false},
		{"<table><tr><td><!-- c -->&lt;b<td></table>", true},
		{"<table><!-- c -->&lt;b</table>", false},
		{"<td><!-- c -->&lt;b</td>", false},
		{"<p><!-- c -->&lt;b<table></table></p>", false},
		{"<b><p><!-- c -->&lt;x</b>", true},
		{"<b><p><!-- c -->x</b>&lt;y", false},
		{"<select><option><!-- c -->&lt;b</option></select>", true},
		{"<p><!-- c --><!-- d -->&lt;b&gt;</p><p>Used the standby.</p>", false},
	} {
		if got := IsBlankRichText(c.in); got != c.blank {
			t.Errorf("IsBlankRichText(%q) = %v, want %v (isBlankHtml's answer)", c.in, got, c.blank)
		}
	}
}

// The cases the deviation's blank rule exists for, named.
func TestIsBlankRichText_EmptyEditorMarkup(t *testing.T) {
	for _, blank := range []string{"", "   ", "\n\t", "<p><br></p>", "<p>&nbsp;</p>", "<p> &nbsp; </p>", "<ul><li><br></li></ul>",
		// isBlankHtml strips img tags like any other: an image alone is no text.
		"<p><img src=\"https://example.com/diagram.png\"></p>",
	} {
		if !IsBlankRichText(blank) {
			t.Errorf("IsBlankRichText(%q) = false, want true", blank)
		}
	}
	for _, text := range []string{"x", "<p>Restarted the nodes one at a time.</p>", "<p><br></p><p>Used the standby.</p>", "<p><img src=x>caption</p>"} {
		if IsBlankRichText(text) {
			t.Errorf("IsBlankRichText(%q) = true, want false", text)
		}
	}
}

// IsBlankRichText runs under a change request's row locks on input up to the 1
// MiB request body cap, and nothing can stop it once it runs, so it must take
// time linear in its input. These shapes are the ones that make a parser
// quadratic when a lookup walks what is open; parsed in linear time, each
// finishes far inside the limit, which leaves room for a slow or
// race-instrumented build.
func TestIsBlankRichText_LinearTime(t *testing.T) {
	const size = 1 << 20
	repeat := func(unit string, n int) string { return strings.Repeat(unit, n/len(unit)) }
	half := func(a, b string) string { return repeat(a, size/2) + repeat(b, size/2) }
	for _, c := range []struct {
		name string
		in   string
	}{
		{"comments", repeat("<!--a-->", size)},
		{"comments closed by --!>", repeat("<!--a--!>", size)},
		{"open dropped elements, then other end tags", half("<video>", "</p>")},
		{"open dropped elements, then table sections", half("<video>", "<tbody>")},
		{"table headers, then table ends", half("<table><thead>", "</table>")},
		{"open formatting elements, then end tags of none", half("<b>", "</i>")},
		{"open inline elements, then their end tags behind a block", "<div>" + half("<b><span>", "</span>")},
		{"list items behind a block", "<li><div>" + half("<b>", "<li></li>")},
		{"headings, then other headings' end tags", half("<h1>", "</h2>")},
		{"paragraphs", repeat("<p>", size)},
		{"paragraphs of bold text", repeat("<p><b>", size)},
		{"bold text across paragraphs", repeat("<b><p></b>", size)},
		{"links", repeat("<a>", size)},
		{"table cells", "<table><tr>" + repeat("<td>", size)},
		{"one element's text and comments", "<p>" + repeat("&lt;b<!--c-->", size)},
		{"character references", repeat("&amp;", size)},
		{"text", repeat("a ", size)},
	} {
		start := time.Now()
		IsBlankRichText(c.in)
		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("%s: IsBlankRichText of %d bytes took %v, want under 2s", c.name, len(c.in), took)
		}
	}
}

// The parser refuses elements nested more than 512 deep. Such a value is kept as
// written, not judged blank: the editor never writes it, and dropping it would
// lose what an API caller sent.
func TestIsBlankRichText_TooDeepIsNotBlank(t *testing.T) {
	if IsBlankRichText(strings.Repeat("<b>", 600) + "x") {
		t.Error("IsBlankRichText of 600 nested elements = true, want false")
	}
	if IsBlankRichText(strings.Repeat("<div>", 600)) {
		t.Error("IsBlankRichText of 600 nested empty elements = true, want false (kept as written)")
	}
}
