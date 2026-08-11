/**
 * Single source of truth for the SSO redirect target.
 *
 * Two places must agree on this string, and on Android they fail silently if
 * they don't:
 *   - `app/(auth)/login.tsx` hands it to `makeRedirectUri()`, so it is what
 *     the identity provider redirects back to.
 *   - `app/+native-intent.ts` matches it to stop expo-router from treating
 *     that redirect as a navigation.
 *
 * Why the second one exists: iOS captures the redirect inside
 * `ASWebAuthenticationSession` and never emits a `Linking` url event. Android
 * has no equivalent, so `expo-web-browser` polyfills the auth session with a
 * `Linking` listener — which makes the redirect an app-wide deep link that
 * expo-router also subscribes to. Left alone, the router wins the race,
 * navigates to a route that doesn't exist, and unmounts the login screen
 * before its `response` effect can exchange the code for a token.
 */

/** Path segment of the redirect, without a scheme or leading slash. */
export const SSO_CALLBACK_PATH = "auth/mobile-callback";

/** Fully-qualified native redirect URI registered with the SSO provider. */
export const SSO_CALLBACK_URL = `multica://${SSO_CALLBACK_PATH}`;

/**
 * True when `url` is the SSO redirect coming back into the app.
 *
 * Scheme-agnostic on purpose: the app registers both `multica://` and the
 * dev-client's `exp+multica-mobile://`, and only the path identifies the
 * callback. Matching is exact after stripping scheme, query, fragment, and
 * surrounding slashes — a prefix match would also swallow unrelated deep
 * links nested under the same segment.
 */
export function isSsoCallbackUrl(url: string): boolean {
  const withoutScheme = url.replace(/^[a-z][a-z0-9+.-]*:(\/\/)?/i, "");
  const [withoutQuery = ""] = withoutScheme.split(/[?#]/);
  return withoutQuery.replace(/^\/+|\/+$/g, "") === SSO_CALLBACK_PATH;
}
