CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_design_scenario_recipe_builtin_slug ON design_scenario_recipe (slug) WHERE workspace_id IS NULL;
