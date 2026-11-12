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

package notifications

import (
	"strings"
	"testing"
)

func caseCreatedFor(caseType, reporter string) string {
	out, _ := RenderCaseCreatedEmail(CaseCreatedEmailData{
		ReporterName: reporter, ProjectName: "Proj", CaseNumber: "ANN-1", CaseTitle: "Planned maintenance",
		CaseType: caseType, CreatedAt: "2026-01-01", Description: "details", CaseLink: "https://x/case", CommentLink: "https://x/c",
	})
	return out
}

func TestRenderCaseCreatedEmail_AnnouncementHidesReporterName(t *testing.T) {
	for _, ct := range []string{"Announcement", "ANNOUNCEMENT", " announcement "} {
		out := caseCreatedFor(ct, "Jane Doe")
		if strings.Contains(out, "Jane Doe") {
			t.Errorf("%q: email contains the reporter name", ct)
		}
		if !strings.Contains(out, "A new <b>announcement</b> was published") {
			t.Errorf("%q: missing name-less headline", ct)
		}
		if strings.Contains(out, "Reported</td>") || strings.Contains(out, "<b>created</b> a case") {
			t.Errorf("%q: still has the Reported row or the named headline", ct)
		}
		if strings.Contains(out, "[BLOCK:") {
			t.Errorf("%q: unreplaced block marker", ct)
		}
	}
}

func TestRenderCaseCreatedEmail_NonAnnouncementKeepsReporterName(t *testing.T) {
	for _, ct := range []string{"Case", "Service Request", ""} {
		out := caseCreatedFor(ct, "Jane Doe")
		if strings.Count(out, "Jane Doe") != 2 {
			t.Errorf("%q: reporter name should appear in headline and Reported row, got %d", ct, strings.Count(out, "Jane Doe"))
		}
		if !strings.Contains(out, "<b>created</b> a case") || !strings.Contains(out, "Reported</td>") {
			t.Errorf("%q: headline or Reported row missing", ct)
		}
		if strings.Contains(out, "announcement</b> was published") {
			t.Errorf("%q: announcement headline leaked", ct)
		}
	}
}

func TestRenderAnnouncementCommentAddedEmail_HidesCommenterName(t *testing.T) {
	out, _ := RenderAnnouncementCommentAddedEmail("ANN-1", "Planned maintenance", "update", "https://x/c", "https://x/case", "")
	if strings.Contains(out, "Jane Doe") || strings.Contains(out, "commented on case") {
		t.Error("announcement comment email shows a commenter or the case wording")
	}
	if !strings.Contains(out, "New comment on announcement ANN-1") {
		t.Error("missing name-less headline")
	}
	if strings.Contains(out, "[BLOCK:") {
		t.Error("unreplaced block marker")
	}
}

func TestRenderCommentAddedEmail_StillShowsCommenterName(t *testing.T) {
	out, _ := RenderCommentAddedEmail("Jane Doe", "CS1", "Broke", "update", "https://x/c", "https://x/case", "")
	if !strings.Contains(out, "Jane Doe commented on case CS1") {
		t.Error("named headline missing")
	}
	if strings.Contains(out, "New comment on announcement") {
		t.Error("announcement headline leaked")
	}
}
