"use client";

import { use } from "react";
import { DesignRestoreTaskPage } from "@multica/views/designs";

export default function Page({ params }: { params: Promise<{ taskId: string }> }) {
  const { taskId } = use(params);
  return <DesignRestoreTaskPage taskId={taskId} />;
}
