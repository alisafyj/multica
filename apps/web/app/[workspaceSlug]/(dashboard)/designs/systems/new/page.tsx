import { WorkspaceDesignSystemCreate } from "@multica/views/designs";

// Static segment wins over the [id] route, so "new" names the creation page
// without colliding with a system id.
export default function Page() {
  return <WorkspaceDesignSystemCreate />;
}
