import type { ReactNode } from "react";

import { useQueryErrorResetBoundary } from "@tanstack/react-query";

import { useNotify } from "@context/snackbar";

import { ErrorState } from "@shared/components/common";
import { ErrorBoundary } from "@shared/components/core";

export function ItemsListErrorBoundary({ children }: { children: ReactNode }) {
  const notify = useNotify();
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
      onError={() => notify.error("Content failed to load. Please try again later.")}
    >
      {children}
    </ErrorBoundary>
  );
}
