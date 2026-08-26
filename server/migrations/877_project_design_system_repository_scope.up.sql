-- Design systems become per repository (DC-052, supersedes DC-025).
--
-- A project can hold a consumer H5 site, a mobile app and an admin console
-- at once. The first and third are both platform='web', so the existing
-- platform axis cannot separate them and they were forced to share one
-- design system.
--
-- project_resource_id is nullable and carries the scope:
--   NULL     -> project-level system: used across repositories and whenever
--               a design task is started without picking a repository.
--   NOT NULL -> the system owned by that repository.
--
-- Existing rows are project-level by construction, so no data migration is
-- required. No foreign key per repository policy: deleting a project_resource
-- clears its design system inside an application transaction.
--
-- The old table-level UNIQUE (workspace_id, project_id) is dropped here and
-- replaced by two partial unique indexes in 878 and 879. A single composite
-- unique key would not work: PostgreSQL treats NULLs as distinct, so it would
-- admit many project-level systems for the same project.
ALTER TABLE project_design_system
    ADD COLUMN project_resource_id UUID;

ALTER TABLE project_design_system
    DROP CONSTRAINT project_design_system_workspace_id_project_id_key;
