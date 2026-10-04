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

import { Logger } from "@utils/logger";
import { ErrorMessages } from "@utils/constants";
import { Topic, type EdgeInsets, type LogLevel, type TopicType } from "./types";

type Callback<T> = (data?: T) => void;

// Bridge event topics used for communication between the main app and micro apps.
const TOPIC = {
  TOKEN: "token",
  QR_REQUEST: "qr_request",
  SAVE_LOCAL_DATA: "save_local_data",
  GET_LOCAL_DATA: "get_local_data",
  ALERT: "alert",
  CONFIRM_ALERT: "confirm_alert",
  TOTP: "totp",
  OPEN_URL: "open_url",
  MICRO_APP_VERSION: "micro_app_version",
};

export interface BrowserConfiguration {
  url: string;
  presentationStyle: string;
  dismissButtonStyle?: string;
}

declare global {
  interface Window {
    nativebridge?: {
      requestToken: () => void;
      resolveToken: (token: string) => void;
      requestIdToken: () => void;
      resolveIdToken: (token: string) => void;
      resolveDeviceSafeAreaInsets?: (data: { insets: EdgeInsets }) => void;
      resolveConfirmAlert: (action: string) => void;
      resolveSaveLocalData: () => void;
      rejectSaveLocalData: (error: string) => void;
      resolveGetLocalData: (encodedData: { value?: string }) => void;
      rejectGetLocalData: (error: string) => void;
      resolveOpenUrl?: () => void;
      rejectOpenUrl?: (error: string) => void;
      requestMicroAppVersion: () => void;
      resolveMicroAppVersion: (version: string) => void;
      rejectMicroAppVersion: (error: string) => void;
    };
    csmMicroApp?: {
      clearSession: () => void;
    };
    ReactNativeWebView?: {
      postMessage: (message: string) => void;
    };
  }
}

// The host answers a request by calling a single well-known resolver on window.nativebridge, with
// no request id, so replies cannot be matched to individual callers. Instead each topic keeps one
// request in flight and a list of waiters: callers arriving while a request is pending join the
// list rather than re-posting and overwriting the resolver, and the one reply settles them all.
const BRIDGE_REQUEST_TIMEOUT_MS = 10_000;

interface Waiter<T> {
  resolve: (value: T) => void;
  reject: (reason: Error) => void;
}

interface PendingRequest {
  waiters: Waiter<string>[];
  timer: ReturnType<typeof setTimeout>;
}

const pendingRequests = new Map<string, PendingRequest>();

function requestStringFromBridge(
  topic: string,
  installResolver: (bridge: NonNullable<Window["nativebridge"]>, settle: (value: string) => void) => void,
  post: (bridge: NonNullable<Window["nativebridge"]>) => void,
  timeoutMs = BRIDGE_REQUEST_TIMEOUT_MS,
): Promise<string> {
  const bridge = window.nativebridge;
  if (!bridge) {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
    return Promise.reject(new Error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE));
  }

  return new Promise<string>((resolve, reject) => {
    const existing = pendingRequests.get(topic);
    if (existing) {
      existing.waiters.push({ resolve, reject });
      return;
    }

    const pending: PendingRequest = {
      waiters: [{ resolve, reject }],
      timer: setTimeout(() => {
        if (pendingRequests.get(topic) !== pending) return;
        pendingRequests.delete(topic);
        pending.waiters.forEach((waiter) => waiter.reject(new Error(`Native bridge ${topic} request timed out`)));
      }, timeoutMs),
    };
    pendingRequests.set(topic, pending);

    // The resolver must be in place before the request is posted: a host that answers
    // synchronously would otherwise reply to nobody.
    installResolver(bridge, (value) => {
      if (pendingRequests.get(topic) !== pending) return;
      pendingRequests.delete(topic);
      clearTimeout(pending.timer);
      pending.waiters.forEach((waiter) => waiter.resolve(value));
    });
    post(bridge);
  });
}

export const getAccessTokenFromBridge = (): Promise<string> =>
  requestStringFromBridge(
    TOPIC.TOKEN,
    (bridge, settle) => {
      bridge.resolveToken = settle;
    },
    (bridge) => bridge.requestToken(),
  );

// Function to get the ID token from React Native
export const getToken = (): Promise<string> =>
  requestStringFromBridge(
    `${TOPIC.TOKEN}:id`,
    (bridge, settle) => {
      bridge.resolveIdToken = settle;
    },
    (bridge) => bridge.requestIdToken(),
  );

// Function to show alert in React Native
export const showAlert = (title: string, message: string, buttonText: string): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    const alertData = JSON.stringify({
      topic: TOPIC.ALERT,
      data: { title, message, buttonText },
    });

    window.ReactNativeWebView.postMessage(alertData);
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

