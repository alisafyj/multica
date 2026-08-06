import type { NativeIntent } from "expo-router";
import { isSsoCallbackUrl } from "@/lib/sso-callback";

/**
 * Deep-link gate that runs before expo-router turns an incoming URL into a
 * navigation. Returning a falsy value cancels the navigation entirely
 * (expo-router only calls its listener when this resolves truthy).
 *
 * The SSO redirect is the one URL we must cancel. On Android it arrives as an
 * app-wide `Linking` event, which `expo-auth-session` needs in order to
 * resolve `promptAsync()` — but expo-router subscribes to the same event and
 * would navigate to `auth/mobile-callback`, a path with no route. That
 * unmounts `(auth)/login.tsx` before its `response` effect exchanges the code
 * for a token, so sign-in dies with no request and no error message.
 *
 * iOS never reaches this branch: `ASWebAuthenticationSession` captures the
 * redirect natively and emits no `Linking` event at all.
 */
export const redirectSystemPath: NonNullable<
  NativeIntent["redirectSystemPath"]
> = ({ path }) => (isSsoCallbackUrl(path) ? null : path);
