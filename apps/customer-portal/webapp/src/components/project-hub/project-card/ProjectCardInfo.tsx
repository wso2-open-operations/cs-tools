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

import { Form, Typography } from "@wso2/oxygen-ui";
import { type JSX, useMemo } from "react";
import { stripHtmlTags } from "@utils/projectCard";

interface ProjectCardInfoProps {
  title: string;
  subtitle: string;
}

/**
 * Component to render the title and subheader for the Project Card.
 *
 * @param {ProjectCardInfoProps} props - The props for the component.
 * @returns {JSX.Element} The rendered info section.
 */
export default function ProjectCardInfo({
  title,
  subtitle,
}: ProjectCardInfoProps): JSX.Element {
  const strippedSubtitle = useMemo(() => stripHtmlTags(subtitle), [subtitle]);
  const displayTitle = title || "--";
  const displaySubtitle = strippedSubtitle || "--";

  return (
    <Form.CardHeader
      sx={{ pt: 1.5 }}
      title={
        <Typography
          variant="h6"
          sx={{
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
            textOverflow: "ellipsis",
            mb: 1,
          }}
        >
          {displayTitle}
        </Typography>
      }
      subheader={
        <Typography
          variant="body2"
          sx={{
            wordBreak: "break-word",
            whiteSpace: "normal",
            display: "-webkit-box",
            WebkitLineClamp: 4,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
            textOverflow: "ellipsis",
            minHeight: "5rem",
          }}
        >
          {displaySubtitle}
        </Typography>
      }
    />
  );
}
