import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ProjectDesignSystemLocator, ProjectDesignSystemScope } from "@multica/core/types";
import { ProjectDesignSystemPreview } from "./project-design-system-preview";

const locators: ProjectDesignSystemLocator[] = [
  { id: "overview", kind: "block", label: "Overview" },
  { id: "button-primary", kind: "component", label: "Primary button" },
];

function dispatchSelection(id: string, source: MessageEventSource | null) {
  const event = new MessageEvent("message", {
    data: { type: "multica:project-design-system-select", id },
  });
  Object.defineProperty(event, "source", { value: source });
  window.dispatchEvent(event);
}

describe("ProjectDesignSystemPreview", () => {
  it("uses sandbox allow-scripts without same-origin forms navigation or popups", () => {
    render(
      <ProjectDesignSystemPreview
        previewHtml="<!doctype html><html><body><button>Save</button></body></html>"
        locators={locators}
        onSelect={vi.fn()}
      />,
    );

    const frame = screen.getByTitle("项目设计体系 UI Kit");
    expect(frame).toHaveAttribute("sandbox", "allow-scripts");
    const sandbox = frame.getAttribute("sandbox") ?? "";
    expect(sandbox).not.toContain("allow-same-origin");
    expect(sandbox).not.toContain("allow-forms");
    expect(sandbox).not.toContain("allow-top-navigation");
    expect(sandbox).not.toContain("allow-popups");
  });

  it("accepts locator messages only from its own iframe and known IDs", () => {
    const onSelect = vi.fn<(scope: ProjectDesignSystemScope) => void>();
    render(
      <ProjectDesignSystemPreview
        previewHtml="<!doctype html><html><body><button>Save</button></body></html>"
        locators={locators}
        onSelect={onSelect}
      />,
    );

    const frame = screen.getByTitle("项目设计体系 UI Kit") as HTMLIFrameElement;
    dispatchSelection("button-primary", window);
    dispatchSelection("unknown", frame.contentWindow);
    expect(onSelect).not.toHaveBeenCalled();

    dispatchSelection("button-primary", frame.contentWindow);
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith({ kind: "component", id: "button-primary" });
  });

  it("shows a preview error instead of treating blank HTML as success", () => {
    render(
      <ProjectDesignSystemPreview
        previewHtml="   "
        locators={locators}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent("UI Kit 暂时不可用");
    expect(screen.queryByTitle("项目设计体系 UI Kit")).not.toBeInTheDocument();
  });
});
