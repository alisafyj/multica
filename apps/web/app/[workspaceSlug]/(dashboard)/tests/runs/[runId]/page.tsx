"use client";

import { use } from "react";
import { TestRunDetail } from "@multica/views/testing";

export default function Page({
  params,
}: {
  params: Promise<{ runId: string }>;
}) {
  const { runId } = use(params);
  return <TestRunDetail runId={runId} />;
}
