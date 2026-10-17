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

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// maxUsageReportBytes caps how much of an uploaded report is decompressed here.
// The ZIP itself is capped at maxZipUploadBytes; this bounds what it may expand to,
// so a crafted archive cannot exhaust memory.
const maxUsageReportBytes = 100 << 20 // 100 MiB

var errNotUsageReport = errors.New("not a usage report exported by a WSO2 product")

// usageReportDeploymentNumber reads the deployment number from an offline usage
// report: a ZIP holding exactly one JSON file whose export_metadata.deployment_id
// names the deployment, as WSO2 products export it. It reads nothing else; the
// report's signature and content are verified by the product-consumption service.
func usageReportDeploymentNumber(zipFile []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipFile), int64(len(zipFile)))
	if err != nil {
		return "", errNotUsageReport
	}
	var report *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if report != nil {
			return "", errNotUsageReport
		}
		report = f
	}
	if report == nil || !strings.HasSuffix(strings.ToLower(report.Name), ".json") {
		return "", errNotUsageReport
	}

	rc, err := report.Open()
	if err != nil {
		return "", errNotUsageReport
	}
	defer func() { _ = rc.Close() }()

	var doc struct {
		ExportMetadata *struct {
			DeploymentID *string `json:"deployment_id"`
		} `json:"export_metadata"`
	}
	limited := &io.LimitedReader{R: rc, N: maxUsageReportBytes + 1}
	if err := json.NewDecoder(limited).Decode(&doc); err != nil || limited.N <= 0 {
		return "", errNotUsageReport
	}
	if doc.ExportMetadata == nil || doc.ExportMetadata.DeploymentID == nil {
		return "", errNotUsageReport
	}
	number := strings.TrimSpace(*doc.ExportMetadata.DeploymentID)
	if number == "" {
		return "", errNotUsageReport
	}
	return number, nil
}
