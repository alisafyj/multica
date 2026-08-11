"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import {
  createWorkspaceAwareStorage,
  registerForWorkspaceRehydration,
} from "../../platform/workspace-storage";
import { defaultStorage } from "../../platform/storage";

// Durable list preferences only: which project and module the user was looking
// at, which filters are on, which columns are hidden. Server data never lands
// here — cases live in the Query cache.

export type TestCaseColumnKey =
  | "module"
  | "caseType"
  | "priority"
  | "origin"
  | "repos"
  | "updated";

/** Repos is opt-in; it is only meaningful on multi-repo projects. */
export const TEST_CASE_DEFAULT_HIDDEN_COLUMNS: TestCaseColumnKey[] = ["repos"];

/** Multi-select filters. An empty array means the dimension is inactive. */
export interface TestCaseViewFilters {
  statuses: string[];
  priorities: string[];
  caseTypes: string[];
  origins: string[];
}

export const EMPTY_TEST_CASE_FILTERS: TestCaseViewFilters = {
  statuses: [],
  priorities: [],
  caseTypes: [],
  origins: [],
};

export interface TestCaseViewState {
  projectId: string | null;
  module: string | null;
  hiddenColumns: TestCaseColumnKey[];
  filters: TestCaseViewFilters;
  setProjectId: (projectId: string | null) => void;
  setModule: (module: string | null) => void;
  toggleColumn: (key: TestCaseColumnKey) => void;
  toggleFilter: (key: keyof TestCaseViewFilters, value: string) => void;
  setFilter: (key: keyof TestCaseViewFilters, values: string[]) => void;
  clearFilters: () => void;
}

const DEFAULTS = {
  projectId: null as string | null,
  module: null as string | null,
  hiddenColumns: TEST_CASE_DEFAULT_HIDDEN_COLUMNS,
  filters: EMPTY_TEST_CASE_FILTERS,
};

export const useTestCaseViewStore = create<TestCaseViewState>()(
  persist(
    (set) => ({
      ...DEFAULTS,
      // Switching project invalidates the module selection: modules are
      // per-project, so keeping the old one would filter to nothing.
      setProjectId: (projectId) => set({ projectId, module: null }),
      setModule: (module) => set({ module }),
      toggleColumn: (key) =>
        set((state) => ({
          hiddenColumns: state.hiddenColumns.includes(key)
            ? state.hiddenColumns.filter((candidate) => candidate !== key)
            : [...state.hiddenColumns, key],
        })),
      toggleFilter: (key, value) =>
        set((state) => {
          const current = state.filters[key];
          const next = current.includes(value)
            ? current.filter((candidate) => candidate !== value)
            : [...current, value];
          return { filters: { ...state.filters, [key]: next } };
        }),
      setFilter: (key, values) =>
        set((state) => ({ filters: { ...state.filters, [key]: values } })),
      clearFilters: () => set({ filters: EMPTY_TEST_CASE_FILTERS }),
    }),
    {
      name: "multica_test_cases_view",
      storage: createJSONStorage(() => createWorkspaceAwareStorage(defaultStorage)),
      partialize: (state) => ({
        projectId: state.projectId,
        module: state.module,
        hiddenColumns: state.hiddenColumns,
        filters: state.filters,
      }),
      // Deep-merge filters so a payload persisted before a dimension existed
      // still gets that key's default and never hits `.length` on undefined.
      merge: (persisted, current) => {
        if (!persisted) return { ...current, ...DEFAULTS };
        const previous = persisted as Partial<TestCaseViewState>;
        return {
          ...current,
          ...previous,
          filters: { ...EMPTY_TEST_CASE_FILTERS, ...(previous.filters ?? {}) },
        };
      },
    },
  ),
);

registerForWorkspaceRehydration(() => useTestCaseViewStore.persist.rehydrate());
