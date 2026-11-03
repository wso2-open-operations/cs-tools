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

package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modelSDKs are module paths of model provider SDKs. The model calls belong
// to integrations/csm-ai-integration-service: this backend must hold no
// model client, so none of these may be required or imported here.
var modelSDKs = []string{"github.com/anthropics/", "github.com/openai/", "github.com/sashabaranov/go-openai", "google.golang.org/genai"}

func TestTheBackendHoldsNoModelSDK(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, sdk := range modelSDKs {
		if strings.Contains(string(mod), sdk) {
			t.Errorf("go.mod requires %s: model calls belong to the AI integration service", sdk)
		}
	}
	err = filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) && path != "../.." {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			for _, sdk := range modelSDKs {
				if strings.HasPrefix(strings.Trim(imp.Path.Value, `"`), sdk) {
					t.Errorf("%s imports %s: model calls belong to the AI integration service", path, imp.Path.Value)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
