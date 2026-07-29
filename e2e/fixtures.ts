/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import "./env";
import { createHmac, randomBytes } from "node:crypto";
import pg from "pg";

// `||` (not `??`) so an empty `NEXT_PUBLIC_API_URL=` in .env still falls
// back to localhost. dotenv sets unset-vs-empty both as "" — treating them
// the same matches user intent.
const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";
const JWT_SECRET = process.env.JWT_SECRET || "multica-dev-secret-change-in-production";

function signInternalToken(userId: string, email: string, expiresAt: number) {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const header = encode({ alg: "HS256", typ: "JWT" });
  const claims = encode({
    sub: userId,
    email,
    auth_source: "sso",
    iat: Math.floor(Date.now() / 1000),
    exp: expiresAt,
  });
  const unsigned = `${header}.${claims}`;
  const signature = createHmac("sha256", JWT_SECRET).update(unsigned).digest("base64url");
  return `${unsigned}.${signature}`;
}

function csrfTokenFor(token: string) {
  const nonce = randomBytes(16);
  const signature = createHmac("sha256", token).update(nonce).digest("hex");
  return `${nonce.toString("hex")}.${signature}`;
}

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

export class TestApiClient {
  private token: string | null = null;
  private csrfToken: string | null = null;
  private expiresAt: number | null = null;
  private workspaceSlug: string | null = null;
  private workspaceId: string | null = null;
  private createdIssueIds: string[] = [];

  async login(email: string, name: string) {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const normalizedEmail = email.trim().toLowerCase();
      const result = await client.query<{
        id: string;
        name: string;
        email: string;
        account_kind: string;
      }>(
        `INSERT INTO "user" AS existing_user (name, email, account_kind)
         VALUES ($1, $2, 'human')
         ON CONFLICT (email) DO UPDATE
           SET name = EXCLUDED.name, updated_at = now()
           WHERE existing_user.account_kind = 'human'
         RETURNING id, name, email, account_kind`,
        [name, normalizedEmail],
      );
      if (result.rows.length === 0) {
        throw new Error(`E2E login email belongs to a service account: ${normalizedEmail}`);
      }
      const user = result.rows[0];
      this.expiresAt = Math.floor(Date.now() / 1000) + 60 * 60;
      this.token = signInternalToken(user.id, user.email, this.expiresAt);
      this.csrfToken = csrfTokenFor(this.token);
      return { token: this.token, user };
    } finally {
      await client.end();
    }
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  setWorkspaceSlug(slug: string) {
    this.workspaceSlug = slug;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    let workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (!workspace) {
      const res = await this.authedFetch("/api/workspaces", {
        method: "POST",
        body: JSON.stringify({ name, slug }),
      });
      if (res.ok) {
        workspace = (await res.json()) as TestWorkspace;
      } else {
        const refreshed = await this.getWorkspaces();
        workspace = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
      }
    }

    if (!workspace) {
      throw new Error(`Failed to ensure workspace ${slug}`);
    }

    this.workspaceId = workspace.id;
    this.workspaceSlug = workspace.slug;
    const questionnaire = await this.authedFetch("/api/me/onboarding", {
      method: "PATCH",
      body: JSON.stringify({
        questionnaire: {
          source: [],
          source_other: "",
          source_skipped: true,
          role: "",
          role_other: "",
          role_skipped: true,
          use_case: [],
          use_case_other: "",
          use_case_skipped: true,
          version: 2,
        },
      }),
    });
    if (!questionnaire.ok) {
      throw new Error(`Failed to complete E2E questionnaire: ${questionnaire.status}`);
    }
    const onboarding = await this.authedFetch("/api/me/onboarding/complete", {
      method: "POST",
      body: JSON.stringify({ completion_path: "skip_existing", workspace_id: workspace.id }),
    });
    if (!onboarding.ok) {
      throw new Error(`Failed to complete E2E onboarding: ${onboarding.status}`);
    }
    return workspace;
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  /** Clean up all issues created during this test. */
  async cleanup() {
    for (const id of this.createdIssueIds) {
      try {
        await this.deleteIssue(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdIssueIds = [];
  }

  getToken() {
    return this.token;
  }

  getBrowserSession() {
    if (!this.token || !this.csrfToken || !this.expiresAt) {
      throw new Error("test api client not logged in");
    }
    return { token: this.token, csrfToken: this.csrfToken, expiresAt: this.expiresAt };
  }

  private async authedFetch(path: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceSlug) headers["X-Workspace-Slug"] = this.workspaceSlug;
    else if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
