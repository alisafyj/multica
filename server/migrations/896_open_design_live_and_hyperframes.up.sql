-- Finishes aligning the community catalogue with Open Design's mode row.
--
-- Three gaps 894/895 left open:
--
-- 1. The ten hand-written recipes from migration 889 kept their original
--    Multica categories (产品 / 工程 / 营销 / 设计 / 运营), which surfaced as
--    extra pills under 原型 that Open Design does not have. They now sit in the
--    Open Design bucket their content belongs to; four share a slug with an
--    Open Design template and take its bucket, the rest are placed by the same
--    keywords the rule table uses. team-okr matches nothing, as its Open Design
--    twin does, and so carries no facet.
--
-- 2. Six HyperFrames were missed. Open Design ships six templates in both
--    examples/ and video-templates/ under one folder name; the earlier pass
--    bound both seeded rows to the examples/ copy, so the video-templates/
--    copies — the ones carrying the hyperframes tags — were never inspected.
--
-- 3. 实时产物 was empty. Open Design fills it from a curated id list of five
--    plugins; three were already seeded under prototype / image and move over,
--    two carry mode=template upstream (which the import had excluded as
--    non-content) and are inserted here. `live` joins the mode CHECK.
ALTER TABLE design_scenario_recipe DROP CONSTRAINT design_scenario_recipe_mode_check;
ALTER TABLE design_scenario_recipe ADD CONSTRAINT design_scenario_recipe_mode_check
    CHECK (mode IN ('prototype', 'deck', 'image', 'video', 'hyperframes', 'live', 'audio'));

UPDATE design_scenario_recipe AS r SET category = v.category, updated_at = now()
FROM (VALUES
    ('saas-landing', '落地页 / 营销'),
    ('web-prototype', '落地页 / 营销'),
    ('mobile-onboarding', '应用'),
    ('docs-page', '开发者工具'),
    ('ops-dashboard', '数据看板'),
    ('admin-console', '数据看板'),
    ('mobile-app-screens', '应用'),
    ('product-spec', '文档 / 报告'),
    ('incident-runbook', '开发者工具'),
    ('team-okr', '')
) AS v(slug, category)
WHERE r.slug = v.slug AND r.workspace_id IS NULL AND r.origin = 'builtin';

UPDATE design_scenario_recipe AS r SET mode = v.mode, category = v.category, updated_at = now()
FROM (VALUES
    ('motion-frames', 'hyperframes', ''),
    ('live-dashboard', 'live', ''),
    ('live-artifact', 'live', ''),
    ('notion-team-dashboard-live-artifact', 'live', ''),
    ('hyperframes', 'hyperframes', ''),
    ('video-hyperframes', 'hyperframes', ''),
    ('frame-nyt-graph', 'hyperframes', ''),
    ('frame-data-chart-nyt-video', 'hyperframes', ''),
    ('vfx-text-cursor-video', 'hyperframes', ''),
    ('frame-takram-organic', 'hyperframes', ''),
    ('frame-product-promo', 'hyperframes', ''),
    ('frame-product-promo-30s', 'hyperframes', ''),
    ('frame-decision-tree', 'hyperframes', ''),
    ('frame-creative-voltage', 'hyperframes', ''),
    ('frame-kinetic-type', 'hyperframes', ''),
    ('frame-logo-outro-video', 'hyperframes', ''),
    ('frame-bold-signal', 'hyperframes', ''),
    ('frame-bold-poster', 'hyperframes', ''),
    ('frame-build-minimal', 'hyperframes', ''),
    ('frame-glitch-title-video', 'hyperframes', ''),
    ('frame-data-rollup', 'hyperframes', ''),
    ('frame-warm-grain', 'hyperframes', ''),
    ('frame-liquid-bg-hero-video', 'hyperframes', ''),
    ('frame-play-mode', 'hyperframes', ''),
    ('frame-swiss-grid', 'hyperframes', ''),
    ('frame-pentagram-stat', 'hyperframes', ''),
    ('frame-electric-studio', 'hyperframes', ''),
    ('frame-vignelli', 'hyperframes', ''),
    ('frame-light-leak-cinema-video', 'hyperframes', '')
) AS v(slug, mode, category)
WHERE r.slug = v.slug AND r.workspace_id IS NULL AND r.origin = 'builtin';

INSERT INTO design_scenario_recipe (
    workspace_id, slug, title, summary, category, subcategory,
    mode, platform, prompt, origin, published_at, position
) VALUES
(NULL, 'social-media-matrix-tracker-template', '社媒矩阵数据追踪面板模板', '社媒矩阵数据追踪面板模板（Social Media Matrix Tracker）。适用于需要电影级、数据密集型社交媒体分析仪表板的场景，支持多平台指标、交互式图表、悬停洞察、范围对比和深浅主题切换，单个 HTML 文件即可实现。', '', NULL,
 'live', NULL,
 '使用这个插件完成以下任务：Create a social media matrix tracker dashboard template using my DESIGN.md. Keep the cinematic glassmorphism style, multi-chart analytics sections, hover tooltips, pin/drag range analysis, and light/dark switching.',
 'builtin', now(), 3810),

(NULL, 'trading-analysis-dashboard-template', '交易分析仪表板模板', '专业交易分析仪表板模板（单文件 HTML），包含明暗主题切换、密集市场面板、图表交互、演示/实时回放和命令面板功能。适用于华尔街风格分析终端、交易驾驶舱或高科技金融仪表板模板需求。', '', NULL,
 'live', NULL,
 '使用这个插件完成以下任务：Create a Wall-Street-grade trading analysis dashboard template with a left rail, risk cockpit, market charts, live/demo mode, and realistic dense data. Keep it single-file HTML.',
 'builtin', now(), 3820);
