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

// The email service trims the tail of what it is sent; a body ending in a bare
// "</div>" showed "</di" in the received email. Both outage emails must end the
// way every working template does.
func TestOutageEmailsEndLikeTheOtherTemplates(t *testing.T) {
	comm := RenderOutageCommunicationEmail(OutageCommunicationEmailData{PhaseWord: "Resolved", Message: "Hello Team,\nBest regards,\nSRE\n", Link: "https://csm.example/operations/outages/x"})
	notif := RenderOutageNotificationEmail(OutageNotificationEmailData{PhaseWord: "Declared", Number: "OUT1", Message: "m", Link: "https://csm.example/operations/outages/x"})
	for name, body := range map[string]string{"communication": comm, "notification": notif} {
		if !strings.HasSuffix(body, "</html>\n") {
			t.Errorf("%s email ends %q, want the template ending \"</html>\\n\"", name, body[max(0, len(body)-20):])
		}
	}
	for _, want := range []string{"<!DOCTYPE html", `alt="Logo"`, "Outage Resolved", "Hello Team,<br>", "View outage in the CSM portal", "WSO2 LLC. All Rights Reserved."} {
		if !strings.Contains(comm, want) {
			t.Errorf("communication email missing %q (the house shell or its content)", want)
		}
	}
	if strings.Contains(comm, "<!-- [") {
		t.Error("a template placeholder was left unreplaced")
	}
}
