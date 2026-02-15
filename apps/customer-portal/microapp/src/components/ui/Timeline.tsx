import { Timeline as MuiTimeline } from "@mui/lab";

export function Timeline({ children }: React.PropsWithChildren) {
  return (
    <MuiTimeline
      position="right"
      sx={{
        p: 0,
        [`& .MuiTimelineItem-root:before`]: {
          flex: 0,
          padding: 0,
        },
      }}
    >
      {children}
    </MuiTimeline>
  );
}
