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
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const (
	idA = "b1000000-0000-4000-8000-00000000000a"
	idB = "b1000000-0000-4000-8000-00000000000b"
)

type fakeLocator struct {
	locs  map[string]entity.InlineImageLocation
	err   error
	calls map[string]int
}

func (f *fakeLocator) LocateInlineImage(_ context.Context, id string) (entity.InlineImageLocation, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[id]++
	if f.err != nil {
		return entity.InlineImageLocation{}, f.err
	}
	loc, ok := f.locs[id]
	if !ok {
		return entity.InlineImageLocation{}, errors.New("not found")
	}
	return loc, nil
}

type fakeReader struct {
	files map[string][]byte
	opens int
}

func (f *fakeReader) Open(_ context.Context, key string, _ *azblobstore.Range) (*azblobstore.Object, error) {
	f.opens++
	b, ok := f.files[key]
	if !ok {
		return nil, azblobstore.ErrNotFound
	}
	return &azblobstore.Object{Body: io.NopCloser(bytes.NewReader(b)), ContentLength: int64(len(b))}, nil
}

func loc(key, ct string, size int64) entity.InlineImageLocation {
	return entity.InlineImageLocation{Available: true, StorageKey: key, ContentType: &ct, SizeBytes: &size}
}

func dataURI(ct string, b []byte) string {
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func TestResolve_ReplacesAReferenceWithTheImage(t *testing.T) {
	png := []byte("png-bytes")
	r := New(&fakeLocator{locs: map[string]entity.InlineImageLocation{idA: loc("k/a.png", "image/png", int64(len(png)))}},
		&fakeReader{files: map[string][]byte{"k/a.png": png}})

	for name, src := range map[string]string{
		"bare reference":       "/" + idA,
		"content url":          "https://portal.example/v1.0/attachments/inline/" + idA + "/content",
		"relative content":     "attachments/inline/" + idA + "/content",
		"upper case id":        "/" + strings.ToUpper(idA),
		"migrated .iix suffix": "/" + idA + ".iix",
	} {
		got := r.Resolve(context.Background(), `<p>hi</p><img alt="x" src="`+src+`">`)
		want := `<p>hi</p><img alt="x" src="` + dataURI("image/png", png) + `">`
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestResolve_LeavesOtherContentAlone(t *testing.T) {
	loc := &fakeLocator{}
	rd := &fakeReader{}
	r := New(loc, rd)
	for _, html := range []string{
		"plain text",
		"<p>no image</p>",
		`<img src="https://example.com/x.png">`,
		`<img src="data:image/png;base64,AAAA">`,
		`<a href="/` + idA + `">link</a><img src="x.png">`,
	} {
		if got := r.Resolve(context.Background(), html); got != html {
			t.Errorf("changed %q to %q", html, got)
		}
	}
	if len(loc.calls) != 0 || rd.opens != 0 {
		t.Errorf("looked something up: %v, %d opens", loc.calls, rd.opens)
	}
}

func TestResolve_AnUnreadableImageIsLeftAsAReference(t *testing.T) {
	html := `<img src="/` + idA + `">`
	big := loc("k/big.png", "image/png", 6<<20)
	svg := loc("k/a.svg", "image/svg+xml", 10)
	gone := entity.InlineImageLocation{Available: false}
	for name, tc := range map[string]struct {
		loc   entity.InlineImageLocation
		err   error
		files map[string][]byte
	}{
		"entity-service error": {err: errors.New("503")},
		"not available":        {loc: gone},
		"too large":            {loc: big, files: map[string][]byte{"k/big.png": []byte("x")}},
		"not an allowed type":  {loc: svg, files: map[string][]byte{"k/a.svg": []byte("<svg/>")}},
		"file missing":         {loc: loc("k/none.png", "image/png", 5)},
	} {
		r := New(&fakeLocator{locs: map[string]entity.InlineImageLocation{idA: tc.loc}, err: tc.err}, &fakeReader{files: tc.files})
		if got := r.Resolve(context.Background(), html); got != html {
			t.Errorf("%s: got %q, want the reference kept", name, got)
		}
	}
}

func TestResolve_AFileLargerThanItsDeclaredSizeIsRefused(t *testing.T) {
	// Entity-service declared 4 bytes; the file is bigger than the cap.
	huge := bytes.Repeat([]byte("x"), maxImageBytes+10)
	r := New(&fakeLocator{locs: map[string]entity.InlineImageLocation{idA: loc("k/a.png", "image/png", 4)}},
		&fakeReader{files: map[string][]byte{"k/a.png": huge}})
	html := `<img src="/` + idA + `">`
	if got := r.Resolve(context.Background(), html); got != html {
		t.Errorf("a file over the cap was embedded")
	}
}

func TestResolve_TheSameImageIsReadOnce(t *testing.T) {
	lc := &fakeLocator{locs: map[string]entity.InlineImageLocation{idA: loc("k/a.png", "image/png", 3)}}
	rd := &fakeReader{files: map[string][]byte{"k/a.png": []byte("abc")}}
	html := `<img src="/` + idA + `"><br><img src="/` + idA + `">`
	got := New(lc, rd).Resolve(context.Background(), html)
	if strings.Count(got, "data:image/png;base64,") != 2 || lc.calls[idA] != 1 || rd.opens != 1 {
		t.Errorf("got %q, lookups %d, opens %d", got, lc.calls[idA], rd.opens)
	}
}

func TestResolve_StopsAtTheImageCountAndTheTotalSize(t *testing.T) {
	locs := map[string]entity.InlineImageLocation{}
	files := map[string][]byte{}
	var html strings.Builder
	for i := 0; i < 7; i++ {
		id := "b1000000-0000-4000-8000-00000000010" + string(rune('0'+i))
		locs[id] = loc("k/"+id, "image/png", 10)
		files["k/"+id] = []byte("0123456789")
		html.WriteString(`<img src="/` + id + `">`)
	}
	got := New(&fakeLocator{locs: locs}, &fakeReader{files: files}).Resolve(context.Background(), html.String())
	if n := strings.Count(got, "data:image/png;base64,"); n != maxImages {
		t.Errorf("%d images resolved, want %d", n, maxImages)
	}

	// Two images of 5 MiB: the second would take the body past the total.
	five := bytes.Repeat([]byte("y"), maxImageBytes)
	two := New(&fakeLocator{locs: map[string]entity.InlineImageLocation{
		idA: loc("k/a", "image/png", int64(len(five))), idB: loc("k/b", "image/png", int64(len(five))),
	}}, &fakeReader{files: map[string][]byte{"k/a": five, "k/b": five}})
	got = two.Resolve(context.Background(), `<img src="/`+idA+`"><img src="/`+idB+`">`)
	if n := strings.Count(got, "data:image/png;base64,"); n != 1 {
		t.Errorf("%d images resolved, want 1 within the total cap", n)
	}
}

func TestResolve_NilResolverIsANoOp(t *testing.T) {
	var r *Resolver
	html := `<img src="/` + idA + `">`
	if got := r.Resolve(context.Background(), html); got != html {
		t.Errorf("got %q", got)
	}
}

// A body of unreadable references stops after maxLookups lookups, and leaves the
// rest as references.
func TestResolve_BoundsLookupsOfUnreadableImages(t *testing.T) {
	loc := &countingLocator{}
	r := New(loc, nil)
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, `<img src="/%08x-0000-0000-0000-000000000000">`, i)
	}
	r.Resolve(context.Background(), b.String())
	if loc.calls != maxLookups {
		t.Fatalf("lookups = %d, want %d", loc.calls, maxLookups)
	}
}

type countingLocator struct{ calls int }

func (c *countingLocator) LocateInlineImage(context.Context, string) (entity.InlineImageLocation, error) {
	c.calls++
	return entity.InlineImageLocation{}, errors.New("gone")
}
