"use client";

import { useParams } from "next/navigation";
import { TestCaseDetail } from "@multica/views/testing";

export default function Page() {
  const params = useParams<{ id: string }>();
  return <TestCaseDetail refId={params.id} />;
}
