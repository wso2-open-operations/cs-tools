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

import { useIdleTimer } from "react-idle-timer";
import { useState, type JSX, type ReactNode } from "react";
import { useAsgardeo } from "@asgardeo/react";
import SessionWarningDialog from "@components/SessionWarningDialog";
import {
  IDLE_TIMEOUT_MS,
  IDLE_PROMPT_BEFORE_MS,
  IDLE_THROTTLE_MS,
} from "@constants/authConstants";
import { clearUserPreferredTimeZone } from "@utils/dateTime";
import { SIGNING_OUT_EVENT } from "@utils/sessionCleanup";
import { useLogger } from "@/hooks/useLogger";

interface IdleTimeoutProviderProps {
  children: ReactNode;
}

/**
 * Provider that detects user idle time and shows a session warning dialog
 * before timeout. Continue resets the timer; Logout signs out, and so does
 * letting the prompt run out.
 *
 * @param {IdleTimeoutProviderProps} props - children.
 * @returns {JSX.Element} Children wrapped with idle timeout behavior.
 */
export default function IdleTimeoutProvider({
  children,
}: IdleTimeoutProviderProps): JSX.Element {
  const [sessionWarningOpen, setSessionWarningOpen] = useState(false);
  const { signOut, isSignedIn, isLoading } = useAsgardeo();
  const logger = useLogger();

  const onPrompt = () => {
    if (isSignedIn && !isLoading) {
      setSessionWarningOpen(true);
    }
  };

  // Ends the session: the signing-out event lets the central cleanup listener
  // drop app-owned storage and the query cache, then the IdP session is closed.
  const endSession = async () => {
    setSessionWarningOpen(false);
    clearUserPreferredTimeZone();
    window.dispatchEvent(new CustomEvent(SIGNING_OUT_EVENT));
    try {
      await signOut();
    } catch {
      logger.error("Error signing out");
    }
  };

  // When the user doesn't respond to the prompt within IDLE_PROMPT_BEFORE_MS
  // the timer goes idle and the session is ended. `crossTab` keeps one timer
  // across tabs, so activity in any tab keeps every tab signed in and idling
  // out signs out all of them.
  const { activate } = useIdleTimer({
    onPrompt,
    onIdle: () => {
      if (isSignedIn) void endSession();
    },
    timeout: IDLE_TIMEOUT_MS,
    promptBeforeIdle: IDLE_PROMPT_BEFORE_MS,
    throttle: IDLE_THROTTLE_MS,
    crossTab: true,
  });

  const handleContinue = () => {
    setSessionWarningOpen(false);
    activate();
  };

  const handleLogout = endSession;

  return (
    <>
      <SessionWarningDialog
        open={sessionWarningOpen}
        onContinue={handleContinue}
        onLogout={handleLogout}
      />
      {children}
    </>
  );
}
