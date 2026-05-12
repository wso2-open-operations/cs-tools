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

import { useCallback, useRef, useState, type ReactNode } from "react";
import { SnackbarContext, type SnackbarQueueItem } from "./SnackbarContext";
import { Snackbar } from "@components/core";

export default function SnackbarProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<SnackbarQueueItem[]>([]);
  const [open, setOpen] = useState(false);

  const enqueue = useCallback((item: SnackbarQueueItem) => {
    setItems((prev) => [...prev, item]);
    setOpen(true);
  }, []);

  const dequeue = useCallback(() => {
    setItems((prev) => prev.slice(1));
  }, []);

  const currentRef = useRef<SnackbarQueueItem | undefined>(undefined);

  if (items.length > 0) {
    currentRef.current = items[0];
  }

  const handleClose = () => {
    setOpen(false);
    setTimeout(() => {
      setItems((prev) => {
        const next = prev.slice(1);
        if (next.length > 0) {
          setOpen(true);
        }
        return next;
      });
    }, 500);
  };

  return (
    <SnackbarContext.Provider value={{ queue: items, enqueue, dequeue }}>
      <Snackbar
        open={open}
        message={currentRef.current?.message ?? ""}
        severity={currentRef.current?.severity ?? "info"}
        onClose={handleClose}
      />
      {children}
    </SnackbarContext.Provider>
  );
}
