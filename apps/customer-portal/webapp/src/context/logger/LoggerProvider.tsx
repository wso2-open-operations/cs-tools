/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useMemo, type PropsWithChildren, type ReactElement } from "react";
import { Logger, type ILogger } from "@/utils/logger";
import LoggerContext from "@/context/logger/LoggerContext";

interface LoggerProviderProps {
  logger?: ILogger;
  config?: {
    level: string;
    prefix: string;
  };
}

/**
 * Provider component that wraps the app and provides the logger context.
 *
 * @param props - Props injected to the component.
 * @returns The LoggerProvider component.
 */
function LoggerProvider({
  children,
  logger,
  config,
}: PropsWithChildren<LoggerProviderProps>): ReactElement {
  const loggerInstance = useMemo(() => {
    if (logger) {
      return logger;
    }

    if (config) {
      return new Logger(Logger.parseLogLevel(config.level), config.prefix);
    }

    return new Logger();
  }, [logger, config]);

  return (
    <LoggerContext.Provider value={loggerInstance}>
      {children}
    </LoggerContext.Provider>
  );
}

export default LoggerProvider;
