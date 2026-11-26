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

package notify

import (
	"strings"
	"testing"
)

// A support group with no rotation name shows only its hours under its
// people; one with a name shows "name, hours".
func TestRenderWeekendRota_SupportGroupLabel(t *testing.T) {
	render := func(rotation string) string {
		return RenderWeekendRota(WeekendRotaData{
			TeamTitle: "CRE Team", SupportTitle: "SRE Team",
			Days: []WeekendRotaDay{{Title: "Saturday, 24 October", Rows: []WeekendRotaRow{{
				Rotation: "LK Weekend", Hours: "06:00 AM – 09:00 PM IST", Team: []string{"Ann"},
				Support: []WeekendRotaGroup{{Rotation: rotation, Hours: "06:00 AM – 06:00 PM IST", People: []string{"Kim (L1)", "Lee (L2)"}}},
			}}}},
		})
	}
	unnamed := render("")
	if !strings.Contains(unnamed, `Kim (L1)<br/>Lee (L2)<br/><span style="color:#555555; font-size:12px;">06:00 AM &#8211; 06:00 PM IST</span>`) {
		t.Errorf("unnamed group: want the people and the hours only:\n%s", unnamed)
	}
	if strings.Contains(unnamed, ">, 06:00") {
		t.Errorf("unnamed group starts its label with a comma:\n%s", unnamed)
	}
	if named := render("SRE Night"); !strings.Contains(named, ">SRE Night, 06:00 AM &#8211; 06:00 PM IST<") {
		t.Errorf("named group: want \"name, hours\":\n%s", named)
	}
}
