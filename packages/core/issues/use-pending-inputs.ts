import { useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { PendingInput, PendingInputAnswer } from "../types";
import { createSafeId } from "../utils";
import { issueKeys } from "./queries";

export function pendingInputsRefetchInterval(data: PendingInput[] | undefined): number | false {
  return data?.some((item) => item.state === "open") ? 5_000 : false;
}

export function usePendingInputs(wsId: string, issueId: string, enabled: boolean) {
  return useQuery({
    queryKey: issueKeys.pendingInputs(wsId, issueId),
    queryFn: async () => (await api.listIssuePendingInputs(issueId)).data,
    enabled: enabled && issueId.length > 0,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => pendingInputsRefetchInterval(query.state.data),
  });
}

function canonicalizeAnswers(
  answers: Record<string, PendingInputAnswer>,
): Record<string, PendingInputAnswer> {
  return Object.fromEntries(
    Object.keys(answers).sort().map((questionId) => [
      questionId,
      { answers: [...(answers[questionId]?.answers ?? [])] },
    ]),
  );
}

export function useAnswerPendingInput(wsId: string, issueId: string) {
  const queryClient = useQueryClient();
  const requestKeys = useRef(new Map<string, { signature: string; key: string }>());
  return useMutation({
    mutationFn: async ({
      pendingInputId,
      answers,
    }: {
      pendingInputId: string;
      answers: Record<string, PendingInputAnswer>;
    }) => {
      const canonicalAnswers = canonicalizeAnswers(answers);
      const signature = JSON.stringify(canonicalAnswers);
      const previous = requestKeys.current.get(pendingInputId);
      const key = previous?.signature === signature ? previous.key : createSafeId();
      requestKeys.current.set(pendingInputId, { signature, key });

      const updated = await api.answerIssuePendingInput(issueId, pendingInputId, {
        idempotency_key: key,
        answers: canonicalAnswers,
      });
      const current = requestKeys.current.get(pendingInputId);
      if (current?.signature === signature && current.key === key) {
        requestKeys.current.delete(pendingInputId);
      }
      return updated;
    },
    onSuccess: (updated) => {
      queryClient.setQueryData<PendingInput[]>(
        issueKeys.pendingInputs(wsId, issueId),
        (current = []) => current.map((item) => item.id === updated.id ? updated : item),
      );
    },
  });
}
