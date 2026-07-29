"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { sanitizeNextUrl, useAuthStore } from "@multica/core/auth";
import { paths, resolvePostAuthDestination } from "@multica/core/paths";
import type { User, Workspace } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@multica/ui/components/ui/card";
import { Loader2, RefreshCw } from "lucide-react";

async function destinationFor(
  qc: QueryClient,
  user: User,
  workspaces: Workspace[],
  nextUrl: string | null,
): Promise<string> {
  if (nextUrl) return nextUrl;
  if (!user.onboarded_at) {
    try {
      const invitations = await api.listMyInvitations();
      if (invitations.length > 0) {
        qc.setQueryData(workspaceKeys.myInvitations(), invitations);
        return paths.invitations();
      }
    } catch {
      // Invitation lookup is not required to finish authentication.
    }
  }
  return resolvePostAuthDestination(workspaces, user.onboarded_at != null);
}

function LoginPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const qc = useQueryClient();
  const loginWithSSO = useAuthStore((state) => state.loginWithSSO);
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState("");
  const nextUrl = sanitizeNextUrl(searchParams.get("next"));

  useEffect(() => {
    let active = true;
    setError("");
    void (async () => {
      try {
        const user = await loginWithSSO();
        const workspaces = await api.listWorkspaces();
        qc.setQueryData(workspaceKeys.list(), workspaces);
        const destination = await destinationFor(qc, user, workspaces, nextUrl);
        if (active) router.replace(destination);
      } catch (err) {
        if (active) setError(err instanceof Error ? err.message : "SSO sign-in failed");
      }
    })();
    return () => {
      active = false;
    };
  }, [attempt, loginWithSSO, nextUrl, qc, router]);

  return (
    <main className="flex min-h-svh items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle>{error ? "Sign-in failed" : "Signing in"}</CardTitle>
          <CardDescription>
            {error || "Completing company SSO authentication..."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center">
          {error ? (
            <Button onClick={() => setAttempt((value) => value + 1)}>
              <RefreshCw />
              Retry
            </Button>
          ) : (
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          )}
        </CardContent>
      </Card>
    </main>
  );
}

export default function Page() {
  return (
    <Suspense fallback={null}>
      <LoginPageContent />
    </Suspense>
  );
}
