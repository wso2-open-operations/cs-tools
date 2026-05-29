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
import { createContext, type ReactNode, useContext, useState } from "react";

interface ClassificationContextProps {
  classified: Set<string>;
  set: (fields: string[]) => void;
  remove: (fields: string[]) => void;
}

const ClassificationContext = createContext<ClassificationContextProps | undefined>(undefined);

export function ClassificationProvider({ children }: { children: ReactNode }) {
  const [classified, setClassified] = useState<Set<string>>(new Set());

  const set = (fields: string[]) => {
    setClassified((prev) => new Set([...prev, ...fields]));
  };

  const remove = (fields: string[]) => {
    setClassified((prev) => {
      const next = new Set(prev);
      fields.forEach((field) => next.delete(field));
      return next;
    });
  };

  return (
    <ClassificationContext.Provider value={{ classified, set, remove }}>{children}</ClassificationContext.Provider>
  );
}

export function useClassification() {
  const context = useContext(ClassificationContext);
  if (!context) {
    throw new Error("useClassification must be within a ClassificationProvider");
  }
  return context;
}
