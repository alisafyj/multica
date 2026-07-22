CREATE TABLE design_template_blueprint (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    template_id uuid NOT NULL REFERENCES design_catalog_template(id) ON DELETE CASCADE,
    template_revision_id uuid NOT NULL REFERENCES design_template_revision(id) ON DELETE CASCADE,
    source_revision_id uuid NOT NULL REFERENCES design_revision(id) ON DELETE RESTRICT,
    analysis_version integer NOT NULL,
    schema_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('valid', 'invalid', 'archived')),
    structure_json jsonb NOT NULL,
    blueprint_json jsonb NOT NULL,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (template_revision_id, analysis_version)
);

CREATE INDEX idx_design_template_blueprint_latest
    ON design_template_blueprint (workspace_id, template_revision_id, analysis_version DESC)
    WHERE status = 'valid';

CREATE TABLE design_component_recipe_set (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    design_system_profile_id uuid NOT NULL REFERENCES design_system_profile(id) ON DELETE CASCADE,
    source_revision_id uuid NOT NULL REFERENCES design_revision(id) ON DELETE RESTRICT,
    analysis_version integer NOT NULL,
    schema_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('valid', 'invalid', 'archived')),
    recipes_json jsonb NOT NULL,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (design_system_profile_id, analysis_version)
);

CREATE INDEX idx_design_component_recipe_set_latest
    ON design_component_recipe_set (workspace_id, design_system_profile_id, analysis_version DESC)
    WHERE status = 'valid';
