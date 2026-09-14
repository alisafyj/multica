"use client";

import { useEffect, useState } from "react";
import { GithubRepoResourceRefSchema } from "@multica/core/api/schemas";
import type { GithubRepoResourceRef, RepositorySetupStep } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n/use-t";

interface GithubRepositorySettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  value: GithubRepoResourceRef;
  saving?: boolean;
  onSave: (value: GithubRepoResourceRef) => Promise<void>;
}

export function GithubRepositorySettingsDialog({
  open,
  onOpenChange,
  value,
  saving = false,
  onSave,
}: GithubRepositorySettingsDialogProps) {
  const { t } = useT("projects");
  const [trusted, setTrusted] = useState(false);
  const [serverNames, setServerNames] = useState("");
  const [goModDownload, setGoModDownload] = useState(false);
  const [pnpmInstall, setPnpmInstall] = useState(false);
  const [goModDirectory, setGoModDirectory] = useState("");
  const [pnpmDirectory, setPnpmDirectory] = useState("");
  const [setupTimeout, setSetupTimeout] = useState(300);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTrusted(value.configuration_policy === "trusted");
    setServerNames((value.mcp_servers ?? []).join("\n"));
    setGoModDownload(value.setup?.steps.includes("go_mod_download") ?? false);
    setPnpmInstall(value.setup?.steps.includes("pnpm_install") ?? false);
    setGoModDirectory(value.setup?.step_directories?.go_mod_download ?? "");
    setPnpmDirectory(value.setup?.step_directories?.pnpm_install ?? "");
    setSetupTimeout(value.setup?.timeout_seconds ?? 300);
    setError(null);
  }, [open, value]);

  const handleSave = async () => {
    const names = serverNames
      .split(/[\n,]/)
      .map((name) => name.trim())
      .filter(Boolean);
    if (names.length > 64 || names.some((name) => new TextEncoder().encode(name).length > 128) || new Set(names).size !== names.length) {
      setError(t(($) => $.resources.github_settings_mcp_invalid));
      return;
    }
    if (trusted && (goModDownload || pnpmInstall) && (setupTimeout < 1 || setupTimeout > 900)) {
      setError(t(($) => $.resources.github_settings_setup_invalid));
      return;
    }
    const setupSteps = [
      ...(goModDownload ? (["go_mod_download"] as const) : []),
      ...(pnpmInstall ? (["pnpm_install"] as const) : []),
    ];
    const stepDirectories: Partial<Record<RepositorySetupStep, string>> = {};
    if (goModDownload && goModDirectory !== "") {
      stepDirectories.go_mod_download = goModDirectory;
    }
    if (pnpmInstall && pnpmDirectory !== "") {
      stepDirectories.pnpm_install = pnpmDirectory;
    }
    const nextValue: GithubRepoResourceRef = {
      ...value,
      configuration_policy: trusted ? "trusted" : "restricted",
      mcp_servers: names,
      setup: trusted && setupSteps.length > 0
        ? {
            steps: setupSteps,
            timeout_seconds: setupTimeout,
            ...(Object.keys(stepDirectories).length > 0 ? { step_directories: stepDirectories } : {}),
          }
        : undefined,
    };
    if (!GithubRepoResourceRefSchema.safeParse(nextValue).success) {
      setError(t(($) => $.resources.github_settings_setup_directory_invalid));
      return;
    }
    setError(null);
    try {
      await onSave(nextValue);
    } catch {
      setError(t(($) => $.resources.github_settings_save_failed));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(($) => $.resources.github_settings_title)}</DialogTitle>
        </DialogHeader>
        <label className="flex items-center gap-2 text-caption">
          <Checkbox
            checked={trusted}
            onCheckedChange={(checked) => setTrusted(checked === true)}
            aria-label={t(($) => $.resources.github_settings_trust_label)}
          />
          <span>{t(($) => $.resources.github_settings_trust_label)}</span>
        </label>
        <label className="space-y-1 text-caption">
          <span className="font-medium">{t(($) => $.resources.github_settings_mcp_label)}</span>
          <textarea
            value={serverNames}
            onChange={(event) => setServerNames(event.target.value)}
            aria-label={t(($) => $.resources.github_settings_mcp_label)}
            maxLength={8256}
            rows={4}
            className="w-full resize-y rounded-md border bg-transparent px-2 py-1.5 font-mono text-caption outline-none focus-visible:ring-1 focus-visible:ring-ring"
          />
        </label>
        <fieldset className="space-y-2" disabled={!trusted}>
          <legend className="mb-1 text-caption font-medium">
            {t(($) => $.resources.github_settings_setup_label)}
          </legend>
          <label className="flex items-center gap-2 text-caption">
            <Checkbox
              checked={goModDownload}
              onCheckedChange={(checked) => setGoModDownload(checked === true)}
              aria-label={t(($) => $.resources.github_settings_setup_go)}
            />
            <span>{t(($) => $.resources.github_settings_setup_go)}</span>
          </label>
          {goModDownload && (
            <label className="block space-y-1 pl-6 text-caption">
              <span>{t(($) => $.resources.github_settings_setup_go_directory)}</span>
              <input
                type="text"
                value={goModDirectory}
                onChange={(event) => setGoModDirectory(event.target.value)}
                aria-label={t(($) => $.resources.github_settings_setup_go_directory)}
                className="h-8 w-full rounded-md border bg-transparent px-2 font-mono text-caption outline-none focus-visible:ring-1 focus-visible:ring-ring"
              />
            </label>
          )}
          <label className="flex items-center gap-2 text-caption">
            <Checkbox
              checked={pnpmInstall}
              onCheckedChange={(checked) => setPnpmInstall(checked === true)}
              aria-label={t(($) => $.resources.github_settings_setup_pnpm)}
            />
            <span>{t(($) => $.resources.github_settings_setup_pnpm)}</span>
          </label>
          {pnpmInstall && (
            <label className="block space-y-1 pl-6 text-caption">
              <span>{t(($) => $.resources.github_settings_setup_pnpm_directory)}</span>
              <input
                type="text"
                value={pnpmDirectory}
                onChange={(event) => setPnpmDirectory(event.target.value)}
                aria-label={t(($) => $.resources.github_settings_setup_pnpm_directory)}
                className="h-8 w-full rounded-md border bg-transparent px-2 font-mono text-caption outline-none focus-visible:ring-1 focus-visible:ring-ring"
              />
            </label>
          )}
          <label className="block space-y-1 text-caption">
            <span>{t(($) => $.resources.github_settings_setup_timeout)}</span>
            <input
              type="number"
              min={1}
              max={900}
              value={setupTimeout}
              onChange={(event) => setSetupTimeout(Number(event.target.value))}
              aria-label={t(($) => $.resources.github_settings_setup_timeout)}
              className="h-8 w-28 rounded-md border bg-transparent px-2 outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
          </label>
        </fieldset>
        {error && <p className="text-caption text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
            {t(($) => $.resources.github_settings_cancel)}
          </Button>
          <Button onClick={() => void handleSave()} disabled={saving}>
            {t(($) => $.resources.github_settings_save)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
