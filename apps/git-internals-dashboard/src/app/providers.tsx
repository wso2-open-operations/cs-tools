"use client";

import { useState, type ReactNode } from "react";
import dynamic from "next/dynamic";
import { QueryClientProvider } from "@tanstack/react-query";
import { OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import { queryClient } from "@/lib/query";
import { theme } from "@/lib/theme";
import { createEmotionRegistry, EmotionRegistryProvider } from "@/lib/emotion-cache";

const AUTH_MODE = process.env.NEXT_PUBLIC_AUTH_MODE ?? "stub";

const AsgardeoAuthWrapper = dynamic(() => import("./AsgardeoAuthWrapper"), { ssr: false });

function AuthWrapper({ children }: { children: ReactNode }) {
  if (AUTH_MODE !== "asgardeo") {
    if (process.env.NODE_ENV === "production") {
      // Production must never ship with the auth gate disabled: refuse to
      // render the app tree instead of silently exposing it unauthenticated.
      return (
        <div role="alert" style={{ padding: "2rem", textAlign: "center" }}>
          Application misconfigured: authentication is not enabled for this production build.
        </div>
      );
    }
    return <>{children}</>; // stub mode: zero auth surface (non-production only)
  }
  return <AsgardeoAuthWrapper>{children}</AsgardeoAuthWrapper>;
}

export function Providers({ children }: { children: ReactNode }) {
  const [registry] = useState(() => createEmotionRegistry({ key: "css", prepend: true }));

  return (
    <EmotionRegistryProvider registry={registry}>
      <OxygenUIThemeProvider theme={theme} emotionCache={registry.cache}>
        <QueryClientProvider client={queryClient}>
          <AuthWrapper>{children}</AuthWrapper>
        </QueryClientProvider>
      </OxygenUIThemeProvider>
    </EmotionRegistryProvider>
  );
}
