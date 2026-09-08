export type PendingInputState = "open" | "answered" | "cancelled" | "expired";

export interface PendingInputOption {
  label: string;
  description: string;
}

export interface PendingInputQuestion {
  id: string;
  header: string;
  question: string;
  options: PendingInputOption[];
  allow_other: boolean;
  multi_select: boolean;
}

export interface PendingInputAnswer {
  answers: string[];
}

export interface PendingInput {
  id: string;
  task_id: string;
  issue_id: string;
  question_comment_id: string;
  state: PendingInputState;
  version: number;
  questions: PendingInputQuestion[];
  answers: Record<string, PendingInputAnswer> | null;
  created_at: string;
  answered_at: string | null;
  acked_at: string | null;
}

export interface ListPendingInputsResponse {
  data: PendingInput[];
}

export interface AnswerPendingInputRequest {
  idempotency_key: string;
  answers: Record<string, PendingInputAnswer>;
}
