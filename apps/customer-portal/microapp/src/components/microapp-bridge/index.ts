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

import { ErrorMessages } from "@utils/constants";
import { Topic } from "./types";
import type { LogLevel, TopicType } from "./types";

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
};

declare global {
  interface Window {
    nativebridge?: {
      requestToken: () => void;
      resolveToken: (token: string) => void;
      requestQR: () => void;
      resolveQR: (qrString: string) => void;
      requestItemList: () => void;
      resolveConfirmAlert: (action: string) => void;
      resolveQRCode: (qrData: string) => void;
      rejectQRCode: (error: string) => void;
      resolveSaveLocalData: () => void;
      rejectSaveLocalData: (error: string) => void;
      resolveGetLocalData: (encodedData: { value?: string }) => void;
      rejectGetLocalData: (error: string) => void;
      resolveTotpQrMigrationData: (encodedData: { data: string }) => void;
      rejectTotpQrMigrationData: (error: string) => void;
    };
    ReactNativeWebView?: {
      postMessage: (message: string) => void;
    };
  }
}

// Function to get token from React Native
export const getToken = (callback: Callback<string>): void => {
  if (window.nativebridge) {
    window.nativebridge.requestToken();
    window.nativebridge.resolveToken = (token: string) => {
      callback(token);
    };
  } else {
    console.error("Native bridge is not available");
    callback();
  }
};

// Function to show alert in React Native
export const showAlert = (title: string, message: string, buttonText: string): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    const alertData = JSON.stringify({
      topic: TOPIC.ALERT,
      data: { title, message, buttonText },
    });

    window.ReactNativeWebView.postMessage(alertData);
  } else {
    console.error("Native bridge is not available");
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
    console.error("Native bridge is not available");
  }
};

// Scan QR Code
export const scanQRCode = (
  successCallback: (qrData: string) => void,
  failedToRespondCallback: (error: string) => void,
): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    window.ReactNativeWebView.postMessage(JSON.stringify({ topic: TOPIC.QR_REQUEST }));

    window.nativebridge.resolveQRCode = (qrData: string) => successCallback(qrData);
    window.nativebridge.rejectQRCode = (error: string) => failedToRespondCallback(error);
  } else {
    console.error("Native bridge is not available");
  }
};

// Save Local Data
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
    console.error("Native bridge is not available");
  }
};

// Get Local Data
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
    console.error("Native bridge is not available");
  }
};

// TOTP QR Migration Data
export const totpQrMigrationData = (
  callback: (data: string[]) => void,
  failedToRespondCallback: (error: string) => void,
): void => {
  if (window.nativebridge && window.ReactNativeWebView) {
    window.ReactNativeWebView.postMessage(JSON.stringify({ topic: TOPIC.TOTP }));

    window.nativebridge.resolveTotpQrMigrationData = (encodedData: { data: string }) => {
      if (encodedData.data) {
        callback(encodedData.data.replace(" ", "").split(","));
      } else {
        callback([]);
      }
    };

    window.nativebridge.rejectTotpQrMigrationData = (error: string) => failedToRespondCallback(error);
  } else {
    console.error("Native bridge is not available");
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
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
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
    // TODO: Replace this with Logger.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE)
    console.error(ErrorMessages.NATIVE_BRIDGE_NOT_AVAILABLE);
  }
};
