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

final ProductVulnerability[] & readonly MOCK_PRODUCT_VULNERABILITIES = [
    {
        id: "8f3d2c1a9b7e4d6f8a1c2b3d4e5f6a7b",
        cveId: "CVE-2099-5555",
        vulnerabilityId: "XRAY-999001",
        severity: {
            id: 1,
            label: "High"
        },
        componentName: "org.example.identity.apps",
        version: "1.5.0",
        'type: "maven",
        useCase: "Not Applicable",
        justification: "This library is used for string processing and does not expose external endpoints.",
        resolution: "Need to update to 5.18.0 or later."
    },
    {
        id: "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d",
        cveId: "CVE-2099-4444",
        vulnerabilityId: "XRAY-999002",
        severity: {
            id: 2,
            label: "Low"
        },
        componentName: "ubuntu.libuconf",
        version: "oval:8.5.0-2ubuntu16.6",
        'type: "debian",
        useCase: "",
        justification: "",
        resolution: "-"
    },
    {
        id: "abcdef1234567890fedcba0987654321",
        cveId: "CVE-2099-3333",
        vulnerabilityId: "XRAY-999003",
        severity: {
            id: 2,
            label: "Low"
        },
        componentName: "com.example.templating.engine",
        version: "2.6.0",
        'type: "maven",
        useCase: "Batch for Patching",
        justification: "Template engine is used with controlled inputs and no direct user input exposure.",
        resolution: "We will proactively upgrade to the latest secure version."
    }
];

final ProductVulnerabilityResponse & readonly MOCK_PRODUCT_VULNERABILITY = {
    id: "abcdef1234567890fedcba0987654321",
    cveId: "CVE-2099-3333",
    vulnerabilityId: "XRAY-999003",
    severity: {
        id: 2,
        label: "Low"
    },
    componentName: "com.example.templating.engine",
    version: "2.6.0",
    'type: "maven",
    useCase: "Batch for Patching",
    justification: "Template engine is used with controlled inputs and no direct user input exposure.",
    resolution: "We will proactively upgrade to the latest secure version.",
    componentType: "library",
    updateLevel: "2.6.1"
};
