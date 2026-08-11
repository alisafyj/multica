"use client";

import { use } from "react";
import { TestPlanDetail } from "@multica/views/testing";

export default function Page({
  params,
}: {
  params: Promise<{ planId: string }>;
}) {
  const { planId } = use(params);
  return <TestPlanDetail planId={planId} />;
}
