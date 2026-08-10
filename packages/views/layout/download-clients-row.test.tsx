import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { configStore } from "@multica/core/config";
import enLayout from "../locales/en/layout.json";
import { DownloadClientsRow } from "./download-clients-row";

// react-i18next isn't initialised in the views test env, so resolve the
// selector against the real en/layout.json to assert on actual copy.
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (sel: (r: typeof enLayout) => string) => sel(enLayout),
  }),
}));

afterEach(() => {
  configStore.getState().setDaemonConfig({});
});

describe("DownloadClientsRow", () => {
  it("links to the configured app origin's /download page", () => {
    configStore
      .getState()
      .setDaemonConfig({ daemonAppUrl: "https://app.example.com/" });
    render(<DownloadClientsRow />);
    const link = screen.getByText("Download apps").closest("a");
    expect(link).toHaveAttribute("href", "https://app.example.com/download");
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("degrades to a same-origin /download link before config resolves", () => {
    render(<DownloadClientsRow />);
    expect(screen.getByText("Download apps").closest("a")).toHaveAttribute(
      "href",
      "/download",
    );
  });

  // The row replaced a dismissible promo; as the only in-app path to the
  // download page it must not be dismissible.
  it("offers no dismiss affordance", () => {
    render(<DownloadClientsRow />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
