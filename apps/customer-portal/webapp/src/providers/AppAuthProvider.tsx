// Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
//
// This software is the property of WSO2 LLC. and its suppliers, if any.
// Dissemination of any information or reproduction of any material contained
// herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
// You may not alter or remove any copyright or other notice from copies of this content.

import React, { useCallback, useContext, useEffect, useState } from "react";
import { useIdleTimer } from "react-idle-timer";
import { SecureApp, useAuthContext } from "@asgardeo/auth-react";
import { setIdToken, setRefreshTokenCallback } from "../services/auth";
import PreLoader from "../components/PreLoader/PreLoader";
import ErrorHandler from "../components/ErrorHandler/ErrorHandler";
import StatusWithAction from "../components/StatusWithAction/StatusWithAction";
import IdleTimeoutDialog from "../components/IdleTimeoutDialog/IdleTimeoutDialog";

type AuthContextType = {
  appSignIn: () => void;
  appSignOut: () => void;
};

const AuthContext = React.createContext<AuthContextType>({} as AuthContextType);

const TIMEOUT = 1800_000; // 30 minutes
const PROMPT_BEFORE_IDLE = 4_000; // 4 seconds before timeout
const STORAGE_KEYS = {
  STATE: "customer-portal-state",
  REDIRECT_URL: "customer-portal-redirect-url",
} as const;

const AppAuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [idlePromptOpen, setIdlePromptOpen] = useState(false);
  const [ui, setUi] = useState<"loading" | "active" | "logout" | "error">(
    "loading"
  );
  const [errorMessage, setErrorMessage] = useState<string>("");

  const {
    signIn,
    signOut,
    getIDToken,
    isAuthenticated,
    state: asg,
  } = useAuthContext();

  useIdleTimer({
    timeout: TIMEOUT,
    promptBeforeIdle: PROMPT_BEFORE_IDLE,
    throttle: 500,
    onPrompt: () => ui === "active" && setIdlePromptOpen(true),
  });

  const handleContinue = () => setIdlePromptOpen(false);

  const appSignIn = useCallback(async () => {
    localStorage.setItem(STORAGE_KEYS.STATE, "active");
    window.location.reload();
  }, []);

  const appSignOut = useCallback(async () => {
    setUi("loading");
    localStorage.setItem(STORAGE_KEYS.STATE, "logout");
    await signOut();
    setUi("logout");
  }, [signOut]);

  // Register token refresh callback on mount
  useEffect(() => {
    setRefreshTokenCallback(getIDToken);
  }, [getIDToken]);

  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      if (asg.isLoading) return;

      // Check if user manually logged out
      if (localStorage.getItem(STORAGE_KEYS.STATE) === "logout") {
        setUi("logout");
        return;
      }

      // Save redirect URL
      if (!localStorage.getItem(STORAGE_KEYS.REDIRECT_URL)) {
        localStorage.setItem(
          STORAGE_KEYS.REDIRECT_URL,
          window.location.href.replace(window.location.origin, "")
        );
      }

      // If not authenticated, trigger sign-in
      if (!asg.isAuthenticated) {
        await signIn();
        return;
      }

      try {
        // Get ID token to verify authentication
        const idToken = await getIDToken();
        if (cancelled) return;

        if (!idToken) {
          throw new Error("Failed to get ID token");
        }

        // Store ID token in localStorage for API requests
        setIdToken(idToken);

        // Set to active state
        if (!cancelled) {
          localStorage.setItem(STORAGE_KEYS.STATE, "active");
          setUi("active");
        }
      } catch (err: unknown) {
        console.error("Failed to authenticate:", err);

        const errorMsg =
          (err as any)?.message ||
          "You are not authorized to access this application.";
        setErrorMessage(errorMsg);

        // Check if still authenticated
        try {
          const stillAuth = await isAuthenticated();
          if (!stillAuth) {
            localStorage.removeItem(STORAGE_KEYS.STATE);
            await signIn();
            return;
          }
        } catch {
          await signIn();
          return;
        }

        if (!cancelled) setUi("error");
      }
    };

    bootstrap();
    return () => {
      cancelled = true;
    };
  }, [asg.isAuthenticated, asg.isLoading, signIn, getIDToken, isAuthenticated]);

  // Re-authenticate if session expires
  useEffect(() => {
    if (ui === "active" && !asg.isAuthenticated) {
      signIn();
    }
  }, [ui, asg.isAuthenticated, signIn]);

  if (ui === "loading") {
    if (!asg.isAuthenticated && !asg.isLoading) {
      return null;
    }
    return <PreLoader isLoading={true} message="Authenticating..." />;
  }

  if (ui === "logout") {
    return <StatusWithAction action={appSignIn} />;
  }

  if (ui === "error") {
    return <ErrorHandler message={errorMessage} onRetry={appSignIn} />;
  }

  return (
    <>
      <IdleTimeoutDialog
        open={idlePromptOpen}
        onContinue={handleContinue}
        onLogout={appSignOut}
      />

      <AuthContext.Provider value={{ appSignIn, appSignOut }}>
        <SecureApp>{children}</SecureApp>
      </AuthContext.Provider>
    </>
  );
};

export const useAppAuthContext = (): AuthContextType => useContext(AuthContext);

export default AppAuthProvider;