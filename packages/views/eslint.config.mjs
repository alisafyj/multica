import reactConfig from "@multica/eslint-config/react";
import i18next from "eslint-plugin-i18next";
import {
  NO_UNTRANSLATED_ATTRIBUTES,
  NO_UNTRANSLATED_TOAST,
} from "./eslint-i18n-guard.mjs";

// Global i18n protection. Every JSX text node in this package must pass
// through useT() — raw strings become a build error. Scope of
// `mode: "jsx-text-only"`: flags raw strings inside JSX children only;
// attribute values and plain TS literals are allowed through — the two doors
// `eslint-i18n-guard.mjs` pins shut.

export default [
  ...reactConfig,
  {
    files: ["**/*.tsx"],
    ignores: ["**/*.test.tsx", "test/**"],
    plugins: { i18next },
    rules: {
      "i18next/no-literal-string": [
        "error",
        { mode: "jsx-text-only" },
      ],
    },
  },
  {
    // Toasts are fired from hooks (`.ts`) as often as from components.
    files: ["**/*.{ts,tsx}"],
    ignores: ["**/*.test.{ts,tsx}", "test/**"],
    rules: {
      "no-restricted-syntax": [
        "error",
        NO_UNTRANSLATED_ATTRIBUTES,
        NO_UNTRANSLATED_TOAST,
      ],
    },
  },
  {
    // The design workbench is currently a Chinese-only internal surface. Keep
    // raw copy visible in CI without blocking unrelated releases until the
    // design namespace is added to all four locale bundles. Listed last so the
    // downgrade wins over the global blocks above.
    //
    // The toast/attribute guard is downgraded on the same surface and for the
    // same reason. It arrived from upstream after this exemption existed, and
    // leaving it at "error" would have made every unrelated change to this
    // package fail CI on 49 pre-existing Chinese strings — the exact outcome
    // the exemption was written to avoid. Remove both together when the design
    // namespace lands.
    files: [
      "designs/**/*.{ts,tsx}",
      "issues/components/issue-design-restore-section.tsx",
    ],
    plugins: { i18next },
    rules: {
      "i18next/no-literal-string": "warn",
      "no-restricted-syntax": "warn",
    },
  },
];
