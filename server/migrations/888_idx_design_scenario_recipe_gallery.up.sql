CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_scenario_recipe_gallery ON design_scenario_recipe (category, position, created_at) WHERE published_at IS NOT NULL;
