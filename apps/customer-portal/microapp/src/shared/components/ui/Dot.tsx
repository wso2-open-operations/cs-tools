import { Circle } from "@mui/icons-material";

export function Dot() {
  return (
    <Circle
      sx={(theme) => ({
        color: "text.tertiary",
        fontSize: theme.typography.pxToRem(4),
      })}
    />
  );
}
