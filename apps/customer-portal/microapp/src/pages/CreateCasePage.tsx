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
import { FormikContext } from "formik";

import { useDeclareLayout } from "@context/layout";

import { CreateCaseForm } from "@features/cases/components";
import { ClassificationProvider } from "@features/cases/context";
import { useCreateCase, useCreateCaseForm } from "@features/cases/hooks";

import { Tab } from "@shared/constants";

export default function CreateCasePage() {
  const formik = useCreateCaseForm();
  const { state } = useCreateCase();

  useDeclareLayout({
    tabIndex: Tab.Support,
    title: state.case ? "Create Related Case" : "Create Case",
    visibility: { backAction: true },
  });

  return (
    <FormikContext value={formik}>
      <ClassificationProvider>
        <CreateCaseForm />
      </ClassificationProvider>
    </FormikContext>
  );
}
