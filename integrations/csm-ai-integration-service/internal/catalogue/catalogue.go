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

// Package catalogue holds the catalogue the model is given as its system
// prompt: catalogue.md, for a dashboard definition (POST
// /ai/dashboards/generate). It tells the model what it may use, and is the
// only thing about the portal the model sees besides the user's prompt, the
// draft being edited and the team list.
//
// It ships inside the binary (embedded at build time), so it is changed and
// reviewed like code, and a new catalogue reaches a deployment with the next
// build. Nothing at run time replaces it.
package catalogue

import _ "embed"

//go:embed catalogue.md
var dashboardEmbedded string

// DashboardEmbedded returns the dashboard catalogue built into the binary.
func DashboardEmbedded() string { return dashboardEmbedded }

// Source is the catalogue's text.
type Source struct {
	text string
}

// NewDashboard is the dashboard catalogue: the copy built into the binary.
func NewDashboard() Source { return Source{text: dashboardEmbedded} }

// Text returns the catalogue.
func (s Source) Text() string { return s.text }
