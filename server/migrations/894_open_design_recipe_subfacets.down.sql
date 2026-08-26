-- Symmetric rollback only; migration 893's facets it restores were the wrong
-- ones for every mode except deck.
UPDATE design_scenario_recipe SET category = '', updated_at = now()
WHERE workspace_id IS NULL AND origin = 'builtin' AND position >= 1000;
