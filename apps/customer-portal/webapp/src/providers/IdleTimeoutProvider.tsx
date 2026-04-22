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
import { useEffect, useState, type JSX, type ReactNode } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import SessionWarningDialog from "@components/SessionWarningDialog";
import {
  IDLE_TIMEOUT_MS,
  IDLE_PROMPT_BEFORE_MS,
  IDLE_THROTTLE_MS,
} from "@constants/authConstants";
import { clearUserPreferredTimeZone } from "@utils/dateTime";

interface IdleTimeoutProviderProps {
  children: ReactNode;
}

/**
 * Provider that detects user idle time and shows a session warning dialog
 * before timeout. Continue resets the timer; Logout signs out.
 *
 * @param {IdleTimeoutProviderProps} props - children.
 * @returns {JSX.Element} Children wrapped with idle timeout behavior.
 */
export default function IdleTimeoutProvider({
  children,
}: IdleTimeoutProviderProps): JSX.Element {
  const [sessionWarningOpen, setSessionWarningOpen] = useState(false);
  const navigate = useNavigate();
  const { signOut, isSignedIn, isLoading } = useAsgardeo();

  const onPrompt = () => {
    if (isSignedIn && !isLoading) {
      setSessionWarningOpen(true);
    }
  };

  const { activate } = useIdleTimer({
    onPrompt,
    timeout: IDLE_TIMEOUT_MS,
    promptBeforeIdle: IDLE_PROMPT_BEFORE_MS,
    throttle: IDLE_THROTTLE_MS,
  });

  const handleContinue = () => {
    setSessionWarningOpen(false);
    activate();
  };

  const handleLogout = async () => {
    setSessionWarningOpen(false);
    try {
      await signOut();
    } finally {
      clearUserPreferredTimeZone();
      navigate("/home");
    }
  };

  useEffect(() => {
    if (!isSignedIn) {
      setSessionWarningOpen(false);
    }
  }, [isSignedIn]);

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
