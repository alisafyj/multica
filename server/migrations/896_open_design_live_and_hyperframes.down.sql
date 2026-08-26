-- Symmetric rollback: drop the two inserted live artifacts, fold live and the
-- late HyperFrames back into their prior modes, then narrow the CHECK.
DELETE FROM design_scenario_recipe
WHERE workspace_id IS NULL AND origin = 'builtin'
  AND slug IN ('social-media-matrix-tracker-template', 'trading-analysis-dashboard-template');
UPDATE design_scenario_recipe SET mode = 'prototype', updated_at = now() WHERE mode = 'live';
ALTER TABLE design_scenario_recipe DROP CONSTRAINT design_scenario_recipe_mode_check;
ALTER TABLE design_scenario_recipe ADD CONSTRAINT design_scenario_recipe_mode_check
    CHECK (mode IN ('prototype', 'deck', 'image', 'video', 'hyperframes', 'audio'));
