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

package flows

// All returns every flow registered to run in production, in evaluation order.
//
// It is intentionally empty in the scaffold: a flow starts firing only when it
// is added here, and adding one is a paired change with disabling its
// ServiceNow (and any hardcoded Go) counterpart in the SAME commit — the
// double-fire guard (docs/cutover-porting-plan.md §7). Add each flow as it is
// ported, following the Band order in the plan's §11:
//
//	func All() []Flow {
//	    return []Flow{
//	        &watchList{},            // Band 1 — reference flow
//	        &commentNotifications{}, // Band 1 — folds the four comment flows
//	        // ...
//	    }
//	}
func All() []Flow {
	return []Flow{}
}
