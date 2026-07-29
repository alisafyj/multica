"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { sanitizeNextUrl, useAuthStore } from "@multica/core/auth";
import { useConfigStore } from "@multica/core/config";
import { paths, resolvePostAuthDestination } from "@multica/core/paths";
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

function CallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const qc = useQueryClient();
  const loginWithGoogle = useAuthStore((state) => state.loginWithGoogle);
  const [error, setError] = useState("");
  const [desktopToken, setDesktopToken] = useState<string | null>(null);

  useEffect(() => {
    const code = searchParams.get("code");
    if (!code) {
      setError("Missing authorization code");
      return;
    }
    const errorParam = searchParams.get("error");
    if (errorParam) {
      setError(errorParam === "access_denied" ? "Access denied" : errorParam);
      return;
    }

    const stateParts = (searchParams.get("state") || "").split(",");
    const isDesktop = stateParts.includes("platform:desktop");
    const nextPart = stateParts.find((part) => part.startsWith("next:"));
    const nextUrl = sanitizeNextUrl(nextPart ? nextPart.slice(5) : null);
    const redirectUri = `${window.location.origin}/auth/callback`;

    if (isDesktop) {
      api
        .googleLogin(code, redirectUri)
        .then(({ token }) => {
          setDesktopToken(token);
          window.location.href = `multica://auth/callback?token=${encodeURIComponent(token)}`;
        })
        .catch((err) => {
          setError(err instanceof Error ? err.message : "Login failed");
        });
      return;
    }

    loginWithGoogle(code, redirectUri)
      .then(async (user) => {
        const workspaces = await api.listWorkspaces();
        qc.setQueryData(workspaceKeys.list(), workspaces);
        const onboarded = user.onboarded_at != null;
        if (nextUrl) {
          router.push(nextUrl);
          return;
        }
        if (!onboarded) {
          try {
            const invitations = await api.listMyInvitations();
            if (invitations.length > 0) {
              qc.setQueryData(workspaceKeys.myInvitations(), invitations);
              router.push(paths.invitations());
              return;
            }
          } catch {
            // Invitation lookup is not required to finish authentication.
          }
        }
        router.push(resolvePostAuthDestination(workspaces, onboarded));
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Login failed");
      });
  }, [loginWithGoogle, qc, router, searchParams]);

  if (desktopToken) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Opening Multica</CardTitle>
            <CardDescription>
              You should see a prompt to open the Multica desktop app. If
              nothing happens, click the button below.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center">
            <Button
              variant="outline"
              onClick={() => {
                window.location.href = `multica://auth/callback?token=${encodeURIComponent(desktopToken)}`;
              }}
            >
              Open Multica Desktop
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Login Failed</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center">
            <a href={paths.login()} className="text-primary underline-offset-4 hover:underline">
              Back to login
            </a>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Signing in...</CardTitle>
          <CardDescription>Please wait while we complete your login</CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    </div>
  );
}

function ConfigStatus({ error }: { error: string | null }) {
  const loadConfig = useConfigStore((state) => state.loadConfig);
  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle>
            {error ? "Unable to load sign-in" : "Loading sign-in configuration"}
          </CardTitle>
          <CardDescription>
            {error || "Checking the server authentication mode..."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center">
          {error ? (
            <Button
              onClick={() => {
                void loadConfig(() => api.getConfig()).catch(() => {});
              }}
            >
              <RefreshCw />
              Retry
            </Button>
          ) : (
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function RedirectFromSSOCallback() {
  const router = useRouter();
  useEffect(() => {
    router.replace(paths.login());
  }, [router]);
  return null;
}

function CallbackModeContent() {
  const useSySso = useConfigStore((state) => state.useSySso);
  const configError = useConfigStore((state) => state.authConfigError);
  if (useSySso === null) return <ConfigStatus error={configError} />;
  if (useSySso) return <RedirectFromSSOCallback />;
  return <CallbackContent />;
}

export default function CallbackPage() {
  return (
    <Suspense fallback={null}>
      <CallbackModeContent />
    </Suspense>
  );
}
