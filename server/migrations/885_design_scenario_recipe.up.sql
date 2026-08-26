-- Design scenario recipes: the catalogue behind the design centre's community
-- tab and the home composer's template chip (DC-041 / DC-049).
--
-- Named "scenario recipe" rather than "template" on purpose. Four template
-- tables already exist — design_template, design_template_library,
-- design_catalog_template and design_template_blueprint — and every one of
-- them is about Figma-derived design assets. This is a different thing: a
-- recipe for a page-design task, which is why design_document.recipe holds one
-- of these slugs.
--
-- There is deliberately NO revision table. A recipe is a prompt plus
-- presentation metadata, and applying one seeds the document's brief — which
-- is already frozen in design_document.input_snapshot and covered by that
-- revision's input_snapshot_sha256. The applied content is therefore already
-- immutable at the point it matters, and a second versioning mechanism would
-- only add a way for the two to disagree.
--
-- workspace_id NULL marks a built-in recipe shipped with the product and
-- visible to every workspace. A non-null value is a workspace's own.
CREATE TABLE design_scenario_recipe (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID,
    -- Stable identifier stored on design_document.recipe. Kebab-case.
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    -- Two-level facets, matching how the gallery filters.
    category TEXT NOT NULL,
    subcategory TEXT,
    -- The artifact this recipe produces. Only prototype ships in this phase;
    -- the column exists so adding deck or image later is data, not migration.
    mode TEXT NOT NULL DEFAULT 'prototype' CHECK (mode IN ('prototype', 'deck', 'image', 'video', 'audio')),
    -- Suggested target platform. NULL means the recipe works on any.
    platform TEXT CHECK (platform IS NULL OR platform IN ('web', 'mobile', 'cross_platform')),
    -- Seeded into the composer's brief when the user picks this recipe.
    prompt TEXT NOT NULL,
    -- Optional preview media for the gallery card, in object storage.
    preview_object_key TEXT,
    origin TEXT NOT NULL DEFAULT 'builtin' CHECK (origin IN ('builtin', 'workspace', 'community')),
    -- Unpublished recipes are drafts and do not appear in the gallery.
    published_at TIMESTAMPTZ,
    position INT NOT NULL DEFAULT 0,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
