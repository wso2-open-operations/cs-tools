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
import { useEffect, useRef, useState, type JSX, type ReactNode } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import SessionWarningDialog from "@components/SessionWarningDialog";
import {
  IDLE_TIMEOUT_MS,
  IDLE_PROMPT_BEFORE_MS,
  IDLE_THROTTLE_MS,
  CONTINUE_LOADER_MS,
  CONTINUE_LOADER_IDLE_THRESHOLD_MS,
} from "@constants/authConstants";

interface IdleTimeoutProviderProps {
  children: ReactNode;
}

/**
 * Provider that detects user idle time and shows a session warning dialog
 * before timeout. Continue shows a brief loader then resets the timer;
 * Logout signs out.
 *
 * @param {IdleTimeoutProviderProps} props - children.
 * @returns {JSX.Element} Children wrapped with idle timeout behavior.
 */
export default function IdleTimeoutProvider({
  children,
}: IdleTimeoutProviderProps): JSX.Element {
  const [sessionWarningOpen, setSessionWarningOpen] = useState(false);
  const [isContinuing, setIsContinuing] = useState(false);
  const continueTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const navigate = useNavigate();
  const { signOut, isSignedIn, isLoading } = useAsgardeo();

  const onPrompt = () => {
    if (isSignedIn && !isLoading) {
      setSessionWarningOpen(true);
    }
  };

  const { activate, getIdleTime } = useIdleTimer({
    onPrompt,
    timeout: IDLE_TIMEOUT_MS,
    promptBeforeIdle: IDLE_PROMPT_BEFORE_MS,
    throttle: IDLE_THROTTLE_MS,
  });

  const handleContinue = () => {
    if (continueTimerRef.current !== null) {
      clearTimeout(continueTimerRef.current);
      continueTimerRef.current = null;
    }

    if (getIdleTime() < CONTINUE_LOADER_IDLE_THRESHOLD_MS) {
      setSessionWarningOpen(false);
      activate();
      return;
    }

    setIsContinuing(true);
    continueTimerRef.current = setTimeout(() => {
      setIsContinuing(false);
      setSessionWarningOpen(false);
      activate();
    }, CONTINUE_LOADER_MS);
  };

  const handleLogout = async () => {
    if (continueTimerRef.current !== null) {
      clearTimeout(continueTimerRef.current);
      continueTimerRef.current = null;
    }
    setIsContinuing(false);
    setSessionWarningOpen(false);
    try {
      await signOut();
    } finally {
      navigate("/home");
    }
  };

  useEffect(() => {
    if (!isSignedIn) {
      if (continueTimerRef.current !== null) {
        clearTimeout(continueTimerRef.current);
        continueTimerRef.current = null;
      }
      setIsContinuing(false);
      setSessionWarningOpen(false);
    }
  }, [isSignedIn]);

  // Clean up the timer on unmount.
  useEffect(() => {
    return () => {
      if (continueTimerRef.current !== null) {
        clearTimeout(continueTimerRef.current);
      }
    };
  }, []);

  return (
    <>
      <SessionWarningDialog
        open={sessionWarningOpen}
        isContinuing={isContinuing}
        onContinue={handleContinue}
        onLogout={handleLogout}
      />
      {children}
    </>
  );
}
