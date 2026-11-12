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

import "testing"

func TestLoadLogCollectionConfig_UnsetIsOff(t *testing.T) {
	_, enabled, err := loadLogCollectionConfig(aiEnv(map[string]string{"LOG_COLLECTION_REPO": "example-org/pipelines"}))
	if err != nil || enabled {
		t.Errorf("enabled = %v, err = %v; want off without an error", enabled, err)
	}
}

func TestLoadLogCollectionConfig_ReadsEveryVariable(t *testing.T) {
	cfg, enabled, err := loadLogCollectionConfig(aiEnv(map[string]string{
		"LOG_COLLECTION_CATALOG_ITEM_ID": " 9d48f18f-0ae3-466b-b0ab-fdd14038864b ",
		"LOG_COLLECTION_REPO":            " example-org/pipelines ",
		"LOG_COLLECTION_TEAM":            "sre",
	}))
	if err != nil || !enabled {
		t.Fatalf("enabled = %v, err = %v", enabled, err)
	}
	if cfg.CatalogItemID != "9d48f18f-0ae3-466b-b0ab-fdd14038864b" || cfg.Owner != "example-org" || cfg.Repo != "pipelines" || cfg.Team != "sre" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestLoadLogCollectionConfig_RejectsBadValues(t *testing.T) {
	const item = "9d48f18f-0ae3-466b-b0ab-fdd14038864b"
	for name, env := range map[string]map[string]string{
		"item id not a UUID": {"LOG_COLLECTION_CATALOG_ITEM_ID": "component-log-request", "LOG_COLLECTION_REPO": "example-org/pipelines"},
		"repo unset":         {"LOG_COLLECTION_CATALOG_ITEM_ID": item},
		"repo without owner": {"LOG_COLLECTION_CATALOG_ITEM_ID": item, "LOG_COLLECTION_REPO": "pipelines"},
		"repo with a path":   {"LOG_COLLECTION_CATALOG_ITEM_ID": item, "LOG_COLLECTION_REPO": "example-org/pipelines/extra"},
		"repo as a URL":      {"LOG_COLLECTION_CATALOG_ITEM_ID": item, "LOG_COLLECTION_REPO": "https://github.com/example-org/pipelines"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := loadLogCollectionConfig(aiEnv(env)); err == nil {
				t.Error("want an error")
			}
		})
	}
}
