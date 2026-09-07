"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { useRecommendTestCases } from "@multica/core/testing";
import type { TestCaseRecommendation } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

/** Pure: "one path per line" text to trimmed, de-duplicated paths. */
export function parseChangedPaths(text: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (line.length === 0 || seen.has(line)) continue;
    seen.add(line);
    out.push(line);
  }
  return out;
}

/**
 * Change-based regression selection. The tester pastes the paths a change
 * touched (a diff, a PR file list); the server answers with the cases whose
 * repo bindings claim them. From here the cases can be selected in the
 * library (to approve, or add to a plan) or run straight away.
 */
export function RecommendCasesDialog({
  open,
  onOpenChange,
  projectId,
  onSelect,
  onStartRun,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  /** Select these cases in the library list. */
  onSelect: (caseIds: string[]) => void;
  /** Create and open a run for these cases; the page owns navigation. */
  onStartRun: (caseIds: string[]) => Promise<void>;
}) {
  const { t } = useT("testing");
  const recommend = useRecommendTestCases();

  const [text, setText] = useState("");
  const [repo, setRepo] = useState("");
  const [result, setResult] = useState<{ cases: TestCaseRecommendation[]; unmatched: string[] } | null>(null);
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [isStarting, setIsStarting] = useState(false);

  useEffect(() => {
    if (open) return;
    setText("");
    setRepo("");
    setResult(null);
    setChecked(new Set());
  }, [open]);

  const paths = useMemo(() => parseChangedPaths(text), [text]);
  const busy = recommend.isPending || isStarting;
  const selected = useMemo(
    () => (result ? result.cases.map((c) => c.test_case.id).filter((id) => checked.has(id)) : []),
    [result, checked],
  );

  async function find() {
    if (paths.length === 0 || projectId.length === 0) return;
    try {
      const answer = await recommend.mutateAsync({
        project_id: projectId,
        paths,
        ...(repo.trim().length > 0 ? { repo: repo.trim() } : {}),
      });
      setResult({ cases: answer.cases, unmatched: answer.unmatched_paths });
      // Everything the change touches is the default selection; the tester
      // unticks rather than ticks.
      setChecked(new Set(answer.cases.map((c) => c.test_case.id)));
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : t(($) => $.toast.saveFailed));
    }
  }

  function toggle(id: string) {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function startRun() {
    if (selected.length === 0) return;
    setIsStarting(true);
    try {
      await onStartRun(selected);
      onOpenChange(false);
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : t(($) => $.recommend.runFailed));
    } finally {
      setIsStarting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (busy ? undefined : onOpenChange(next))}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.recommend.title)}</DialogTitle>
          <DialogDescription>{t(($) => $.recommend.description)}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="flex flex-col gap-1">
            <Label htmlFor="recommend-paths">{t(($) => $.recommend.paths)}</Label>
            <Textarea
              id="recommend-paths"
              autoFocus
              rows={5}
              value={text}
              disabled={busy}
              placeholder={t(($) => $.recommend.pathsPlaceholder)}
              onChange={(event) => setText(event.target.value)}
              className="font-mono text-caption"
            />
          </div>
          <div className="flex items-end gap-2">
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <Label htmlFor="recommend-repo">{t(($) => $.recommend.repo)}</Label>
              <Input
                id="recommend-repo"
                value={repo}
                disabled={busy}
                onChange={(event) => setRepo(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") void find();
                }}
              />
            </div>
            <Button size="sm" variant="outline" disabled={busy || paths.length === 0} onClick={() => void find()}>
              {recommend.isPending ? t(($) => $.recommend.finding) : t(($) => $.recommend.find)}
            </Button>
          </div>

          {result ? (
            result.cases.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t(($) => $.recommend.empty)}</p>
            ) : (
              <div className="max-h-72 overflow-auto rounded-md border border-border">
                <ul className="divide-y divide-border">
                  {result.cases.map((rec) => (
                    <li key={rec.test_case.id} className="flex items-start gap-3 px-3 py-2">
                      <Checkbox
                        aria-label={rec.test_case.key}
                        checked={checked.has(rec.test_case.id)}
                        disabled={busy}
                        onCheckedChange={() => toggle(rec.test_case.id)}
                        className="mt-0.5"
                      />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="shrink-0 font-mono text-caption text-muted-foreground">{rec.test_case.key}</span>
                          <span className="truncate text-body">{rec.test_case.title}</span>
                          {rec.test_case.module ? (
                            <span className="shrink-0 text-caption text-muted-foreground">{rec.test_case.module}</span>
                          ) : null}
                        </div>
                        <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-caption text-muted-foreground">
                          <span>{t(($) => $.recommend.claimed, { count: rec.path_count })}</span>
                          {rec.matches.map((match) => (
                            <span key={`${match.alias}:${match.glob}`} className="font-mono">
                              {match.alias}:{match.glob}
                            </span>
                          ))}
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            )
          ) : null}
          {result && result.unmatched.length > 0 ? (
            <p className="text-caption text-muted-foreground" title={result.unmatched.join("\n")}>
              {t(($) => $.recommend.unmatched, { count: result.unmatched.length })}
            </p>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="ghost" size="sm" disabled={busy} onClick={() => onOpenChange(false)}>
            {t(($) => $.actions.cancel)}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={busy || selected.length === 0}
            onClick={() => {
              onSelect(selected);
              onOpenChange(false);
            }}
          >
            {t(($) => $.recommend.select)} ({selected.length})
          </Button>
          <Button size="sm" disabled={busy || selected.length === 0} onClick={() => void startRun()}>
            {t(($) => $.recommend.startRun)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
