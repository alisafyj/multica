CREATE TABLE design_template_blueprint (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    template_id uuid NOT NULL,
    template_revision_id uuid NOT NULL,
    source_revision_id uuid NOT NULL,
    analysis_version integer NOT NULL,
    schema_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('valid', 'invalid', 'archived')),
    structure_json jsonb NOT NULL,
    blueprint_json jsonb NOT NULL,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (template_revision_id, analysis_version)
);

CREATE TABLE design_component_recipe_set (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    design_system_profile_id uuid NOT NULL,
    source_revision_id uuid NOT NULL,
    analysis_version integer NOT NULL,
    schema_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('valid', 'invalid', 'archived')),
    recipes_json jsonb NOT NULL,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (design_system_profile_id, analysis_version)
);
