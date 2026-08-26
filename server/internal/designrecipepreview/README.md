# Built-in recipe covers

`data/<slug>/` holds the cover the community card shows for each built-in
recipe, mirrored from Open Design (`nexu-io/open-design` @ `30fc648f6`,
2026-08-14) the way its own community tab resolves a cover
(`apps/web/src/components/plugins-home/preview.ts` and
`apps/daemon/src/routes/plugins/assets.ts`):

| Open Design decides | we carry |
| --- | --- |
| `od.preview` of type video / audio / image, or one that names a poster | `preview.jpg` — the poster still, re-encoded at ≤720px JPEG q76 (remote posters fetched once; the hover clip is not bundled) |
| `od.preview.entry` (type html) or `useCase.exampleOutputs[0]` | `example.html` — the first existing file of the daemon's candidate list; `example-slides.html` assembled into its `template.html`; an iframe-only shell unwrapped to its target; relative references re-based to the bundle root and the referenced siblings copied beside it (capped at 512 KB a file / 1 MB a slug) |
| design-system plugin or nothing | no directory — the card shows its composed tile |

Everything here is read-only product content embedded in the binary; the
handler serves it at `/api/design-recipes/{slug}/preview/{digest}/`, where
the digest is a hash of the slug's files, so a change here is a new URL.
