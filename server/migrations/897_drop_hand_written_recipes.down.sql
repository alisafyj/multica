-- Restore the ten hand-written recipes as they stood after 896 (889's rows
-- with the categories 896 gave them).
INSERT INTO design_scenario_recipe (
    workspace_id, slug, title, summary, category, subcategory,
    mode, platform, prompt, origin, published_at, position
) VALUES
(NULL, 'saas-landing', 'SaaS 落地页', '首屏、功能、定价与转化路径俱全的产品落地页。', '落地页 / 营销', '落地页',
 'prototype', 'web',
 '为我们的产品做一个落地页：首屏说明我们解决什么问题，往下是核心功能、适用场景、定价方案和一个明确的注册入口。语气克制，不要堆砌营销词。',
 'builtin', now(), 10),

(NULL, 'ops-dashboard', '运营数据看板', '带筛选、排序与关键指标卡的后台看板。', '数据看板', '看板',
 'prototype', 'web',
 '做一个运营数据看板：顶部是几张关键指标卡，下面是一张可筛选、可排序的明细表，支持按时间范围和状态过滤。信息密度要高，适合每天盯盘的人使用。',
 'builtin', now(), 20),

(NULL, 'admin-console', '后台管理界面', '列表、详情、批量操作与表单校验的中后台界面。', '数据看板', '中后台',
 'prototype', 'web',
 '做一个后台管理界面：左侧导航，主区是数据列表，支持搜索、批量选择和批量操作，点击进入详情可以编辑并做本地校验。要覆盖加载中、空数据、报错和保存成功四种状态。',
 'builtin', now(), 30),

(NULL, 'mobile-app-screens', '移动应用界面', '带底部导航的移动端主流程界面。', '应用', '移动端',
 'prototype', 'mobile',
 '做一组移动应用界面：底部导航切换主要模块，包含列表页、详情页和一个需要填写的表单页。按手机尺寸排布，点击区域不小于 44px。',
 'builtin', now(), 40),

(NULL, 'mobile-onboarding', '移动端引导流程', '启动页、价值说明与登录注册的完整引导。', '应用', '移动端',
 'prototype', 'mobile',
 '做一套移动端首次使用引导：启动页、两到三屏价值说明、然后是登录或注册。要能左右切换，最后一屏进入主界面。',
 'builtin', now(), 50),

(NULL, 'product-spec', '产品需求文档', '带目录与决策记录的需求说明页。', '文档 / 报告', '文档',
 'prototype', 'web',
 '做一个产品需求文档页面：左侧是可跳转的目录，正文包含背景、目标、非目标、方案、关键决策记录和验收标准。适合长文阅读。',
 'builtin', now(), 60),

(NULL, 'team-okr', '团队 OKR 记分卡', '目标、关键结果与进度的季度视图。', '', '协作',
 'prototype', 'web',
 '做一个团队 OKR 记分卡：按目标分组，每个目标下面是关键结果和当前进度，可以按负责人和季度筛选。进度落后的要一眼能看出来。',
 'builtin', now(), 70),

(NULL, 'incident-runbook', '故障处理手册', '分步骤、可勾选的应急操作页。', '开发者工具', '运维',
 'prototype', 'web',
 '做一个故障处理手册页面：按严重级别分类，每个场景是一串可勾选的处置步骤，附带判断依据、回滚方式和升级联系人。要能在紧张的时候快速找到。',
 'builtin', now(), 80),

(NULL, 'docs-page', '文档页', '带侧边目录与代码示例的技术文档页。', '开发者工具', '文档',
 'prototype', 'web',
 '做一个技术文档页面：左侧是分层导航，正文有标题层级、代码示例和提示框，右侧是当前页的小目录。代码块要能横向滚动。',
 'builtin', now(), 90),

(NULL, 'web-prototype', '通用网页原型', '不预设结构的空白起点，适合自由描述。', '落地页 / 营销', '通用',
 'prototype', 'web',
 '按下面的描述做一个网页原型：',
 'builtin', now(), 100)
;
