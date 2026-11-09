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
package inlineimages

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
)

const (
	idA = "b1000000-0000-4000-8000-00000000000a"
	idB = "b1000000-0000-4000-8000-00000000000b"

	testParent = "c1000000-0000-4000-8000-000000000001"
)

var testScope = Scope{WorkItemID: testParent}

// fakeSource stands in for entity-service: it returns an image only when the record
// asked for is the one the image belongs to, as entity-service does.
type fakeSource struct {
	images map[string]fakeImage
	err    error
	calls  map[string]int
	asked  []string // the record each call named
}

type fakeImage struct {
	owner       string // the record it is linked to
	contentType string
	data        []byte
	// declared overrides Content-Length (-1 when the response did not say).
	declared *int64
}

func (f *fakeSource) ReadInlineImage(_ context.Context, id, workItemID string) (*entity.InlineImage, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[id]++
	f.asked = append(f.asked, workItemID)
	if f.err != nil {
		return nil, f.err
	}
	img, ok := f.images[id]
	if !ok || img.owner != workItemID {
		return nil, errors.New("not found")
	}
	length := int64(len(img.data))
	if img.declared != nil {
		length = *img.declared
	}
	return &entity.InlineImage{ContentType: img.contentType, ContentLength: length, Body: io.NopCloser(bytes.NewReader(img.data))}, nil
}

func img(ct string, data []byte) fakeImage {
	return fakeImage{owner: testParent, contentType: ct, data: data}
}

func dataURI(ct string, b []byte) string {
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func TestResolve_ReplacesAReferenceWithTheImage(t *testing.T) {
	png := []byte("png-bytes")
	src := &fakeSource{images: map[string]fakeImage{idA: img("image/png", png)}}
	r := New(src)

	for name, ref := range map[string]string{
		"bare reference":       "/" + idA,
		"content url":          "https://portal.example/v1.0/attachments/inline/" + idA + "/content",
		"relative content":     "attachments/inline/" + idA + "/content",
		"upper case id":        "/" + strings.ToUpper(idA),
		"migrated .iix suffix": "/" + idA + ".iix",
	} {
		got := r.Resolve(context.Background(), `<p>hi</p><img alt="x" src="`+ref+`">`, testScope)
		want := `<p>hi</p><img alt="x" src="` + dataURI("image/png", png) + `">`
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	// Every lookup named the record the email is about.
	for _, w := range src.asked {
		if w != testParent {
			t.Errorf("asked for record %q, want %q", w, testParent)
		}
	}
}

func TestResolve_LeavesOtherContentAlone(t *testing.T) {
	src := &fakeSource{}
	r := New(src)
	for _, html := range []string{
		"plain text",
		"<p>no image</p>",
		`<img src="https://example.com/x.png">`,
		`<img src="data:image/png;base64,AAAA">`,
		`<a href="/` + idA + `">link</a><img src="x.png">`,
	} {
		if got := r.Resolve(context.Background(), html, testScope); got != html {
			t.Errorf("changed %q to %q", html, got)
		}
	}
	if len(src.calls) != 0 {
		t.Errorf("looked something up: %v", src.calls)
	}
}

func TestResolve_AnUnreadableImageIsLeftAsAReference(t *testing.T) {
	html := `<img src="/` + idA + `">`
	unknown := int64(-1)
	for name, tc := range map[string]struct {
		src *fakeSource
	}{
		"entity-service error": {&fakeSource{err: errors.New("503")}},
		"not found":            {&fakeSource{}},
		"declared too large":   {&fakeSource{images: map[string]fakeImage{idA: {owner: testParent, contentType: "image/png", data: []byte("x"), declared: ptr(6 << 20)}}}},
		"not an allowed type":  {&fakeSource{images: map[string]fakeImage{idA: img("image/svg+xml", []byte("<svg/>"))}}},
		"octet-stream":         {&fakeSource{images: map[string]fakeImage{idA: img("application/octet-stream", []byte("x"))}}},
		"empty file":           {&fakeSource{images: map[string]fakeImage{idA: img("image/png", nil)}}},
		"length not declared and too large": {&fakeSource{images: map[string]fakeImage{idA: {owner: testParent, contentType: "image/png",
			data: bytes.Repeat([]byte("x"), maxImageBytes+10), declared: &unknown}}}},
	} {
		if got := New(tc.src).Resolve(context.Background(), html, testScope); got != html {
			t.Errorf("%s: got %q, want the reference kept", name, got)
		}
	}
}

func ptr(n int64) *int64 { return &n }

func TestResolve_AFileLargerThanItsDeclaredSizeIsRefused(t *testing.T) {
	// The response declares 4 bytes; the body is bigger than the cap.
	huge := bytes.Repeat([]byte("x"), maxImageBytes+10)
	src := &fakeSource{images: map[string]fakeImage{idA: {owner: testParent, contentType: "image/png", data: huge, declared: ptr(4)}}}
	html := `<img src="/` + idA + `">`
	if got := New(src).Resolve(context.Background(), html, testScope); got != html {
		t.Errorf("a file over the cap was embedded")
	}
}

func TestResolve_TheSameImageIsReadOnce(t *testing.T) {
	src := &fakeSource{images: map[string]fakeImage{idA: img("image/png", []byte("abc"))}}
	html := `<img src="/` + idA + `"><br><img src="/` + idA + `">`
	got := New(src).Resolve(context.Background(), html, testScope)
	if strings.Count(got, "data:image/png;base64,") != 2 || src.calls[idA] != 1 {
		t.Errorf("got %q, lookups %d", got, src.calls[idA])
	}
}

func TestResolve_StopsAtTheImageCountAndTheTotalSize(t *testing.T) {
	images := map[string]fakeImage{}
	var html strings.Builder
	for i := 0; i < 7; i++ {
		id := "b1000000-0000-4000-8000-00000000010" + string(rune('0'+i))
		images[id] = img("image/png", []byte("0123456789"))
		html.WriteString(`<img src="/` + id + `">`)
	}
	got := New(&fakeSource{images: images}).Resolve(context.Background(), html.String(), testScope)
	if n := strings.Count(got, "data:image/png;base64,"); n != maxImages {
		t.Errorf("%d images resolved, want %d", n, maxImages)
	}

	// Two images of 5 MiB: the second would take the body past the total.
	five := bytes.Repeat([]byte("y"), maxImageBytes)
	two := New(&fakeSource{images: map[string]fakeImage{idA: img("image/png", five), idB: img("image/png", five)}})
	got = two.Resolve(context.Background(), `<img src="/`+idA+`"><img src="/`+idB+`">`, testScope)
	if n := strings.Count(got, "data:image/png;base64,"); n != 1 {
		t.Errorf("%d images resolved, want 1 within the total cap", n)
	}
}

func TestResolve_NilResolverIsANoOp(t *testing.T) {
	var r *Resolver
	html := `<img src="/` + idA + `">`
	if got := r.Resolve(context.Background(), html, testScope); got != html {
		t.Errorf("got %q", got)
	}
}

// A body of unreadable references stops after maxLookups lookups, and leaves the
// rest as references.
func TestResolve_BoundsLookupsOfUnreadableImages(t *testing.T) {
	src := &fakeSource{err: errors.New("gone")}
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, `<img src="/%08x-0000-0000-0000-000000000000">`, i)
	}
	New(src).Resolve(context.Background(), b.String(), testScope)
	total := 0
	for _, n := range src.calls {
		total += n
	}
	if total != maxLookups {
		t.Fatalf("lookups = %d, want %d", total, maxLookups)
	}
}

