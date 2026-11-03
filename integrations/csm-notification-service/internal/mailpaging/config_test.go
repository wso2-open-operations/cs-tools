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

package mailpaging

import (
	"os"
	"path/filepath"
	"testing"
)

const loadDoc = `{"rules":{"r":{"levels":[{"name":"R1","emails":["a@example.com"]}]}}}`

func TestLoad_InlineJSONAndFile(t *testing.T) {
	inline, err := Load("  " + loadDoc)
	if err != nil || len(inline.Rules["r"].Levels) != 1 {
		t.Fatalf("inline: cfg=%+v err=%v", inline, err)
	}
	path := filepath.Join(t.TempDir(), "ladder.json")
	if err := os.WriteFile(path, []byte(loadDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile, err := Load(path)
	if err != nil || len(fromFile.Rules["r"].Levels) != 1 {
		t.Fatalf("file: cfg=%+v err=%v", fromFile, err)
	}
	if _, err := Load(`{"rules":`); err == nil {
		t.Fatal("malformed inline JSON must be refused")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("a missing file must be refused")
	}
}
