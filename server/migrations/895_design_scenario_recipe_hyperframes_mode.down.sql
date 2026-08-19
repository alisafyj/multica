-- Folds HyperFrames back into video before narrowing the constraint, so the
-- re-added CHECK cannot reject rows already in the table.
UPDATE design_scenario_recipe SET mode = 'video', updated_at = now()
WHERE mode = 'hyperframes';
ALTER TABLE design_scenario_recipe DROP CONSTRAINT design_scenario_recipe_mode_check;
ALTER TABLE design_scenario_recipe ADD CONSTRAINT design_scenario_recipe_mode_check
    CHECK (mode IN ('prototype', 'deck', 'image', 'video', 'audio'));
