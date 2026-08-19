-- Restores the scenario-derived facets migration 892 seeded. Kept only so the
-- rollback is symmetric; the values it restores are the wrong ones.
UPDATE design_scenario_recipe SET category = '其他', updated_at = now()
WHERE workspace_id IS NULL AND origin = 'builtin' AND position >= 1000;
