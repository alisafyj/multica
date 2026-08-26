-- HyperFrames becomes its own artifact mode.
--
-- Open Design's home row lists HyperFrames beside Video and pulls its plugins
-- out of Video (plugins-home/facets.ts: isVideoPlugin excludes anything
-- tagged hyperframes / html-video / video-composition / interactive-video).
-- Multica's mode column is what the community mode row filters on, so the
-- honest way to carry that bucket is a mode value of its own, not a marker
-- hidden in another column that the row would then have to decode.
--
-- The constraint is dropped and re-added in one migration: no index depends on
-- it, and the runner executes each file outside a transaction, so this is two
-- statements against the same table with no concurrent build between them.
ALTER TABLE design_scenario_recipe DROP CONSTRAINT design_scenario_recipe_mode_check;
ALTER TABLE design_scenario_recipe ADD CONSTRAINT design_scenario_recipe_mode_check
    CHECK (mode IN ('prototype', 'deck', 'image', 'video', 'hyperframes', 'audio'));

UPDATE design_scenario_recipe SET mode = 'hyperframes', updated_at = now()
WHERE workspace_id IS NULL
  AND origin = 'builtin'
  AND slug IN (
    'motion-frames',
    'hyperframes',
    'video-hyperframes',
    'frame-nyt-graph',
    'frame-takram-organic',
    'frame-product-promo',
    'frame-product-promo-30s',
    'frame-decision-tree',
    'frame-creative-voltage',
    'frame-kinetic-type',
    'frame-bold-signal',
    'frame-bold-poster',
    'frame-build-minimal',
    'frame-data-rollup',
    'frame-warm-grain',
    'frame-play-mode',
    'frame-swiss-grid',
    'frame-pentagram-stat',
    'frame-electric-studio',
    'frame-vignelli'
  );
