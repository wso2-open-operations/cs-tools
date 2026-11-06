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

// Command splitcatalogue writes the dashboard catalogue's parts for Choreo:
// internal/catalogue/catalogue.md cut by catalogue.Split into files of at most
// catalogue.MaxPartBytes, as catalogue-parts/catalogue-01.md, ... Run it from
// the service's root after editing catalogue.md (`make catalogue-parts`) and
// commit the parts; a test fails while they are out of date.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/catalogue"
)

// main writes the parts and prints each one's size, or the error and exit 1.
func main() {
	if err := run(catalogue.PartsDir); err != nil {
		fmt.Fprintln(os.Stderr, "splitcatalogue:", err)
		os.Exit(1)
	}
}

// run splits the built-in catalogue (the catalogue.md this binary was built
// from) and writes the parts to dir.
func run(dir string) error {
	parts, err := catalogue.Split(catalogue.DashboardEmbedded(), catalogue.MaxPartBytes)
	if err != nil {
		return err
	}
	names, err := catalogue.WriteParts(dir, parts)
	if err != nil {
		return err
	}
	for i, n := range names {
		fmt.Printf("%s  %6d bytes\n", filepath.Join(dir, n), len(parts[i]))
	}
	fmt.Printf("%d parts, each at most %d bytes\n", len(names), catalogue.MaxPartBytes)
	return nil
}