// An image of another record is not put into the email, however valid its id:
// the record is named in the request, and entity-service answers only for its own.
func TestResolve_RefusesAnImageOfAnotherRecord(t *testing.T) {
	html := `<img src="/` + idA + `">`
	other := fakeImage{owner: "c1000000-0000-4000-8000-000000000002", contentType: "image/png", data: []byte("abcd")}
	src := &fakeSource{images: map[string]fakeImage{idA: other}}
	if got := New(src).Resolve(context.Background(), html, testScope); got != html {
		t.Errorf("got %q, want the reference left", got)
	}
	if len(src.asked) != 1 || src.asked[0] != testParent {
		t.Errorf("asked for %v, want the record the email is about (%q)", src.asked, testParent)
	}
}

// With no record to check against, entity-service would not filter at all, so the
// resolver must not ask.
func TestResolve_NoRecordMeansNoLookup(t *testing.T) {
	src := &fakeSource{images: map[string]fakeImage{idA: img("image/png", []byte("abcd"))}}
	html := `<img src="/` + idA + `">`
	if got := New(src).Resolve(context.Background(), html, Scope{}); got != html {
		t.Errorf("got %q, want the reference left", got)
	}
	if len(src.calls) != 0 {
		t.Errorf("entity-service was asked without a record: %v", src.calls)
	}
}

// Only an img's own src is rewritten, not data-src or similar attributes.
func TestResolve_LeavesDataSrcAlone(t *testing.T) {
	html := `<img data-src="/` + idA + `"><img xsrc="/` + idA + `">`
	src := &fakeSource{images: map[string]fakeImage{idA: img("image/png", []byte("abcd"))}}
	if got := New(src).Resolve(context.Background(), html, testScope); got != html || src.calls[idA] != 0 {
		t.Errorf("got %q, lookups %d, want the tags untouched and no lookup", got, src.calls[idA])
	}
}
