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

import "testing"

func TestWrapCodeBlock(t *testing.T) {
	if wrapCodeBlock(nil) != nil {
		t.Fatal("expected nil in, nil out")
	}
	in := "<p>hello</p>"
	got := wrapCodeBlock(&in)
	want := "[code]<p>hello</p>[/code]"
	if got == nil || *got != want {
		t.Fatalf("got %v, want %q", got, want)
	}
}

func TestTrimCodeBlock(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strips a paired wrapper",
			in:   "[code]<p>hello</p>[/code]",
			want: "<p>hello</p>",
		},
		{
			name: "leaves an interior, unwrapped mention of the markers alone",
			in:   "Type [code] in the example",
			want: "Type [code] in the example",
		},
		{
			name: "leaves an unpaired opening marker alone",
			in:   "[code]no closing marker",
			want: "[code]no closing marker",
		},
		{
			name: "leaves an unpaired closing marker alone",
			in:   "no opening marker[/code]",
			want: "no opening marker[/code]",
		},
		{
			name: "strips only the outermost pair, keeping an interior mention",
			in:   "[code]see [code] inside[/code]",
			want: "see [code] inside",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimCodeBlock(&tt.in)
			if got == nil || *got != tt.want {
				t.Fatalf("got %v, want %q", got, tt.want)
			}
		})
	}

	if trimCodeBlock(nil) != nil {
		t.Fatal("expected nil in, nil out")
	}
}
