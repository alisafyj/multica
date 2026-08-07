/**
 * What this client calls itself when talking to the server.
 *
 * Sent on every HTTP request as `X-Client-OS` / `X-Client-Version`
 * (`data/api.ts`) and on the WebSocket upgrade URL as `client_os` /
 * `client_version` (`data/realtime/realtime-provider.tsx` → `ws-client.ts`).
 * The server only logs these — see `server/internal/middleware/client.go`,
 * which treats them as best-effort and never gates on them — so the cost of
 * getting them wrong is silent: every log line points at the wrong platform
 * or a stale release, and nobody notices until someone reads the logs.
 *
 * Both values were previously hardcoded at three call sites (`"ios"` and
 * `"0.1.0"`), which is how Android builds ended up reporting themselves as
 * iPhones and how the version was guaranteed to drift from `app.config.ts`
 * at the first release.
 *
 * Resolved at module load rather than injected because the values are
 * process-wide constants and this dodges any "did setOptions run before the
 * first request" ordering question. That does bind this module to the native
 * runtime — but `data/api.ts`, its main consumer, is already outside the
 * Node-only vitest lane by design (see `vitest.config.ts`: data-layer tests
 * mock `@/data/api` so the native fetch chain never loads). Modules that DO
 * need to stay Node-loadable — `data/realtime/ws-client.ts` — take these as
 * options instead of importing them.
 */
import Constants from "expo-constants";
import { Platform } from "react-native";

/** `"ios"` / `"android"`, matching the lowercase OS vocabulary the CLI and
 *  desktop clients already send (`"macos"` / `"windows"` / `"linux"`). */
export const CLIENT_OS: string = Platform.OS;

/** App version from `app.config.ts`, surfaced through the Expo manifest so
 *  the number lives in exactly one place. Falls back to `"unknown"` — an
 *  honest value the server already handles — rather than a stale literal. */
export const CLIENT_VERSION: string = Constants.expoConfig?.version ?? "unknown";
