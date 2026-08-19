-- Standalone design systems, owned by the workspace itself (DC-052 amended:
-- a designer may keep several independent systems with no project behind
-- them, as Open Design's design-systems page allows). project_id becomes
-- nullable; NULL marks a standalone system. Project-bound behaviour is
-- unchanged, and the partial unique indexes of 878/879 never constrained
-- rows with a NULL project_id in the first place (NULLs are distinct), so
-- any number of standalone systems may coexist with no index change.
ALTER TABLE project_design_system
    ALTER COLUMN project_id DROP NOT NULL;
