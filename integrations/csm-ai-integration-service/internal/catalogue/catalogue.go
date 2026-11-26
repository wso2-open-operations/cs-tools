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

// Package catalogue holds the catalogues the model is given as its system
// prompt: catalogue.md for a dashboard definition (POST
// /ai/dashboards/generate) and code_catalogue.md for a dashboard written as
// JavaScript (POST /ai/dashboards/generate-code). Each tells the model what it
// may use, and is the only thing about the portal the model sees besides the
// user's prompt, the draft being edited and the team list.
//
// Both ship inside the binary (embedded at build time), so they are changed and
// reviewed like code, and a new catalogue reaches a deployment with the next
// build. Nothing at run time replaces them.
package catalogue

import _ "embed"

//go:embed catalogue.md
var dashboardEmbedded string

//go:embed code_catalogue.md
var codeEmbedded string

// DashboardEmbedded returns the dashboard catalogue built into the binary.
func DashboardEmbedded() string { return dashboardEmbedded }

// CodeEmbedded returns the code catalogue built into the binary.
func CodeEmbedded() string { return codeEmbedded }

// Source is the catalogue's text.
type Source struct {
	text string
}

// NewDashboard is the dashboard catalogue: the copy built into the binary.
func NewDashboard() Source { return Source{text: dashboardEmbedded} }

// NewCode is the code catalogue: the copy built into the binary.
func NewCode() Source { return Source{text: codeEmbedded} }

// Text returns the catalogue.
func (s Source) Text() string { return s.text }
