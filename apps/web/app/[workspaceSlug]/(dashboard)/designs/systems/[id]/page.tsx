"use client";

import { use } from "react";
import { ProjectDesignSystemPage } from "@multica/views/designs";

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <ProjectDesignSystemPage designSystemId={id} />;
}
