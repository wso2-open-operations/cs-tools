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

package repository

import "testing"

// TestKBManagerUserFKMessage: each foreign key 0251 puts on
// knowledge_base_manager_user names its own request field, and a constraint of
// any other name names both rather than guess.
func TestKBManagerUserFKMessage(t *testing.T) {
	for constraint, want := range map[string]string{
		"knowledge_base_manager_user_knowledge_base_id_fkey": "knowledgeBaseId does not refer to an existing knowledge base",
		"knowledge_base_manager_user_user_id_fkey":           "userId does not refer to an existing user",
		"fk_kb_manager_user_user":                            "knowledgeBaseId or userId does not refer to an existing record",
		"":                                                   "knowledgeBaseId or userId does not refer to an existing record",
	} {
		if got := kbManagerUserFKMessage(constraint); got != want {
			t.Errorf("kbManagerUserFKMessage(%q) = %q, want %q", constraint, got, want)
		}
	}
}
