import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { CaseIssueLinks } from "./case-issue-links";

const mocks = vi.hoisted(() => ({
  links: [] as unknown[],
  link: vi.fn(),
  unlink: vi.fn(),
  pickerProps: null as Record<string, unknown> | null,
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: mocks.links, isLoading: false }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));

vi.mock("@multica/core/testing", () => ({
  testCaseIssuesOptions: () => ({ queryKey: ["test-cases", "ws-1", "issues"] }),
  useLinkTestCaseIssues: () => ({ mutate: mocks.link, isPending: false }),
  useUnlinkTestCaseIssue: () => ({ mutate: mocks.unlink, isPending: false }),
}));

// The picker is a shared modal with its own coverage; stub it down to a button
// that hands back one issue so this suite tests the wiring, not the search.
vi.mock("../../modals/issue-picker-modal", () => ({
  IssuePickerModal: (props: Record<string, unknown>) => {
    mocks.pickerProps = props;
    return props.open ? (
      <button
        type="button"
        onClick={() =>
          (props.onSelect as (issue: { id: string; identifier: string }) => void)({
            id: "issue-9",
            identifier: "MUL-9",
          })
        }
      >
        pick-issue
      </button>
    ) : null;
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function makeAdapter(): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/tests/TC-1",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
  };
}

function renderLinks() {
  return renderWithI18n(
    <NavigationProvider value={makeAdapter()}>
      <CaseIssueLinks wsId="ws-1" caseRef="TC-1" />
    </NavigationProvider>,
  );
}

function makeLink(overrides = {}) {
  return {
    test_case_id: "c-1",
    issue_id: "issue-1",
    issue_number: 12,
    issue_identifier: "MUL-12",
    issue_title: "Checkout must not double-charge",
    issue_status: "in_progress",
    issue_priority: "high",
    origin: "human",
    created_at: "2024-05-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.links = [];
  mocks.pickerProps = null;
});

describe("CaseIssueLinks", () => {
  it("lists each covered issue with a link to it", async () => {
    mocks.links = [makeLink()];
    renderLinks();

    expect(await screen.findByText("Checkout must not double-charge")).toBeTruthy();
    expect(screen.getByText("MUL-12").closest("a")).toHaveAttribute(
      "href",
      "/acme/issues/issue-1",
    );
  });

  it("links the picked issue to this case", async () => {
    renderLinks();
    const buttons = await screen.findAllByRole("button");
    const add = buttons.find((b) => b.textContent?.match(/关联|Link|紐づけ|연결/));
    expect(add, "the section must offer a way to link an issue").toBeTruthy();
    await userEvent.click(add!);
    await userEvent.click(await screen.findByText("pick-issue"));

    expect(mocks.link).toHaveBeenCalledWith(
      { ref: "TC-1", issueIds: ["issue-9"] },
      expect.anything(),
    );
  });

  // Re-offering an issue the case already covers would let the reviewer create
  // a link the server has to reject as a duplicate.
  it("hides already-linked issues from the picker", async () => {
    mocks.links = [makeLink({ issue_id: "issue-1" })];
    renderLinks();
    const buttons = await screen.findAllByRole("button");
    const add = buttons.find((b) => b.textContent?.match(/关联|Link|紐づけ|연결/));
    await userEvent.click(add!);

    expect(mocks.pickerProps?.excludeIds).toEqual(["issue-1"]);
  });

  it("unlinks one issue", async () => {
    mocks.links = [makeLink()];
    renderLinks();
    const unlink = await screen.findByRole("button", {
      name: /取消关联|Unlink|紐づけを解除|연결 해제/,
    });
    await userEvent.click(unlink);

    expect(mocks.unlink).toHaveBeenCalledWith(
      { ref: "TC-1", issueId: "issue-1" },
      expect.anything(),
    );
  });
});