// Function to show confirm alert in React Native
export const showConfirmAlert = (
  title: string,
  message: string,
  confirmButtonText: string,
  cancelButtonText: string,
  confirmCallback: () => void,
  cancelCallback: () => void,
): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    const confirmData = JSON.stringify({
      topic: TOPIC.CONFIRM_ALERT,
      data: { title, message, confirmButtonText, cancelButtonText },
    });

    window.ReactNativeWebView.postMessage(confirmData);

    // Handling response from React Native side
    window.nativebridge.resolveConfirmAlert = (action: string) => {
      if (action === "confirm") {
        confirmCallback();
      } else if (action === "cancel") {
        cancelCallback();
      }
    };
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

// Save Local Data
export const saveLocalData = (
  key: string,
  value: unknown,
  callback: () => void,
  failedToRespondCallback: (error: string) => void,
): void => {
  key = key.toString().replaceAll(" ", "-").toLowerCase();

  let encodedValue: string;
  try {
    encodedValue = btoa(JSON.stringify(value));
  } catch (error) {
    Logger.error("Failed to encode value for local data storage", error);
    failedToRespondCallback("Failed to encode value for local data storage");
    return;
  }

  if (window.nativebridge && window.ReactNativeWebView) {
    window.ReactNativeWebView.postMessage(
      JSON.stringify({
        topic: TOPIC.SAVE_LOCAL_DATA,
        data: { key, value: encodedValue },
      }),
    );

    window.nativebridge.resolveSaveLocalData = callback;
    window.nativebridge.rejectSaveLocalData = (error: string) => failedToRespondCallback(error);
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
    failedToRespondCallback(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

// Get Local Data
export const getLocalData = <T>(
  key: string,
  callback: (data: T | null) => void,
  failedToRespondCallback: (error: string) => void,
): void => {
  key = key.toString().replaceAll(" ", "-").toLowerCase();

  if (window.nativebridge && window.ReactNativeWebView) {
    window.ReactNativeWebView.postMessage(JSON.stringify({ topic: TOPIC.GET_LOCAL_DATA, data: { key } }));

    window.nativebridge.resolveGetLocalData = (encodedData: { value?: string }) => {
      if (!encodedData.value) {
        callback(null);
        return;
      }

      try {
        callback(JSON.parse(atob(encodedData.value)) as T);
      } catch (error) {
        Logger.error("Failed to decode stored local data", error);
        callback(null);
      }
    };

    window.nativebridge.rejectGetLocalData = (error: string) => failedToRespondCallback(error);
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
    failedToRespondCallback(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

/**
 * Trigger an action in the super app
 * @param topic - The topic to trigger
 * @param data - The data to send
 */
const triggerSuperAppAction = (topic: TopicType, data?: unknown): void => {
  if (window.ReactNativeWebView) {
    const messageData = JSON.stringify({
      topic,
      data,
    });
    window.ReactNativeWebView.postMessage(messageData);
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

/**
 * Send a log message to the native side
 * @param message - The message to send
 * @param data - The data to send
 * @param level - The level of the log
 */
export const sendNativeLog = (message?: string, data?: unknown, level: LogLevel = "debug"): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    triggerSuperAppAction(Topic.nativeLog, {
      message,
      data,
      level,
    });
  } else {
    // Logger.error() calls back into sendNativeLog() — must not call it here, or the two recurse forever.
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE, { message, data, level });
  }
};

/**
 * Send a request to the native app to navigate back to the previous screen. I.e., close the webview.
 */
export const goToMyAppsScreen = (): void => {
  if (window.nativebridge) {
    triggerSuperAppAction(Topic.navigateToMyApps);
  }
};

/**
 * Request the device safe area insets from the native app
 * @param callback - The callback to receive the device safe area insets
 */
export const requestDeviceSafeAreaInsets = (callback: Callback<{ insets: EdgeInsets }>): void => {
  if (window.nativebridge) {
    triggerSuperAppAction(Topic.deviceSafeAreaInsets);
    window.nativebridge.resolveDeviceSafeAreaInsets = (data) => {
      callback(data);
    };
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE + " to fetch device safe area insets");
    callback();
  }
};

// Open URL in Browser
export const openUrl = (config: BrowserConfiguration): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    const alertData = JSON.stringify({
      topic: TOPIC.OPEN_URL,
      data: { config },
    });

    window.ReactNativeWebView.postMessage(alertData);
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

export const getVersion = (callback: Callback<string>): void => {
  if (window.nativebridge) {
    triggerSuperAppAction(Topic.version);
    window.nativebridge.resolveMicroAppVersion = (version) => {
      callback(version);
    };
  } else {
    Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
    callback();
  }
};
