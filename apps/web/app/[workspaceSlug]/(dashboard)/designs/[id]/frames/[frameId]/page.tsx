"use client";

import { use } from "react";
import { DesignFramePage } from "@multica/views/designs";

export default function Page({ params }: { params: Promise<{ id: string; frameId: string }> }) {
  const { id, frameId } = use(params);
  return <DesignFramePage designId={id} frameId={frameId} />;
}
