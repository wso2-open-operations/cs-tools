import React from "react";
import { Box, Typography, Button } from "@mui/material";
import type { CallRequest } from "@/types/case.types";
import { PhoneIcon, PhoneCallIcon } from "@/assets/icons/common-icons";

interface CaseCallsProps {
  callRequests: CallRequest[];
}

export const CaseCalls: React.FC<CaseCallsProps> = ({ callRequests }) => {
  return (
    <Box sx={{ outline: "none", flex: 1, overflowY: "auto", m: 0 }}>
      <Box sx={{ maxWidth: "1440px", mx: "auto" }}>
        <Box sx={{ mb: 2 }}>
          <Button
            startIcon={<PhoneCallIcon width={16} height={16} />}
            sx={{
              bgcolor: "#ea580c", // orange-600
              "&:hover": { bgcolor: "#c2410c" }, // orange-700
              color: "white",
              textTransform: "none",
              gap: 1,
              px: 2,
              py: 1,
              borderRadius: "12px",
              fontSize: "0.875rem",
              //height: 36,
            }}
          >
            Request Call
          </Button>
        </Box>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          {callRequests.map((call) => (
            <Box
              key={call.sysId}
              sx={{
                bgcolor: "background.paper",
                color: "text.primary",
                display: "flex",
                flexDirection: "column",
                gap: 3,
                borderRadius: "12px",
                border: "1px solid",
                borderColor: "grey.200",
                p: 2,
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  justifyContent: "space-between",
                  mb: 0,
                }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                  <Box
                    sx={{
                      width: 40,
                      height: 40,
                      bgcolor: "#dbeafe", // blue-100
                      borderRadius: "50%",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                    }}
                  >
                    <PhoneIcon width={20} height={20} color="#2563eb" />{" "}
                    {/* blue-600 */}
                  </Box>
                  <Box>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1,
                        mb: 0.5,
                      }}
                    >
                      <Typography
                        sx={{ fontSize: "0.875rem", color: "grey.900" }}
                      >
                        Call Request
                      </Typography>
                      <Box
                        component="span"
                        sx={{
                          display: "inline-flex",
                          alignItems: "center",
                          justifyContent: "center",
                          borderRadius: "6px",
                          border: "1px solid #bfdbfe",
                          px: 1,
                          py: 0.25,
                          fontSize: "0.75rem",
                          fontWeight: 500,
                          whiteSpace: "nowrap",
                          bgcolor: "#dbeafe",
                          color: "#1d4ed8",
                        }}
                      >
                        {call.state || "Scheduled"}
                      </Box>
                    </Box>
                    <Typography sx={{ fontSize: "0.75rem", color: "grey.600" }}>
                      Requested on{" "}
                      {new Date(call.requestedDate).toLocaleString()}
                    </Typography>
                  </Box>
                </Box>
              </Box>

              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: "repeat(2, 1fr)",
                  gap: 1.5,
                  fontSize: "0.875rem",
                }}
              >
                <Box>
                  <Typography sx={{ color: "grey.600" }}>
                    Preferred Time
                  </Typography>
                  <Typography sx={{ color: "grey.900" }}>
                    {call.preferredTime || "-"}
                  </Typography>
                </Box>
                <Box>
                  <Typography sx={{ color: "grey.600" }}>Reason</Typography>
                  <Typography sx={{ color: "grey.900" }}>
                    {call.reason || "-"}
                  </Typography>
                </Box>
                <Box>
                  <Typography sx={{ color: "grey.600" }}>Duration</Typography>
                  <Typography sx={{ color: "grey.900" }}>
                    {call.duration || "-"}
                  </Typography>
                </Box>
              </Box>

              <Box
                sx={{
                  mt: 1.5,
                  pt: 1.5,
                  borderTop: "1px solid",
                  borderColor: "grey.200",
                }}
              >
                <Typography
                  sx={{ fontSize: "0.75rem", color: "grey.600", mb: 0.5 }}
                >
                  Notes
                </Typography>
                <Typography sx={{ fontSize: "0.875rem", color: "grey.900" }}>
                  {call.reason || "No notes provided."}
                </Typography>
              </Box>
            </Box>
          ))}

          {callRequests.length === 0 && (
            <Typography
              sx={{ textAlign: "center", color: "text.secondary", mt: 4 }}
            >
              No call requests found.
            </Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
};
