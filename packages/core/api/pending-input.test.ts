import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

const pendingInput = {
  id: "input-1",
  task_id: "task-1",
  issue_id: "issue-1",
  question_comment_id: "comment-1",
  state: "open",
  version: 1,
  questions: [
    {
      id: "environment",
      header: "Environment",
      question: "Where should this run?",
      options: [{ label: "Staging", description: "Use staging data" }],
      allow_other: true,
      multi_select: false,
    },
  ],
  answers: null,
  created_at: "2026-09-06T00:00:00Z",
  answered_at: null,
  acked_at: null,
};

function stubFetch(body: unknown, status = 200) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
}

afterEach(() => vi.unstubAllGlobals());

describe("pending input API", () => {
  it("parses the issue list wrapper", async () => {
    stubFetch({ data: [pendingInput] });
    const client = new ApiClient("https://api.example.test");

    await expect(client.listIssuePendingInputs("issue-1")).resolves.toEqual({
      data: [pendingInput],
    });
  });

  it("treats an old server 404 as no pending inputs", async () => {
    stubFetch({ error: "not found" }, 404);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listIssuePendingInputs("issue-1")).resolves.toEqual({
      data: [],
    });
  });

  it("posts the exact answer contract and returns the updated input", async () => {
    stubFetch({
      ...pendingInput,
      state: "answered",
      answers: { environment: { answers: ["Staging"] } },
      answered_at: "2026-09-06T00:01:00Z",
    });
    const client = new ApiClient("https://api.example.test");

    await client.answerIssuePendingInput("issue-1", "input-1", {
      idempotency_key: "answer-1",
      answers: { environment: { answers: ["Staging"] } },
    });

    expect(vi.mocked(fetch)).toHaveBeenCalledWith(
      "https://api.example.test/api/issues/issue-1/pending-inputs/input-1/answer",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          idempotency_key: "answer-1",
          answers: { environment: { answers: ["Staging"] } },
        }),
      }),
    );
  });
});
