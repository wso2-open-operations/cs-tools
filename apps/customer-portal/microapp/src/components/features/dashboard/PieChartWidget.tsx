import { Box, Stack, Typography } from "@wso2/oxygen-ui";
import { PieChart } from "@wso2/oxygen-ui-charts-react";
import { Circle } from "@mui/icons-material";
import { WidgetBox } from "@components/ui";

export interface PieDataItem {
  label: string;
  value: number;
  color: string;

  [key: string]: string | number;
}

interface PieChartWidgetProps {
  title: string;
  data: PieDataItem[];
}

export function PieChartWidget({ title, data }: PieChartWidgetProps) {
  const total = data.reduce((sum, item) => sum + item.value, 0);

  return (
    <WidgetBox title={title}>
      <Box>
        <Box position="relative" width={200} height={200}>
          <PieChart
            height={200}
            data={data}
            colors={data.map((item) => item.color)}
            pies={[{ nameKey: "label", dataKey: "value", innerRadius: "50%", paddingAngle: 5 }]}
            margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
            legend={{ show: false }}
            tooltip={{ show: false }}
          />
          <Typography
            variant="h5"
            sx={{
              position: "absolute",
              top: 0,
              left: 0,
              height: "100%",
              width: "100%",
              display: "grid",
              placeItems: "center",
            }}
          >
            {total}
          </Typography>
        </Box>
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
