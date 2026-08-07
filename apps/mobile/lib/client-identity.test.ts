/**
 * `lib/client-identity.ts` itself can't be imported here — it reads
 * `expo-constants` / `react-native`, which don't parse in the Node-only
 * vitest lane. So these are structural guards instead, in the spirit of
 * `lib/i18n/parity.test.ts`: they protect the two ways this silently rots.
 *
 * 1. Someone re-hardcodes the OS or version at a call site. That is the
 *    original bug — `"ios"` on an Android build, `"0.1.0"` frozen forever.
 * 2. `app.config.ts` stops exposing a version, so `Constants.expoConfig
 *    ?.version` quietly resolves to `"unknown"` on every request.
 *
 * Neither failure surfaces in the UI, which is exactly why they need a test.
 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import appConfig from "../app.config";

const ROOT = path.resolve(__dirname, "..");

function source(relativePath: string): string {
  return readFileSync(path.join(ROOT, relativePath), "utf8");
}

/** Every file that reports the client identity to the server. */
const IDENTITY_CALL_SITES = [
  "data/api.ts",
  "data/realtime/realtime-provider.tsx",
];

describe("client identity call sites", () => {
  it.each(IDENTITY_CALL_SITES)(
    "%s takes the OS from the shared constant, not a literal",
    (file) => {
      const text = source(file);
      // `"X-Client-OS": "ios"` / `clientOs: "android"` — a quoted value is
      // always wrong here; the real one comes from `Platform.OS`.
      expect(text).not.toMatch(/["']?X-Client-OS["']?\s*:\s*["']/);
      expect(text).not.toMatch(/\bclientOs\s*:\s*["']/);
    },
  );

  it.each(IDENTITY_CALL_SITES)(
    "%s takes the version from the shared constant, not a literal",
    (file) => {
      const text = source(file);
      expect(text).not.toMatch(/["']?X-Client-Version["']?\s*:\s*["']/);
      expect(text).not.toMatch(/\bclientVersion\s*:\s*["']/);
    },
  );

  it.each(IDENTITY_CALL_SITES)("%s imports the shared constants", (file) => {
    expect(source(file)).toMatch(/from "@\/lib\/client-identity"/);
  });
});

describe("app.config.ts version", () => {
  it("still exposes a version for expo-constants to surface", () => {
    // Without this, CLIENT_VERSION silently degrades to "unknown" and every
    // request stops carrying a usable release number.
    const config = appConfig({ config: {} } as never);
    expect(config.version).toEqual(expect.any(String));
    expect(config.version).toMatch(/^\d+\.\d+\.\d+/);
  });
});
