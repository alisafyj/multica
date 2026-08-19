-- Retire the ten hand-written recipes 889 seeded before the Open Design
-- catalogue was imported (892–896). They were the community tab's first
-- content; now the tab mirrors Open Design's catalogue and covers, and these
-- ten are the only built-ins with no counterpart there — no cover, no
-- upstream bucket (team-okr has an empty category for that reason), and a
-- prompt the imported recipes already express better. Keeping them meant ten
-- tiles among covers and a second, unmaintained voice in the catalogue.
DELETE FROM design_scenario_recipe
WHERE workspace_id IS NULL
  AND origin = 'builtin'
  AND slug IN (
    'saas-landing', 'ops-dashboard', 'admin-console', 'mobile-app-screens',
    'mobile-onboarding', 'product-spec', 'team-okr', 'incident-runbook',
    'docs-page', 'web-prototype'
  );
