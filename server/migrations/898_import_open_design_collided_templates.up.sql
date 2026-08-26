-- Import the four Open Design templates that 892 skipped because a
-- hand-written recipe of 889 held their slug (DC-056). 897 retired those, so
-- the upstream versions now take their place: same extraction as 892, row-two
-- facet from the same rule table as 894 (subcategory stays NULL like every
-- imported row), positions at the tail of the prototype block.
INSERT INTO design_scenario_recipe (
    workspace_id, slug, title, summary, category, subcategory,
    mode, platform, prompt, origin, published_at, position
) VALUES
(NULL, 'saas-landing', 'SaaS 落地页', '单页 SaaS 落地页，包含主视觉、功能特性、社会证明、价格方案和行动号召。遵循当前 DESIGN.md 的颜色/排版/布局规范。触发关键词："saas landing"、"marketing page"、"product landing"。', '落地页 / 营销', NULL,
 'prototype', 'web',
 '使用这个插件完成以下任务：Single-page SaaS landing with hero, features, social proof, pricing, and CTA. Respects the active DESIGN.md color/typography/layout tokens. Trigger keywords: "saas landing", "marketing page", "product landing".',
 'builtin', now(), 1782),

(NULL, 'mobile-onboarding', '移动端引导', '多屏幕移动引导流程，呈现为三个并排的手机框架——启动页、价值主张、登录页。包含状态栏、滑动指示点和主要CTA按钮。适用于需要''移动端引导''、''iOS引导''、''手机注册''或''移动端引导''的场景。', '应用', NULL,
 'prototype', 'mobile',
 '使用这个插件完成以下任务：Design a 3-screen mobile onboarding flow for a meditation app — welcome, value props, sign-in.',
 'builtin', now(), 1784),

(NULL, 'docs-page', '文档页面', '文档页面 — 侧边导航、可滚动正文和目录。当需求提到''文档''、''指南''、''API 参考''或''教程''时使用。', '开发者工具', NULL,
 'prototype', 'web',
 '使用这个插件完成以下任务：A documentation page — inline-start nav, scrollable article body, inline-end table of contents. Use when the brief mentions "docs", "documentation", "guide", "API reference", or "tutorial".',
 'builtin', now(), 1786),

(NULL, 'web-prototype', '网页原型', '通用桌面网页原型。通过复制种子文件 `assets/template.html` 并粘贴 `references/layouts.md` 中的布局生成单个独立 HTML 文件。适用于落地页、营销页、文档或 SaaS 页面的默认选项。', '落地页 / 营销', NULL,
 'prototype', 'web',
 '用高端产品工作室的完成度，为 {{audience}} 打磨一个 {{fidelity}} 的 {{artifactKind}}：信息架构清晰、视觉层级优雅、交互状态完整，整体像顶级产品团队交付的可演示原型。设计系统方向使用 {{designSystem}}，从 {{template}} 开始。使用 `assets/template.html` 种子并从 `references/layouts.md` 粘贴版面，输出单文件 HTML。',
 'builtin', now(), 1788);
