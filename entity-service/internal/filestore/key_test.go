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

package filestore

import (
	"strings"
	"testing"
)

func TestKey_MatchesTheMigratorLayout(t *testing.T) {
	tests := []struct {
		name string
		in   KeyParts
		want string
	}{
		{"work item file", KeyParts{ProjectNumber: "PRJ-001", WorkItemNumber: "CS0439849", AttachmentNumber: "WIA0000001", Name: "Log.PDF"},
			"csm/projects/prj_001/workitems/cs0439849/attachments/wia0000001_Log.PDF"},
		{"inline image", KeyParts{ProjectNumber: "PRJ-001", WorkItemNumber: "CS0439849", Inline: true, AttachmentNumber: "CA0000002", Name: "image.png"},
			"csm/projects/prj_001/workitems/cs0439849/inline_attachments/ca0000002_image.png"},
		{"deployment file", KeyParts{ProjectNumber: "PRJ-001", DeploymentNumber: "DEP-7", AttachmentNumber: "DA0000003", Name: "cfg.zip"},
			"csm/projects/prj_001/deployments/dep_7/da0000003_cfg.zip"},
		{"no project", KeyParts{WorkItemNumber: "CS1", AttachmentNumber: "WIA0000004", Name: "a.txt"},
			"csm/default/workitems/cs1/attachments/wia0000004_a.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Key(tt.in); got != tt.want {
				t.Errorf("Key = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeName_CannotEscapeItsFolder(t *testing.T) {
	for _, in := range []string{"../../etc/passwd", `..\..\win.ini`, "a/b/c.txt", "..", ".", "", "  ", "x\x00y.txt", `we:ird*na?me.txt`} {
		got := SanitizeName(in)
		if strings.ContainsAny(got, `/\`) || strings.Contains(got, "..") || got == "" || got == "." {
			t.Errorf("SanitizeName(%q) = %q, not a safe single path segment", in, got)
		}
	}
	if got := SanitizeName("report.final.PDF"); got != "report.final.PDF" {
		t.Errorf("a normal name changed: %q", got)
	}
}

func TestSanitizeName_LongNameKeepsItsExtension(t *testing.T) {
	got := SanitizeName(strings.Repeat("a", 400) + ".pdf")
	if len(got) > maxNameBytes || !strings.HasSuffix(got, ".pdf") {
		t.Errorf("long name = %q (len %d)", got, len(got))
	}
}

func TestKey_HostileNamesStayInsideTheAttachmentFolder(t *testing.T) {
	k := Key(KeyParts{ProjectNumber: "P1", WorkItemNumber: "W1", AttachmentNumber: "WIA1", Name: "../../../x.txt"})
	if !strings.HasPrefix(k, "csm/projects/p1/workitems/w1/attachments/") || strings.Contains(k, "..") {
		t.Errorf("Key escaped its folder: %q", k)
	}
}

func TestStagingDir_IsOneFolderPerAttachment(t *testing.T) {
	a := StagingDir("0190ab12-0000-7000-8000-000000000001")
	b := StagingDir("0190ab12-0000-7000-8000-000000000002")
	if a == b {
		t.Fatal("two attachments share a staging folder")
	}
	if !strings.HasPrefix(a, "csm/staging/") || strings.Contains(a, "..") {
		t.Errorf("StagingDir = %q", a)
	}
}
