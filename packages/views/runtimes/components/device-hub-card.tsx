"use client";

import { useState } from "react";
import { QrCode, Smartphone } from "lucide-react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import QRCode from "react-qr-code";
import type { AgentRuntime } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { runtimeDeviceHubOptions, useUpdateRuntime } from "@multica/core/runtimes";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT, useTimeAgo } from "../../i18n";

/**
 * The device hub (multica-device-mcp) on this machine, as its daemon last
 * reported it, plus the "test host" designation that lets a device round be
 * bound to the hub's phones. The pairing QR is the same string the hub CLI
 * prints; the Multica app scans it (More → Device executor).
 *
 * Only people who may edit the runtime see the switch and the pairing code:
 * the code lets anyone on the LAN pair a phone into the hub.
 */
export function DeviceHubCard({ runtime, canEdit }: { runtime: AgentRuntime; canEdit: boolean }) {
  const { t } = useT("runtimes");
  const timeAgo = useTimeAgo();
  const wsId = useWorkspaceId();
  const { data: hub } = useQuery(runtimeDeviceHubOptions(runtime.id));
  const update = useUpdateRuntime(wsId);
  const [showPairing, setShowPairing] = useState(false);

  const testHost = runtime.test_host_enabled === true;
  const reachable = hub?.reachable === true;

  function setTestHost(enabled: boolean) {
    update.mutate(
      { runtimeId: runtime.id, patch: { test_host_enabled: enabled } },
      {
        onError: (err) =>
          toast.error(err instanceof Error ? err.message : t(($) => $.detail.device_hub_toggle_failed)),
      },
    );
  }

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between gap-2 border-b px-4 py-2.5">
        <span className="inline-flex items-center gap-1.5 text-caption font-semibold">
          <Smartphone className="h-3.5 w-3.5" />
          {t(($) => $.detail.device_hub_title)}
        </span>
      </div>
      <div className="space-y-3 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="text-caption font-medium">{t(($) => $.detail.device_hub_test_host)}</div>
            <p className="text-micro text-muted-foreground">{t(($) => $.detail.device_hub_test_host_hint)}</p>
          </div>
          <Switch
            checked={testHost}
            disabled={!canEdit || update.isPending}
            onCheckedChange={(checked) => setTestHost(checked === true)}
            aria-label={t(($) => $.detail.device_hub_test_host)}
          />
        </div>

        <div className="text-caption">
          {reachable ? (
            <>
              <span className="font-medium text-success">
                {t(($) => $.detail.device_hub_online, { version: hub?.version || "?" })}
              </span>
              <span className="ml-2 text-muted-foreground">
                {t(($) => $.detail.device_hub_counts, {
                  devices: hub?.devices ?? 0,
                  phones: hub?.phones ?? 0,
                  leases: hub?.leases ?? 0,
                })}
              </span>
            </>
          ) : (
            <>
              <span className="font-medium text-muted-foreground">{t(($) => $.detail.device_hub_offline)}</span>
              <p className="mt-1 text-micro text-muted-foreground">{t(($) => $.detail.device_hub_offline_hint)}</p>
            </>
          )}
          {hub?.reported_at ? (
            <p className="mt-1 text-micro text-muted-foreground">
              {t(($) => $.detail.device_hub_reported, { when: timeAgo(hub.reported_at) })}
            </p>
          ) : null}
        </div>

        {canEdit && hub?.pairing_url ? (
          <div className="border-t pt-3">
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 px-2 text-caption"
              onClick={() => setShowPairing((v) => !v)}
            >
              <QrCode className="h-3.5 w-3.5" />
              {showPairing ? t(($) => $.detail.device_hub_hide_pairing) : t(($) => $.detail.device_hub_show_pairing)}
            </Button>
            {showPairing ? (
              <div className="mt-2 flex flex-col items-start gap-2">
                <div className="rounded-md bg-white p-2">
                  <QRCode value={hub.pairing_url} size={160} />
                </div>
                <div className="text-micro text-muted-foreground">
                  <div className="font-mono text-caption text-foreground">{hub.pairing_code}</div>
                  <div className="break-all font-mono">{hub.pairing_url}</div>
                </div>
                <p className="text-micro text-muted-foreground">{t(($) => $.detail.device_hub_pairing_hint)}</p>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}
