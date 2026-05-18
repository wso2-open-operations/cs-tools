import { Grid } from "@wso2/oxygen-ui";

import { CalloutFeedback } from "@features/chats/components";
import { BubbleList, InfoField, Layout } from "@features/detail/components";
import { useActions, useChat } from "@features/detail/hooks";

import { SectionCard } from "@shared/components/common";
import { StatusChip } from "@shared/components/support";

import { CASE_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export function ChatItemView() {
  const type = CASE_TYPES.CHAT;

  const { format } = useDateTime();
  const { data, isLoading } = useChat();
  const actions = useActions();

  return (
    <Layout type={type} title={data?.description} id={data?.id} actions={actions}>
      <SectionCard>
        <Grid spacing={1.5} container>
          <Grid size={6}>
            <InfoField label="Started" value={data?.createdOn && format(data.createdOn)} loading={isLoading} />
          </Grid>
          <Grid size={6}>
            <InfoField label="Status" value={<StatusChip type={type} id={data?.statusId} size="small" />} />
          </Grid>
          <Grid size={6}>
            <InfoField label="Messages Exchanged" value={`${data?.count} messages`} loading={isLoading} />
          </Grid>
          <Grid size={6}>
            <InfoField label="Available KBs" value="0 KB articles" loading={isLoading} />
          </Grid>
        </Grid>
      </SectionCard>

      <SectionCard title="Conversation">
        <BubbleList />
      </SectionCard>

      <CalloutFeedback />
    </Layout>
  );
}
