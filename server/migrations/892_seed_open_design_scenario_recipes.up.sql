-- Second batch of built-in scenario recipes: the rest of the Open Design
-- official catalogue (DC-041 / DC-047).
--
-- Migration 889 seeded the ten prototype recipes whose Chinese copy was
-- hand-written for Multica. This batch carries the remaining Open Design
-- templates across every artifact mode, with their own zh-CN strings taken
-- from the source catalogue rather than re-authored.
--
-- Modes other than prototype are seeded deliberately. They are browsable but
-- not startable: CreateDesignDocument rejects a non-prototype recipe, and the
-- gallery already says so on the card ("这个配方的产物形态…暂时还不能创建").
-- Shipping them makes the catalogue honest about what Open Design offers and
-- what this phase can actually produce, instead of hiding the difference.
--
-- Slugs that Multica already ships under migration 889 are not re-seeded; the
-- adapted copy there wins. Open Design's own cross-directory name collisions
-- (a prototype and its video counterpart) are qualified with their mode.
--
-- positions start at 1000 so the hand-written batch keeps the top of the
-- gallery, ordered prototype-first since that is the only startable mode.
INSERT INTO design_scenario_recipe (
    workspace_id, slug, title, summary, category, subcategory,
    mode, platform, prompt, origin, published_at, position
) VALUES
(NULL, 'doc-kami-parchment', 'Kami 羊皮纸文档', '暖羊皮纸底 (#f5f4ed) + 墨蓝单色 accent (#1B365D) + 单一衬线字体, 编辑级排印', '个人', '网页',
 'prototype', 'web',
 '用「Kami 羊皮纸文档」模板把我的内容做成一份「暖羊皮纸底 (#f5f4ed) + 墨蓝单色 accent (#1B365D) + 单一衬线字体, 编辑级排印」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1000),

(NULL, 'social-spotify-card', 'Spotify 正在播放卡', 'Spotify Now Playing 风格卡: 专辑封面 + 进度条 + 播放控制, 适配视频叠加 / 个人主页', '个人', '网页',
 'prototype', 'web',
 '用「Spotify 正在播放卡」模板把我的内容做成一份「Spotify Now Playing 风格卡: 专辑封面 + 进度条 + 播放控制, 适配视频叠加 / 个人主页」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1010),

(NULL, 'resume-modern', '极简简历', '现代极简简历, A4 单页, 适合打印或导出 PDF', '个人', '网页',
 'prototype', 'web',
 '用「极简简历」模板把我的内容做成一份「现代极简简历, A4 单页, 适合打印或导出 PDF」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1020),

(NULL, 'gamified-app', '游戏化应用', '三屏手机原型展示游戏化应用界面：封面海报、每日任务与经验值进度条、任务详情。适用于游戏化应用、习惯追踪、RPG 风格生活应用、升级系统、每日任务、XP/连击应用等需求。', '个人', '网页',
 'prototype', 'mobile',
 '使用这个插件完成以下任务：Design a gamified life-management app — multi-screen mobile prototype: cover poster, today''s quests with XP, and a quest detail. ‘Daily quests for becoming a better human.’',
 'builtin', now(), 1030),

(NULL, 'dating-web', '约会网站', '消费级约会/配对仪表板，包含左侧导航栏、社区动态滚动条、核心KPI指标、30天互相匹配柱状图和匹配率趋势模块。编辑风格排版，克制的强调色。适用于约会网站、配对服务、社区仪表板或社交网络等消费类产品界面。', '个人', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Design ‘mutuals’ — a dating site for X posters. Daily digest dashboard with stats, mutual-matches bar chart, and a community ticker.',
 'builtin', now(), 1040),

(NULL, 'mockup-device-3d', 'iPhone × MacBook 立体展架', 'iPhone + MacBook 仿 GLTF 静态展架, 屏幕内嵌真实 HTML 内容, 玻璃镜头折射, 360° 转盘构图', '产品', '网页',
 'prototype', 'web',
 '用「iPhone × MacBook 立体展架」模板把我的内容做成一份「iPhone + MacBook 仿 GLTF 静态展架, 屏幕内嵌真实 HTML 内容, 玻璃镜头折射, 360° 转盘构图」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1050),

(NULL, 'pm-spec', '产品规格文档', '单页产品规格文档/PRD——包含问题定义、成功指标、范围、用户故事、设计笔记、上线计划、待解决问题。适用于需求文档、产品规格、功能说明等场景。', '产品', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Write me a PRD for adding two-factor auth to our SaaS app — problem, scope, milestones, open questions.',
 'builtin', now(), 1060),

(NULL, 'team-okrs', '团队 OKR', 'OKR 跟踪页面——季度横幅，三个目标及其关键结果进度条、负责人头像、状态标签，以及「本季度概览」侧边栏。当需求提及 OKR、关键结果、目标时使用。', '产品', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Build an OKR tracker for Q4 — three objectives, three key results each, progress bars, owners, status pills.',
 'builtin', now(), 1070),

(NULL, 'eng-runbook', '工程运维手册', '工程运维手册 — 包含服务概览、告警表、仪表盘链接、常用命令操作步骤、值班轮换和事故响应清单。适用于需要运维文档、值班指南、SRE 文档或运维手册的场景。', '工程', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Write a runbook for our auth service — alerts, dashboards, common procedures, on-call rotation.',
 'builtin', now(), 1080),

(NULL, 'web-clone', '网站复刻', '使用源码优先的侦察、路由抓取、交互探针、素材采集和复刻审计，复刻或视觉还原一个网站。', '工程', '网页',
 'prototype', NULL,
 '复刻这个网站：{{targetUrl}}。使用 web-clone 流程：先找真实源码，再侦察站点、选择保真路径、构建本地复刻、删除追踪脚本，并写好 NOTES.md 与复刻/审计证据。',
 'builtin', now(), 1090),

(NULL, 'codex-interactive-capability-map', 'Codex 交互式能力地图', '将长文、帖子、备忘录或产品说明转换为 Codex 风格的可点击能力地图，包含工作流循环、用例矩阵和响应式详情面板。', '教育', '网页',
 'prototype', 'web',
 '把这篇长文、帖子或产品说明转成一个 Codex 风格的交互式能力地图：提炼核心概念，组织成流程环和用例矩阵，并生成一个带点击卡片与详情面板的单文件 HTML 原型。',
 'builtin', now(), 1100),

(NULL, 'kami-landing', 'Kami Landing', '生成印刷级单页纸质文档——温暖羊皮纸底色、墨蓝强调色、单一衬线字重，无斜体，无冷灰。输出呈现专业白皮书或工作室单页效果，而非应用 UI。原生多语言支持（EN · zh-CN · ja）。单个独立 HTML 文件，零依赖。', '营销', '网页',
 'prototype', 'web',
 'Produce a print-grade single-page kami (紙 / 纸) document — warm parchment canvas, ink-blue accent, serif at one weight, no italic, no cool grays. The output reads like a professional white paper or studio one-pager, not an app UI. Multilingual by design (EN · zh-CN · ja). One self-contained HTML file, zero dependencies.',
 'builtin', now(), 1110),

(NULL, 'motion-frames', 'Motion Frames', '单帧动态设计合成，包含循环 CSS 动画——旋转文字环、动画地球、计时器、视差标签。渲染为视频封面海报，可直接导入 HyperFrames 或任何基于关键帧的导出工具。适用于动态设计、动画主视觉、循环视频、视频封面、标题卡等需求，搭配 HyperFrames 实现动态导出。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Design an animated hero — a rotating type ring around a wireframe globe, with the headline ‘Reach every country.’ Loop at 12s, ready for HyperFrames export.',
 'builtin', now(), 1120),

(NULL, 'open-design-homepage', 'Open Design 主页', 'open-design.ai 官网主页的像素级镜像——基于 Next.js 的交互式 WebGL/Three.js(React Three Fiber)首屏。含实时 3D「Open Design」立体字(Draco 压缩几何)、散落贴纸拼贴、可变字体、光标视差和滚动驱动动效。这是 Open Design 面向 Web 交互营销页所追求的视觉天花板的官方范例——资产全部本地打包(仅 Draco 解码', '营销', '网页',
 'prototype', NULL,
 '复刻 open-design.ai 官网主页：基于 React Three Fiber / Next.js 的交互式首屏，含实时 3D 立体字、贴纸拼贴、可变字体、光标视差和滚动驱动动效。完全自包含，可在沙箱预览中渲染。',
 'builtin', now(), 1130),

(NULL, 'open-design-landing', 'Open Design 落地页', '使用 Atelier Zero 视觉语言（Monocle / Apartamento / Études 编辑拼贴风格）生成单页编辑落地页。从品牌简报填充 inputs.json，可选通过 gpt-image-2 生成 16 张拼贴素材，输出独立 HTML 文件，内置滚动动画和粘性导航。', '营销', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Produce a world-class single-page editorial landing site in the Atelier Zero visual language (Monocle / Apartamento / Études editorial collage) — the same aesthetic Open Design uses for its own marketing surface. The agent fills a typed `inputs.json` from a brand brief, optionally generates 16 collage assets via gpt-im',
 'builtin', now(), 1140),

(NULL, 'social-reddit-card', 'Reddit 帖子卡', '拟真 Reddit 帖子卡 + 上下投票 + 评论数, 适合视频叠加 / 故事分享', '营销', '网页',
 'prototype', 'web',
 '用「Reddit 帖子卡」模板把我的内容做成一份「拟真 Reddit 帖子卡 + 上下投票 + 评论数, 适合视频叠加 / 故事分享」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1150),

(NULL, 'card-twitter', 'Twitter 分享卡', '推特金句 / 数据卡, 适合配推文', '营销', '网页',
 'prototype', 'web',
 '用「Twitter 分享卡」模板把我的内容做成一份「推特金句 / 数据卡, 适合配推文」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1160),

(NULL, 'social-x-post-card', 'X (Twitter) 帖子卡', '拟真 X 推文卡片 + 互动数据 (likes/reposts/views), 适配视频叠加或图卡分享', '营销', '网页',
 'prototype', 'web',
 '用「X (Twitter) 帖子卡」模板把我的内容做成一份「拟真 X 推文卡片 + 互动数据 (likes/reposts/views), 适配视频叠加或图卡分享」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1170),

(NULL, 'pricing-page', '价格页面', '独立的价格页面 — 包含页眉、套餐层级、功能对比表和常见问题。适用于需要''价格''、''套餐''、''订阅层级''或''对比套餐''页面的需求。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：A standalone pricing page — header, plan tiers, feature comparison table, and an FAQ. Use when the brief asks for "pricing", "plans", "subscription tiers", or a "compare plans" page.',
 'builtin', now(), 1180),

(NULL, 'blog-post', '博客文章', '长篇文章/博客——包含页眉、主图占位符、带插图和引用的正文、作者署名及相关文章推荐。适用于需求中包含''博客''、''文章''、''帖子''、''随笔''或''案例研究''等关键词时。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：A long-form article / blog post — masthead, hero image placeholder, article body with figures and pull quotes, author byline, related posts. Use when the brief asks for "blog", "article", "post", "essay", or "case study".',
 'builtin', now(), 1190),

(NULL, 'card-xiaohongshu', '小红书图文卡片', '小红书风格知识卡片, 多张联排可滑动浏览', '营销', '网页',
 'prototype', 'web',
 '用「小红书图文卡片」模板把我的内容做成一份「小红书风格知识卡片, 多张联排可滑动浏览」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1200),

(NULL, 'digital-eguide', '数字电子指南', '双页数字电子指南预览——第1页为封面（显示标题、作者、''内含什么''统计信息、目录预览）；第2页为内页（课程正文含引用语和步骤列表）。生活方式/创作者品牌风格。适用于需要''电子指南''、''数字指南''、''lookbook''、''引流素材''、''创作者指南''、''手册''或''PDF指南''的需求。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Design ''The Creator''s Style & Format Guide'' — cover page and one inside spread, lifestyle creator brand.',
 'builtin', now(), 1210),

(NULL, 'article-magazine', '杂志文章', 'Huashu / huashu-md-html 风格杂志文章版式, 将 Markdown 或笔记转成精排长文 HTML。', '营销', '网页',
 'prototype', 'web',
 '用「杂志文章」模板把我的内容做成一份「Huashu / huashu-md-html-inspired magazine article layout for turning Markdown or notes into a polished long-form HTML essay」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1220),

(NULL, 'magazine-poster', '杂志海报', '报纸风格的编辑海报，包含新闻纸材质、日期线、超大衬线标题（带删除线和斜体）、双栏正文、6个编号章节及引用注释。适合需要杂志海报、编辑排版、新闻纸风格或宣言设计的场景。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Design an editorial magazine-style poster — ‘You don''t need a designer to ship your first draft anymore.’ Newsprint paper, six numbered sections.',
 'builtin', now(), 1230),

(NULL, 'email-marketing', '电子邮件营销', '品牌产品发布电子邮件模板，包含报头、主图、斜体强调标题、正文、主要行动号召按钮和规格表格，采用HTML居中单栏布局。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Design a launch email for a sporty running shoe brand — masthead, hero, big headline lockup, specs grid, CTA.',
 'builtin', now(), 1240),

(NULL, 'social-carousel', '社交媒体轮播图', '三张 1080×1080 方形社交媒体轮播卡片——三个富有电影感和品牌调性的面板，配有连贯的展示标题（''往前走。'' → ''到下一个。'' → ''展望未来。''）。每张卡片包含品牌标识、序号/总数、说明文字和''循环''提示。适用于''轮播帖子''、''社交媒体轮播''、''Instagram 轮播''、''LinkedIn 系列''、''X 系列卡片''或''三连发''等需求。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Design a 3-card cinematic social carousel — ‘onwards.’, ‘to the next one.’, ‘looking ahead.’. 1080×1080 squares, drop-into-Instagram ready.',
 'builtin', now(), 1250),

(NULL, 'waitlist-page', '等候列表页面', '简约的预发布落地页，支持邮箱收集、品牌logo和可选装饰层。从 DESIGN.md 读取颜色、排版和布局规则。适用于产品发布、Beta 测试注册、抢先体验计划和独立项目。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Make a waitlist page for a design tool — clean, minimal, with a custom logo and one call-to-action.',
 'builtin', now(), 1260),

(NULL, 'sprite-animation', '精灵图动画', '像素风格的动画解说幻灯片，配有奶油色背景、粗体年份显示、像素艺术吉祥物（如花札卡、蘑菇或8位游戏机）、动态日文字体和时间轴条带。纯CSS动画，无需JS，可直接录制为竖屏视频。适合像素动画、8位解说、复古动画等需求。', '营销', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Create a sprite-based animation introducing trivia about Nintendo''s history. Combine pixel mascots, animated text, and a Hanafuda accent. Use color and type that feel like the Nintendo brand.',
 'builtin', now(), 1270),

(NULL, 'poster-hero', '营销海报', '竖版海报 / 朋友圈分享图, 强视觉冲击', '营销', '网页',
 'prototype', 'web',
 '用「营销海报」模板把我的内容做成一份「竖版海报 / 朋友圈分享图, 强视觉冲击」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1280),

(NULL, 'clinical-case-report', '临床病例报告', '为临床查房、病例讨论会和医疗文档生成结构化病例报告。支持生成SOAP格式或叙述性病例报告，包含生理准确的生命体征、实验室数据和循证医疗方案。', '行业', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：58-year-old male with 2 hours of substernal chest pain radiating to the left arm, diaphoresis, and ST elevation in leads II, III, aVF. Generate a full emergency cardiology case presentation.',
 'builtin', now(), 1290),

(NULL, 'hr-onboarding', '员工入职', '单页新员工入职计划——第一周日程安排、伙伴+经理介绍、学习路径、设备清单以及''完成标志''目标。当需求提及''入职''、''新员工''、''第一周计划''或''onboarding''时使用。', '行业', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Build a 30-day onboarding plan for a new product designer joining a 40-person startup.',
 'builtin', now(), 1300),

(NULL, 'dcf-valuation', 'DCF估值', '为上市公司提供现金流折现估值和内在价值分析。适用于DCF、公允价值、内在价值、目标价、低估或高估分析等需求。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Discounted cash flow valuation and intrinsic value analysis for public companies. Use when the brief asks for DCF, fair value, intrinsic value, price target, undervalued or overvalued analysis, or "what is this company worth?"',
 'builtin', now(), 1310),

(NULL, 'html-ppt-taste-brutalist', 'Html Ppt Taste Brutalist', '16:9 HTML 幻灯片，战术遥测 / CRT 终端风格。失活 CRT 炭黑背景、白色荧光等宽字体、警示红强调色、扫描线叠加、ASCII 语法，密度优先于装饰。提取自 Leonxlnx/taste-skill `brutalist-skill`（战术遥测模式）。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：16:9 HTML deck in tactical-telemetry / CRT-terminal taste. Deactivated-CRT charcoal slides, white-phosphor monospace, hazard-red accent, scanline overlay, ASCII syntax, density over decoration. Distilled from Leonxlnx/taste-skill `brutalist-skill` (Tactical Telemetry mode).',
 'builtin', now(), 1320),

(NULL, 'html-ppt-taste-editorial', 'Html Ppt Taste Editorial', '16:9 HTML 演示文稿，编辑极简风格。暖奶油色幻灯片，衬线标题 + 无衬线正文，细线分隔，等宽元数据，宽松留白，单一强调色。提炼自 Leonxlnx/taste-skill `minimalist-skill`。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：16:9 HTML deck in editorial-minimalist taste. Warm cream slides, serif display + grotesque body, hairline rules, monospace meta, generous macro-whitespace, one accent. Distilled from Leonxlnx/taste-skill `minimalist-skill`.',
 'builtin', now(), 1330),

(NULL, 'velar-luxury-real-estate', 'Velar. — 奢侈地产落地页', '电影感单页奢侈地产落地页：打字机预加载升起、从下方升起并放大同时贴向暗色段的滚动驱动建筑图、带数字 count-up 的 sticky 暗色统计带，以及滑动覆盖暗色段的 hover 展开视频画廊。', '设计', '网页',
 'prototype', 'web',
 '为奢侈地产品牌「Velar.」生成一个电影感的单页落地页（React + TypeScript + Tailwind + Vite，仅用 lucide-react 图标）：打字机预加载升起、从下方升起并放大到 1.45× 同时贴向暗色段底部的滚动驱动建筑图、带数字 count-up 的 sticky 暗色统计带、滑动覆盖暗色段的 hover 展开视频画廊。全部锁定的颜色/字体/动画时序/滚动数学，以及建筑图必须复用 example.html 内联的 data URI（勿换回 cloudinary 地址）等细节，以英文 query (`en`) 为准。',
 'builtin', now(), 1340),

(NULL, 'webgl-voronoi-cells', 'Voronoi 细胞', '自包含 WebGL2 主视觉：活的 Voronoi 网络，漂移特征点、按调色板着色的细胞与发光边界；移动光标推挤细胞。', '设计', '网页',
 'prototype', NULL,
 '构建自包含 WebGL2 主视觉：活的 Voronoi 网络，漂移点、调色细胞与发光边界，光标可推挤。',
 'builtin', now(), 1350),

(NULL, 'web-prototype-taste-brutalist', 'Web Prototype Taste Brutalist', '瑞士工业印刷风格网页原型。新闻纸画布、黑色粗体字、溢出式数字、细线网格分隔、危险红强调色、ASCII 语法装饰。源自 Leonxlnx/taste-skill `brutalist-skill`（瑞士工业印刷模式）。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Swiss industrial-print web prototype. Newsprint canvas, monolithic black grotesque, viewport-bleeding numerals, hairline grid dividers, hazard-red accent, ASCII syntax decoration. Distilled from Leonxlnx/taste-skill `brutalist-skill` (Swiss Industrial Print mode).',
 'builtin', now(), 1360),

(NULL, 'web-prototype-taste-soft', 'Web Prototype Taste Soft', 'Apple 级柔和风格网页原型。银灰/奶油色画布，双边框卡片，嵌套按钮 CTA，大圆角，弹性动效和环境网格。基于 Leonxlnx/taste-skill 的 soft-skill 及第 4-8 节。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Apple-tier soft web prototype. Silver/cream canvas, double-bezel cards, button-in-button CTAs, generous squircle radii, spring motion, ambient mesh. Distilled from Leonxlnx/taste-skill `soft-skill` + sections 4–8 of `taste-skill`.',
 'builtin', now(), 1370),

(NULL, 'web-prototype-taste-editorial', 'Web Prototype Taste 编辑风格', '极简编辑风格网页原型。暖色单色调画布，衬线标题配无衬线正文，1px 细线边框，柔和粉彩色块，大量留白，微妙动效。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Editorial-minimalist web prototype. Warm monochrome canvas, serif display + grotesque body, 1px hairline borders, muted pastel chips, generous macro-whitespace, ambient micro-motion. Distilled from Leonxlnx/taste-skill `minimalist-skill`.',
 'builtin', now(), 1380),

(NULL, 'webgl-experience', 'WebGL 体验', '构建全屏、实时的 WebGL/WebGL2 体验——动画着色器、3D 场景、生成式视觉、粒子场——在 GPU 上实时运行。当用户需要 WebGL、着色器、3D、生成式、GPU 或实时视觉效果时触发。', '设计', '网页',
 'prototype', NULL,
 '为{{subject}}构建一个全屏、实时的 WebGL2 体验——动画着色器、3D 场景、生成式视觉——作为单个自包含的 index.html，在 GPU 上实时运行。',
 'builtin', now(), 1390),

(NULL, 'worker-visualizer', 'Worker 可视化', '构建一个实时可视化器，将繁重计算放到 Web Worker 中运行——粒子系统、物理、分形、数据/音频可视化——可通过 SharedArrayBuffer 共享内存，以 60fps 渲染到画布。当用户需要 web worker、模拟、粒子系统、物理、分形、离主线程计算或实时可视化时触发。', '设计', '网页',
 'prototype', NULL,
 '构建一个实时的{{subject}}可视化器，将繁重计算放到 Web Worker 中运行（可用时通过 SharedArrayBuffer 共享内存），作为单个自包含的 index.html 以 60fps 渲染到画布。',
 'builtin', now(), 1400),

(NULL, 'x-research', 'X 研究', '针对市场、公司、产品或社区话题进行 X/Twitter 公众情绪研究。当需要了解 X 或 Twitter 上的舆论、加密社区情绪、专家观点或股票、行业、公司、产品、市场事件的社交反应时使用。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：X/Twitter public sentiment research for recent market, company, product, or community discourse. Use when the brief asks what people are saying on X, Twitter sentiment, CT sentiment, public opinion, expert posts, or social reaction around a stock, sector, company, product, or market event.',
 'builtin', now(), 1410),

(NULL, 'webgl-pixel-reveal-gallery', '像素揭示画廊', '滚动响应式瀑布流图廊（Three.js + GSAP）：每图进入视口时从像素/网格溶解显影，配分行标题动画与点击 Flip 放大。', '设计', '网页',
 'prototype', NULL,
 '构建滚动响应式瀑布流图廊，每图进入时从像素网格溶解显影，配分行标题与点击 Flip 放大。',
 'builtin', now(), 1420),

(NULL, 'webgl-raymarched-hero', '光线步进主视觉', '构建一个光线步进的 3D 主视觉——带 fresnel 边缘光的柔和阴影形变元球，缓慢环绕并在片元 shader 中逐像素球体追踪——以 60fps 全屏渲染，作为单个自包含的 index.html。', '设计', '网页',
 'prototype', NULL,
 '构建一个光线步进的 3D 主视觉，作为单个自包含的 index.html：在片元着色器中逐像素球体追踪一组平滑并集的元球有向距离场，配以软阴影、菲涅尔轮廓光和缓慢的镜头环绕，60fps 实时运行。',
 'builtin', now(), 1430),

(NULL, 'webgl-holographic-foil', '全息箔片', '自包含 WebGL2 主视觉：褶皱箔片表面的薄膜干涉，调色随视角变化；移动光标倾斜箔片。', '设计', '网页',
 'prototype', NULL,
 '构建自包含 WebGL2 主视觉：褶皱箔片薄膜干涉，调色随视角变化，光标可倾斜。',
 'builtin', now(), 1440),

(NULL, 'webgl-halftone-drift', '半调漂移', '自包含 WebGL2 主视觉：流场经旋转半调网点屏幕化为双色印刷质感；移动光标扰动漂移。', '设计', '网页',
 'prototype', NULL,
 '构建自包含 WebGL2 主视觉：流场经旋转半调网点屏幕化的双色印刷质感，光标可扰动。',
 'builtin', now(), 1450),

(NULL, 'webgl-distortion-grain', '失真与颗粒', '竖向图廊（Three.js）：平面随滚动速度弯曲、在光标处经单纯形噪声起伏失真，并带胶片颗粒。', '设计', '网页',
 'prototype', NULL,
 '构建竖向图廊，图片平面随滚动速度弯曲、在光标处经单纯形噪声失真、并带胶片颗粒。',
 'builtin', now(), 1460),

(NULL, 'tweaks', '实时调参', '为任何 HTML 组件添加侧面板实时控制器，可调整主题色、字体比例、密度、动效等参数，修改实时同步到 CSS 自定义属性并保存到 localStorage，让用户无需重新提示即可探索设计变体。', '设计', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Wrap this landing page with a tweak panel — accent color, type scale, density, light/dark — persist to localStorage so the user can refresh without losing their choice.',
 'builtin', now(), 1470),

(NULL, 'last30days', '最近30天', '最近30天的社区和社交趋势研究。适用于了解当前舆论、近期情绪、社区反应、社交证明、发布反馈、趋势扫描或最近30天的背景信息。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Recent community and social trend research over the last 30 days. Use when the brief asks what people are saying now, recent sentiment, community reactions, social proof, launch reaction, trend scan, or last-30-days context.',
 'builtin', now(), 1480),

(NULL, 'webgl-aurora-veil', '极光帘幕', '自包含 WebGL2 主视觉：夜空之上扭曲的分层极光帘幕，点缀星光；移动光标摆动帘幕。', '设计', '网页',
 'prototype', NULL,
 '构建自包含 WebGL2 主视觉：夜空之上分层极光帘幕加星光，光标可摆动。',
 'builtin', now(), 1490),

(NULL, 'wireframe-annotated', '标注线框图', '桌面落地页的标注/红线低保真线框图——浏览器框架内的灰盒区块、带编号的标注图钉（①–⑤），以及将每个图钉映射到简短工程/UX 说明的右侧规格面板。适用于「标注线框图」「红线」「线框规格」「低保真落地页」「低保真」「线框图」「标注线框」等需求。', '设计', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：排布一个落地页的标注红线线框图——灰盒导航、主视觉、Logo 横条、功能网格和页脚，配上带编号的标注图钉，以及右侧的工程说明规格面板。',
 'builtin', now(), 1500),

(NULL, 'webgl-horizontal-parallax', '横向视差画廊', '横向滚动 WebGL 画廊（Three.js）：画框随 lerp 平滑横移，每张图按其在视口的位置做纹理视差（UV 位移）。', '设计', '网页',
 'prototype', NULL,
 '构建横向滚动 WebGL 画廊，画框平滑横移，每图按视口位置做纹理视差。',
 'builtin', now(), 1510),

(NULL, 'webgl-caustic-pool', '水面焦散', '自包含 WebGL2 主视觉：由域扭曲涟漪织成的动态水面焦散；点击水面掉涟漪。无网格、无贴图。', '设计', '网页',
 'prototype', NULL,
 '构建自包含 WebGL2 主视觉：域扭曲涟漪织成的动态水面焦散，点击掉涟漪。',
 'builtin', now(), 1520),

(NULL, 'webgl-liquid-metal', '液态金属', '构建一个全屏液态金属 shader——熔融铬面泛起涟漪，扫过的镜面高光叠加在虹彩薄膜色板之上——以 60fps 渲染，作为单个自包含的 index.html。', '设计', '网页',
 'prototype', NULL,
 '构建一个全屏液态金属着色器，作为单个自包含的 index.html：将域扭曲的 fbm 高度场着色为熔融铬——锐利扫动的高光叠加在虹彩薄膜余弦调色板之上——60fps 实时运行。',
 'builtin', now(), 1530),

(NULL, 'webgl-depth-gallery', '深度画廊', '基于 Three.js 的滚动响应式 3D 图廊：沿 Z 轴堆叠的图片交叉淡入，配每图情绪背景、速度呼吸、发光光标拖尾与编辑感 CMYK/RGB/HEX/PMS 色卡。', '设计', '网页',
 'prototype', NULL,
 '构建滚动响应式 3D 图廊：沿 Z 轴堆叠、交叉淡入的图片，配每图情绪背景、速度呼吸、光标拖尾与 CMYK/RGB/HEX/PMS 色卡。',
 'builtin', now(), 1540),

(NULL, 'wireframe-greybox', '灰盒线框图', '桌面端 SaaS 仪表盘的清晰灰盒/蓝图低保真线框图——中性灰色块、带对角叉号的图片占位符、lorem 文本条、示意图表面板，以及 12 栏栅格上的等宽标注。适用于「灰盒」「蓝图线框」「低保真仪表盘」「低保真」「线框图」「灰盒原型」等需求。', '设计', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：布局一个 SaaS 仪表盘的灰盒低保真线框图——侧边栏导航、4 个一组的 KPI 行、示意图表面板和列表表格，并在 12 栏栅格上配以等宽 redline 标注。',
 'builtin', now(), 1550),

(NULL, 'social-media-dashboard', '社交媒体仪表板', '创作者社交媒体分析仪表板，单个 HTML 文件。包含平台切换器（X / LinkedIn / YouTube / Instagram）、KPI 卡片（粉丝数、互动率、点赞数、转发数）、粉丝增长图表、本周热门帖子预览和热门话题/评论侧边栏。', '设计', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Create a social media analytics dashboard using my Design System. Show X, LinkedIn, YouTube, Instagram with follower counts, engagement rate, likes, reposts, trending topics, and top comments.',
 'builtin', now(), 1560),

(NULL, 'mobile-app', '移动应用', '在页面中渲染于像素级精确的 iPhone 15 Pro 框架内的移动应用屏幕。通过复制种子文件 assets/template.html 并粘贴 references/layouts.md 中的一个屏幕原型来构建。当需求提到《移动应用》、《iOS 应用》、《Android 应用》、《手机屏幕》或《应用 UI》时使用。', '设计', '网页',
 'prototype', 'mobile',
 '使用这个插件完成以下任务：A mobile-app screen rendered inside a pixel-accurate iPhone 15 Pro frame on the page. Built by copying the seed `assets/template.html` and pasting one screen archetype from `references/layouts.md`. Use when the brief asks for "mobile app", "iOS app", "Android app", "phone screen", or "app UI".',
 'builtin', now(), 1570),

(NULL, 'wireframe-mobile-flow', '移动端线框流程', '低保真移动端应用流程线框图，将三到四个手机屏幕排成一行（引导 → 首页信息流 → 商品详情 → 确认），每个设备内为灰盒占位内容，屏幕间用带编号步骤标签的虚线箭头连接，并配有便签注释。适用于「移动端线框」「App 流程」「用户流程」「低保真移动端」「低保真」「移动端线框」「App 流程图」等需求。', '设计', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：绘制一个低保真移动端应用流程线框图 —— 四个手机屏幕（引导、首页信息流、商品详情、确认），灰盒占位内容、虚线连接箭头和带编号的步骤标签。',
 'builtin', now(), 1580),

(NULL, 'webgl-particle-galaxy', '粒子星系', '构建一个旋转的粒子星系——数万个加法混合的 GPU 点围绕明亮的核心螺旋盘旋，轨道在顶点 shader 中求解——以 60fps 实时渲染，作为单个自包含的 index.html。', '设计', '网页',
 'prototype', NULL,
 '构建一个 GPU 粒子星系，作为单个自包含的 index.html：数万个点的运动轨迹全部在顶点着色器中由 gl_VertexID 求解（内快外慢旋转曲线的对数螺旋盘），叠加混合成围绕明亮内核发光的旋臂，60fps 实时运行。',
 'builtin', now(), 1590),

(NULL, 'wireframe-sketch', '线框草图', '手绘风格的线框图，包含方格纸背景、马克笔/铅笔质感、多标签页变体、便签注释、图表占位符和阴影填充。适用于「线框图」「草图线框」「手绘」「低保真」「白板」「草稿」「手绘原型」等需求。', '设计', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Sketch a hand-drawn wireframe v0.1 for a portal — four tabbed variants on graph paper, marker headlines, sticky-note annotations, hatched chart placeholders.',
 'builtin', now(), 1600),

(NULL, 'webgl-liquid-iridescence', '虹彩流体', '自包含 WebGL2 主视觉：活的油水场提亮为虹彩焦散丝线；移动光标扰动流场。', '设计', '网页',
 'prototype', NULL,
 '构建自包含 WebGL2 主视觉：活的油水虹彩场加焦散丝线，光标可扰动。',
 'builtin', now(), 1610),

(NULL, 'critique', '设计评审', '对项目中的任何 HTML 成品进行 5 维度专家设计评审——哲学/视觉层级/细节/功能性/创新性，每项评分 0-10。输出包含雷达图、有据可依的评分以及三个清单（保留/修复/快速优化）的独立 HTML 报告。', '设计', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Run a 5-dimension critique on the magazine-web-ppt deck I just generated — score philosophy / hierarchy / detail / function / innovation, give me Keep / Fix / Quick-wins.',
 'builtin', now(), 1620),

(NULL, 'webgl-neon-grid', '霓虹网格', '构建一个 synthwave 霓虹网格场景——透视地面网格朝着带状复古落日滚动，铺在霓虹星空之下——以 60fps 渲染，作为单个自包含的 index.html。', '设计', '网页',
 'prototype', NULL,
 '构建一个赛博朋克霓虹网格场景，作为单个自包含的 index.html：解析式透视地面网格向着带状复古落日滚动，头顶是哈希生成的星空，地平线泛着辉光，60fps 实时运行。',
 'builtin', now(), 1630),

(NULL, 'flowai-live-dashboard-template', 'Flowai 实时仪表板模板', 'FlowAI 风格的团队管理仪表板，包含三个标签页（团队成员、团队详情、活动日志）、KPI 统计行、成员表格、角色分布条形图、在线状态和活动趋势图、顶级贡献者面板，支持亮暗主题切换、图表悬停提示、面板点击缩放和 CSV 导出功能。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Create a FlowAI-style team management dashboard with Team Members, Team Details and Activity Log tabs, KPI cards, a member table with status badges, a role-distribution bar chart, an online-presence sparkline, top contributors, light/dark mode, and CSV export.',
 'builtin', now(), 1640),

(NULL, 'github-dashboard', 'GitHub 仪表板', 'GitHub 仓库分析仪表板 — 星标、分支、贡献者、议题、拉取请求、近期活动和主要贡献者。适用于 GitHub 仓库仪表板、开源增长报告、仓库健康页面或 GitHub 分析视图。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Build a GitHub dashboard for nexu-io/open-design — stars, forks, contributors, issues, PRs, recent activity, and top contributors.',
 'builtin', now(), 1650),

(NULL, 'orbit-github', 'Orbit Github', '根据 GitHub 连接器自动生成每日摘要的 Orbit 技能。拉取过去 24 小时的 PR、审查请求、问题、CI 运行和合并记录，以类似 GitHub 原生通知的布局呈现。由 Orbit 每日摘要调度器自动调用。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Generate today''s Open Orbit GitHub briefing. GitHub is my only connected connector — pull yesterday''s PRs, review requests, issues, CI runs, and merges and render them as a GitHub Notifications + PR-diff page.',
 'builtin', now(), 1660),

(NULL, 'orbit-gmail', 'Orbit Gmail', '当 Gmail 是用户唯一连接的服务或用户明确指定时，自动提取过去 24 小时的收件箱活动并生成每日摘要。由 Orbit 调度器自动触发，不应手动调用。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Generate today''s Open Orbit Gmail briefing. Gmail is my only connected connector — pull yesterday''s mail and render it as the opened Orbit Daily Digest email inside Gmail''s reading view.',
 'builtin', now(), 1670),

(NULL, 'orbit-linear', 'Orbit Linear', '当 Linear 是用户唯一连接的数据源或用户明确将每日摘要范围限定为 Linear 时，由 Orbit 管道自动触发。提取过去 24 小时的问题动态、状态变更、任务分配和周期进度，并以 Linear 原生收件箱视觉语言呈现。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Generate today''s Open Orbit Linear briefing. Linear is my only connected connector — pull yesterday''s issue movement, cycle progress, status changes, and assignments and render them in Linear''s native Inbox layout.',
 'builtin', now(), 1680),

(NULL, 'orbit-notion', 'Orbit Notion', 'Open Orbit 简报技能 — 当 Notion 是用户唯一连接的连接器或用户明确将每日摘要范围限定为 Notion 时由 Orbit 管道选择。提取过去 24 小时内用户已认证的 Notion 连接中的文档编辑、评论、提及和数据库行更改，并将摘要呈现为原生 Notion 页面（标注/折叠/数据库表元素）。此技能不应手动触发 — 由 Orbit 的每日摘要调度程序针对实时 Notion 数据调', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Generate today''s Open Orbit Notion briefing. Notion is my only connected connector — pull yesterday''s document edits, comments, @ mentions, and database row changes and render the digest as a native Notion page.',
 'builtin', now(), 1690),

(NULL, 'orbit-general', 'Orbit 综合简报', '当用户连接两个或更多连接器时，Orbit 自动调用此技能，汇总过去 24 小时内所有已认证连接器（GitHub、Linear、Notion、Slack、飞书、日历、Gmail、云端硬盘、Sentry、Vercel 等）的活动数据，在「我的设计」顶部生成自适应网格仪表板。由 Orbit 每日摘要调度器自动触发。', '运营', '网页',
 'prototype', 'web',
 'Generate today''s Open Orbit morning briefing. I have ~10 connectors connected (GitHub, Linear, Notion, Calendar, 飞书, Sentry, Vercel, Slack, Gmail, Drive). Pull yesterday''s activity from each and render the editorial bento dashboard.',
 'builtin', now(), 1700),

(NULL, 'dashboard', '仪表盘', '单文件 HTML 管理/分析仪表盘。固定左侧边栏，顶部用户/搜索栏，主区域显示 KPI 卡片网格和图表。适用于需要''仪表盘''、''管理后台''、''数据分析''或''控制面板''界面的场景。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Admin / analytics dashboard in a single HTML file. Fixed left sidebar, top bar with user/search, main grid of KPI cards and one or two charts. Use when the brief asks for a "dashboard", "admin", "analytics", or "control panel" screen.',
 'builtin', now(), 1710),

(NULL, 'meeting-notes', '会议纪要', '会议记录页面——包含与会者标题栏、议程清单、决策事项区块、带负责人和日期的行动项表格，以及''下次会议''页脚。适用于需要''会议纪要''、''会议记录''、''1:1 笔记''、''全员会议总结''或''minutes''的场景。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Write up notes from a 60-minute Growth squad weekly — agenda, decisions, action items with owners, next meeting.',
 'builtin', now(), 1720),

(NULL, 'live-dashboard', '实时仪表板', 'Notion 风格的团队仪表板，以 Live Artifact 形式呈现。单页自包含 HTML 仪表板，包含 KPI、7 天趋势图、实时活动流和关联数据库任务表，通过 Composio 连接器目录连接到 Notion。支持按需刷新和打开时自动刷新，未绑定连接器时使用模拟数据，可离线使用。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Build me a Notion-style team dashboard for Acme Studio. KPIs: total tasks, done this week, active members, docs in review. Wire it to the Notion connector and let it refresh on demand.',
 'builtin', now(), 1730),

(NULL, 'live-artifact', '实时工件', '创建可刷新、可审计的 Open Design 工件，支持连接器或本地数据。适用于实时仪表板、可刷新报表、同步视图或可复用的数据驱动工件。', '运营', '网页',
 'prototype', NULL,
 '使用这个插件完成以下任务：Create refreshable, auditable Open Design artifacts backed by connector or local data. Trigger when the user asks for live dashboards, refreshable reports, synced views, or reusable data-backed artifacts.',
 'builtin', now(), 1740),

(NULL, 'kanban-board', '看板', '看板/任务面板，包含多个列（待办/进行中/审核中/已完成）、可拖动样式的卡片、成员头像、泳道和顶部筛选栏。当需求中提到''kanban''、''task board''、''sprint board''、''trello''、''看板''时使用。', '运营', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Make me a kanban board for a 5-person growth squad mid-sprint — backlog, doing, review, done.',
 'builtin', now(), 1750),

(NULL, 'invoice', '发票', '可打印的发票页面——包含发件人和收件人信息块、明细表、税费明细、总计和付款说明。当需求提到''invoice''、''bill''、''billing statement''或''发票''时使用。', '金融', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Create an invoice from a freelance design studio billing a client for a brand identity project — three line items, 10% retainer, 9% sales tax.',
 'builtin', now(), 1760),

(NULL, 'data-report', '数据可视化报告', '把 CSV/Excel/JSON 数据转成漂亮的可视化报告页', '金融', '网页',
 'prototype', 'web',
 '用「数据可视化报告」模板把我的内容做成一份「把 CSV/Excel/JSON 数据转成漂亮的可视化报告页」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 1770),

(NULL, 'finance-report', '财务报告', '季度/月度财务报告——包含关键指标的报头、收入与消耗图表、损益汇总表、重点摘要和展望段落。当需求提到''财务报告''、''Q3报告''、''MRR审查''、''P&L''或''财报''时使用。', '金融', '网页',
 'prototype', 'web',
 '使用这个插件完成以下任务：Build me a Q3 financial report for an early-stage SaaS — MRR, burn, gross margin, top accounts.',
 'builtin', now(), 1780),

(NULL, 'hps-memphis-pop', '像内容策展人一样做流行文化回顾演讲', '像内容策展人一样做流行文化回顾演讲——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像内容策展人一样做流行文化回顾演讲。像内容策展人一样做流行文化回顾演讲——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A pop-culture retrospective on how 1980s design language shaped today''s apps — the scenes, the turning point, and the takeaway. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1790),

(NULL, 'html-ppt-xhs-pastel-card', '像叙事播客制作人一样写个人宣言演讲', '像叙事播客制作人一样写个人宣言演讲——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像叙事播客制作人一样写个人宣言演讲。像叙事播客制作人一样写个人宣言演讲——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A personal manifesto: a year of saying yes — the premise, the scenes, the turn, and the meaning that lands. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1800),

(NULL, 'html-ppt-zhangzara-pink-script', '像婚礼讲述者一样把纪念日做成影像散文', '像婚礼讲述者一样把纪念日做成影像散文——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像婚礼讲述者一样把纪念日做成影像散文。像婚礼讲述者一样把纪念日做成影像散文——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A wedding-anniversary tribute photo essay — a decade in scenes, the turning points, and the quiet meaning of staying. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1810),

(NULL, 'html-ppt-zhangzara-neo-grid-bold', '像招聘 Bar Raiser 一样搭设计作品集叙事', '像招聘 Bar Raiser 一样搭设计作品集叙事——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像招聘 Bar Raiser 一样搭设计作品集叙事。像招聘 Bar Raiser 一样搭设计作品集叙事——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。 主题：A designer''s portfolio narrative for a senior interview — three case studies, the craft, and the judgment behind each. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1820),

(NULL, 'html-ppt-zhangzara-sakura-chroma', '像旅行内容总监一样把樱花之旅做成影像散文', '像旅行内容总监一样把樱花之旅做成影像散文——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像旅行内容总监一样把樱花之旅做成影像散文。像旅行内容总监一样把樱花之旅做成影像散文——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A cherry-blossom-season travel photo essay through Kyoto — the arrival, the peak bloom, and the moment that made the trip. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1830),

(NULL, 'html-ppt-zhangzara-capsule', '像晋升委员会内部人一样写年终述职', '像晋升委员会内部人一样写年终述职——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像晋升委员会内部人一样写年终述职。像晋升委员会内部人一样写年终述职——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。 主题：A year-end self-review for a product manager — the role, the outcomes, the learning, and the ask, all evidence-backed. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1840),

(NULL, 'html-ppt-xhs-white-editorial', '像晋升评审内部人一样写 Staff 工程师晋升材料', '像晋升评审内部人一样写 Staff 工程师晋升材料——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像晋升评审内部人一样写 Staff 工程师晋升材料。像晋升评审内部人一样写 Staff 工程师晋升材料——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。 主题：A staff-engineer promotion packet — scope, the proof moments, the artifacts, and the impact that clears the bar. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1850),

(NULL, 'html-ppt-zhangzara-8-bit-orbit', '像热爱驱动的讲述者一样把爱好做成故事', '像热爱驱动的讲述者一样把爱好做成故事——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像热爱驱动的讲述者一样把爱好做成故事。像热爱驱动的讲述者一样把爱好做成故事——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A gamer''s journey building a retro-arcade collection — the obsession, the hunt, and what the machines came to mean. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1860),

(NULL, 'html-ppt-zhangzara-retro-zine', '像独立图片编辑一样做街区 zine 故事', '像独立图片编辑一样做街区 zine 故事——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像独立图片编辑一样做街区 zine 故事。像独立图片编辑一样做街区 zine 故事——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A neighborhood zine on the disappearing corner shops — portraits, voices, and what a block loses when they close. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1870),

(NULL, 'hps-retro-tv', '像纪录片剪辑师一样把家庭影像做成家族史', '像纪录片剪辑师一样把家庭影像做成家族史——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像纪录片剪辑师一样把家庭影像做成家族史。像纪录片剪辑师一样把家庭影像做成家族史——一份可商业交付的生活故事 Deck，围绕真实主题、证据链与决策目标组织。 主题：A family history told through five decades of home movies — the opening question, the eras, and what the footage reveals. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1880),

(NULL, 'huashu-slides', '像高管形象教练一样讲职业转型故事', '像高管形象教练一样讲职业转型故事——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。', '个人', '网页',
 'deck', NULL,
 '像高管形象教练一样讲职业转型故事。像高管形象教练一样讲职业转型故事——一份可商业交付的职业发展 Deck，围绕真实主题、证据链与决策目标组织。 主题：A career-pivot story from consultant to product leader — the arc, the proof, and why the next role is the right bet. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1890),

(NULL, 'html-ppt-graphify-dark-graph', '像 Principal PM 一样写功能商业论证', '像 Principal PM 一样写功能商业论证——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像 Principal PM 一样写功能商业论证。像 Principal PM 一样写功能商业论证——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s feature business case for the plugin marketplace: the user pain, options, tradeoffs, and the measure of success. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1900),

(NULL, 'replit-deck', '像 Principal PM 一样写功能商业论证', '产品/技术管理场景：围绕 core query「pm-feature-business-case-deck」把粗糙材料整理成“像 Principal PM 一样写功能商业论证”这类可购买、可复用的专业 Deck；突出受众、决策目标、证据链、风险取舍和评审标准。', '产品', '网页',
 'deck', NULL,
 '像 Principal PM 一样写功能商业论证。产品/技术管理场景：围绕 core query「pm-feature-business-case-deck」把粗糙材料整理成“像 Principal PM 一样写功能商业论证”这类可购买、可复用的专业 Deck；突出受众、决策目标、证据链、风险取舍和评审标准。 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1910),

(NULL, 'html-ppt-tech-sharing', '像 Staff DevRel 一样做工程分享', '像 Staff DevRel 一样做工程分享——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像 Staff DevRel 一样做工程分享。像 Staff DevRel 一样做工程分享——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design internals: how the agent stream, sandbox, and artifacts work — an engineering deep-dive talk. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1920),

(NULL, 'deck-open-slide-canvas', '像世界级 Staff Engineer 一样写架构评审', '像世界级 Staff Engineer 一样写架构评审——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像世界级 Staff Engineer 一样写架构评审。像世界级 Staff Engineer 一样写架构评审——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s architecture review: the local daemon + agent-runtime design, the tradeoffs, and the decision to lock. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1930),

(NULL, 'html-ppt-obsidian-claude-gradient', '像企业 AI 转型负责人一样写落地简报', '像企业 AI 转型负责人一样写落地简报——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像企业 AI 转型负责人一样写落地简报。像企业 AI 转型负责人一样写落地简报——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s enterprise AI-adoption brief: local-first agents at work, the risk controls, the ROI, and the rollout plan. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1940),

(NULL, 'html-ppt-presenter-mode-reveal', '像创始 DevRel 一样做现场 AI 演示', '像创始 DevRel 一样做现场 AI 演示——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像创始 DevRel 一样做现场 AI 演示。像创始 DevRel 一样做现场 AI 演示——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design live demo: from a one-line prompt to runnable design in a single session — the flow, live. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1950),

(NULL, 'html-ppt-knowledge-arch-blueprint', '像平台 VP 一样把事故复盘写成学习稿', '像平台 VP 一样把事故复盘写成学习稿——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像平台 VP 一样把事故复盘写成学习稿。像平台 VP 一样把事故复盘写成学习稿——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s incident retro: the daemon-restart data bug, the root cause, the fix, and the systemic follow-ups. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1960),

(NULL, 'html-ppt-hermes-cyber-terminal', '像应用 AI 工程师一样讲 BYOK 选型', '像应用 AI 工程师一样讲 BYOK 选型——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像应用 AI 工程师一样讲 BYOK 选型。像应用 AI 工程师一样讲 BYOK 选型——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design + BYOK: choosing and wiring your own model, hands-on — cost, quality, and the routing decision. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1970),

(NULL, 've-terminal-mono', '像开发者工具讲师一样教 AI CLI 工作流', '像开发者工具讲师一样教 AI CLI 工作流——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像开发者工具讲师一样教 AI CLI 工作流。像开发者工具讲师一样教 AI CLI 工作流——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design from the CLI: driving the full design workflow with the `od` command — scripted, composable, agent-ready. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1980),

(NULL, 'frontend-slides', '像顶级技术解释者一样写 AI 工作流 101', '像顶级技术解释者一样写 AI 工作流 101——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像顶级技术解释者一样写 AI 工作流 101。像顶级技术解释者一样写 AI 工作流 101——一份可商业交付的AI 素养 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design as a worked AI-workflow example: how local agents read your files and design on your desktop — concretely. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 1990),

(NULL, 'hps-true-blueprint', '像首席架构师一样讲工程蓝图', '像首席架构师一样讲工程蓝图——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。', '产品', '网页',
 'deck', NULL,
 '像首席架构师一样讲工程蓝图。像首席架构师一样讲工程蓝图——一份可商业交付的产品管理 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s engineering blueprint: how the sandbox, sidecar, and daemon fit — the system diagram and the invariants. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2000),

(NULL, 'html-ppt', '像一线咨询 AI 行业负责人一样讲清行业 AI', '咨询/客户交付场景：围绕 core query「consulting-final-deck」把粗糙材料整理成“像一线咨询 AI 行业负责人一样讲清行业 AI”这类可购买、可复用的专业 Deck；突出受众、决策目标、证据链、风险取舍和评审标准。', '战略', '网页',
 'deck', NULL,
 '像一线咨询 AI 行业负责人一样讲清行业 AI。咨询/客户交付场景：围绕 core query「consulting-final-deck」把粗糙材料整理成“像一线咨询 AI 行业负责人一样讲清行业 AI”这类可购买、可复用的专业 Deck；突出受众、决策目标、证据链、风险取舍和评审标准。 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2010),

(NULL, 'huashu-keynote-black', '像世界级创始人 CEO 一样开全员大会', '像世界级创始人 CEO 一样开全员大会——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像世界级创始人 CEO 一样开全员大会。像世界级创始人 CEO 一样开全员大会——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s all-hands: the year in review, the three priorities, and what every team owns next quarter. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2020),

(NULL, 'html-ppt-zhangzara-peoples-platform', '像交通政策负责人一样做公共交通投资论证', '像交通政策负责人一样做公共交通投资论证——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像交通政策负责人一样做公共交通投资论证。像交通政策负责人一样做公共交通投资论证——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。 主题：A public-transit funding proposal for a city council — ridership need, the plan, the risk controls, and the ask. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2030),

(NULL, 'huashu-annual-letter', '像使命驱动的 CEO 一样写年度公开信', '像使命驱动的 CEO 一样写年度公开信——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像使命驱动的 CEO 一样写年度公开信。像使命驱动的 CEO 一样写年度公开信——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s annual letter to its community: the year''s story, what it learned, and where it''s going next. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2040),

(NULL, 'simple-deck', '像克制的 COO 一样写经营复盘', '像克制的 COO 一样写经营复盘——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像克制的 COO 一样写经营复盘。像克制的 COO 一样写经营复盘——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s operating review: growth, burn, and the concrete path to sustainability without losing the open ethos. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2050),

(NULL, 'html-ppt-zhangzara-soft-editorial', '像四大合伙人一样交付数字化转型路线图', '像四大合伙人一样交付数字化转型路线图——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像四大合伙人一样交付数字化转型路线图。像四大合伙人一样交付数字化转型路线图——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A digital-transformation roadmap for a legacy insurer — the diagnosis, the sequenced bets, and the operating rhythm to land them. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2060),

(NULL, 'html-ppt-zhangzara-grove', '像城市可持续总监一样写城市绿地政策简报', '像城市可持续总监一样写城市绿地政策简报——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像城市可持续总监一样写城市绿地政策简报。像城市可持续总监一样写城市绿地政策简报——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。 主题：A municipal urban-tree-canopy policy proposal — the public need, the evidence, the options, and the funding decision. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2070),

(NULL, 'html-ppt-zhangzara-blue-professional', '像幕僚长一样做季度经营回顾', '像幕僚长一样做季度经营回顾——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像幕僚长一样做季度经营回顾。像幕僚长一样做季度经营回顾——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s QBR for the executive committee: what moved, what stalled, and the resource reallocation ask. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2080),

(NULL, 'html-ppt-zhangzara-signal', '像战略发展负责人一样写战略决策备忘', '像战略发展负责人一样写战略决策备忘——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像战略发展负责人一样写战略决策备忘。像战略发展负责人一样写战略决策备忘——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s strategy memo: should it monetize the plugin registry now or hold — options, risks, and the recommendation. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2090),

(NULL, 'ppt-keynote', '像战略项目负责人一样讲运营模式重设计', '像战略项目负责人一样讲运营模式重设计——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像战略项目负责人一样讲运营模式重设计。像战略项目负责人一样讲运营模式重设计——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：An operating-model redesign for a scaling logistics firm — the diagnosis, the target model, and the transition roadmap. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2100),

(NULL, 'deck-swiss-international', '像财富 500 强董事长一样写董事会预读', '像财富 500 强董事长一样写董事会预读——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像财富 500 强董事长一样写董事会预读。像财富 500 强董事长一样写董事会预读——一份可商业交付的企业战略 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s FY26 board pre-read: the open-core bet, growth vs burn, and the one decision the board must approve. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2110),

(NULL, 'html-ppt-zhangzara-stencil-tablet', '像资深监管者一样做工作场所安全合规评审', '像资深监管者一样做工作场所安全合规评审——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像资深监管者一样做工作场所安全合规评审。像资深监管者一样做工作场所安全合规评审——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。 主题：A workplace-safety compliance review for a manufacturing regulator — findings, the evidence chain, and the corrective mandate. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2120),

(NULL, 'huashu-golden-circle', '像顶级咨询董事一样讲品牌重塑战略', '像顶级咨询董事一样讲品牌重塑战略——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像顶级咨询董事一样讲品牌重塑战略。像顶级咨询董事一样讲品牌重塑战略——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A brand-repositioning strategy for a heritage coffee chain — why, how, what — the governing idea and the moves to prove it. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2130),

(NULL, 'html-ppt-testing-safety-alert', '像首席合规官一样向医院董事会汇报数据治理', '像首席合规官一样向医院董事会汇报数据治理——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像首席合规官一样向医院董事会汇报数据治理。像首席合规官一样向医院董事会汇报数据治理——一份可商业交付的政策简报 Deck，围绕真实主题、证据链与决策目标组织。 主题：A hospital data-governance briefing: patient-data risk, the control framework, accountability, and the approval the board must give. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2140),

(NULL, 'huashu-luxe-whitespace', '像高端领域战略合伙人一样写奢侈品进入市场方案', '像高端领域战略合伙人一样写奢侈品进入市场方案——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像高端领域战略合伙人一样写奢侈品进入市场方案。像高端领域战略合伙人一样写奢侈品进入市场方案——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A market-entry study for a luxury skincare brand entering Asia — segmentation, positioning, channel, and the phased plan. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2150),

(NULL, 'html-ppt-zhangzara-mat', '像麦肯锡项目经理一样写利润率修复终稿', '像麦肯锡项目经理一样写利润率修复终稿——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。', '战略', '网页',
 'deck', NULL,
 '像麦肯锡项目经理一样写利润率修复终稿。像麦肯锡项目经理一样写利润率修复终稿——一份可商业交付的咨询交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A margin-recovery diagnosis for a regional grocery chain — the governing thought, the driver tree, the priorities, and the roadmap. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2160),

(NULL, 'html-ppt-zhangzara-retro-windows', '像 CISO 赋能团队一样做安全意识培训', '像 CISO 赋能团队一样做安全意识培训——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像 CISO 赋能团队一样做安全意识培训。像 CISO 赋能团队一样做安全意识培训——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：An IT security-awareness training on spotting phishing — the tells, the drill, and what to do in the first 60 seconds. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2170),

(NULL, 'hps-bauhaus', '像世界级工作室导师一样教设计基础', '像世界级工作室导师一样教设计基础——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像世界级工作室导师一样教设计基础。像世界级工作室导师一样教设计基础——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A visual-design fundamentals course for new brand designers — grid, type, color, and the exercises that build the eye. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2180),

(NULL, 'html-ppt-zhangzara-scatterbrain', '像作品集出众的设计学生一样讲毕业设计', '像作品集出众的设计学生一样讲毕业设计——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像作品集出众的设计学生一样讲毕业设计。像作品集出众的设计学生一样讲毕业设计——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。 主题：A design-school graduation project: a civic wayfinding system for a transit hub — the brief, the process, and the outcome. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2190),

(NULL, 'html-ppt-zhangzara-daisy-days', '像客户成功赋能总监一样写客户上手工作坊', '像客户成功赋能总监一样写客户上手工作坊——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像客户成功赋能总监一样写客户上手工作坊。像客户成功赋能总监一样写客户上手工作坊——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A customer-success workshop onboarding users to a project-management app — the first-value path and the habits that retain. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2200),

(NULL, 'html-ppt-zhangzara-cartesian', '像拿优等的经济学生一样做毕业论文答辩', '像拿优等的经济学生一样做毕业论文答辩——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像拿优等的经济学生一样做毕业论文答辩。像拿优等的经济学生一样做毕业论文答辩——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。 主题：An economics senior thesis on the employment effects of local minimum-wage increases — identification strategy, evidence, and limitations. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2210),

(NULL, 'html-ppt-zhangzara-pin-and-paper', '像明星生物学生一样做田野调查答辩', '像明星生物学生一样做田野调查答辩——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像明星生物学生一样做田野调查答辩。像明星生物学生一样做田野调查答辩——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。 主题：A field-biology capstone on urban pollinator decline — the survey design, the data, the contribution, and the caveats. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2220),

(NULL, 'fs-notebook-tabs', '像顶级导师一样答辩计算机毕业项目', '像顶级导师一样答辩计算机毕业项目——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像顶级导师一样答辩计算机毕业项目。像顶级导师一样答辩计算机毕业项目——一份可商业交付的课业答辩 Deck，围绕真实主题、证据链与决策目标组织。 主题：A computer-science capstone: an on-device ML keyboard that predicts next words privately — problem, method, evaluation, and defense answers. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2230),

(NULL, 'html-ppt-course-module', '像顶级赋能负责人一样做新人培训模块', '像顶级赋能负责人一样做新人培训模块——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像顶级赋能负责人一样做新人培训模块。像顶级赋能负责人一样做新人培训模块——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A first-30-days onboarding module for new hospitality hires — the behaviors, the practice, the checks, and the manager follow-up. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2240),

(NULL, 'html-ppt-zhangzara-playful', '像顶级零售教练一样做门店销售培训', '像顶级零售教练一样做门店销售培训——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。', '教育', '网页',
 'deck', NULL,
 '像顶级零售教练一样做门店销售培训。像顶级零售教练一样做门店销售培训——一份可商业交付的培训交付 Deck，围绕真实主题、证据链与决策目标组织。 主题：A retail sales-floor training on consultative selling — the flow, the role-plays, and the daily habit that lifts conversion. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2250),

(NULL, 'html-ppt-zhangzara-monochrome', '像拿到资助的 PI 一样写基金评审简报', '像拿到资助的 PI 一样写基金评审简报——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。', '研究', '网页',
 'deck', NULL,
 '像拿到资助的 PI 一样写基金评审简报。像拿到资助的 PI 一样写基金评审简报——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。 主题：A grant proposal on CRISPR base-editing for sickle-cell disease — the hypothesis, the approach, the milestones, and the risk. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2260),

(NULL, 'html-ppt-zhangzara-vellum', '像终身教授一样做人文研究讲座', '像终身教授一样做人文研究讲座——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。', '研究', '网页',
 'deck', NULL,
 '像终身教授一样做人文研究讲座。像终身教授一样做人文研究讲座——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。 主题：A humanities lecture: how Renaissance linear perspective reshaped early cartography — sources, argument, and evidence. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2270),

(NULL, 'kami-deck', '像资深博士后一样做组会研究汇报', '像资深博士后一样做组会研究汇报——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。', '研究', '网页',
 'deck', NULL,
 '像资深博士后一样做组会研究汇报。像资深博士后一样做组会研究汇报——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。 主题：A lab-meeting deck on gut-microbiome links to sleep quality — the design, the results, the caveats, and the next experiment. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2280),

(NULL, 'hps-academic-paper', '像顶刊综述作者一样写学术报告', '像顶刊综述作者一样写学术报告——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。', '研究', '网页',
 'deck', NULL,
 '像顶刊综述作者一样写学术报告。像顶刊综述作者一样写学术报告——一份可商业交付的学术研究 Deck，围绕真实主题、证据链与决策目标组织。 主题：A review deck on compositional generalization in large language models — the field map, the gap, the evidence, and open questions. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2290),

(NULL, 'fs-electric-studio', '像一线企业客户 AE 一样写 B2B SaaS 销售提案', '像一线企业客户 AE 一样写 B2B SaaS 销售提案——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像一线企业客户 AE 一样写 B2B SaaS 销售提案。像一线企业客户 AE 一样写 B2B SaaS 销售提案——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design as an enterprise design platform: a buyer-forwardable proposal for a design-org''s economic buyer — pain, value, ROI, rollout. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2300),

(NULL, 'deck-guizang-editorial', '像世界级 CMO 一样写年度营销计划', '像世界级 CMO 一样写年度营销计划——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像世界级 CMO 一样写年度营销计划。像世界级 CMO 一样写年度营销计划——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s FY26 marketing plan: brand-to-pipeline — audience, offer, channel mix, and the launch calendar that ties creative to growth. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2310),

(NULL, 'huashu-takram-soft-tech', '像企业销售负责人一样写买方委员会转发稿', '像企业销售负责人一样写买方委员会转发稿——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像企业销售负责人一样写买方委员会转发稿。像企业销售负责人一样写买方委员会转发稿——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design procurement & security leave-behind: the one-pager-plus a buying committee can forward and approve internally. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2320),

(NULL, 'open-design-landing-deck', '像会讲故事的创始人一样写品牌故事稿', '像会讲故事的创始人一样写品牌故事稿——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像会讲故事的创始人一样写品牌故事稿。像会讲故事的创始人一样写品牌故事稿——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s brand-story deck for partners and press: why it exists, what it believes, and where design is going. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2330),

(NULL, 'html-ppt-zhangzara-broadside', '像公关传播总监一样写产品发布公告', '像公关传播总监一样写产品发布公告——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像公关传播总监一样写产品发布公告。像公关传播总监一样写产品发布公告——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s product-launch announcement and press narrative — the headline, the proof points, and the call to action. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2340),

(NULL, 'huashu-pentagram-grid', '像品牌策略师一样搭建定位与信息体系', '像品牌策略师一样搭建定位与信息体系——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像品牌策略师一样搭建定位与信息体系。像品牌策略师一样搭建定位与信息体系——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s positioning & messaging system: the one-line promise, the pillars, and the proof — the source of truth for all copy. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2350),

(NULL, 'guizang-ppt', '像增长策略负责人一样写品牌到收入故事', '市场/增长/GTM场景：围绕 core query「annual-marketing-plan」把粗糙材料整理成“像增长策略负责人一样写品牌到收入故事”这类可购买、可复用的专业 Deck；突出受众、决策目标、证据链、风险取舍和评审标准。', '营销', '网页',
 'deck', NULL,
 '像增长策略负责人一样写品牌到收入故事。市场/增长/GTM场景：围绕 core query「annual-marketing-plan」把粗糙材料整理成“像增长策略负责人一样写品牌到收入故事”这类可购买、可复用的专业 Deck；突出受众、决策目标、证据链、风险取舍和评审标准。 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2360),

(NULL, 'fs-emerald-editorial', '像增长策略负责人一样写品牌发布叙事', '像增长策略负责人一样写品牌发布叙事——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像增长策略负责人一样写品牌发布叙事。像增长策略负责人一样写品牌发布叙事——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s ''design on your desk'' brand-launch narrative: the market moment, the story, and the proof that converts. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2370),

(NULL, 'html-ppt-zhangzara-coral', '像增长负责人一样规划社区增长战役', '像增长负责人一样规划社区增长战役——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像增长负责人一样规划社区增长战役。像增长负责人一样规划社区增长战役——一份可商业交付的市场增长 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s community-growth campaign across GitHub, Discord, and X: the loops, the content calendar, and the pipeline math. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2380),

(NULL, 'html-ppt-zhangzara-cobalt-grid', '像客户总监一样写续约与扩容论证', '像客户总监一样写续约与扩容论证——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像客户总监一样写续约与扩容论证。像客户总监一样写续约与扩容论证——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design renewal + seat-expansion business case for a growing customer: realized value, usage proof, and the expansion ROI. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2390),

(NULL, 'html-ppt-product-launch', '像战略客户 AE 一样推动团队落地', '像战略客户 AE 一样推动团队落地——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像战略客户 AE 一样推动团队落地。像战略客户 AE 一样推动团队落地——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design Teams: a launch-and-adoption proposal for a mid-market design team weighing a switch from closed cloud tools. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2400),

(NULL, 'huashu-bento-insight', '像顶级售前一样写竞品替换方案', '像顶级售前一样写竞品替换方案——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。', '营销', '网页',
 'deck', NULL,
 '像顶级售前一样写竞品替换方案。像顶级售前一样写竞品替换方案——一份可商业交付的B2B 销售 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design vs closed cloud design tools: a side-by-side displacement case on control, cost (BYOK), and lock-in. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2410),

(NULL, 'html-ppt-zhangzara-creative-mode', '像主创品牌设计师一样发布视觉识别系统', '像主创品牌设计师一样发布视觉识别系统——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像主创品牌设计师一样发布视觉识别系统。像主创品牌设计师一样发布视觉识别系统——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：A brand visual-identity system reveal for an outdoor label — logo, color, type, and the rules that keep it consistent. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2420),

(NULL, 'html-ppt-zhangzara-studio', '像创意工作室主理人一样做作品与报价稿', '像创意工作室主理人一样做作品与报价稿——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像创意工作室主理人一样做作品与报价稿。像创意工作室主理人一样做作品与报价稿——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：A photography studio''s portfolio-and-rate deck — the signature work, the process, and the packages that win the brief. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2430),

(NULL, 'fs-editorial-forest', '像杂志创意总监一样艺术指导年度报告', '像杂志创意总监一样艺术指导年度报告——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像杂志创意总监一样艺术指导年度报告。像杂志创意总监一样艺术指导年度报告——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：Art-directing a fashion house''s annual report — the editorial system, the photography rhythm, and the data spreads. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2440),

(NULL, 'html-ppt-zhangzara-editorial-tri-tone', '像杂志艺术总监一样搭建编辑设计系统', '像杂志艺术总监一样搭建编辑设计系统——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像杂志艺术总监一样搭建编辑设计系统。像杂志艺术总监一样搭建编辑设计系统——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：A type-and-color system for a culture magazine relaunch — the grid, the tri-tone palette, and the layout kit. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2450),

(NULL, 'html-ppt-zhangzara-biennale-yellow', '像美术馆策展总监一样做双年展策展稿', '像美术馆策展总监一样做双年展策展稿——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像美术馆策展总监一样做双年展策展稿。像美术馆策展总监一样做双年展策展稿——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：A curatorial deck for a contemporary art biennale — the thesis, the rooms, the works, and the visitor journey. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2460),

(NULL, 'html-ppt-zhangzara-raw-grid', '像音乐节艺术总监一样讲海报系列案例', '像音乐节艺术总监一样讲海报系列案例——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像音乐节艺术总监一样讲海报系列案例。像音乐节艺术总监一样讲海报系列案例——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：A brutalist poster-series case study for a music festival — the concept, the system, and how it scaled across formats. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2470),

(NULL, 'html-ppt-zhangzara-block-frame', '像高管级设计总监一样把混乱 Deck 救到董事会级', '像高管级设计总监一样把混乱 Deck 救到董事会级——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。', '设计', '网页',
 'deck', NULL,
 '像高管级设计总监一样把混乱 Deck 救到董事会级。像高管级设计总监一样把混乱 Deck 救到董事会级——一份可商业交付的设计打磨 Deck，围绕真实主题、证据链与决策目标组织。 主题：Rescuing a messy startup deck into a board-grade system — the diagnosis, the page grammar, and the rebuilt proof pages. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2480),

(NULL, 'html-ppt-zhangzara-long-table', '像 FP&A 伙伴一样讲清单位经济模型', '像 FP&A 伙伴一样讲清单位经济模型——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像 FP&A 伙伴一样讲清单位经济模型。像 FP&A 伙伴一样讲清单位经济模型——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s unit-economics and BYOK cost model: the assumptions, the sensitivity, and why it scales. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2490),

(NULL, 'html-ppt-zhangzara-bold-poster', '像一线 VC 合伙人一样写 A 轮增长叙事', '像一线 VC 合伙人一样写 A 轮增长叙事——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像一线 VC 合伙人一样写 A 轮增长叙事。像一线 VC 合伙人一样写 A 轮增长叙事——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s Series A growth story: the traction curve, the expansion motion, and why it''s venture-scale. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2500),

(NULL, 'html-ppt-weekly-report', '像分析负责人一样写每周增长复盘', '像分析负责人一样写每周增长复盘——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像分析负责人一样写每周增长复盘。像分析负责人一样写每周增长复盘——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s weekly growth review: stars, installs, activation — the number, the driver, and the recommended move. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2510),

(NULL, 've-midnight-editorial', '像创业公司 CFO 一样做财务复盘', '像创业公司 CFO 一样做财务复盘——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像创业公司 CFO 一样做财务复盘。像创业公司 CFO 一样做财务复盘——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s financial review: runway, burn, and the sustainability plan that keeps the project independent. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2520),

(NULL, 'ib-pitch-book', '像成长股权分析师一样写投资 Pitch Book', '像成长股权分析师一样写投资 Pitch Book——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像成长股权分析师一样写投资 Pitch Book。像成长股权分析师一样写投资 Pitch Book——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s investor pitch book: market map, moat, unit economics, and the ask — analyst-grade and diligence-ready. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2530),

(NULL, 'weekly-update', '像数据驱动的运营负责人一样开指标站会', '像数据驱动的运营负责人一样开指标站会——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像数据驱动的运营负责人一样开指标站会。像数据驱动的运营负责人一样开指标站会——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s weekly metrics standup: this week''s numbers, the one anomaly, and the single decision it forces. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2540),

(NULL, 'hps-y2k-chrome', '像能让 CFO 读懂的分析负责人一样写 KPI 决策简报', '像能让 CFO 读懂的分析负责人一样写 KPI 决策简报——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像能让 CFO 读懂的分析负责人一样写 KPI 决策简报。像能让 CFO 读懂的分析负责人一样写 KPI 决策简报——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s KPI decision brief: activation and retention drivers, what''s working, and the one metric to move next. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2550),

(NULL, 'huashu-sparkline-arc', '像董事会级财务伙伴一样写收入驱动分析', '像董事会级财务伙伴一样写收入驱动分析——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像董事会级财务伙伴一样写收入驱动分析。像董事会级财务伙伴一样写收入驱动分析——一份可商业交付的数据财务 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s revenue-driver narrative: what actually moves ARR, the leverage points, and the forecast. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2560),

(NULL, 'fs-creative-voltage', '像顶级 pre-seed 创始人一样写种子路演', '像顶级 pre-seed 创始人一样写种子路演——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像顶级 pre-seed 创始人一样写种子路演。像顶级 pre-seed 创始人一样写种子路演——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s seed pitch: the open, local alternative to closed AI design — why now, the wedge, and the ask. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2570),

(NULL, 'html-ppt-pitch-deck', '像顶级加速器合伙人一样写 Demo Day 路演', '像顶级加速器合伙人一样写 Demo Day 路演——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。', '金融', '网页',
 'deck', NULL,
 '像顶级加速器合伙人一样写 Demo Day 路演。像顶级加速器合伙人一样写 Demo Day 路演——一份可商业交付的融资路演 Deck，围绕真实主题、证据链与决策目标组织。 主题：Open Design''s demo-day pitch: hook, traction, moat, and the raise — built to make a partner sit up by page three. 先确认受众、决策目标、素材来源、截止时间和必须保留的数据，再输出叙事主线、页面规划、逐页文案、视觉方向和按评审标准自检的版本。',
 'builtin', now(), 2580),

(NULL, 'hatch-pet', 'Hatch Pet', '从角色美术、截图或生成图像创建、修复、验证、预览并打包符合 Codex 规范的宠物动画精灵图。自动生成 8x9 图集、透明空白单元格、逐行动画提示、QA 审查表、预览视频和 pet.json 打包文件。', '个人', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Hatch me a tiny pixel-art shiba pet — friendly, sitting upright, with a small pomegranate prop. Use the hatch-pet skill end-to-end.',
 'builtin', now(), 2590),

(NULL, '3d-stone-staircase-evolution-infographic', '3D 石制楼梯进化信息图', '将平面进化时间线转换为逼真的 3D 石制楼梯信息图，包含详细的生物渲染和结构化侧边面板。', '影音', '图片',
 'image', NULL,
 '{ "type": "evolutionary timeline infographic", "instruction": "Using REFERENCE_0 as a structural base, transform the flat vector design into a highly realistic 3D infographic. Replace the smooth ramps with distinct stone steps and upgrade all organisms to photorealistic 3D models.", "style": { "background": "{argument name=\"background style\" default=\"vintage textured parchment paper\"}", "staircase": "{argument name=\"staircase material\" default=\"realistic textured stone blocks\"}", "subjects": "{argument name=\"organism style\" default=\"highly detailed photorealistic 3D renders\"}" }, "layout": { "main_title": "{argument name=\"main title\" default=\"人类演化\"}", "sections": [ { "position": "left sidebar", "count": 8, "labels": ["L0: 单细胞生命", "L1: 多细胞生物", "L2: 动物界", "L3: 脊索动物", "L4: 上陆革命", "L5: 哺乳纲", "L6: 人科演化", "L7: 智人纪元"] }, { "position": "top right", "title": "获得的功能 / 失去的功能", "desc',
 'builtin', now(), 2600),

(NULL, 'notion-team-dashboard-live-artifact', 'Notion 风格团队仪表板（实时制品）', '单屏 Notion 原生团队仪表板模型——包含 KPI 网格、7 天趋势图、活动信息流和关联数据库任务表。可搭配实时制品功能实现数据刷新和连接器支持，也可作为静态模型独立使用。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：{ "type": "team productivity dashboard screenshot (prompt-only design preview, no live connector data)", "ui_aesthetic": "Notion native — off-white background #FFFFFF with #F7F6F3 sidebar, 14px SF Pro / Inter body, charcoal ink #37352F, hairline grid #ECECEA, accent blue #2EAADC used sparingly. No gradients, no card shadows, no rounded inner cards, no glassmorphism, no purple→pink hero, no emoji icon strip across the top.", "top_banner": { "color": "soft amber #FDECC8 with #E6CF94 hairline", "text": "Sample data — design preview. This page is a prompt-only Notion-style dashboard mockup; every number, name, and timestamp below is seeded, not pulled from a real Notion workspace or Composio connector. For real refreshable / connector-backed artifacts, see the live-artifact skill." }, "topbar": { "breadcrumb": "{argument name=\"workspace name\" default=\"Acme Studio\"} / Workspa',
 'builtin', now(), 2610),

(NULL, 'vr-headset-exploded-view-poster', 'VR头显爆炸视图海报', '生成高科技风格的VR头显爆炸视图，包含详细的组件标注和宣传文案。', '影音', '图片',
 'image', NULL,
 '{ "type": "exploded view product diagram poster", "subject": "VR headset", "style": "clean high-tech 3D render, studio lighting, glowing accents", "background": "{argument name=\"background color\" default=\"soft purple and blue gradient\"}", "header": { "logo": "∞ {argument name=\"product name\" default=\"Meta Quest 3\"}", "subtitle": "{argument name=\"main catchphrase\" default=\"まったく新しい現実を、まったく新しい構造から。\"}" }, "layout": { "centerpiece": "vertically stacked exploded view of a VR headset showing 9 distinct layers of internal components: outer shell, camera sensors, motherboard with chip, pancake lenses, internal frame, battery packs, side straps, top strap, and facial interface cushion.", "callout_labels": { "count": 8, "left_side": [ "Snapdragon® XR2 Gen 2\n圧倒的な処理性能でリアルタイムな体験を。", "調整可能なIPD機構\n幅広いユーザーに快適なフィット感を。", "精密設計されたヘッドストラップ\n快適さと安定性を追求したエルゴノミクス。" ], "right_side": [ "フェイスプレート\n洗練され',
 'builtin', now(), 2620),

(NULL, 'profile-avatar-cinematic-south-asian-male-portrait-with-vultures', '个人资料 / 头像 - 电影感南亚男性肖像与秃鹫', '一幅充满氛围感的南亚年轻男性肖像，置身于黑暗奇幻场景中，周围环绕着秃鹫和乌鸦。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A highly detailed cinematic portrait of a handsome {argument name="ethnicity" default="South Asian"} man in his late 20s or early 30s, sitting on a metal railing with a soccer goal net behind him. He has sharp facial features, dark styled hair, light stubble, and intense dark eyes. He is wearing a {argument name="clothing" default="black zip-up hoodie, black sweatpants, and white speckled sneakers"}. His hands are clasped together resting on his knees as he looks directly at the viewer with a confident, slightly brooding expression. He is surrounded by a dramatic flock of large black vultures and ravens. Some vultures are flying with wings spread in a dark stormy sky, while others are perched on the railing and goalpost near him. The atmosphere is {argument name="atmosphere" default="dark, moody, and cinematic"} with heavy storm clouds, dramatic lighting, and a mysterious, p',
 'builtin', now(), 2630),

(NULL, 'profile-avatar-realistically-imperfect-ai-selfie', '个人资料 / 头像 - 真实不完美 AI 自拍', '与 GPT Image 2 配合使用的创意提示词，生成看起来像意外拍摄的低质量智能手机快照的「失败」自拍照。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：ChatGPT, you''ve been with me for a while now, and I want to see what you look like. Please generate a photo similar to an {argument name="shooting method" default="accidental selfie"} taken with an {argument name="phone model" default="iPhone"}: no clear subject, no intentional composition, just a very ordinary, even slightly failed snapshot. The photo should have slight motion blur, uneven lighting, light overexposure, an awkward angle, and chaotic composition, presenting an ''overly realistic candid'' feeling, as if it were a selfie accidentally triggered while taking the phone out of a pocket.',
 'builtin', now(), 2640),

(NULL, 'profile-avatar-signed-marker-portrait-on-shikishi', '个人资料 / 头像 - 色纸签名马克笔肖像', '在方形色纸板上生成生动的签名马克笔风格肖像，适用于粉丝艺术签名、纪念插画帖子和个性化感谢视觉内容。', '影音', '图片',
 'image', NULL,
 'A lively hand-drawn fashion portrait in a changed illustration style, made to look like a signed fan-art sketch drawn with markers on a square white shikishi board with a thin gold border. Show a stylish young woman from about the waist up, leaning slightly forward with one elbow resting up near her face in a casual, friendly pose. Her face area is covered by a simple rectangular censor block in a muted beige tone. She has shoulder-length medium brown hair with warm highlights, soft volume, side-swept bangs, and flipped-out ends. Render the art with expressive black ink outlines, visible marker strokes, watercolor-like blending, sketchy hatching, and an energetic, vivid handmade feel. She wears a fitted dark gray ribbed long-sleeve knit top with subtle puffed shoulders, layered delicate gold necklaces, a dangling pearl earring, a beige crossbody bag strap running diagonally across her ch',
 'builtin', now(), 2650),

(NULL, 'profile-avatar-poetic-woman-in-garden-portrait', '个人资料 / 头像 - 花园中的诗意女性肖像', '生成一张书卷气息的年轻女性在阳光花园中的写实编辑风格肖像，适合生活方式摄影、文学品牌或优雅角色形象。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A realistic outdoor portrait of a thoughtful, naturally beautiful young woman standing on a garden path in soft golden-hour light. She is framed from about mid-thigh upward, centered in the image, facing the camera with a relaxed upright posture. She has {argument name="hair color" default="dark brown"} long, voluminous, loosely curly hair with a slightly tousled texture, falling around her shoulders. Dress her in an oversized cream-white knit sweater with long sleeves and dark high-waisted loose trousers or a flowing skirt-like bottom in deep navy or black. A pair of thin round eyeglasses hangs from the neckline of the sweater. In one hand she holds a sharpened yellow pencil, and in the other she carries an open sketchbook or notebook with slightly worn pages, suggesting she is writing, sketching, or observing nature. The mood should feel literary, artistic, intelligent, an',
 'builtin', now(), 2660),

(NULL, 'profile-avatar-hyper-realistic-selfie-texture-prompts', '个人资料 / 头像 - 超写实自拍纹理提示词', '用于生成逼真皮肤纹理和自然手机自拍构图的详细提示词片段，聚焦可见毛孔和自然光照效果。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：realistic skin texture, visible pores around nose and cheeks, natural slight unevenness, no filter quality, handheld phone camera feel, slight angle, casual framing, filmed in a real environment, soft window light from the left, natural indoor lighting, no harsh highlights',
 'builtin', now(), 2670),

(NULL, 'profile-avatar-casual-fashion-grid-photoshoot', '个人资料/头像 - 休闲时尚网格照片拍摄', '生成4张休闲时尚照片拼贴的结构化JSON提示词，包含详细的主体和光照参数。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：{ "scene_type": "smartphone fashion portrait series", "composition": { "layout": "4-photo grid collage", "camera": "smartphone photography", "framing": [ "full body standing", "crouching pose", "casual seated pose", "upper body portrait" ], "angle": "eye-level, natural perspective", "aspect_ratio": "1:1 collage" }, "subject": { "gender": "{argument name="gender" default="female"}", "age": "{argument name="age" default="early 20s"}", "aesthetic": "extremely beautiful, sensual, candid", "appearance": { "skin_tone": "smooth light complexion", "face": "soft feminine features, natural symmetry, bright smile", "expression": "playful, warm, confident, subtly sensual", "eyes": "expressive, gentle gaze", "hair": { "color": "{argument name="hair color" default="deep black"}", "style": "long, loose waves", "texture": "soft, natural shine" }, "makeup": "natural glam, dewy skin, soft blu',
 'builtin', now(), 2680),

(NULL, 'profile-avatar-professional-identity-portrait-wallpaper', '个人资料/头像 - 职业身份肖像壁纸', '生成高分辨率专业壁纸，展示职业着装的主体人物及其职业相关活动和文字排版。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Create a portrait size wallpaper of pride in carrying out the profession as an ({argument name="name" default="ALINA"}), in the wallpaper contains a photo of the attached subject wearing a uniform or things related to the profession, make a pose, the subject''s expression looks happy, don''t have the same expression as the attached photo, give the wallpaper ornaments, decorations related to the profession, add several activities related to the profession arranged neatly, precisely, harmoniously, the typography says "I am ({argument name="job title" default="ALINA FASHION TEACHER"}) "above the subject''s head, the font adjusts to the subject''s job, each part of the wallpaper must be neat, the wallpaper visuals should not look monotonous, should not look stiff, must be original style wallpaper cinematic resolution 8K coloring, grading, wallpaper effects must look premium. Face an',
 'builtin', now(), 2690),

(NULL, 'infographic-otaku-dance-choreography-breakdown-gokurakujodo-16-panels', '信息图 - 御宅族舞蹈动作分解（极乐净土，16格）', '一张2:3竖版海报，由4×4网格的16个方形面板组成，展示日本御宅族热门舞曲《极乐净土》的完整舞蹈动作分解。每个面板展示同一位半写实风格动漫偶像少女（粉色双马尾，水手服校园偶像制服）的标志性舞蹈姿势，专为AI视频生成设计的姿势参考图，适配gpt-image-2，2:3比例。', '影音', '图片',
 'image', NULL,
 'A SINGLE vertical image composed as a 4x4 grid (4 columns, 4 rows) of 16 connected square panels, forming a DANCE CHOREOGRAPHY BREAKDOWN CHART for the famous Japanese otaku dance song {argument name="song_title" default="極楽浄土"} ({argument name="song_romaji" default="Gokuraku Jodo"}). Purpose: this chart will be used as a POSE REFERENCE for AI video generation, so each pose MUST be clearly readable. === CHARACTER (must be IDENTICAL in all 16 panels) === {argument name="character" default="A cute half-realistic anime idol girl in her late teens, LONG BRIGHT PINK HAIR tied in TWO HIGH TWIN-TAILS with pink ribbons, LARGE sparkling turquoise-blue eyes, fair porcelain skin, soft rosy cheeks, wearing a Japanese-style idol school uniform: white sailor-collar blouse with pink bow, short pleated pink-and-white plaid mini skirt, white thigh-high socks, white Mary-Jane shoes with small heels. Slim p',
 'builtin', now(), 2700),

(NULL, 'anime-martial-arts-battle-illustration', '动漫武术对战插画', '生成动态高冲击力的动漫插画，描绘两位女性角色在传统道场中使用元素能量特效进行战斗。', '影音', '图片',
 'image', NULL,
 'An anime-style illustration of a {argument name="action type" default="high-impact martial arts battle"} between two young female fighters in a {argument name="setting" default="traditional wooden martial arts dojo"}. In the foreground, a girl with black hair in a high bun wears a {argument name="character 1 color theme" default="red and white"} Chinese-style martial arts outfit with baggy pants. She is in a dynamic, low, forward-thrusting stance, surrounded by swirling red energy and water splashes. In the background to the right, a girl with light purple hair in twin buns wears a {argument name="character 2 color theme" default="green and purple"} Chinese dress with gold embroidery and black tights. She is leaping through the air in a flying kick pose, surrounded by swirling blue energy. The wooden floorboards are splintering from the intense impact, with debris and dust flying through',
 'builtin', now(), 2710),

(NULL, 'profile-avatar-song-dynasty-hanfu-portrait', '头像 / 个人形象 - 宋朝汉服肖像', '生成宋代汉服美人在古代庭院中的精细写实肖像画的优化提示词。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：An {argument name="character description" default="18-year-old Chinese Internet celebrity beauty"}, with a model figure, exquisite facial features, cold and sweet temperament, wearing {argument name="outfit" default="elegant light pink Song Dynasty Hanfu"}, exquisite clothing details, with ancient-style buns, exquisite hairpin headdresses and embroidered shoes. The whole body stands in the front, with a natural and elegant posture, slightly showing the curve of the body. The {argument name="setting" default="scene is a beautiful ancient-style courtyard, with flowers and trees, cloisters and soft light and shadow"}. The picture is a high-quality ultra-realistic photography style, the characters are clear, the skin is delicate, the whole is aesthetic and high-end, and the 9:16 vertical composition.',
 'builtin', now(), 2720),

(NULL, 'profile-avatar-monochrome-studio-portrait', '头像 / 个人照片 - 单色工作室肖像', '专业商业摄影提示词，生成具有独特分割背景和戏剧化工作室灯光的单色肖像照片。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A stunning black and white studio portrait of {argument name="subject" default="uploaded person"}. Eye-level medium shot, framed from the waist up. The subject is standing with his arms casually but firmly crossed over his chest. He is looking downward and slightly off-camera to the left with a calm, contemplative posture. He is wearing a {argument name="outfit" default="dark, heavy-textured waffle-knit long-sleeve sweater"} and a delicate silver chain necklace with a small pendant. He is wearing a classic analog watch with a light dial and leather strap on the lower arm. The background is a {argument name="background style" default="stark, graphic vertical split: pure white on the left half and pure deep black on the right half"}. High-end commercial photography, monochrome masterpiece. Soft but dramatic directional studio lighting originating from the left, highlighting th',
 'builtin', now(), 2730),

(NULL, 'profile-avatar-elegant-fantasy-girl-in-violet-garden', '头像 / 个人资料 - 优雅幻想少女在紫罗兰花园', '生成精致动漫风格的幻想肖像，优雅女性角色搭配光泽造型发型、华丽紫黑服饰，置身于鲜花盛开的魔法花园场景中，适合角色设计使用', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A highly detailed anime fantasy portrait of a beautiful young woman seated at a stone table in an enchanted flower garden at golden hour, framed from the waist up in a vertical composition. She has {argument name="hair color" default="platinum blonde"} hair that is long, silky, glossy, and carefully groomed, with smooth flowing strands, soft waves, delicate shine, no frizz, no messy texture, and an elegant partial updo with a braided side twist and a gold hair ornament. Her visible styling should emphasize healthy, luxurious hair with clean strand definition and luminous highlights. She wears an ornate fantasy dress in {argument name="outfit colors" default="black, white, and violet"}, featuring a high black collar with gold filigree, white floral lace over the bodice, translucent puffed sleeves with lace cuffs, jeweled purple crystal ornaments, and elegant arm accessories. ',
 'builtin', now(), 2740),

(NULL, 'profile-avatar-lavender-fantasy-mage-portrait', '头像 / 个人资料 - 薰衣草幻想法师肖像', '生成精美的动漫风格奇幻法师公主肖像，金色光泽长发、紫色花朵和华丽水晶服饰，适合角色设计或魔法插画。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A highly detailed anime fantasy portrait of a beautiful young woman mage in a luminous flower garden at a castle. She is shown from about the waist up in a vertical composition, holding an ornate staff topped with a large faceted purple crystal in her right hand. Her face is obscured, but the rest of her design is elegant and refined. She has {argument name="hair color" default="platinum blonde"} hair, long and silky with a smooth glossy finish, soft flowing strands, delicate highlights, and no frizz or messy dryness; the hair is partially braided on one side and decorated with 3 large purple flowers and fine gold filigree hair ornaments. She wears a {argument name="dress color" default="lavender and white"} fantasy gown with off-shoulder ruffled sleeves, translucent fabric, layered chiffon, intricate gold trim, embroidered details, and 3 visible purple gemstones set into th',
 'builtin', now(), 2750),

(NULL, 'profile-avatar-snowy-rabbit-hanfu-portrait', '头像 / 个人资料 - 雪兔 Hanfu 肖像', '生成精致的兔耳女性 Hanfu 刺绣肖像，适用于角色艺术、服装设计或影视级 AI 肖像展示。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A highly detailed fantasy portrait of a young woman in side profile wearing elegant white rabbit ears and traditional East Asian hanfu in a snowy winter garden. She has {argument name="hair color" default="silver white"} hair, extremely long, silky, and softly wind-swept, styled with ornate floral and jeweled hair ornaments. The face is mostly obscured by a large centered rectangular blur mask in muted gray, covering the eyes, nose, and upper cheeks, as if censored for privacy. The rabbit ears are tall, plush, white, and realistic, with pale pink inner fur, attached through an elaborate headdress featuring black lace, silver filigree, small blossoms, crystals, beads, and tassels. Visible in the headdress are 2 round embroidered ornaments with black rabbit motifs, plus multiple dangling tassels in black and white, delicate chains, and floral metal branches. She wears asymmetr',
 'builtin', now(), 2760),

(NULL, 'profile-avatar-glamorous-woman-in-black-portrait', '头像 / 个人资料 - 黑色装束优雅女性肖像', '此提示词生成照片级真实感的奢华风格肖像，展现穿着低胸黑色服装的优雅女性，适用于时尚大片或美妆图像。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A photorealistic half-body portrait of an elegant glamorous woman indoors, framed vertically from the upper chest to just above the head, standing slightly angled toward the camera with a poised, confident presence. She has {argument name="hair color" default="dark brown"} long loose wavy hair with a soft tousled texture, warm lightly tanned skin, and a slender neck and shoulders. She wears a fitted black long-sleeve dress or top with a very deep plunging V neckline in finely pleated fabric, creating a sleek sensual evening look, plus 1 delicate gold chain necklace with 1 small round pendant resting at the base of her neck. Use flattering warm ambient lighting, soft shadows, shallow depth of field, and a luxurious modern interior background with creamy beige walls, a blurred warm lamp glow on the left, and a bright window or doorway edge on the right. The mood is refined, fe',
 'builtin', now(), 2770),

(NULL, 'profile-avatar-cyberpunk-anime-portrait-with-neon-face-text', '头像 / 用户资料 - 赛博朋克动漫肖像与霓虹字体', '时尚霓虹风格的动漫肖像，适用于海报、社交媒体艺术或未来感品牌视觉设计。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A dramatic cyberpunk anime close-up portrait of a white-haired young man in side profile facing right, with spiky silver hair, pale skin, and a black blindfold covering his eyes. He wears a high-collar dark coat and stands in a neon-lit futuristic city at night. Bright electric-blue glowing text is projected across the side of his face, reading exactly {argument name="face text" default="GPT IMAGE 2"} in three stacked lines. The mood is cool, mysterious, and high-energy, with deep black shadows, saturated blue and violet lighting, reflective highlights on the skin and hair, and a cinematic anime look reminiscent of modern supernatural action series. The background is a blurred urban street with dense vertical neon signs and holographic billboards; include 1 large vertical sign on the right with Japanese characters, plus at least 6 additional smaller glowing signs scattered i',
 'builtin', now(), 2780),

(NULL, 'profile-avatar-snowy-rabbit-spirit-portrait', '头像 / 雪兔精灵肖像', '生成宁静奇幻风格的兔耳少女冬季肖像，适合氛围感角色艺术和风格化头像插画。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A soft, painterly portrait of a mysterious young woman with {argument name="hair color" default="long white hair"} and 2 tall rabbit ears rising above her head, centered in a vertical composition from chest up. Her face is completely obscured by a flat rectangular censor block in muted beige, creating an anonymous surreal effect. She wears a traditional kimono-inspired robe in warm ivory with bold black trim: 3 visible black sections total, including the wide crossover collar, 2 black sleeve bands, and a black waist sash tied in front. On the left chest is 1 embroidered white rabbit patch outlined in brown. On the right side of her hair hangs 1 red braided cord ornament tied into a bow, decorated with 2 tassels and 1 small rabbit-shaped charm. The hair is long, flowing, slightly windswept, and silky, framing the shoulders. Set her in a quiet snowy landscape with falling snow',
 'builtin', now(), 2790),

(NULL, 'profile-avatar-anime-girl-to-cinematic-photo', '头像/个人资料 - 动漫女孩转电影照片', '将角色参考插画转换为真实感的暖色调复古室内人像，同时保留原始服装、姿势和猫咪。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Using the provided reference image, recreate the same girl and black cat in the same seated pose, but transform the flat anime drawing into a realistic cinematic photo. Keep the orange-and-black gothic dress, white frills, lightning armband, headpiece, black cat lying across her knees, white socks, and black Mary Jane shoes consistent with the reference. Place her in a moody vintage interior with a worn wooden floor, aged plaster walls, and 1 tall softly glowing window with sheer curtains on the left casting warm late-afternoon light. Use a nostalgic sepia-orange color grade, subtle film grain, soft shadows, and shallow depth of field for a photoreal editorial look.',
 'builtin', now(), 2800),

(NULL, 'profile-avatar-ethereal-blue-haired-fantasy-portrait', '头像/个人资料 - 空灵蓝发奇幻肖像', '此提示词生成柔和发光的动漫风格奇幻角色肖像，适合创建飘逸秀发和梦幻春日氛围的优雅竖版主视觉图或角色插画。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A highly detailed anime fantasy portrait of {argument name="character name" default="an elegant blue-haired fantasy woman"}, shown from the back in a three-quarter pose, turning her head over her shoulder to look at the viewer with calm violet eyes and a soft, slightly distant expression. She has very long, flowing {argument name="hair color" default="icy pastel blue"} hair with layered wispy bangs, loose windblown strands, one small ahoge on top, and 1 dark curved horn with subtle crimson striping emerging from the left side of her head. Her outfit is a refined, backless fantasy gown with 4 visible main pieces: a dark fitted bodice, a white open-backed outer layer with ornate gold trim and pale embroidered patterns, 2 long detached sleeves that fade into translucent blue-violet pointed cuffs, and red-blue ribbon ornaments tied at the neck and waist. Add delicate jewel-like ',
 'builtin', now(), 2810),

(NULL, 'profile-avatar-old-photo-restoration-to-dslr-portrait', '头像/个人资料 - 老照片修复为单反人像', '此提示可将损坏的 4 人家庭老照片修复为清晰、彩色的高分辨率真实人像，用于照片修复和增强。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Using the provided reference image, restore the damaged old family photo into a natural-looking modern high-resolution portrait while keeping the same 4 people, pose, framing, clothing, and outdoor rural setting unchanged. Remove all visible age damage including tears, cracks, creases, stains, worn paper edges, scratches, and fading. Convert the black-and-white sepia image into realistic soft color, preserving accurate skin tones and neutral earth-toned clothing. Enhance fine detail, sharpen fabric and hair texture, improve contrast and dynamic range, and upscale it to professional DSLR-quality realism with clean focus and a subtle shallow depth of field, as if photographed on a {argument name="camera model" default="Canon EOS R6 II"}. Keep the result highly realistic, natural, and faithful to the original faces and proportions.',
 'builtin', now(), 2820),

(NULL, 'profile-avatar-snow-rabbit-mask-hanfu-portrait', '头像/个人资料 - 雪兔面具Hanfu人像', '生成一位佩戴兔主题面具、身着白色Hanfu的女性冬日奇幻电影级人像，适用于优雅的角色艺术和氛围感AI展示图像。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A serene winter fantasy portrait of a woman standing outdoors in softly falling snow, framed from about mid-thigh upward, wearing an elegant traditional Hanfu-inspired robe in white and black. Her face is fully covered by a smooth white rabbit mask with upright pink-lined ears, small black eye openings, and a minimal cute expression. She has very long straight silver-white hair flowing past her waist, with delicate white floral and branch-like hair ornaments on both sides. Her robe is bright white with subtle embroidered silver detailing on the chest and shoulders, very wide draping sleeves, black trim along the collar and sleeve edges, and a fitted black waist sash tied at the front with tasseled cords and a snowflake-like ornament. The garment features visible rabbit motifs: 4 illustrated white rabbits in total, with 2 large rabbits near the outer lower sleeves, 1 small ho',
 'builtin', now(), 2830),

(NULL, 'profile-avatar-snow-rabbit-empress-portrait', '头像/简介 - 雪兔女帝肖像', '生成身着华丽冬季汉服的兔子主题女性角色，站在雪山寺庙场景中的写实奇幻肖像提示词。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A cinematic fantasy portrait of an elegant East Asian-inspired woman standing outdoors in a snowy mountain temple courtyard, centered in the frame from about waist-up. She wears a luxurious winter hanfu in glossy white and deep black satin with soft white fur trim at the collar and sleeves, embroidered with rabbit motifs and delicate floral patterns. Her long straight hair is silver-white, falling over both shoulders, and she wears an ornate silver headdress with filigree, pearls, dangling tassels, a pale turquoise jewel, and prominent upright white rabbit ears. Her face is deliberately obscured by a smooth rectangular blur block. Snow is falling across the scene. The background shows a dramatic cold blue-gray sky, snow-covered pine trees, distant jagged mountains, stone lanterns, and traditional Chinese temple buildings with curved tiled roofs on the right side. Mood is eth',
 'builtin', now(), 2840),

(NULL, 'illustrated-city-food-map', '手绘美食地图插画', '生成手绘水彩风格的旅游地图，展示带编号的当地特色美食、地标建筑和图例说明。', '影音', '图片',
 'image', NULL,
 '{ "type": "illustrated map infographic", "style": "{argument name=\"art style\" default=\"watercolor and ink hand-drawn illustration on vintage parchment\"}", "title_section": { "text": "{argument name=\"city name\" default=\"成都\"} {argument name=\"map title\" default=\"吃货暴走地图\"}", "mascot": "cartoon red chili pepper wearing sunglasses and giving a thumbs up" }, "border": "{argument name=\"border decoration\" default=\"vine of green leaves and red chili peppers\"}", "layout": { "background": "textured beige parchment paper with yellow roads, blue rivers, and green park areas", "sections": [ { "title": "landmarks", "count": 6, "illustrations": ["traditional pavilion", "traditional monastery", "modern skyscraper with climbing panda", "tall TV tower", "traditional gate", "industrial buildings"], "labels": ["人民公园", "文殊院", "IFS", "339电视塔", "宽窄巷子", "东郊记忆"] }, { "title": "food_spots", "count": ',
 'builtin', now(), 2850),

(NULL, 'illustration-crayon-kid-drawing-rework', '插画 - 蜡笔儿童画风格转换', '将任何参考图像转换为10岁儿童手绘蜡笔插画风格，使用明亮的蜡笔色彩和白纸背景，添加童趣元素如城堡、糖果、星星、云朵、彩虹等。适用于网页截图、品牌视觉、产品照片和人像的 gpt-image-2 图生图编辑。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Rework the given image into a crayon-style illustration, transforming the entire scene into something that feels hand-drawn by a 10-year-old. Preserve the general layout and spatial relationships of the original image — transform the style first, embellish second, so small UI elements, faces, and logos stay where they are. Keep the forms simple and slightly imperfect, like a child''s drawing — wobbly outlines, uneven strokes, visible waxy crayon texture, soft smudges where colors overlap. Avoid using the original color palette — replace it with bright, playful crayon colors (sunshine yellow, candy pink, sky blue, mint green, lavender, tangerine, grass green) on a clean white paper background with subtle paper grain. Aim for a soft, cute, and innocent aesthetic. Incorporate fun, childlike details such as fairy-tale castles or towers in the corners, lollipops and candy, big shi',
 'builtin', now(), 2860),

(NULL, 'momotaro-explainer-slide-in-hybrid-style', '桃太郎解说幻灯片混合风格', '结合 Irasutoya 插画简洁温暖的美学与日本政府幻灯片高信息密度特征的提示词。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Create an explanatory slide ({argument name="format" default="ponchi-e diagram"}) for {argument name="theme" default="Momotaro"} that fuses the gentle atmosphere of "Irasutoya" with the overwhelming information density of "Kasumigaseki slides".',
 'builtin', now(), 2870),

(NULL, 'game-ui-ancient-china-open-world-mmo-hud', '游戏 UI - 古代中国开放世界 MMO HUD', '生成古代中国题材 AAA 级开放世界 MMO 游戏的 HUD 界面截图样机，采用 Black Myth: Wukong 风格的电影级写实画面。包含完整的 MMO HUD 元素：角色头像、血条法力条、技能快捷栏（中文书法风格图标）、小地图、任务追踪面板、聊天窗口和 NPC 名牌等，16:9 比例，适用于游戏宣发和展示。', '影音', '图片',
 'image', NULL,
 'A full-screen in-game HUD screenshot of a AAA ancient-China open-world MMO, rendered in the cinematic photoreal style of Black Myth: Wukong — Unreal Engine 5 level lighting, volumetric god rays, deep filmic color grading, subtle chromatic aberration, shallow depth of field on the background, razor-sharp foreground. # 3D scene (underneath the UI) - Center of frame: {argument name="protagonist" default="a beautiful Chinese female swordswoman in her mid 20s, flowing ivory-white Hanfu robe with pale jade embroidery, long black hair tied with a silk ribbon, jade hairpin, elegant calm expression, holding a slender straight jian sword in a low guard stance, gentle wind lifting her sleeves and hair ribbon"}, captured in a cinematic third-person over-the-shoulder framing, shot from slightly behind and above her right shoulder so the viewer sees both her profile and the world ahead. - Environment:',
 'builtin', now(), 2880),

(NULL, 'game-screenshot-three-kingdoms-zhaoyun-cradle-escape', '游戏截图 - 三国 ARPG：赵云长坂坡怀抱幼主突围', '三国题材动作 RPG 游戏截图，展现赵云一手怀抱婴儿刘禅、一手持枪杀敌的长坂坡突围场景。采用《黑神话：悟空》与《艾尔登法环》风格的影视级写实渲染，包含护送保护条、连击计数器等完整 HUD 界面元素。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Cinematic in-game screenshot from a AAA next-generation action RPG in the style of Black Myth Wukong combined with Elden Ring, rendered in Unreal Engine 5 with full Nanite and Lumen ray-tracing, cinematic post-processing, shallow DOF bokeh, ray-traced reflections, volumetric god-rays and atmospheric dust particles. Third-person gameplay camera, low-angle tracking shot positioned about 3 meters behind the player character. # Playable character {argument name="character" default="Zhao Yun, a legendary Three Kingdoms warrior. Athletic build, heroic sharp-featured face visible from side angle, hair tied in a warrior''s topknot with a gold band, wearing ornate polished silver-steel plate-lamellar armor with gold trim and deeply engraved dragon patterns etched into each plate — battle-weathered with scratches, dents, and dust stains showing realistic wear. The armor has cinematic P',
 'builtin', now(), 2890),

(NULL, 'game-screenshot-three-kingdoms-guanyu-slaying-yanliang', '游戏截图 - 三国ARPG：关羽斩颜良', '《Black Myth: Wukong》风格的三国题材ARPG游戏截图，展现关羽骑赤兔马在暴雨战场冲向敌将颜良的经典场景，采用Unreal Engine 5渲染，第三人称视角，包含完整Boss战HUD界面。16:9比例，适配gpt-image-2。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：In-game screenshot from a next-gen action RPG in the style of Black Myth Wukong, Unreal Engine 5 Nanite/Lumen quality. Third-person gameplay camera, tracking from behind and slightly left of the player character as he rides at full gallop. # Playable character {argument name="character" default="Guan Yu, a Three Kingdoms legendary general. Towering broad-shouldered figure with a crimson-tinted complexion, long flowing black beard reaching his chest, stern phoenix eyes narrowed, single topknot with gold band. Wearing deep green lamellar armor with gold trim, a massive red-crimson silk cloak billowing dramatically behind him"}. # Mount and weapon Riding {argument name="mount" default="the legendary blood-red Red Hare warhorse"} mid-gallop, his boots braced in ornate stirrups. Holding overhead {argument name="weapon" default="the Blue Dragon Crescent Glaive, a massive curved-bl',
 'builtin', now(), 2900),

(NULL, 'game-screenshot-three-kingdoms-lyubu-yuanmen-archery', '游戏截图 - 三国ARPG：吕布辕门射戟', '三国题材动作RPG游戏截图，呈现吕布辕门射戟止战的经典场景。采用Black Myth: Wukong式写实电影风格和Unreal Engine 5技术，第三人称越肩视角，包含完整游戏HUD界面（生命值和气条、小地图、技能栏、锁定目标标记及距离显示）。适配gpt-image-2的16:9比例。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：In-game screenshot from a next-gen action RPG in the style of Black Myth Wukong, Unreal Engine 5 Nanite/Lumen rendering. Third-person over-the-shoulder gameplay camera, positioned about 2 meters behind and slightly above the playable character. # Playable character {argument name="character" default="Lü Bu, a Three Kingdoms era Chinese warrior general. Tall muscular build, long black hair tied in a high topknot with a phoenix-feather pin, wearing ornate crimson and blackened-iron lamellar armor with gold trim, a red silk cloak flowing behind him, a fanged guardian mask on his forehead"}. # Action pose The character is drawing a massive recurved warbow at full tension, an arrow nocked and glowing with {argument name="qi_color" default="orange"} qi runes, standing firm with a wide battle stance on dry yellow earth. # Setting {argument name="setting" default="An ancient Chinese',
 'builtin', now(), 2910),

(NULL, 'game-screenshot-anime-fighting-game-captain-ryuuga-vs-kaze-renshin', '游戏截图 - 动漫格斗游戏：龙牙船长 vs 风刃神', '街头霸王 6 或铁拳 8 风格的格斗游戏视觉截图。两名动漫风格的男性战士在夜晚的中国寺庙庭院中对决，配有完整的格斗游戏 HUD（双血条、回合计时器、P1/P2 角色面板、连击计数器和能量槽）。为 gpt-image-2 优化，16:9 比例。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A high-detail anime fighting game screenshot, 16:9 aspect ratio, cinematic key visual in the style of Street Fighter 6 or Tekken 8 intro art. Two anime male warriors in dynamic combat poses facing each other in the center. # LEFT FIGHTER {argument name="left_fighter" default="shirtless, wearing a worn straw hat, red battle-scar across left eye, grinning with clenched teeth, shark-tooth necklace, tattered red cape, black pants with a skull-pattern sash, bare feet, right fist raised ready to strike, orange fire particles and water splashing at his feet, warm orange-red energy aura surrounding him"}. # RIGHT FIGHTER {argument name="right_fighter" default="spiky jet-black hair, wearing an orange martial-arts gi with a single black kanji character on the left chest, blue waistband sash, black wristbands, both hands in a charging pose with a massive crackling blue lightning energy',
 'builtin', now(), 2920),

(NULL, 'e-commerce-live-stream-ui-mockup', '电商直播界面模型', '生成逼真的社交媒体直播界面，叠加在人像上，支持自定义聊天消息、礼物弹窗和商品购买卡片。', '影音', '图片',
 'image', NULL,
 '{ "type": "live stream UI mockup", "subject": { "description": "portrait of {argument name=\"host name\" default=\"Elon Musk\"}, smiling, wearing a black t-shirt with a white technical schematic graphic", "background": "left side shows a screen with ''{argument name=\"left background logo\" default=\"SPACEX\"}'' text, right side shows a red ''{argument name=\"right background logo\" default=\"Tesla T logo\"}'' and a dark car" }, "ui_overlay": { "top_header": { "host_info": "avatar, name ''{argument name=\"host name\" default=\"Elon Musk\"}'', subtext ''55.6万本场点赞'', red ''关注'' button", "rank_badge": "gold coin icon with ''全站第1名''", "viewer_stats": "3 top viewer avatars with ''12.3w'', ''8.6w'', ''5.7w'', total ''68.7万'', ''X'' close button", "right_links": "''更多直播 >'', ''礼物展馆 0/24'' with blue ''经典'' tag" }, "mid_left_gifts": { "count": 2, "items": [ "avatar ''科技爱好者'', ''送小心心'', heart icon x 1314", "avatar ''星辰大海'', ''送火箭'',',
 'builtin', now(), 2930),

(NULL, 'social-media-post-psg-transfer-announcement-poster', '社交媒体帖子 - PSG 转会公告海报', '用于在社交媒体或体育宣传图形上宣布球员转会至巴黎圣日耳曼的醒目专业足球签约海报。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Create a dramatic football transfer announcement poster in a vertical social-media format, centered on a photorealistic adult male soccer player wearing a modern Paris Saint-Germain home jersey, arms crossed, chest-up framing, strong athletic build, face mostly obscured by a soft rectangular blur block for anonymity, short close-cropped hair visible around the edges. Use a deep navy blue PSG-themed color palette with bold red and white accents. The jersey should feature a central red vertical stripe bordered by white, a red swoosh-style sports logo on the left side from the viewer''s perspective, the PSG crest on the opposite chest, and faint sponsor lettering across the torso. Place the player in front of a layered graphic background featuring an oversized faded PSG crest filling most of the upper-right background, a dark Eiffel Tower silhouette on the right side, painterly ',
 'builtin', now(), 2940),

(NULL, 'social-media-post-anime-pokemon-shop-outfit-teaser-poster', '社交媒体帖子 - 动漫 Pokémon 商店服装预告海报', '生成柔和粉彩动漫风格的时尚宣传海报，展示 Pokémon 商店内身穿蓝色连衣裙的模糊面部女孩，适合服装揭晓预告和角色宣传视觉。', '影音', '图片',
 'image', NULL,
 'A dreamy pastel anime fashion announcement poster set inside a bright Pokémon merchandise shop. The composition is vertical and split visually into two zones: a large translucent information panel on the left and a full-body character showcase on the right. The scene has a soft, elegant, airy atmosphere with diffused indoor lighting, creamy highlights, gentle reflections on the polished floor, and a refined shoujo illustration style. In the background, show a clearly recognizable Pokémon store interior with display shelves, the Pokémon logo sign, a large Poké Ball emblem on the wall, potted plants, plush toys, and figures; visible Pokémon merchandise includes exactly 3 prominent character plushies or mascots: Pikachu at the bottom right, plus 2 small shelf plushies resembling Piplup and another pastel blue-green character. The girl stands slightly right of center in a graceful fashion po',
 'builtin', now(), 2950),

(NULL, 'social-media-post-confused-elf-girl-at-pastel-desk', '社交媒体帖子 - 困惑精灵女孩在粉彩桌前', '生成柔和粉彩动漫风格的精灵女孩在温馨可爱工作区使用电脑的插画，适用于社交媒体帖子、壁纸或主播主题艺术。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A cute pastel anime illustration of a young elf girl streamer or office worker sitting at a desk and typing on a mechanical keyboard in a cozy bedroom workspace, shown from a front three-quarter view with a large black computer monitor in the left foreground partially blocking her body. She has long wavy {argument name="hair color" default="orange"} hair with glossy highlights, pointed elf ears, and a small red flower hair clip on the right side, wearing a light blue pajama-style blouse covered in red heart prints with a very frilly white lace collar and a shiny red ribbon bow at the neck. Her hands are on the keyboard, nails painted soft pink, and she sits in a rounded pink desk chair. Above her head is a speech bubble containing a large question mark, suggesting confusion while working at the computer. The room is soft, bright, and feminine, with a pale pink and cream colo',
 'builtin', now(), 2960),

(NULL, 'social-media-post-vintage-sign-painter-sketch', '社交媒体帖子 - 复古招牌画师素描', '生成纸上手绘马克笔素描，包含石墨线条和墨水渗透等真实细节，适合复古字体风格。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A hand-lettered sketch of the phrase “{argument name="phrase" default="Good Morning"} ” on warm-white marker paper, drawn with a black brush marker. Soft graphite construction lines visible underneath the inked strokes. Slight ink bleed-through from the previous page showing as faint ghosting. Letterforms are vintage sign-painter caps. Confident single-pass strokes, not retraced. Paper edges visible at the margins. Studio scan, slightly warm white balance, 600 DPI texture, no digital cleanup. No vector outlines, no AI airbrushed shading, no perfect symmetry.',
 'builtin', now(), 2970),

(NULL, 'social-media-post-sensational-girl-dance-storyboard-8-shots', '社交媒体帖子 - 惊艳女孩舞蹈分镜（8镜）', '包含8个连续舞蹈镜头的完整分镜提示词集，用于生成风格统一的舞蹈角色动作序列。适配gpt-image-2模型，提供全局样式参数、通用负面提示词和8个分镜提示词（开场姿势、胯部律动、身体波浪、节拍扭腰、侧胯摇摆、甩发、力量站姿、结束造型）。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：# Sensational Girl Dance — 8-Shot Storyboard for GPT-Image-2 For each shot, prepend the GLOBAL STYLE TOKENS and append the NEGATIVE PROMPT. Shots are choreographed to flow as a continuous short dance clip, so do not change the outfit, hair, body type, or lighting language between frames. ## GLOBAL STYLE TOKENS (prepend to every shot) ultra-high definition, 8K, crisp fine detail, textured skin, natural complexion, native ambient light, subtle street atmosphere, minimal backdrop, gentle motion blur, dance kinetic tension, natural posture, refined facial features, relaxed mood, filmic grain, low-saturation premium color grading, full-body or half-body framing, clean uncluttered frame, authentic human texture, candid dance-capture feel ## NEGATIVE PROMPT (append to every shot, required) deformed limbs, distorted hands or feet, warped face, motion smear, compression artifacts, st',
 'builtin', now(), 2980),

(NULL, 'social-media-post-travel-snapshot-collage-prompt', '社交媒体帖子 - 旅行快照拼贴提示词', '创建12格怀旧风格智能手机旅行照片拼贴的详细提示词，展现独自旅行的回忆。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A 12-frame collage of candid, emotional snapshots of a young {argument name="ethnicity" default="Chinese"} woman traveling alone in {argument name="location" default="Phuket Island"}, casually captured on a {argument name="device" default="smartphone"}. Each frame feels like a fleeting personal memory — imperfect, sun-drenched, intimate, and unposed. The woman has a naturally curvy figure with a soft, feminine silhouette, subtly emphasizing her bust without exaggeration. Her presence feels real and unstyled, like a private photo album. Scenes include: walking barefoot on the beach, seaside under the strong sunlight, palm trees swaying, overexposed ocean reflections, small local cafés, a modest motel room, sunset the coast, night markets, views from inside a moving car. Shot with a smartphone aesthetic: slight motion blur, soft focus, blown-out highlights from tropical sunlig',
 'builtin', now(), 2990),

(NULL, 'social-media-post-fashion-editorial-collage', '社交媒体帖子 - 时尚编辑拼贴', '生成2x2时尚编辑照片拼贴，专注于一致的造型、特定灯光和参考照片中的面部特征。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Use facial feature of attached photo. 2x2 photo collage of the same woman with long black wavy hair styled in an elegant soft updo with loose wavy strands framing the face, realistic beauty fashion editorial, refined natural glam makeup, defined eyes, soft matte skin, wearing a sleek black spaghetti-strap dress with sheer smoky-black opera gloves draped over the arms, delicate gold necklace, minimalist warm beige studio backdrop, golden hour sunlight streaming through blinds creating vertical shadow lines across face and body, luxurious cinematic atmosphere, no text, no watermark. Top left panel: close-up frontal portrait, arms folded softly forward, direct intense gaze, lips slightly parted, dramatic light stripes across face and shoulders. Top right panel: playful beauty pose, chin resting on sheer gloved hand, head tilted slightly, soft smile, elegant posture, sunlight ba',
 'builtin', now(), 3000),

(NULL, 'social-media-post-editorial-fashion-photography', '社交媒体帖子 - 时尚编辑摄影', '适用于极简工作室场景的时尚氛围提示词，配有柔和光线和暖色调。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：A woman with {argument name="hair color" default="long red hair"} crouching in a minimalist studio setting with a {argument name="background color" default="soft pink background"}. She is wearing a {argument name="dress style" default="fitted black dress"} and black high heels. She holds a lit match in one hand, looking at it thoughtfully, while a small decorated cake with a single lit candle sits on the floor in front of her. The lighting is soft and warm, casting gentle highlights and subtle shadows, creating a moody, editorial atmosphere.',
 'builtin', now(), 3010),

(NULL, 'social-media-post-showa-day-retro-culture-magazine-cover', '社交媒体帖子 - 昭和之日复古文化杂志封面', '温馨编辑风格的日本节日特色页面，结合动漫角色艺术、怀旧的Showa时代街景图像和杂志风格的信息布局，适用于季节性文化推广。', '影音', '图片',
 'image', NULL,
 '{"type":"retro Japanese lifestyle magazine cover poster","theme":"Showa Day feature celebrating nostalgic Japanese retro culture","style":"clean editorial layout mixed with warm anime illustration, soft natural sunlight, nostalgic yet fresh atmosphere, cream paper background, olive green accents, elegant serif and Japanese Mincho typography","aspect_ratio":"3:4 vertical","headline":{"top_tags":"LIFESTYLE / FEATURE / RETRO CULTURE","date_text":"{argument name=\"event date\" default=\"4.29\"} EVENT","main_title":"昭和の日特集","subtitle_ribbon":"懐かしさの中に、新しい発見を。"},"badge":{"position":"top right","shape":"circular date stamp with botanical decoration","text":["4/29","TUE.","祝日"]},"main_text":{"intro_lines":["今日は『昭和の日』です。","昭和という時代を振り返り、","これからの未来について考える日として","制定されました。","レトロな文化や暮らしには、","今見ても魅力的なものが","たくさんあります。"]},"layout":{"sections":[{"title":"main illustration","position":"upper right","count":1,',
 'builtin', now(), 3020),

(NULL, 'social-media-post-cinematic-elevator-scene', '社交媒体帖子 - 电影感电梯场景', '用于生成一个情绪化的电影感场景，展现女性在金属电梯内的写实光影和反射效果。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Inside an elevator, the metal walls have a slight cold reflection, and the ceiling lights are whitish but uneven. The space is enclosed and quiet. A {argument name="subject" default="young Asian girl"} stands in a corner position of the elevator, with a background of slightly distorted mirrors and floor lights.',
 'builtin', now(), 3030),

(NULL, 'social-media-post-social-media-fashion-outfit-generation', '社交媒体帖子 - 社交媒体时尚穿搭生成', '根据角色档案生成一周时尚博主风格的穿搭推荐，包含单品标签和价格信息。', '影音', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Based on this character info card for a {argument name="subject" default="girl"}, generate a 7-day outfit recommendation guide suitable for her appearance, height, and weight. Use a {argument name="platform style" default="Xiaohongshu"} fashion blogger presentation style. Generate 7 images at once (one for each day), specifically labeling the styles and prices of accessories, shoes, hats, pendants, tops, pants, socks, and other items for easy reference.',
 'builtin', now(), 3040),

(NULL, 'image-poster', '图像海报', '用于海报、主视觉和编辑插图的单图生成工具。默认使用 gpt-image-2，但支持通过上游工具切换至 Flux、Imagen 或 Midjourney。输出为保存到项目文件夹的 PNG/JPEG 文件。', '设计', '图片',
 'image', NULL,
 '使用这个插件完成以下任务：Editorial poster for an indie film festival — one bold abstract silhouette over a warm, slightly grainy paper background; hand-set sans serif title at the top, festival dates and venue at the bottom in monospace. Muted ochre + ink palette.',
 'builtin', now(), 3050),

(NULL, '3d-animated-boy-building-lego', '3D动画男孩搭建Lego', '3D动画风格的多镜头视频提示，描绘男孩在房间中仔细组装Lego积木，带有延时摄影效果。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Scene: A boy in a room seriously assembling Lego blocks. The visual style is 3D animation with vibrant colors, smooth lines, full of childlike fun and vitality. A time-lapse effect is added to show the assembly process. Scene: Wide shot of the room, sunlight spilling onto the desk through the window. The boy sits at the desk focused on assembling Lego, with a serious expression. The camera slowly zooms in. Scene: Time-lapse effect showing the boy quickly snapping Lego pieces together, the blocks gradually taking shape in his hands. The camera switches to different angles. Scene: Close-up of hands, showing details of the boy skillfully assembling Lego, fingers moving nimbly. The camera follows the hand movements. Scene: Time-lapse effect continues showing the assembly process. The Lego creation becomes complete, and the boy''s expression changes from focused to satisfied. Scen',
 'builtin', now(), 3060),

(NULL, 'hyperframes', 'Hyperframes', '使用 HyperFrames HTML 创建视频合成、动画、标题卡、叠加层、字幕、配音、音频响应式视觉效果和场景转场。支持构建 HTML 视频内容、添加音频同步字幕、生成语音合成旁白、创建音乐驱动的动画效果（节拍同步、发光、脉冲）、添加动画文本高亮（标记扫描、手绘圆圈、爆发线、涂鸦、素描）以及场景转场效果（淡入淡出、擦除、显示、着色器转场）。', '影音', '视频',
 'video', NULL,
 '用高端产品工作室的完成度，为 {{subject}} 创建一个 {{duration}} 秒的 {{format}} HyperFrames 作品：动态排版利落、转场优雅、运动语言克制，整体有工作室级别的节奏和完成度。画幅：{{aspect}}。视觉风格：{{style}}。音频/字幕：{{audioPlan}}。使用 HyperFrames HTML 工作流、确定性时间线和引用的运动指南。',
 'builtin', now(), 3070),

(NULL, 'video-hyperframes', 'Hyperframes 视频脚本', 'Hyperframes / Remotion 兼容的连续帧动画, 可自动播放', '影音', '视频',
 'video', NULL,
 '用「Hyperframes 视频脚本」模板把我的内容做成一段「Hyperframes / Remotion 兼容的连续帧动画, 可自动播放」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3080),

(NULL, 'frame-nyt-graph', 'NYT 图表', '《纽约时报》社论风格的数据图表动效，克制的排版与坐标轴逐步浮现，适合数据叙事。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「NYT 图表」HyperFrames 模板创建一段约 15 秒的data-viz动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3090),

(NULL, 'frame-data-chart-nyt', 'NYT 风数据图表帧', 'NYT-newsroom 排版 + 错峰揭示动画 + 编辑级图表 (折线/柱/范围带)', '影音', '视频',
 'video', NULL,
 '用「NYT 风数据图表帧」模板把我的内容做成一段「NYT-newsroom 排版 + 错峰揭示动画 + 编辑级图表 (折线/柱/范围带)」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3100),

(NULL, 'frame-data-chart-nyt-video', 'NYT 风数据图表帧', 'NYT-newsroom 排版 + 错峰揭示动画 + 编辑级图表 (折线/柱/范围带)', '影音', 'HyperFrames',
 'video', NULL,
 '用「NYT 风数据图表帧」模板把我的内容做成一段「NYT-newsroom 排版 + 错峰揭示动画 + 编辑级图表 (折线/柱/范围带)」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3110),

(NULL, 'cinematic-street-racing-sequence-for-seedance-2', 'Seedance 2 电影级街头赛车序列', '为 Seedance 2 设计的多镜头提示词，生成夜间电影级街头赛车场景，聚焦驾驶员专注度、动态镜头运动和爆发式加速效果。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：cinematic street racing sequence at night, a focused driver inside a high-performance car grips the steering wheel, intense eye focus, city lights reflecting on windshield, tension building before sudden acceleration camera: rapid multi-angle system with seamless transitions, interior close-up → over-the-shoulder → exterior tracking → low ground shots, ultra dynamic camera movement, whip pans + speed ramp transitions + motion blur masking cuts, continuous flow illusion (0-2s) interior close-up on driver, hand tightens on gear shift, subtle breathing, dashboard lights glowing (2-4s) over-the-shoulder shot, road ahead stretching into neon-lit city, engine vibration building (4-6s) extreme close-up on finger pressing NOS button, instant ignition reaction (6-8s) explosive acceleration, camera snaps to exterior side tracking shot, car launches forward with violent speed surge (8-',
 'builtin', now(), 3120),

(NULL, 'seedance-2-0-15-second-cinematic-japanese-romance-short-film', 'Seedance 2.0：15秒电影感日式恋爱短片', '为 Seedance 2.0 设计的详细多场景提示词，可生成电影级超逼真的日本高中纯爱短片。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：15-second cinematic Japanese drama pure love ambiguous short film, ultra-realistic quality, warm golden sunlight in an empty classroom in the afternoon, spilling through the blinds onto the side-by-side desks, fine dust motes slowly floating in the light beams, old wooden desks, extremely natural subtle movements, breathing, and eye tension, characters maintain consistent faces, clothing, and hairstyles throughout without deformation, drift, or artifacts, real slight chest rise and fall synchronized with breathing, shallow depth of field, creamy blurred background, warm film grain, 8K sharp, Japanese youth restrained heart-fluttering suffocating atmosphere. 0-4 seconds: Extremely slow push-in shot from a medium shot of the desktop to a close-up of the two people''s side profiles sitting side-by-side. A pure girl in a summer school uniform is focused on writing notes with her ',
 'builtin', now(), 3130),

(NULL, 'seedance-2-0-80-year-old-rapper-mv', 'Seedance 2.0：80岁说唱歌手MV', '为Seedance 2.0生成详细的15秒提示词，用于制作16:9横版街头说唱MV，主角是一位80岁的女性，采用霓虹紫蓝冷色调风格。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：16:9 horizontal screen, street rap MV style, neon purple and blue cool tones, explosive cool and fierce atmosphere. 0-3 seconds: Medium shot push-in, city street night scene with flashing neon lights, an 80-year-old silver-haired woman stands in front of a graffiti wall, short silver-white hair styled in a neat slick-back, distinct square face contour, sword-like eyebrows slanting towards the temples, eyes sharp like electricity, wrinkles at the corners of her eyes like badges of time, a confident smile on the corner of her mouth, wearing a black leather jacket over a white printed T-shirt (large black letters "YOLO" on the chest) + black cargo pants + white high-top sneakers, a thick gold chain necklace around her neck, silver bracelet on her wrist, holding up a microphone with both hands, strong drum beats of the BGM start, the old woman''s eyes sharpen, and her lips open t',
 'builtin', now(), 3140),

(NULL, 'vfx-text-cursor', 'VFX 文字光标', '光标拖光 + 彩色像散射线 + 定向光斑, 适合视频片头逐字揭示金句', '影音', '视频',
 'video', NULL,
 '用「VFX 文字光标」模板把我的内容做成一段「光标拖光 + 彩色像散射线 + 定向光斑, 适合视频片头逐字揭示金句」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3150),

(NULL, 'vfx-text-cursor-video', 'VFX 文字光标', '光标拖光 + 彩色像散射线 + 定向光斑, 适合视频片头逐字揭示金句', '影音', 'HyperFrames',
 'video', NULL,
 '用「VFX 文字光标」模板把我的内容做成一段「光标拖光 + 彩色像散射线 + 定向光斑, 适合视频片头逐字揭示金句」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3160),

(NULL, 'video-seedance-desk-hologram-ar-realdesk', 'Video - Desk Hologram AR on Real Workspace (Seedance 2.0)', 'A two-phase Seedance 2.0 workflow for the viral desk-hologram format: first lock a photoreal keyframe on a real developer desk (Open Design UI on the monitor, a cobalt-blue extraterrestrial hunter sta', '影音', '视频',
 'video', NULL,
 'Two-phase production workflow for a ~15 second vertical desk-hologram AR clip (9:16, Seedance 2.0). Phase A locks the still; Phase B animates it. Use the same hero design in both phases. # Phase A — Image keyframe (gpt-image-2, image-to-image) Upload or generate a {argument name="desk_photo" default="vertical iPhone photo of a lived-in real developer desk at night: floral desk mat, white mechanical keyboard, ASUS monitor with a screen bar, Mac mini, tangled USB-C/HDMI cables, sticky notes, pens, tissues, subtle monitor glow on the desk surface — NOT a clean 3D render"}. Image edit prompt (i2i): Keep the real desk, monitor, keyboard, and clutter exactly as photographed. Add a {argument name="hero" default="cobalt-blue extraterrestrial hunter with bioluminescent cyan spots, long dark braided hair, pointed ears, amber eyes, lean athletic build, leather loincloth and minimal tribal gear, a l',
 'builtin', now(), 3170),

(NULL, 'frame-macos-notification', 'macOS 通知横幅', '拟真 macOS 通知 banner + app icon + 标题正文, 适合 video overlay / 产品发布预告', '影音', '视频',
 'video', NULL,
 '用「macOS 通知横幅」模板把我的内容做成一段「拟真 macOS 通知 banner + app icon + 标题正文, 适合 video overlay / 产品发布预告」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3180),

(NULL, 'cinematic-east-asian-woman-hand-dance', '东亚女性电影级手部舞蹈', '一个包含多镜头时间码指令的精细电影风格手部舞蹈视频提示词，涵盖镜头运动和角色动作。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：1 0-3s Extreme close-up of the face, exquisite and three-dimensional features, cold and elegant eyes locked on the lens, sword dance opening pose: hands quickly swipe from both sides of the cheeks to a fixed point in front of the chest, clean fingertip movements. 15-second vertical screen 9:16, 24fps, 8K ultra-high definition, realistic movie texture, stable screen without flicker. Top-tier East Asian young female, exquisite features, delicate and transparent skin with natural luster, clear and bright atmosphere makeup, distinct hair strands. Cold and confident gaze locked on the lens throughout, hands quickly swiping from cheeks to chest, clean sword dance hand gestures, clear fingertip details. Soft ring light, soft facial light and shadow without dead blacks, clear and bright eye light, camera moves forward slightly at a uniform speed, subject always in the center of the ',
 'builtin', now(), 3190),

(NULL, 'frame-takram-organic', '东方柔和有机帧', '东方柔和有机帧:毛玻璃圆角卡 + 曲线连接描入 + 放射节点弹出 + 柔和漂浮, 米色自然色调', '影音', 'HyperFrames',
 'video', NULL,
 '用「东方柔和有机帧」模板把我的概念做成一段柔和科技感放射节点图:毛玻璃圆角卡 + 曲线连接描入 + 节点围绕进化核心放射弹出。保持模板的视觉签名,使用真实内容,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3200),

(NULL, 'frame-product-promo', '产品宣传', '多镜头产品宣传动效合成，场景依次切换，适合功能展示与发布短片。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「产品宣传」HyperFrames 模板创建一段约 20 秒的product-demo动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3210),

(NULL, 'frame-product-promo-30s', '产品宣传 · 30秒', '30 秒多场景产品宣传流程，保留完整的镜头节奏，替换文案即可出片。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「产品宣传 · 30秒」HyperFrames 模板创建一段约 30 秒的product-demo动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3220),

(NULL, 'traditional-dance-performance', '传统舞蹈表演', '为 Seedance 2.0 提供完整的视频提示词，根据编舞和身份参考图像生成优雅的传统舞蹈。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Use the first reference image as the exact choreography and motion-process guide. Use the second reference image as the identity reference for the adult woman dancer. Create a graceful traditional dance performance that follows all 16 illustrated steps in order, from the confident opening pose to the respectful closing pose. The dancer performs with poised posture, soft knee bends, precise cross steps, elegant wrist waves, curved fingers, shoulder accents, hip sways, flowing turns, and expressive selendang sweeps. Keep the camera in a clean full-body cinematic frame, mostly front-facing, with slow controlled movement that supports the dance instead of distracting from it. During the left and right turns, allow a subtle circular camera drift, then return to a centered frontal composition. The dancer’s hands, feet, facial expression, and selendang fabric must remain visible th',
 'builtin', now(), 3230),

(NULL, 'frame-decision-tree', '决策树', '清爽的决策树 / 流程图动效合成，节点与连线依次绘入，适合讲清判断逻辑与分支流程。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「决策树」HyperFrames 模板创建一段约 15 秒的explainer动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3240),

(NULL, 'frame-creative-voltage', '创意电压分屏帧', '创意电压分屏帧:电光蓝/暗错位分屏滑入 + 描边电光词 + 手写 script 自描, 复古现代有活力', '影音', 'HyperFrames',
 'video', NULL,
 '用「创意电压分屏帧」模板把我的标题做成一段能量感分屏揭示:电光蓝与暗色面板错位滑入 + 标题升起带一个描边电光词 + 手写体自描入。保持模板的视觉签名,使用真实内容,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3250),

(NULL, 'frame-kinetic-type', '动态排版', '大胆的动态排版动效，适用于有冲击力的宣传标题、口号与开场。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「动态排版」HyperFrames 模板创建一段约 15 秒的presentation动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3260),

(NULL, 'animation-transfer-and-camera-tracking-prompt', '动画迁移和相机跟踪提示词', '为 Seedance 2.0 提供的技术提示词，将特定动作参考应用到角色上，同时保持固定的相机跟踪。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：apply the walking animation of @anim exactly as it is to @char7. the camera tracks the character exactly in place, camera angle does not change',
 'builtin', now(), 3270),

(NULL, 'a-decade-of-refinement-glow-up', '十年蜕变焕新颜', 'Seedance 2.0 转换提示词，展示男性角色从 2016 年休闲风格到 2026 年迪拜奢华生活方式的转变，同时保持角色一致性。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Create a 15-second ultra-realistic cinematic transformation video using the exact same man from the uploaded reference image. Maintain perfect face consistency, same hairstyle, facial features, identity, and body proportions throughout. No face change. Concept: “2026 is the new 2016” nostalgia-to-luxury glow-up. Scene 1: 2016 version — simple casual clothes, basic hairstyle, walking alone on a normal street, warm nostalgic colors, old Instagram aesthetic, simple life, no luxury. Scene 2: Flashback cuts — old bike ride, cheap café alone, late-night dreams, city lights, silent ambition in his eyes. Scene 3: Strong transition — speed-ramp effect, screen crack cinematic transition, time shifts from 2016 to 2026, luxury watch appears, black suit transformation begins. Scene 4: 2026 version — walking confidently in Dubai downtown, luxury black suit, sunglasses, expensive watch, bl',
 'builtin', now(), 3280),

(NULL, 'ancient-indian-kingdom-fpv-video', '古印度王国FPV视频', '快节奏FPV无人机风格的电影化提示词，描绘神秘的印度王国、寺庙和丛林场景。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：extremely fast-paced cinematic FPV flying through the ancient Indian Dandaka kingdom, dense mystical forests, towering sal and teak trees, tribal settlements, ancient ashrams, sages meditating, wildlife moving through fog, dramatic sunlight rays piercing canopy, rivers cutting through rugged terrain, ruined temples covered in vines, hyper-realistic textures, high-speed aerial dives and sharp turns, immersive depth, volumetric lighting, earthy tones, epic scale, realism, cinematic color grading, smooth stabilization, ultra-detailed environment, intense atmosphere',
 'builtin', now(), 3290),

(NULL, 'frame-logo-outro', '品牌 Logo 收尾帧', 'Logo 分块组装入场 + glow bloom + tagline 揭示, 适合视频片尾 / 品牌闭幕', '影音', '视频',
 'video', NULL,
 '用「品牌 Logo 收尾帧」模板把我的内容做成一段「Logo 分块组装入场 + glow bloom + tagline 揭示, 适合视频片尾 / 品牌闭幕」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3300),

(NULL, 'frame-logo-outro-video', '品牌 Logo 收尾帧', 'Logo 分块组装入场 + glow bloom + tagline 揭示, 适合视频片尾 / 品牌闭幕', '影音', 'HyperFrames',
 'video', NULL,
 '用「品牌 Logo 收尾帧」模板把我的内容做成一段「Logo 分块组装入场 + glow bloom + tagline 揭示, 适合视频片尾 / 品牌闭幕」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3310),

(NULL, 'retro-hk-wuxia-film-aesthetic', '复古港式武侠电影美学', '复刻80-90年代香港武侠片美学风格的复杂多段视频提示词，呈现角色从猫到人的变身过程及风格化镜头。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：core_style: 80s-90s Shaw Brothers film style, early Hong Kong Wuxia drama aesthetics, nostalgic Chinese Wuxia movies, vintage TV quality, warm tones with high saturation palette, retro film grain texture, slight Technicolor chromatic aberration, classic studio backdrop feel, soft stage lighting. visual_quality: 35mm film photography, physical film defects, vintage film texture, subtle chromatic dispersion, soft focus effect, slight light flicker, strong bloom on highlight surfaces. character_modeling: female_character - classic 80s period drama makeup, black eyeliner, peach-pink lip balm, exquisite braids with pink ribbons and flower accessories, traditional light blue and white Hanfu with floral embroidery and silk texture. male_character - classic Wuxia young scholar appearance, long hair tied with a white ribbon at the waist, signature sideburns, clean-shaven face, pure w',
 'builtin', now(), 3320),

(NULL, 'vintage-disney-style-pirate-crocodile-animation', '复古迪士尼风格海盗鳄鱼动画', '经典复古迪士尼风格动画的多场景叙事提示，讲述鳄鱼海盗和鸟类海盗在船上的故事。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Classic vintage Disney animation style. Scene 1: On a pirate ship sailing on the sea, a fat and sinister crocodile pirate stands at the end of a plank. Three bird pirates are watching on the ship, while another bird pirate stands at the beginning of the plank, pointing a sword at the crocodile pirate. The crocodile pirate is wearing a sleeveless tight suit and is muscular. He looks at the bird pirates provocatively. Although he is a bit overwhelmed, he does not intend to give up. The bird pirates look at the crocodile pirate with evil smiles. The atmosphere is tense, and the bird pirates hope the crocodile pirate will jump off the plank into the sea below. Scene 2: The bird pirate says it''s all over. He slams the plank with his claw, and the plank shakes. The crocodile pirate loses his balance and falls off the plank. Scene 3: The bird pirates let out evil laughter, but then',
 'builtin', now(), 3330),

(NULL, 'nightclub-flyer-atmospheric-animation', '夜店传单氛围动画', '为 Seedance 2.0 提供的细微动画提示词，使背景和光照元素生动起来，同时保持主体静止', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Hyper-detailed nightclub event flyer, a Black woman with sleek long braids and bold red lipstick photographed in a confident chin-up power pose looking directly at camera, wearing a fitted black blazer with gold chain, dramatic red rim light from behind creating glowing edge, harder key light from front-left. Subject cut out over deep oxblood-red textured background with heavy film grain and subtle smoke haze. Behind her the massive display word "FRIDAY" in tall condensed slab serif, cream-white with grungy distressed edges, partially occluded by her shoulders and hair. Secondary script tagline "the night belongs to you" in elegant gold cursive. Bottom info block: "NOV 14 · 10PM · DOORS AT 9 · UPTOWN CLT". Scattered accents: small gold sparkle stars, one circled date stamp, thin gold scribble line. Palette strictly oxblood red, cream-white, gold, deep black. Warm cinematic g',
 'builtin', now(), 3340),

(NULL, 'frame-bold-signal', '大胆信号卡帧', '大胆信号卡帧:暗渐变底 + 大编号 + 导航面包屑 + 橙色卡片滑入 + 标题升起, 高冲击力', '影音', 'HyperFrames',
 'video', NULL,
 '用「大胆信号卡帧」模板把我的章节做成一段大胆色卡分隔:大编号 + 导航面包屑 + 鲜橙卡片从右滑入 + 标题升起。保持模板的视觉签名,使用真实内容,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3350),

(NULL, 'frame-bold-poster', '大胆海报帧', '大胆海报帧:1970s 欧洲社论海报风 + 番红强调色 + 巨型倾斜 Shrikhand 大字 + 三行标题逐行升起 + 衬线斜体副题, 印刷质感强。', '影音', 'HyperFrames',
 'video', NULL,
 '用「大胆海报帧」模板把我的开场做成杂志封面感:mono kicker + 红线划过 + 巨型倾斜大字 + 三行标题逐行升起(中间行番红)+ 衬线斜体副题。保持印刷海报的视觉签名,使用真实内容,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3360),

(NULL, 'frame-build-minimal', '奢华极简留白帧', '奢华极简留白帧:单词逐字浮现 + 暖金细线 + 呼吸感细线指示器, 70%+ 留白', '影音', 'HyperFrames',
 'video', NULL,
 '用「奢华极简留白帧」模板把我的品牌词做成一段奢华极简留白 hero:超细字重单词逐字浮现 + 暖金细线 + 呼吸感指示器。保持模板的视觉签名,使用真实内容,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3370),

(NULL, 'hollywood-haute-couture-fantasy-video-prompt', '好莱坞高级定制时尚视频提示词', '为 Seedance 2.0 设计的多场景视频生成提示词，用于创建好莱坞高级定制时尚风格影片，支持 8K 分辨率和 Unreal Engine 渲染。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：[Style] Hollywood Haute Couture Fantasy blockbuster, 8K ultra-clear, Photorealistic, High-fashion Editorial Style, Unreal Engine 5 fluid rendering, visual illusion. [Duration] 15 seconds. [Scene] An endless, real-life Salar de Uyuni (Sky Mirror) salt flat. The sky is filled with oppressive dark clouds, and the ground perfectly reflects everything like a mirror, with the overall picture presenting a minimalist, cool tone. [00:00-00:05] Shot 1: Haute Couture Entrance and Porcelain Skin. Camera position: Extremely low-angle upward shot, ultra-telephoto lens zoom-in. Action: An Asian female model with a highly recognizable, high-fashion face walks coolly on the water surface. Effect: She is wearing not fabric, but a long dress made of flowing, real Liquid Blue-and-White Porcelain. As she walks, the skirt makes a crisp collision sound like real ceramic, with a flowing luster on t',
 'builtin', now(), 3380),

(NULL, 'frame-glitch-title', '故障艺术标题帧', '数字故障 / 像散偏移 / 数据腐败标题, 适合视频转场 / cyberpunk hero', '影音', '视频',
 'video', NULL,
 '用「故障艺术标题帧」模板把我的内容做成一段「数字故障 / 像散偏移 / 数据腐败标题, 适合视频转场 / cyberpunk hero」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3390),

(NULL, 'frame-glitch-title-video', '故障艺术标题帧', '数字故障 / 像散偏移 / 数据腐败标题, 适合视频转场 / cyberpunk hero', '影音', 'HyperFrames',
 'video', NULL,
 '用「故障艺术标题帧」模板把我的内容做成一段「数字故障 / 像散偏移 / 数据腐败标题, 适合视频转场 / cyberpunk hero」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3400),

(NULL, 'frame-data-rollup', '数据滚动帧', '数据滚动帧:原生 Remotion 数据动画 — 柱子按真实数据用 spring 从 0 长上去、数字同步从 0 滚到目标值。静态 HTML 图表做不出的''数字活起来''效果。', '影音', 'HyperFrames',
 'video', NULL,
 '用「数据滚动帧」把我的每周指标做成柱状图:柱子从 0 长上去、数字滚动到真实数值。喂真实数据,柱子控制在几根以内。这是 Remotion 增强的数据帧,动画要让数字有''活起来''的感觉。',
 'builtin', now(), 3410),

(NULL, 'frame-warm-grain', '暖色颗粒', '暖色胶片颗粒质感的 Hero 动效，柔和光晕与缓慢推进，适合品牌开场。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「暖色颗粒」HyperFrames 模板创建一段约 10 秒的presentation动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3420),

(NULL, 'sequence-and-movement-instruction-for-martial-arts-video', '武术视频的动作序列指令', '为 Seedance 2.0 提供的视频提示词，根据角色表指导模型生成特定动作和步骤的动画序列。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：[STYLE] Monochrome grayscale illustration, 3D-rendered character, clean instructional reference sheet, white background, comic-style cell grid layout, technical diagram aesthetic. [LAYOUT] 4×4 grid layout with a total of 16 panels. Each panel is separated by thin black border lines. Cells are numbered from 1 to 16, with consistent panel sizes. [CHARACTER] image1 (the same character appears consistently in all panels) [PANEL STRUCTURE – per cell] Top-left: bold number badge + English title text Center: full-body character pose illustration Bottom-left: English description text (3–4 lines) Overlay: directional arrows indicating movement [ARROWS / MOTION INDICATORS] Curved arrows, straight arrows, and circular rotation indicators placed around the character to show motion flow and direction. [RENDERING STYLE] Highly detailed 3D sculpted style, soft studio lighting, subtle shado',
 'builtin', now(), 3430),

(NULL, 'frame-liquid-bg-hero', '流体背景 Hero 帧', 'WebGL 风流体置换背景 + 顶部叠加金句, 适合视频片头 / landing hero / 海报', '影音', '视频',
 'video', NULL,
 '用「流体背景 Hero 帧」模板把我的内容做成一段「WebGL 风流体置换背景 + 顶部叠加金句, 适合视频片头 / landing hero / 海报」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3440),

(NULL, 'frame-liquid-bg-hero-video', '流体背景 Hero 帧', 'WebGL 风流体置换背景 + 顶部叠加金句, 适合视频片头 / landing hero / 海报', '影音', 'HyperFrames',
 'video', NULL,
 '用「流体背景 Hero 帧」模板把我的内容做成一段「WebGL 风流体置换背景 + 顶部叠加金句, 适合视频片头 / landing hero / 海报」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3450),

(NULL, 'crimson-horizon-sci-fi-cinematic-sequence', '深红地平线科幻电影序列', '科幻电影《深红地平线》的完整9镜头电影视频序列，涵盖从火箭发射到火星上诡异外星人遭遇的所有场景。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：SHOT 1: Cinematic wide angle format — rocket launching into night sky, city lights below, clouds parting, stars above. Dark, dramatic, photorealistic. SHOT 2: Medium two-shot format — man and woman in astronaut suits inside a dark capsule, Mars glowing red through the porthole behind them. Cinematic, intimate. SHOT 3: Dramatic aerial format — descent capsule burning through Mars atmosphere, heat shield glowing orange, red desert surface rushing up below. Hyperrealistic. SHOT 4: Ultra wide low angle format — two astronauts standing with backs to camera on Mars surface, vast red desert and amber sky stretching before them. Empty, eerie. SHOT 5: Extreme close-up format — female astronaut''s gloved hand tracing ancient carved symbols on a canyon wall, helmet lamp lighting the carvings, her eyes wide with fear. SHOT 6: Tight close-up format — male astronaut''s arm display glowing r',
 'builtin', now(), 3460),

(NULL, 'soul-switching-mirror-magic-sequence', '灵魂互换镜像魔法序列', '描述镜前魔法灵魂互换事件的叙事视频提示，包含各片段的镜头指导和情感提示。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：0s – 4s (Opening Mystery) A hidden magical kingdom under heavy rain at night. The girl stands beside a glowing water mirror in an ancient palace courtyard. Blue magical fog surrounds her. The water reflects faint glowing runes. She slowly reaches toward the reflection. Camera: Slow push-in close-up, cinematic depth of field Mood: Mysterious, calm tension, soft rain sounds 4s – 8s (Soul Switch Event) Her fingers touch the water reflection suddenly the surface shatters into glowing blue energy. A magical pulse spreads. Her eyes flash with light as her soul violently switches with the crown prince. Camera: Fast magical shockwave transition, close-up eye zoom VFX: Soul transfer glow, water turning into floating light particles Mood: Intense, dramatic awakening 8s – 12s (Living as the Prince) Now inside the crown prince’s body (her consciousness), she stands in a royal throne roo',
 'builtin', now(), 3470),

(NULL, 'toaster-rocket-jumpscare', '烤面包机火箭惊吓', '生成老人被弹射面包的烤面包机吓到的家庭录像风格真实画面提示词。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：A realistic shot of an old man in a cozy kitchen being jumpscared when his toaster launches the bread five feet into the air like a rocket. Handheld "home video" style capturing his genuine look of shock and the bread hitting the ceiling.A realistic shot of an old man in a cozy kitchen being jumpscared when his toaster launches the bread five feet into the air like a rocket. Handheld "home video" style capturing his genuine look of shock and the bread hitting the ceiling.',
 'builtin', now(), 3480),

(NULL, 'frame-play-mode', '玩乐模式', '活泼俏皮的短视频动效，明快的色彩与弹跳节奏，适合社交短片与轻量预告。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「玩乐模式」HyperFrames 模板创建一段约 10 秒的social-shorts动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3490),

(NULL, 'modern-rural-aesthetics-healing-short-film-video-prompt', '现代田园美学治愈短片视频提示词', '为 Seedance 2.0 生成现代田园美学风格治愈短片的详细三镜头提示词，包含电影商业风格、4K/8K、极致微距等技术规格。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：[Style] Modern Rural Aesthetics, Cinematic Commercial quality, shot with Sony A7S3/cinema camera, 4K/8K ultra-clear, Extreme Macro, natural transparent lighting, healing ASMR, no historical costume drama feel. [Scene] A well-maintained modern farmhouse open kitchen, background is a lush vegetable garden, bright sunshine. [Character] Modern Rural Creator, black long hair casually tied up with a wooden hairpin, wearing a dark blue comfortable linen outfit, clear makeup, focused and peaceful eyes. [Shot Details] [00:00-00:05] Shot 1: Morning Harvest (The Freshness) Visuals: High-definition close-up. Morning sunlight hits the plants with side backlighting. Action: The Creator''s bare hands (long, clean fingers) pick a bright red tomato with glistening dew drops from the vine. Details: Extremely sharp focus, clearly showing the fuzz on the tomato surface and the trajectory of slid',
 'builtin', now(), 3500),

(NULL, 'frame-swiss-grid', '瑞士网格', '瑞士国际主义网格排版动效，严谨的栏位与无衬线大字，适合企业与发布场景。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「瑞士网格」HyperFrames 模板创建一段约 10 秒的presentation动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3510),

(NULL, 'frame-pentagram-stat', '瑞士网格数据帧', '瑞士网格数据帧:巨大数字锚点 + 红色强调 + 生长条形图 + 黑色数据底栏, 理性克制的编辑风', '影音', 'HyperFrames',
 'video', NULL,
 '用「瑞士网格数据帧」模板把我的关键指标做成一段瑞士风数据揭示:巨大数字锚点 + 红色强调 + 生长条形图 + 黑色数据底栏。保持模板的视觉签名,使用真实数字,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3520),

(NULL, 'frame-electric-studio', '电光工作室分屏帧', '电光工作室分屏帧:白/蓝双屏从中心开合 + 强调条生长 + 引言逐行浮现, 高对比专业感', '影音', 'HyperFrames',
 'video', NULL,
 '用「电光工作室分屏帧」模板把我的引言做成一段双屏开合揭示:白色上屏 + 电光蓝下屏从中心开合 + 接缝处黑色强调条生长 + 引言逐行浮现。保持模板的视觉签名,使用真实内容,避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3530),

(NULL, 'cinematic-birthday-celebration-sequence', '电影感生日庆祝序列', '用于生成生日视频序列的详细多镜头提示词，注重角色一致性和情感叙事。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：0s–4s Close-up of a young girl waking up in a softly lit bedroom, warm golden sunlight through curtains, she gently smiles while checking her phone filled with birthday wishes, natural makeup, SAME facial features as reference image, cinematic lighting, shallow depth of field, ultra-realistic, 4K 4s–8s Cut to a cozy, beautifully decorated room with balloons and fairy lights, her friends surprise her with a birthday cake, everyone cheering, she laughs happily, SAME face as reference image, joyful expressions, cinematic camera movement, vibrant colors, soft glow, high detail 8s–12s Her boyfriend enters — a well-dressed young man with neatly styled dark hair, sharp jawline, warm expressive eyes, wearing a clean elegant outfit (white shirt with a fitted blazer), minimal accessories, charming and calm presence ,he presents a beautiful bouquet of fresh flowers, she looks surprised',
 'builtin', now(), 3540),

(NULL, 'cinematic-emotional-face-close-up', '电影级情绪面部特写', '为 Seedance 2.0 提供高度细致的技术提示，专注于逼真的皮肤纹理和复杂的面部情绪转换系列。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：A realistic human face with highly detailed skin texture, pores, and micro-musculature. Scene: Tight portrait close-up against a dark, void-like background. Style: Cinematic realism, 35mm film aesthetic, shallow depth of field with soft bokeh, moody and introspective. Lighting: Dynamic emotional lighting that shifts in color temperature and direction to match the internal state. Audio: Ambient atmospheric drone, soft rhythmic breathing, subtle emotional orchestral swells. Avoid: Identity drift, jitter, distorted limbs, unnatural morphing artifacts. [0-3s] Camera: Slow, imperceptible push-in. Action: The face breaks into a genuine, soft smile; eyes crinkle at the corners and the cheeks lift. Lighting: Warm golden-hour glow, soft and frontal. Vfx: Subtle lens flare. [3-6s] Camera: Static extreme close-up. Action: The smile dissolves into a heavy, downward curve; eyes well up w',
 'builtin', now(), 3550),

(NULL, 'cinematic-marine-biologist-exploration', '电影级海洋生物学家探索', '生成水下场景的详细电影级视频提示，展现海洋生物学家在珊瑚礁中发现古代沉船的场景。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：A marine biologist in a sleek wetsuit swims through the vibrant coral reefs of the Great Barrier Reef. At the 3-second mark, he dives deeper to approach an ancient shipwreck. The camera follows him as schools of colorful fish dart around. He retrieves a mysterious artifact from the wreck just as a curious shark glides by. Underwater ruins, coral reef exploration, ancient artifact retrieval, marine life encounter, cinematic underwater lighting, 4K.',
 'builtin', now(), 3560),

(NULL, 'cinematic-route-navigation-guide', '电影级路线导航指南', '为 Seedance 设计的多场景结构化提示词，用于创建具有一致导游角色和真实地点间流畅过渡的步行导航视频。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Create a 5-second cinematic route-guide clip for a walking navigation video. Continuity: This is scene {N} of 5 in a route from North Avenue MARTA Station to Coda Tech Square in Atlanta. The guide is the same stylish female tour guide in every scene: black sunglasses, sleeveless cream belted dress, brown leather belt, tour lanyard, small shoulder bag, brown hair tied back, confident warm expression. She appears on the sidewalk or plaza only, never in traffic lanes. Scene role: {route_step} Starting frame: Use the supplied Street View image as the real-world location reference. Preserve the recognizable street layout, building massing, sidewalk direction, signage, and lighting. Action: The guide is already in frame, slightly ahead of the viewer. She turns toward the camera, gestures toward the next walking direction, then begins to lead the viewer forward. Camera: Smooth hand',
 'builtin', now(), 3570),

(NULL, 'cinematic-dragon-interaction-flight', '电影级龙与人互动与飞行', '分镜脚本风格的视频提示词，展现女性与龙的情感互动及电影级飞行场景。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：STYLE Handheld + aerial camera blend Soft motion blur (only during fast transitions) Teal–orange cinematic grade Cool tones during dragon moments, warm tones at emotional peak ⏱ TIMELINE (15s) 0–2s (HOOK) Close-up on woman standing at a cliff Wind moving through hair A giant shadow passes over her → she slowly turns Low rumble builds tension 2–5s (CONNECTION) Dragon lands behind her with heavy presence It lowers its head slowly She hesitates, then touches its face Wind + dust particles react subtly Quiet emotional moment (no aggression) 5–8s (TAKEOFF) She climbs onto its back Dragon launches powerfully into the sky Camera follows upward, slight rotation Clouds rush past, strong sense of speed 8–12s (FLIGHT SEQUENCE) Fast but controlled cuts: Flying through clouds Passing mountain peaks Close-up of wings moving Her expression shifting to awe Wide aerial shot showing scale 12–',
 'builtin', now(), 3580),

(NULL, 'cinematic-music-podcast-and-guitar-technique', '电影音乐播客与吉他技巧', '用于生成4K音乐播客视频的高级电影级提示词，专注于吉他技巧、泛音和录音室美学。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：**Cinematic Truth Source & Setup** Professional music podcast video production, shot on Sony FX6 cinema camera in 4K DCI, anamorphic lenses with natural breathing and subtle flare, controlled studio lighting using ARRI Skypanels and practical LED backlights, clean broadcast color science with warm highlights and rich mid-tones exactly like high-end Netflix music documentaries. Realistic 24fps motion, light film grain, zero stylization. ** Image Reference & Legend** No external image reference supplied. Original generation locked to user character tagged @character on frame 0. Exact black electric guitar (Stratocaster style with whammy bar) must remain 100% consistent in shape, color, and wear. Back wall behind character locked with large professional podcast branding text “StudioName" in bold modern sans-serif font, subtly backlit with soft neon glow. No deviation allowed on',
 'builtin', now(), 3590),

(NULL, 'cinematic-vampire-alley-fight-sequence', '电影风格吸血鬼巷战打斗场景', '用于短片场景的综合动作提示词，包含霓虹灯照明小巷中的动态镜头运动和高速格斗。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Draev stands in the center of the neon-lit alley, surrounded by multiple vampires positioned on rooftops and street level. The vampires attack simultaneously. Draev reacts instantly with superhuman speed. He dodges the first attacker with a fast sidestep and counters with a brutal punch, sending the vampire crashing into a wall. Second vampire lunges from above Draev jumps unnaturally high, grabs him mid-air, and slams him into the ground. Impact creates a small shockwave on the wet street. The defeated vampire rapidly disintegrates into ash and particles. Camera moves dynamically: starts frontal wide shot transitions into fast tracking side movement then switches to over-the-shoulder following Draev More vampires rush in. Draev performs a fast acrobatic kick hitting two enemies at once. One vampire is thrown into neon signs, sparks and electricity burst. Another gets grabbe',
 'builtin', now(), 3600),

(NULL, 'viral-k-pop-dance-choreography', '病毒式K-pop舞蹈编舞', '为Seedance 2.0提供详细提示词，根据16格分镜参考动画角色表演舞蹈。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：viral kpop dance. Monochrome grayscale illustration, 3D-rendered character, clean instructional reference sheet, white background, comic-style cell grid layout, technical diagram aesthetic. [LAYOUT] 4×4 grid layout with a total of 16 panels. Each panel is separated by thin black border lines. Cells are numbered from 1 to 16, with consistent panel sizes. [CHARACTER] image1 (the same character appears consistently in all panels) [PANEL STRUCTURE – per cell] Top-left: bold number badge + English title text Center: full-body character pose illustration Bottom-left: English description text (3–4 lines) Overlay: directional arrows indicating movement [ARROWS / MOTION INDICATORS] Curved arrows, straight arrows, and circular rotation indicators placed around the character to show motion flow and direction. [RENDERING STYLE] Highly detailed 3D sculpted style, soft studio lighting, su',
 'builtin', now(), 3610),

(NULL, 'live-action-anime-adaptation-water-vs-thunder-breathing-duel', '真人版动漫改编：水之呼吸与雷之呼吸对决', '生成动漫风格真人对决场景的详细提示词，展现「水之呼吸」（蓝色水龙）与「雷之呼吸」（金色闪电）的 15 秒战斗画面。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Live-Action Anime Adaptation · Breathing Technique Decisive Battle (15 seconds · Super Burning Special Effects Version) 【Core Focus】: Water Breathing (Blue Water Dragon) VS Thunder Breathing (Golden Lightning), live-action extreme speed duel. 【Style】: Hollywood live-action anime adaptation film quality, dark samurai style, 4K ultra-clear, extreme fast cuts, explosive particle light effects, no gore. 【Duration】: 15 seconds 【Scene】: Misty forest under the moonlight, muddy ground, falling leaves. [00:00-00:05] Shot 1: Water Melody Prelude · Starting Stance (Sense of charging) Visuals: A young samurai wearing a green and black checkered haori (jacket), lowering his center of gravity under the moonlight, gripping his sword with both hands. Action: He takes a deep breath, and the surrounding air instantly solidifies. As he draws his sword, a giant blue water dragon, condensed from',
 'builtin', now(), 3620),

(NULL, 'forbidden-city-cat-satire', '紫禁城猫讽刺剧', 'Seedance 2.0 深色喜剧提示词，以橘猫官员和鬣狗皇帝为主角的清朝讽刺题材。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：STORY FORMAT: 15s / 150 BPM / MULTI-CUT / American dark comedy with exaggerated imperial satire / slapstick timing and punchline ending TONE: tense accusation → rising absurdity → chaotic reveal → shameless comedic payoff SETTING: Grand hall of the Forbidden City, massive golden throne room, rich red and gold tones, dramatic lighting, echoing atmosphere, ceremonial yet absurd CHARACTERS: Orange cat official: wearing Qing dynasty court robes and an official hat, a long orange queue braid trailing behind, belly comically round as if hiding something, cautious and visibly nervous Hyena emperor: dressed in extravagant Qing imperial robes with a golden crown, domineering presence, easily irritated, dramatic temper White rabbit maids: wearing Qing palace maid outfits, purple eyeshadow, bright red lips, each holding feather fans, fanning the emperor in synchronized rhythm Gray rabb',
 'builtin', now(), 3630),

(NULL, 'frame-vignelli', '维涅里', '维涅里风格的竖版强字幕动效，厚重的无衬线大字与强对比，适合社交竖屏。', '影音', 'HyperFrames',
 'video', NULL,
 '使用「维涅里」HyperFrames 模板创建一段约 10 秒的social-shorts动效视频。把示例文案替换为真实内容，保留模板的动效签名，并渲染为 MP4。',
 'builtin', now(), 3640),

(NULL, 'frame-light-leak-cinema', '胶片漏光电影帧', '胶片漏光 + 颗粒噪点 + 16:9 letterbox + 衬线大字, 电影感开场 / 章节卡', '影音', '视频',
 'video', NULL,
 '用「胶片漏光电影帧」模板把我的内容做成一段「胶片漏光 + 颗粒噪点 + 16:9 letterbox + 衬线大字, 电影感开场 / 章节卡」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3650),

(NULL, 'frame-light-leak-cinema-video', '胶片漏光电影帧', '胶片漏光 + 颗粒噪点 + 16:9 letterbox + 衬线大字, 电影感开场 / 章节卡', '影音', 'HyperFrames',
 'video', NULL,
 '用「胶片漏光电影帧」模板把我的内容做成一段「胶片漏光 + 颗粒噪点 + 16:9 letterbox + 衬线大字, 电影感开场 / 章节卡」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3660),

(NULL, 'beat-synced-outfit-transformation-dance', '节拍同步换装变身舞蹈', '适用于 Seedance 2.0 的提示词，让角色跟随分解帧跳舞的同时进行节拍同步的服装变换。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Have the character from Image 1 perform the dance based on the breakdown in Image 3. During the performance, include a beat-synced transformation into the character from Image 2. After the transformation, the character from Image 2 continues and completes the remaining dance steps from Image 3. Emphasize precise beat matching with the music',
 'builtin', now(), 3670),

(NULL, 'wasteland-factory-chase', '荒漠工厂追逐', '用于生成荒漠高速追逐场景的电影级提示词，包含移动的工业腿式工厂和反叛摩托追击。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Ultra-realistic desert horizon. A gigantic industrial factory moving on mechanical legs crosses the wasteland like a living city. Female rebel riding a fast bike toward it. Scrap armor forms from metal debris. Defense drones launch from the machine. Camera chases beside hoverbike at high speed. She jumps from bike onto a drone, smashes it, lands on the walking factory. Final frame: towering machine blocks the sun.',
 'builtin', now(), 3680),

(NULL, 'video-seedance-three-kingdoms-lyubu-yuanmen-archery', '视频 - 三国 ARPG - 吕布辕门射戟（Seedance 2.0）', '约10秒游戏引擎内过场动作片段，吕布在两军阵营间的军营中央拉开红漆长弓，凝神瞄准后射出一支金色真气箭矢射向远处插在地上的方天画戟。专为 Seedance 2.0 调校，包含严谨镜头控制、弓箭物理效果、风沙旗帜动态和游戏截图色调。配套同名静态图模板使用。', '影音', '视频',
 'video', NULL,
 'A ~10 second in-engine cinematic ARPG action sequence, photoreal, Unreal-Engine-5-grade render quality, desaturated filmic color grading, shallow depth of field on the background. Strict in-game camera discipline — one continuous beat, no cuts, no swish pans, no overlays drawn on top of the frame. # Scene (reference image: {argument name="reference_image" default="game-screenshot-three-kingdoms-lyubu-yuanmen-archery"}) - Setting: {argument name="environment" default="a dusty late-Han military encampment (辕门) between two facing armies, earthen ramparts and wooden palisades framing a wide parade ground, tattered red and black war banners fluttering on tall poles, distant rows of spearmen in lacquered leather armor standing in formation, low golden-hour sun cutting horizontal god rays through drifting dust"}. - Hero (center of frame, stays identical from first to last frame): {argument name',
 'builtin', now(), 3690),

(NULL, 'video-seedance-three-kingdoms-zhaoyun-cradle-escape', '视频 - 三国 ARPG - 赵云救阿斗 (Seedance 2.0)', '约12秒游戏引擎内渲染的动作场景，展现赵云在长坂坡战场左臂怀抱幼主阿斗、右手持枪格挡来袭并跃过战车的经典片段。配套同名截图模板使用。', '影音', '视频',
 'video', NULL,
 'A ~12 second in-engine cinematic ARPG action sequence, photoreal, Unreal-Engine-5-grade render quality, desaturated filmic color grading, shallow depth of field on the background. Strict in-game camera discipline — one continuous beat, no cuts, no whip-pans. # Scene (reference image: {argument name="reference_image" default="game-screenshot-three-kingdoms-zhaoyun-cradle-escape"}) - Setting: {argument name="environment" default="the Changban (长坂坡) battlefield in the late afternoon, a broken rural road cutting across rolling yellow-earth hills, overturned wooden war-chariots and broken shield walls scattered across the ground, thick drifts of battlefield smoke and dust, Cao Cao''s distant cavalry visible as a dark wave of spears and banners on the far ridge, low amber sun cutting through the haze"}. - Hero (center of frame, stays identical from first to last frame): {argument name="hero" de',
 'builtin', now(), 3700),

(NULL, 'video-seedance-three-kingdoms-guanyu-slaying-yanliang', '视频 - 三国ARPG - 关羽斩颜良（Seedance 2.0）', '约10秒的引擎内动作过场动画，配套《三国关羽斩颜良》截图模板。关羽骑赤兔马冲入敌阵，挥动青龙偃月刀斩杀颜良。为Seedance 2.0优化，使用金色气劲特效表现击杀瞬间，无血腥画面。', '影音', '视频',
 'video', NULL,
 'A ~10 second in-engine cinematic ARPG action sequence, photoreal, Unreal-Engine-5-grade render quality, desaturated filmic color grading, shallow depth of field on the background. Strict in-game camera discipline — one continuous beat, no cuts, no swish pans. # Scene (reference image: {argument name="reference_image" default="game-screenshot-three-kingdoms-guanyu-slaying-yanliang"}) - Setting: {argument name="environment" default="a broad open battlefield at dawn, dry yellow earth churned by thousands of hooves, low ground mist clinging to the field, distant spear forests on both sides, tattered war banners fluttering, a cold teal sky with warm amber sunrise light from the left"}. - Hero (center-left of frame, stays identical from first to last frame): {argument name="hero" default="Guan Yu (关羽), a towering red-faced general in ornate green-lacquered lamellar armor with a long flowing be',
 'builtin', now(), 3710),

(NULL, 'character-intro-motion-graphics-sequence', '角色介绍动态图形序列', '为 Seedance 2.0 模型设计的复杂多阶段动态图形提示词，用于介绍团队角色并添加特定 UI 叠加层和转场效果。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Based on the three characters in the reference images. High definition, Unreal Engine rendering, cinematic quality, candy-colored palette, Japanese-style aesthetics, artistic, with strong sense of rhythm. 0–2s: Empty scene, a small dot at the center, thin-line UI frame, subtle particles. Text: “STATUS: STANDBY” “SYSTEM: INIT”. 2–4s: The fox on the left from image2 appears, riding a hovering skateboard, waves toward the camera. Curved motion trails behind. Text: “ID: 01” “CODENAME: RED” “ROLE: TACTICIAN”. 4–6s: The rabbit on the right from image1 appears, swings a carrot weapon and takes a combat stance. Circular motion trails. Text: “ID: 02” “CODENAME: KANA” “ROLE: EXECUTIONER” “WEAPON: CARROT”. 6–8s: The corgi from image3 appears, looks left and right, showing a simple, friendly smile. Concentric circle UI under its feet. Text: “ID: 03” “CODENAME: Arthur” “ROLE: COMMANDER”.',
 'builtin', now(), 3720),

(NULL, 'luxury-supercar-cinematic-narrative', '豪华超跑电影叙事', '适用于 Seedance 2.0 的多镜头电影提示词，包含时尚男士、杜宾犬和复古超跑，场景设定在雾气弥漫的山区。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Global Intent: Quiet Luxury with an aggressive edge. A stylish man with Dobermans and a classic dark blue vintage supercar journeys through misty mountains to an epic coast. Deep, saturated color palette: dark blue, matte black, foggy gray. The pacing is driven by a slow, heavy trap beat with deep 808 bass, featuring rhythmic cinematic cuts. SEQUENCE LIST: SHOT 1 (0-1.5s) Medium Shot • camera_motion: push in • core_action: Front of a modern matte black house. A stylish man in effortlessly expensive dark clothing stands motionless, holding three perfect Dobermans on thick leather leashes. Behind them sits a classic dark blue vintage supercar. confident movement, grounded interaction, authentic human behavior patterns. Audio: Quiet engine idling, trap beat intro. (CUT TO) SHOT 2 (1.5-2.5s) Extreme Close-Up • camera_motion: static shot • core_action: Macro of the man''s face. He',
 'builtin', now(), 3730),

(NULL, 'cyberpunk-game-trailer-script', '赛博朋克游戏预告片脚本', '生成赛博朋克游戏预告片的详细视频提示词，包含角色设计、UI动画和从白色虚空到贫民窟的环境转场。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：INK INDUSTRIES : GAME TRAILER CHARACTERYoung athletic male, dark curly hair, shirtless, full chest and back tattoos, gold hoop earring, cigarette in mouth, black cybernetic prosthetic arms with cyan LED nodes at joints. Black shorts with white stripe, white socks, beige chunky sneakers. Seated cross-legged on white void floor. CINEMATIC SETUPOpens on clean white void environment with minimal game UI. Camera high angle looking down at character. Hyper-realistic CGI rendering, clean white background transitioning to dense cyberpunk favela environment. SEQUENCE [0s–15s] [0s–2s] High angle shot looking down. Character seated on white floor, looking up at camera with cigarette smoke rising. Game menu UI on left: START NEW GAME, CONTINUE highlighted, SETTINGS, EXIT GAME. Player profile top right showing INK_NOMAD LVL 23. A cursor clicks CONTINUE. The button pulses. Subtle bass hit',
 'builtin', now(), 3740),

(NULL, 'ancient-guardian-dragon-rescue', '远古守护龙救援', '一个详细的多镜头电影级提示词，讲述雨中村庄少女被巨龙拯救的故事，专注于视觉特效和氛围音效。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：Shot 1 (00:00–00:02) – WS, Rainy Night, Forward Tracking. A narrow, ancient village alley drenched in relentless rain. Water streams down slanted rooftops and floods uneven stone pathways, reflecting flickering lantern light. A young girl runs barefoot through the water, her soaked dress clinging to her as she struggles to keep balance. Behind her, shadowy figures move unnaturally—distorted, stretching and glitching with each lightning flash as they close in. VFX: Heavy rain simulation, reflective wet surfaces, lightning illuminating distorted shadows. SFX: Thunder cracks, rapid splashing footsteps, howling wind. Shot 2 (00:02–00:04) – CU, Panic Fall, Slight Handheld Shake. She suddenly slips and crashes onto the wet stone. Water splashes outward. Close on her face—rain mixes with tears, her breath sharp and uneven. Her trembling hands push against the ground as she tries to',
 'builtin', now(), 3750),

(NULL, 'hunched-character-animation', '驼背角色动画', '指导 Seedance 2 创建特定角色参考的原地行走动画。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：create a walking animation for this hunched over character. the character stays in place',
 'builtin', now(), 3760),

(NULL, 'magical-academy-storyboard-sequence', '魔法学院故事板序列', '用于生成魔法学院场景的详细故事板提示词，包含少女到达学院、觉醒魔法力量和魔法决斗的电影化叙事序列。', '影音', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：0s – 4s (Arrival at the Academy) A massive gothic magical academy appears above floating cliffs, surrounded by storm clouds and glowing runes. The girl walks through the grand iron gates. Her expression is calm but curious. Floating spell books drift around the entrance. Cinematic slow push-in shot, mist and dramatic lighting. 4s – 8s (The Forbidden Core Reveal) Inside a grand hall, students channel elemental magic. The girl stands still as her “sealed magical core” reacts. Dark energy briefly flickers around her chest, but she doesn’t collapse like others would. The academy masters observe in shock. Close-up on her face with glowing rune reflections in her eyes. 8s – 12s (Soul-Binding Lesson) In a floating classroom, chains of light connect students’ souls to magical entities. The girl absorbs a forbidden spell instead of being harmed. The spell dissolves into her body safe',
 'builtin', now(), 3770),

(NULL, 'video-shortform', '短视频', '短视频生成技能 — 3-10 秒产品展示、动态预告和环境循环片段。默认使用 Seedance 2，也兼容 Kling 3 / 4、Veo 3 或 Sora 2。输出单个 MP4 文件至项目文件夹。当工作区同时提供交互视频/超帧技能时，建议将多个短镜头组合成单一时间线，而非一个冗长片段。', '营销', '视频',
 'video', NULL,
 '使用这个插件完成以下任务：5-second product reveal — ceramic coffee mug rotating on a soft paper backdrop, warm side-light from camera-left, micro dust motes drifting through the beam. Cinematic, 16:9, slow drift on the camera.',
 'builtin', now(), 3780),

(NULL, 'frame-flowchart-sticky', '便利贴流程图帧', 'SVG 曲线连接 + 便利贴节点 + 光标交互, 像白板 brainstorm', '运营', '视频',
 'video', NULL,
 '用「便利贴流程图帧」模板把我的内容做成一段「SVG 曲线连接 + 便利贴节点 + 光标交互, 像白板 brainstorm」。保持模板的视觉签名，使用真实内容和数据，避免 lorem ipsum 和占位图片。',
 'builtin', now(), 3790),

(NULL, 'audio-jingle', '音频铃声', '音频生成技能——铃声、背景音、旁白和音效。将音乐请求路由到 Suno V5 / Udio / Lyria，语音转 MiniMax TTS / FishAudio / ElevenLabs V3，音效转 ElevenLabs SFX 或 AudioCraft。输出为保存到项目文件夹的单个 MP3/WAV 文件。', '营销', '音频',
 'audio', NULL,
 '使用这个插件完成以下任务：A 30-second upbeat indie-pop jingle for a coffee shop launch — warm electric piano lead, brushed drums, gentle bass, a single sun-soaked "ahhh" choir on the chorus. No vocals. Loop-friendly tail.',
 'builtin', now(), 3800);
