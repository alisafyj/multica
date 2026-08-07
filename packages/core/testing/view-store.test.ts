import { beforeEach, describe, expect, it } from "vitest";
import {
  EMPTY_TEST_CASE_FILTERS,
  TEST_CASE_DEFAULT_HIDDEN_COLUMNS,
  useTestCaseViewStore,
} from "./stores/view-store";

describe("useTestCaseViewStore", () => {
  beforeEach(() => {
    useTestCaseViewStore.setState({
      projectId: null,
      module: null,
      hiddenColumns: TEST_CASE_DEFAULT_HIDDEN_COLUMNS,
      filters: EMPTY_TEST_CASE_FILTERS,
    });
  });

  it("clears the module when the project changes", () => {
    useTestCaseViewStore.getState().setModule("订单");
    useTestCaseViewStore.getState().setProjectId("p1");
    expect(useTestCaseViewStore.getState().module).toBeNull();
    expect(useTestCaseViewStore.getState().projectId).toBe("p1");
  });

  it("toggles a filter value on and back off", () => {
    const { toggleFilter } = useTestCaseViewStore.getState();
    toggleFilter("statuses", "draft");
    expect(useTestCaseViewStore.getState().filters.statuses).toEqual(["draft"]);
    useTestCaseViewStore.getState().toggleFilter("statuses", "draft");
    expect(useTestCaseViewStore.getState().filters.statuses).toEqual([]);
  });

  it("clears every filter dimension at once", () => {
    const store = useTestCaseViewStore.getState();
    store.toggleFilter("statuses", "draft");
    store.toggleFilter("priorities", "p0");
    useTestCaseViewStore.getState().clearFilters();
    expect(useTestCaseViewStore.getState().filters).toEqual(EMPTY_TEST_CASE_FILTERS);
  });

  it("keeps the filters object referentially stable when nothing changes", () => {
    const before = useTestCaseViewStore.getState().filters;
    useTestCaseViewStore.getState().setModule("订单");
    expect(useTestCaseViewStore.getState().filters).toBe(before);
  });
});
