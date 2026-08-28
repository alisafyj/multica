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

  it.each([
    {
      name: "missing project_id",
      body: { project_resource_id: "repo-1", count: 1 },
      expectedPath: "project_id",
    },
    {
      name: "non-string project_id",
      body: { project_id: 7, project_resource_id: "repo-1", count: 1 },
      expectedPath: "project_id",
    },
    {
      name: "missing project_resource_id",
      body: { project_id: "project-1", count: 1 },
      expectedPath: "project_resource_id",
    },
    {
      name: "non-string project_resource_id",
      body: { project_id: "project-1", project_resource_id: 7, count: 1 },
      expectedPath: "project_resource_id",
    },
    {
      name: "negative count",
      body: { project_id: "project-1", project_resource_id: "repo-1", count: -1 },
      expectedPath: "count",
    },
    {
      name: "non-integer count",
      body: { project_id: "project-1", project_resource_id: "repo-1", count: 1.5 },
      expectedPath: "count",
    },
  ])("rejects a malformed success response with $name", async ({ body, expectedPath }) => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");
    const malformedResponsePrefix =
      "PUT /api/design-assets/repository-association returned a malformed response: ";

    let rejection: unknown;
    try {
      await client.setDesignAssetRepositoryAssociation({
        project_id: "project-1",
        project_resource_id: "repo-1",
        items: [{ kind: "design_file", id: "file-1" }],
      });
    } catch (error) {
      rejection = error;
    }

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(rejection).toBeInstanceOf(Error);
    const message = (rejection as Error).message;
    expect(message).toContain(malformedResponsePrefix);
    const issues = JSON.parse(message.slice(malformedResponsePrefix.length)) as Array<{
      path: string[];
    }>;
    expect(issues).toEqual(expect.arrayContaining([
      expect.objectContaining({ path: [expectedPath] }),
    ]));
  });
});
