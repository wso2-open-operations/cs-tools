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

import type {
  BeConfigurationItem,
  BeIncidentCategory,
  BeIncidentContactType,
  BeIncidentImpact,
  BeIncidentSubcategory,
  BeIncidentUrgency,
  BeItService,
  BeUser,
} from "@api/backend/types";

// Shared between CreateIncidentPage and EditIncidentDialog so their dropdown
// option sets can't drift apart.

export const CATEGORY_OPTIONS: Array<{ value: BeIncidentCategory; label: string }> = [
  { value: "INQUIRY", label: "Inquiry / Help" },
  { value: "SERVICE_INTERRUPTION", label: "Service Interruption" },
  { value: "SECURITY", label: "Security" },
];

export const IMPACT_OPTIONS: Array<{ value: BeIncidentImpact; label: string }> = [
  { value: "HIGH", label: "High" },
  { value: "MEDIUM", label: "Medium" },
  { value: "LOW", label: "Low" },
];

export const URGENCY_OPTIONS: Array<{ value: BeIncidentUrgency; label: string }> = [
  { value: "HIGH", label: "High" },
  { value: "MEDIUM", label: "Medium" },
  { value: "LOW", label: "Low" },
];

export const CONTACT_TYPE_OPTIONS: Array<{ value: BeIncidentContactType; label: string }> = [
  { value: "SELF_SERVICE", label: "Self-service" },
  { value: "EMAIL", label: "Email" },
  { value: "WALK_IN", label: "Walk-in" },
  { value: "AZURE", label: "Azure" },
  { value: "EMAIL_INTERNAL", label: "Email Internal" },
  { value: "SITE_247", label: "Site 24/7" },
  { value: "DIRECT", label: "Direct" },
  { value: "PHONE", label: "Phone" },
  { value: "SENTINEL", label: "Sentinel" },
  { value: "VIRTUAL_AGENT", label: "Virtual Agent" },
  { value: "CHAT", label: "Chat" },
  { value: "EMAIL_EXTERNAL", label: "Email External" },
];

/**
 * Subcategory options, curated per category rather than offered as one flat
 * 35-value list. The backend's `validIncidentSubcategories` enum has 16 more
 * values than appear here (DHCP, DNS, OS, VPN, disk/memory/CPU-type hardware
 * values, etc.) — they're still valid on the wire, they just don't have an
 * obvious home in Inquiry/Help, Service Interruption, or Security, so this
 * curated picker doesn't surface them.
 */
export const SUBCATEGORY_OPTIONS_BY_CATEGORY: Record<
  BeIncidentCategory,
  Array<{ value: BeIncidentSubcategory; label: string }>
> = {
  INQUIRY: [
    { value: "CONFIG_CHANGE_REQUEST", label: "Config Change Request" },
    { value: "INFORMATION_REQUEST", label: "Information Request" },
  ],
  SERVICE_INTERRUPTION: [
    { value: "FULL_OUTAGE", label: "Full Outage" },
    { value: "PARTIAL_OUTAGE", label: "Partial Outage" },
    { value: "SLOWNESS", label: "Slowness" },
  ],
  SECURITY: [
    { value: "DOS_DDOS", label: "DOS/DDOS" },
    { value: "PRIVILEGE_ESCALATIONS", label: "Privilege Escalations" },
    { value: "THREAT_INTELLIGENCE", label: "Threat Intelligence" },
    { value: "SCANS_AND_PROBES", label: "Scans and Probes" },
    { value: "APPLICATION_SECURITY", label: "Application Security" },
    { value: "PRIVACY", label: "Privacy" },
    { value: "DATA_BREACH", label: "Data Breach" },
    { value: "SYSTEM_COMPROMISES", label: "System Compromises" },
    { value: "MALWARE", label: "Malware" },
    { value: "VULNERABILITY", label: "Vulnerability" },
    { value: "UNAUTHORIZED_ACCESS", label: "Unauthorized Access" },
    { value: "IDENTITY_PROTECTION", label: "Identity Protection" },
    { value: "PHISHING", label: "Phishing" },
    { value: "IMPROPER_CONFIGURATION", label: "Improper Configuration" },
  ],
};

export function userLabel(u: BeUser): string {
  return [u.firstName, u.lastName].filter(Boolean).join(" ").trim() || u.email || u.id || "";
}

export function itServiceLabel(s: BeItService): string {
  return s.name ?? s.id;
}

export function configurationItemLabel(ci: BeConfigurationItem): string {
  return ci.name ?? ci.id;
}
