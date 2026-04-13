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

// Response type for user details.
export type UserDetails = {
  id: string;
  email: string;
  lastName: string;
  firstName: string;
  timeZone: string;
  phoneNumber?: string | null;
  avatar?: string | null;
  roles?: string[];
  lastPasswordUpdateTime?: string;
};

// Response type for contact validation.
export type ValidateContactResponse = {
  isContactValid: boolean;
  message: string;
  contactDetails?: ContactDetails;
};

// Item type for contact details.
export type ContactDetails = {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  isCsAdmin: boolean;
  isCsIntegrationUser: boolean;
  account?: AccountInfo;
};

// Item type for a project contact.
export type ProjectContact = {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  isCsAdmin: boolean;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
  membershipStatus: string;
  account?: AccountInfo;
};

// Item type for account information.
export type AccountInfo = {
  id: string;
  domainList?: string[] | null;
  classification: string;
  isPartner: boolean;
};

// Item type for an integration user.
export type IntegrationUser = {
  id: string;
  email: string;
};

// Request type for patching the current user's profile.
export type PatchUserMeRequest = {
  phoneNumber?: string;
  timeZone?: string;
  firstName?: string;
  lastName?: string;
};

// Request type for creating a project contact.
export type CreateProjectContactRequest = {
  contactEmail: string;
  contactFirstName: string;
  contactLastName: string;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
};

// Request type for validating a contact.
export type ValidateContactRequest = {
  contactEmail: string;
};
