import { useEffect, useRef, useState } from "react";
import { api } from "../api";

/**
 * The last frame the device hub relayed for a running case, as an object URL
 * for an <img>. Polls every `intervalMs` while `enabled`; the server answers
 * 404 until the first frame and after the two-minute retention, so `null`
 * means "nothing to show" rather than an error. Revokes each object URL when
 * the next one replaces it and on unmount, so a long round does not leak.
 */
export function useTestRunCaseLiveFrame(runCaseId: string, enabled: boolean, intervalMs = 2000) {
  const [url, setUrl] = useState<string | null>(null);
  const [capturedAt, setCapturedAt] = useState<number | null>(null);
  const current = useRef<string | null>(null);

  useEffect(() => {
    if (!enabled) {
      if (current.current) URL.revokeObjectURL(current.current);
      current.current = null;
      setUrl(null);
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const tick = async () => {
      try {
        const blob = await api.getTestRunCaseFrame(runCaseId);
        if (cancelled) return;
        if (blob) {
          const next = URL.createObjectURL(blob);
          if (current.current) URL.revokeObjectURL(current.current);
          current.current = next;
          setUrl(next);
          setCapturedAt(Date.now());
        }
      } catch {
        // A transient failure keeps the last frame on screen.
      }
      if (!cancelled) timer = setTimeout(() => void tick(), intervalMs);
    };
    void tick();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      if (current.current) URL.revokeObjectURL(current.current);
      current.current = null;
    };
  }, [runCaseId, enabled, intervalMs]);

  return { url, capturedAt };
}
