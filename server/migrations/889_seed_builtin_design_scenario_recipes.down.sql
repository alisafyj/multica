DELETE FROM design_scenario_recipe
WHERE workspace_id IS NULL
  AND slug IN (
    'saas-landing', 'ops-dashboard', 'admin-console', 'mobile-app-screens',
    'mobile-onboarding', 'product-spec', 'team-okr', 'incident-runbook',
    'docs-page', 'web-prototype'
  );
