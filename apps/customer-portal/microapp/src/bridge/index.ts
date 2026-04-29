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

import { ErrorMessages } from "@shared/constants/app.constants";
import { Topic, type EdgeInsets, type LogLevel, type TopicType } from "./types";

type Callback<T> = (data?: T) => void;

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
    ReactNativeWebView?: {
      postMessage: (message: string) => void;
    };
  }
}

export const getAccessTokenFromBridge = (callback: Callback<string>): void => {
  if (window.nativebridge) {
    window.nativebridge.requestToken();
    window.nativebridge.resolveToken = (token: string) => {
      callback(token);
    };
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
    callback();
  }
};

export const getToken = (callback: Callback<string>): void => {
  if (window.nativebridge) {
    window.nativebridge.requestIdToken();
    window.nativebridge.resolveIdToken = (token: string) => {
      callback(token);
    };
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
    callback();
  }
};

export const showAlert = (title: string, message: string, buttonText: string): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    const alertData = JSON.stringify({
      topic: TOPIC.ALERT,
      data: { title, message, buttonText },
    });

    window.ReactNativeWebView.postMessage(alertData);
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

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

    window.nativebridge.resolveConfirmAlert = (action: string) => {
      if (action === "confirm") {
        confirmCallback();
      } else if (action === "cancel") {
        cancelCallback();
      }
    };
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

export const saveLocalData = (
  key: string,
  value: unknown,
  callback: () => void,
  failedToRespondCallback: (error: string) => void,
): void => {
  key = key.toString().replace(" ", "-").toLowerCase();
  const encodedValue = btoa(JSON.stringify(value));

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
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

export const getLocalData = <T>(
  key: string,
  callback: (data: T | null) => void,
  failedToRespondCallback: (error: string) => void,
): void => {
  key = key.toString().replace(" ", "-").toLowerCase();

  if (window.nativebridge && window.ReactNativeWebView) {
    window.ReactNativeWebView.postMessage(JSON.stringify({ topic: TOPIC.GET_LOCAL_DATA, data: { key } }));

    window.nativebridge.resolveGetLocalData = (encodedData: { value?: string }) => {
      if (!encodedData.value) {
        callback(null);
      } else {
        callback(JSON.parse(atob(encodedData.value)) as T);
      }
    };

    window.nativebridge.rejectGetLocalData = (error: string) => failedToRespondCallback(error);
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

const triggerSuperAppAction = (topic: TopicType, data?: unknown): void => {
  if (window.ReactNativeWebView) {
    const messageData = JSON.stringify({
      topic,
      data,
    });
    window.ReactNativeWebView.postMessage(messageData);
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

export const sendNativeLog = (message?: string, data?: unknown, level: LogLevel = "debug"): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    triggerSuperAppAction(Topic.nativeLog, {
      message,
      data,
      level,
    });
  }
};

export const goToMyAppsScreen = (): void => {
  if (window.nativebridge) {
    triggerSuperAppAction(Topic.navigateToMyApps);
  }
};

export const requestDeviceSafeAreaInsets = (callback: Callback<{ insets: EdgeInsets }>): void => {
  if (window.nativebridge) {
    triggerSuperAppAction(Topic.deviceSafeAreaInsets);
    window.nativebridge.resolveDeviceSafeAreaInsets = (data) => {
      callback(data);
    };
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE + " to fetch device safe area insets");
    callback();
  }
};

export const openUrl = (config: BrowserConfiguration): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    const alertData = JSON.stringify({
      topic: TOPIC.OPEN_URL,
      data: { config },
    });

    window.ReactNativeWebView.postMessage(alertData);
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};

export const getVersion = (callback: Callback<string>): void => {
  if (window.nativebridge) {
    triggerSuperAppAction(Topic.version);
    window.nativebridge.resolveMicroAppVersion = (version) => {
      callback(version);
    };
  } else {
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};
