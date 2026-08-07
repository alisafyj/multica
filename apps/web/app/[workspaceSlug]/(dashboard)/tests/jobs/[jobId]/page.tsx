"use client";

import { use } from "react";
import { TestGenerationJobPage } from "@multica/views/testing";

export default function Page({
  params,
}: {
  params: Promise<{ jobId: string }>;
}) {
  const { jobId } = use(params);
  return <TestGenerationJobPage jobId={jobId} />;
}
