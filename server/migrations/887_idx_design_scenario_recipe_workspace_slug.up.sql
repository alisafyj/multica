CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_design_scenario_recipe_workspace_slug ON design_scenario_recipe (workspace_id, slug) WHERE workspace_id IS NOT NULL;
