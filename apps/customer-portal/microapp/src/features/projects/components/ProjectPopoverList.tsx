import { type PopoverProps, Stack } from "@wso2/oxygen-ui";

import { ProjectPopoverItem } from "@features/projects/components";

export function ProjectPopoverList({ onClose }: { onClose: PopoverProps["onClose"] }) {
  const { projects, projectId, setProjectId } = useProjectSelector();

  return (
    <Stack gap={1} pt={1}>
      {projects.map((props) => (
        <ProjectPopoverItem
          {...props}
          key={props.id}
          active={props.id === projectId}
          onClick={() => {
            setProjectId(props.id);
            onClose?.({}, "backdropClick");
          }}
        />
      ))}
    </Stack>
  );
}
