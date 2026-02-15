import type { ActivityTimelineEntryProps } from "@components/features/detail";

export const MOCK_ACTIVITY_TIMELINE: Omit<ActivityTimelineEntryProps, "variant">[] = [
  {
    author: "System",
    title: "created this case",
    timestamp: "2 days ago",
  },
  {
    author: "System",
    title: "assigned to Support Team",
    timestamp: "2 days ago",
  },
  {
    author: "System",
    title: "assigned to Support Team",
    timestamp: "2 days ago",
  },
  {
    author: "System",
    timestamp: "2 days ago",
    comment: "I've reviewed your case. Could you please share the error logs from your authentication service?",
    attachment: "auth-service-logs.txt",
  },
];
