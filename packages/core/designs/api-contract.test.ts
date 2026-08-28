// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ApiClient repository design asset contracts", () => {
  it("scopes Design Files with snake_case parameters while preserving the legacy URL", async () => {
    const json = () => new Response(JSON.stringify({ design_files: [], total: 0 }));
    const fetchMock = vi.fn().mockImplementation(async () => json());
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.listDesignFiles({ projectId: "project-1", projectResourceId: "repo-1" });
    await client.listDesignFiles();

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "https://api.example.test/api/design-files?project_id=project-1&project_resource_id=repo-1",
      "https://api.example.test/api/design-files",
    ]);
  });

  it("scopes Design Documents with snake_case parameters and keeps project-only reads compatible", async () => {
    const json = () => new Response(JSON.stringify({ documents: [] }));
    const fetchMock = vi.fn().mockImplementation(async () => json());
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.listDesignDocuments("project-1", "repo-1");
    await client.listDesignDocuments("project-1");

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "https://api.example.test/api/design-documents?project_id=project-1&project_resource_id=repo-1",
      "https://api.example.test/api/design-documents?project_id=project-1",
    ]);
  });

  it("submits a typed mixed repository association and returns the strict response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      project_id: "project-1",
      project_resource_id: "repo-1",
      count: 2,
    })));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.setDesignAssetRepositoryAssociation({
      project_id: "project-1",
      project_resource_id: "repo-1",
      items: [
        { kind: "design_file", id: "file-1" },
        { kind: "design_document", id: "document-1" },
      ],
    })).resolves.toEqual({ project_id: "project-1", project_resource_id: "repo-1", count: 2 });

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe("https://api.example.test/api/design-assets/repository-association");
    expect(init?.method).toBe("PUT");
    expect(JSON.parse(String(init?.body))).toEqual({
      project_id: "project-1",
      project_resource_id: "repo-1",
      items: [
        { kind: "design_file", id: "file-1" },
        { kind: "design_document", id: "document-1" },
      ],
    });
  });

  it("rejects a malformed repository association success response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      project_id: "project-1",
      project_resource_id: "repo-1",
      count: -1,
    })));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.setDesignAssetRepositoryAssociation({
      project_id: "project-1",
      project_resource_id: "repo-1",
      items: [{ kind: "design_file", id: "file-1" }],
    })).rejects.toThrow();
  });
});
