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
	"path/filepath"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/catalogue"
)

// TestRunWritesPartsThatLoad checks that the command writes parts the service
// loads back into the built-in catalogue, and fails on a path it cannot use.
func TestRunWritesPartsThatLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), catalogue.PartsDir)
	if err := run(dir); err != nil {
		t.Fatal(err)
	}
	s, err := catalogue.NewDashboard(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Text() != catalogue.DashboardEmbedded() || len(s.Parts()) < 2 {
		t.Errorf("parts %v do not load back into the built-in catalogue", s.Parts())
	}
	if err := run(filepath.Join(dir, "catalogue-01.md", "sub")); err == nil {
		t.Error("a directory under a file: no error")
	}
}
