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

import { useState } from "react";
import { useNavigate } from "react-router";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useSearchSplAccounts } from "../api/useAccountsApi";
import AccountSearch from "./AccountSearch";
import AccountsDataTable from "./AccountsDataTable";

export default function ListMyAccounts({ active }: { active: boolean }) {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const [showTable, setShowTable] = useState(true);
  const navigate = useNavigate();
  const email = useIdTokenClaims()?.email;

  const { data, isLoading, error } = useSearchSplAccounts({
    email,
    offset: page * rowsPerPage,
    limit: rowsPerPage,
    active,
    enabled: Boolean(email),
  });

  const colNameArray = ["Number", "Name", "Region", "ARR", "Account Manager", "Technical Owner"];
  const colAttributeArray = ["number", "name", "region", "arr", "accountManager", "technicalOwner"];

  return (
    <>
      <AccountSearch setShowTable={setShowTable} searchOption="myAccount" />
      {showTable && (
        <AccountsDataTable
          data={data}
          loading={isLoading}
          error={error}
          page={page}
          setPage={setPage}
          rowsPerPage={rowsPerPage}
          setRowsPerPage={setRowsPerPage}
          colNameArray={colNameArray}
          colAttributeArray={colAttributeArray}
          handleRowClick={(rowData) => navigate(`/spl/accounts/${rowData.id}`)}
        />
      )}
    </>
  );
}
