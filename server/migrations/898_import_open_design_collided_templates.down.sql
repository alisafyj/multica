DELETE FROM design_scenario_recipe
WHERE workspace_id IS NULL
  AND origin = 'builtin'
  AND slug IN ('saas-landing', 'mobile-onboarding', 'docs-page', 'web-prototype');
