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

import {
  Box,
  Card,
  CardContent,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { ArrowUpRight, FileText, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import type { Recommendation } from "@/types/conversations";

const KB_BASE_URL = "https://support.wso2.com/kb?id=kb_article_view&sys_kb_id=";

interface RecommendationsCardProps {
  recommendations: Recommendation[];
}

/**
 * Displays knowledge base article recommendations from the Novera AI agent.
 * Shown under the bot's chat bubble when recommendations are available.
 *
 * @param {RecommendationsCardProps} props - Contains the recommendations array.
 * @returns {JSX.Element} The RecommendationsCard component.
 */
export default function RecommendationsCard({
  recommendations,
}: RecommendationsCardProps): JSX.Element {
  if (!recommendations || recommendations.length === 0) {
    return <></>;
  }

  const handleArticleClick = (articleId: string) => {
    window.open(`${KB_BASE_URL}${articleId}`, "_blank", "noopener,noreferrer");
  };

  const orangeMain = colors.orange?.[600] ?? colors.purple[600];

  return (
    <Box sx={{ mt: 2, mb: 2 }}>
      <Box
        sx={{
          bgcolor: alpha(orangeMain, 0.08),
          border: "1px solid",
          borderColor: alpha(orangeMain, 0.2),
          p: 2,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1.5 }}>
          <Sparkles size={16} color={orangeMain} />
          <Typography variant="body2" sx={{ fontWeight: 500 }}>
            Here are some articles that might help:
          </Typography>
        </Box>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            gap: 1.25,
          }}
        >
          {recommendations.slice(0, 4).map((rec, index) => (
            <Card
              key={rec.articleId || index}
              onClick={() => handleArticleClick(rec.articleId)}
              sx={{
                cursor: "pointer",
                bgcolor: "background.paper",
                border: "1px solid",
                borderColor: "divider",
                "&:hover": {
                  boxShadow: 4,
                  transform: "translateY(-2px)",
                  borderColor: alpha(orangeMain, 0.5),
                },
                transition: "all 0.2s ease-in-out",
                p: 1.5,
              }}
            >
              <CardContent sx={{ p: 0, "&:last-child": { pb: 0 } }}>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 1,
                    mb: 1,
                  }}
                >
                  <FileText
                    size={14}
                    style={{
                      color: orangeMain,
                      flexShrink: 0,
                      marginTop: 2,
                    }}
                  />
                  <Typography
                    variant="caption"
                    sx={{
                      display: "-webkit-box",
                      WebkitLineClamp: 2,
                      WebkitBoxOrient: "vertical",
                      overflow: "hidden",
                      fontWeight: 500,
                      lineHeight: 1.4,
                      flex: 1,
                    }}
                  >
                    {rec.title}
                  </Typography>
                </Box>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "flex-end",
                    gap: 0.5,
                    mt: 1,
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{
                      fontSize: "0.65rem",
                      color: orangeMain,
                      fontWeight: 600,
                    }}
                  >
                    Read
                  </Typography>
                  <ArrowUpRight size={12} color={orangeMain} />
                </Box>
              </CardContent>
            </Card>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
