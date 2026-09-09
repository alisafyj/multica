import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../../platform/storage";

/**
 * Display preferences for the issue-detail comment composer.
 *
 * `sticky` pins the bottom comment bar to the scroll viewport so it stays
 * reachable while reading a long timeline. It's a personal reading-ergonomics
 * preference (like theme), so it persists globally via `defaultStorage`
 * rather than per-workspace storage.
 *
 * `concise` opts agent runs triggered by this member's comments into the
 * lightweight task prompt. It is a per-member execution preference shared by
 * the top-level composer and every thread reply box, so a conversation keeps
 * one mode instead of re-answering per box (mirrors the chat store's
 * per-session concise selection, SY-326).
 */
interface CommentComposerStore {
  sticky: boolean;
  toggleSticky: () => void;
  concise: boolean;
  setConcise: (v: boolean) => void;
}

export const useCommentComposerStore = create<CommentComposerStore>()(
  persist(
    (set) => ({
      sticky: true,
      toggleSticky: () => set((s) => ({ sticky: !s.sticky })),
      concise: false,
      setConcise: (v) => set({ concise: v }),
    }),
    {
      name: "multica_comment_composer",
      storage: createJSONStorage(() => defaultStorage),
    },
  ),
);
