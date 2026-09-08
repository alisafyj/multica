/** @vitest-environment jsdom */
import "@testing-library/jest-dom/vitest";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { PendingInput } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { PendingInputForm } from "./pending-input-form";

const openInput = {
  id: "input-1",
  task_id: "task-1",
  issue_id: "issue-1",
  question_comment_id: "comment-1",
  state: "open",
  version: 1,
  questions: [
    {
      id: "details",
      header: "Details",
      question: "What should change?",
      options: [],
      allow_other: true,
      multi_select: false,
    },
  ],
  answers: null,
  created_at: "2026-09-06T00:00:00Z",
  answered_at: null,
  acked_at: null,
} satisfies PendingInput;

describe("PendingInputForm", () => {
  it("does not submit a cancelled request", async () => {
    const onAnswer = vi.fn();
    renderWithI18n(
      <PendingInputForm pendingInput={{ ...openInput, state: "cancelled" }} onAnswer={onAnswer} />,
    );

    expect(screen.getByText("Cancelled")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send answers" })).not.toBeInTheDocument();
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("preserves a stale draft but disables submission when the request closes", async () => {
    const user = userEvent.setup();
    const view = renderWithI18n(
      <PendingInputForm pendingInput={openInput} onAnswer={vi.fn()} />,
    );
    await user.type(screen.getByRole("textbox"), "Keep this draft");

    view.rerender(
      <PendingInputForm pendingInput={{ ...openInput, state: "expired" }} onAnswer={vi.fn()} />,
    );

    expect(screen.getByRole("textbox")).toHaveValue("Keep this draft");
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Send answers" })).not.toBeInTheDocument();
  });

  it("suppresses a rapid double click while the answer is in flight", async () => {
    let resolve!: () => void;
    const onAnswer = vi.fn(() => new Promise<void>((done) => { resolve = done; }));
    const user = userEvent.setup();
    renderWithI18n(<PendingInputForm pendingInput={openInput} onAnswer={onAnswer} />);
    await user.type(screen.getByRole("textbox"), "Use the current design");

    const submit = screen.getByRole("button", { name: "Send answers" });
    fireEvent.click(submit);
    fireEvent.click(submit);

    expect(onAnswer).toHaveBeenCalledTimes(1);
    resolve();
  });

  it("hydrates an answer submitted from another client", () => {
    const view = renderWithI18n(
      <PendingInputForm pendingInput={openInput} onAnswer={vi.fn()} />,
    );

    view.rerender(
      <PendingInputForm
        pendingInput={{
          ...openInput,
          state: "answered",
          answers: { details: { answers: ["Use production"] } },
        }}
        onAnswer={vi.fn()}
      />,
    );

    expect(screen.getByRole("textbox")).toHaveValue("Use production");
    expect(screen.getByRole("textbox")).toBeDisabled();
  });

  it("disables future contract versions without interpreting them as V1", () => {
    renderWithI18n(
      <PendingInputForm pendingInput={{ ...openInput, version: 2 }} onAnswer={vi.fn()} />,
    );

    expect(screen.getByText("Update the app to answer this request")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Send answers" })).not.toBeInTheDocument();
  });

  it("enforces text and answer-count bounds and removes duplicate other answers", async () => {
    const onAnswer = vi.fn().mockResolvedValue(undefined);
    const options = Array.from({ length: 8 }, (_, index) => ({
      label: `Option ${index + 1}`,
      description: "",
    }));
    const multiInput: PendingInput = {
      ...openInput,
      questions: [{
        ...openInput.questions[0]!,
        options,
        multi_select: true,
      }],
    };
    const user = userEvent.setup();
    renderWithI18n(<PendingInputForm pendingInput={multiInput} onAnswer={onAnswer} />);

    for (const option of options) await user.click(screen.getByText(option.label));
    const other = screen.getByRole("textbox", { name: "Other answer" });
    expect(other).toHaveAttribute("maxlength", "2000");
    await user.type(other, "Ninth answer");
    expect(screen.getByRole("button", { name: "Send answers" })).toBeDisabled();

    await user.clear(other);
    await user.type(other, "Option 1");
    await user.click(screen.getByRole("button", { name: "Send answers" }));
    expect(onAnswer).toHaveBeenCalledWith({
      details: { answers: options.map((option) => option.label) },
    });
  });

  it("wraps long question and option text inside the form", () => {
    const longLabel = "A".repeat(200);
    renderWithI18n(
      <PendingInputForm
        pendingInput={{
          ...openInput,
          questions: [{
            ...openInput.questions[0]!,
            question: "Q".repeat(2000),
            options: [{ label: longLabel, description: "D".repeat(500) }],
          }],
        }}
        onAnswer={vi.fn()}
      />,
    );

    expect(screen.getByText(longLabel)).toHaveClass("break-words");
    expect(screen.getByText("Q".repeat(2000))).toHaveClass("break-words");
  });
});
