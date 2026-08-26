-- Deleting standalone systems first: DROP NOT NULL cannot be undone while
-- rows violate it, and a standalone system's packages have no home once the
-- system row is gone (no foreign keys per repository policy; this runs in
-- its own transaction, packages first, mirroring the resource-delete CTE).
WITH deleted_packages AS (
    DELETE FROM project_design_system_package
    WHERE project_design_system_package.design_system_id IN (
        SELECT project_design_system.id
        FROM project_design_system
        WHERE project_design_system.project_id IS NULL
    )
    RETURNING project_design_system_package.id
)
DELETE FROM project_design_system
WHERE project_design_system.project_id IS NULL
  AND (SELECT count(*) FROM deleted_packages) >= 0;

ALTER TABLE project_design_system
    ALTER COLUMN project_id SET NOT NULL;
