# Multica Gallery Native Figma Plugin

Development-only MVP importer.

## Use locally

1. In Figma Desktop: `Plugins → Development → Import plugin from manifest…`
2. Select `apps/figma-plugin/manifest.json`.
3. Open Multica Designs and click **Connect Figma** to generate a one-time code.
4. Run **Multica Gallery Native Importer** in Figma.
5. Fill:
   - API base URL: `http://localhost:8080`
   - Workspace: your workspace slug, e.g. `amc`
   - One-time connection code from the Multica Designs page
6. Click **Export selection**, then **Upload to Multica**.

The plugin exports selected nodes, or the current page if nothing is selected, into `GalleryNativeJson` and stores it through Multica's design file API.

## MVP limitations

- Captures structure, geometry, text, and basic node types.
- Does not upload image assets/slices yet.
- Requires local API CORS to allow Figma plugin requests.
