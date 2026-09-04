// Shared grid templates so issue-list headers and rows stay aligned.
export const gridTemplate = (showSlaState: boolean) =>
  showSlaState
    ? "minmax(0,1fr) 158px 56px 168px 132px 150px 56px"
    : "minmax(0,1fr) 158px 56px 168px 150px 56px";
