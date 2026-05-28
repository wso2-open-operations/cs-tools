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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ServiceRequestsList from "@features/operations/components/service-requests/ServiceRequestsList";
import { SERVICE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE } from "@features/operations/constants/operationsConstants";

describe("ServiceRequestsList", () => {
  it("renders empty state when there are no service requests", () => {
    render(
      <ServiceRequestsList serviceRequests={[]} isLoading={false} isError={false} />,
    );
    expect(screen.getByText(SERVICE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE)).toBeInTheDocument();
  });
});
