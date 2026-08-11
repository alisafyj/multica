CREATE TABLE design_system_profile (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    project_id uuid,
    source_file_id uuid NOT NULL,
    source_revision_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    status text NOT NULL DEFAULT 'analyzed',
    is_default boolean NOT NULL DEFAULT false,
    profile_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    analysis_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT design_system_profile_status_check CHECK (status IN ('draft', 'analyzed', 'failed', 'archived'))
);
