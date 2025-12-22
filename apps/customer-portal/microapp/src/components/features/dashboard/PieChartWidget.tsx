import { Box, Stack, styled, Typography } from "@mui/material";
import { PieChart, useDrawingArea } from "@mui/x-charts";
import { WidgetBox } from "@components/ui";
import { Circle } from "@mui/icons-material";

export interface PieDataItem {
  label: string;
  value: number;
  color: string;
}

interface PieChartWidgetProps {
  title: string;
  data: PieDataItem[];
}

export function PieChartWidget({ title, data }: PieChartWidgetProps) {
  const total = data.reduce((sum, item) => sum + item.value, 0);

  return (
    <WidgetBox title={title}>
      <Stack></Stack>
      <Box>
        <PieChart series={[{ paddingAngle: 2, innerRadius: "50%", outerRadius: "90%", data }]} hideLegend>
          <PieCenterLabel>{total}</PieCenterLabel>
        </PieChart>
      </Box>
      <Stack gap={0.5} mt={1}>
        {data.map((item, index) => (
          <Stack key={index} direction="row" justifyContent="space-between">
            <Stack direction="row" alignItems="center" gap={1}>
              <Circle sx={{ fontSize: 12, color: item.color }} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {item.label}
              </Typography>
            </Stack>
            <Typography variant="subtitle2" color="text.secondary">
              {item.value}
            </Typography>
          </Stack>
        ))}
      </Stack>
    </WidgetBox>
  );
}

const StyledText = styled("text")(({ theme }) => ({
  fill: theme.palette.text.secondary,
  textAnchor: "middle",
  dominantBaseline: "central",
  fontSize: 20,
  fontWeight: 500,
}));

function PieCenterLabel({ children }: { children: React.ReactNode }) {
  const { width, height, left, top } = useDrawingArea();
  return (
    <StyledText x={left + width / 2} y={top + height / 2}>
      {children}
    </StyledText>
  );
}
