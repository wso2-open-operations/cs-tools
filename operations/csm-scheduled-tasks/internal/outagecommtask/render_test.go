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
package outagecommtask

import (
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecomm"
)

func TestRenderBodyLink(t *testing.T) {
	d := outagecomm.Decision{OutageID: "3ca9cf3d", Number: "OUT0010000", Body: "Hi Team,\n\nResolved <now>.\n\nBest regards,\nSRE\n"}

	with := renderBody(d, "https://csm-stg.apps.wso2.com/operations/outages/3ca9cf3d")
	if !strings.Contains(with, `href="https://csm-stg.apps.wso2.com/operations/outages/3ca9cf3d"`) {
		t.Fatalf("body is missing the outage link:\n%s", with)
	}
	if !strings.Contains(with, "Resolved &lt;now&gt;.") {
		t.Fatal("body text is no longer escaped")
	}
	if strings.Index(with, "SRE") > strings.Index(with, "View outage") {
		t.Fatal("link should follow the body, after the sign-off")
	}

	if without := renderBody(d, ""); strings.Contains(without, "View outage") {
		t.Fatalf("no link configured, but the body still carries one:\n%s", without)
	}
}
