"use client";

import { use } from "react";
import { DesignDraftPage } from "@multica/views/designs";

export default function Page({ params }: { params: Promise<{ draftId: string }> }) {
  const { draftId } = use(params);
  return <DesignDraftPage draftId={draftId} />;
}
