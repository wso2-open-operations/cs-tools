import { CalendarDaysIcon } from "../../../assets/icons/common-icons";
import { InfoCard } from "./InfoCard";

export const ChangeRequestCard = () => (
  <InfoCard
    icon={<CalendarDaysIcon width={20} height={20} />}
    title="Change Requests"
    subtitle="Track infrastructure changes"
    colorTheme="purple"
    descriptionTitle="What are Change Requests?"
    descriptionText="Structured workflow for planned infrastructure changes:"
    listItems={[
      "Formal approval process",
      "Scheduled implementation",
      "Customer review and approval",
      "Rollback capabilities",
      "Calendar visualization",
      "Complete audit trail",
    ]}
    secondaryAction={{
      label: "View All Change Requests",
    }}
  />
);
