package projectdesignsystem

import (
	"fmt"
	"html"
	"strings"
)

func buildProgrammaticUIKit(input ProgrammaticInput, theme programmaticTheme) string {
	return buildProgrammaticProductUIKit(input, theme)
}

func buildProgrammaticProductUIKit(input ProgrammaticInput, theme programmaticTheme) string {
	name := html.EscapeString(cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120))
	commit := html.EscapeString(shortProgrammaticCommit(input.CommitSHA))
	providerSource := html.EscapeString(programmaticMappedName(theme.components, "Doctor Card", "doctor", "hospital", "provider"))
	productSource := html.EscapeString(programmaticMappedName(theme.components, "Product Card", "product", "commodity", "coupon"))
	navigationSource := html.EscapeString(programmaticMappedName(theme.components, "Footer Tab", "tab", "nav", "footer", "bottom bar"))
	dialogSource := html.EscapeString(programmaticMappedName(theme.components, "Dialog", "dialog", "popup", "modal", "toast"))
	sourceChips := programmaticSourceChips(theme.components, 10)

	template := `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{NAME}} 在线 UI Kit</title>
  <style>
    * { box-sizing: border-box; }
    html { background: var(--color-surface-muted); }
    body { margin: 0; background: var(--color-surface-muted); color: var(--color-text); font-family: var(--font-family-body); font-size: var(--font-size-body); line-height: var(--line-height-body); }
    button { font: inherit; }
    main { min-height: 100vh; padding: var(--space-6); background: linear-gradient(180deg, var(--color-surface-muted), var(--color-background)); }
    .shell { width: min(1120px, 100%); margin: 0 auto; }
    h1, h2, h3, p { margin: 0; }
    h1, h2, h3 { font-family: var(--font-family-display); line-height: var(--line-height-tight); }
    h1 { margin-top: var(--space-1); font-size: var(--font-size-display); }
    h2 { font-size: var(--font-size-title); }
    h3 { font-size: 16px; }
    .eyebrow, .section-label, .source-note { color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .eyebrow, .section-label { letter-spacing: .08em; text-transform: uppercase; }
    .hero { display: grid; gap: var(--space-4); padding: var(--space-6); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); box-shadow: var(--shadow-card); }
    .hero-row, .section-head, .row, .action-row, .meta, .chips { display: flex; align-items: center; }
    .hero-row, .section-head { justify-content: space-between; gap: var(--space-3); }
    .meta, .chips, .action-row { flex-wrap: wrap; gap: var(--space-2); }
    .pill { display: inline-flex; min-height: 28px; align-items: center; border: 1px solid var(--color-border); border-radius: 999px; padding: 0 var(--space-2); background: var(--color-background); color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .hero-copy { max-width: 720px; color: var(--color-text-muted); }
    section { margin-top: var(--space-8); }
    .section-head { margin-bottom: var(--space-3); }
    .foundation-grid, .component-grid, .state-grid { display: grid; gap: var(--space-3); }
    .foundation-grid { grid-template-columns: 1.05fr .95fr; }
    .component-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .state-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .panel { border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); box-shadow: var(--shadow-card); }
    .panel-body { padding: var(--space-4); }
    .palette { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--space-2); }
    .swatch { min-height: 112px; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
    .swatch-color { height: 68px; }
    .swatch-primary .swatch-color { background: var(--color-primary); }
    .swatch-background .swatch-color { background: var(--color-background); }
    .swatch-muted .swatch-color { background: var(--color-surface-muted); }
    .swatch-text .swatch-color { background: var(--color-text); }
    .swatch-copy { padding: var(--space-1) var(--space-2); background: var(--color-surface); color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .type-sample { display: grid; gap: var(--space-4); min-height: 100%; padding: var(--space-4); }
    .type-display { font-family: var(--font-family-display); font-size: var(--font-size-display); font-weight: 700; line-height: var(--line-height-tight); }
    .type-scale { display: grid; gap: var(--space-2); }
    .type-line { display: grid; grid-template-columns: 72px 1fr; gap: var(--space-2); align-items: baseline; border-top: 1px solid var(--color-border); padding-top: var(--space-2); }
    .type-line span { color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .specimen { overflow: hidden; }
    .specimen-head { display: flex; align-items: start; justify-content: space-between; gap: var(--space-3); padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--color-border); background: var(--color-surface-muted); }
    .specimen-head p { margin-top: 2px; color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .mobile-canvas { position: relative; min-height: 330px; padding: var(--space-3); background: var(--color-surface-muted); }
    .button-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-2); }
    .button { min-height: var(--control-height); display: inline-flex; align-items: center; justify-content: center; gap: var(--space-1); border-radius: var(--radius-md); padding: 0 var(--space-3); font-weight: 600; }
    .button-primary { border: 1px solid var(--color-primary); background: var(--color-primary); color: var(--color-primary-contrast); }
    .button-secondary { border: 1px solid var(--color-primary); background: var(--color-surface); color: var(--color-primary); }
    .button-disabled { border: 1px solid var(--color-border); background: var(--color-surface-muted); color: var(--color-text-muted); opacity: .62; }
    .button-loading::before { width: 12px; height: 12px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; content: ""; }
    .tag-row { display: flex; flex-wrap: wrap; gap: var(--space-1); margin-top: var(--space-3); }
    .tag { display: inline-flex; min-height: 26px; align-items: center; border-radius: 999px; padding: 0 var(--space-2); background: var(--color-background); color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .tag-active { background: var(--color-primary); color: var(--color-primary-contrast); }
    .provider-card, .product-card { border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); padding: var(--space-3); box-shadow: var(--shadow-card); }
    .provider-top { display: grid; grid-template-columns: 56px minmax(0, 1fr); gap: var(--space-3); }
    .avatar { width: 56px; height: 56px; display: grid; place-items: center; border-radius: 50%; background: linear-gradient(145deg, var(--color-primary), var(--color-success)); color: var(--color-primary-contrast); font-size: 20px; font-weight: 700; }
    .verified { color: var(--color-primary); font-size: var(--font-size-caption); }
    .muted { color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .rating { margin-top: var(--space-2); color: var(--color-warning); font-size: var(--font-size-caption); }
    .card-action { margin-top: var(--space-3); display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); border-top: 1px solid var(--color-border); padding-top: var(--space-2); }
    .price { color: var(--color-danger); font-size: 18px; font-weight: 700; }
    .product-image { height: 126px; display: grid; place-items: center; border-radius: var(--radius-md); background: linear-gradient(135deg, var(--color-primary), var(--color-success)); color: var(--color-primary-contrast); font-size: 22px; font-weight: 700; }
    .product-card h3 { margin-top: var(--space-3); }
    .tabs { display: grid; grid-template-columns: repeat(3, 1fr); border-bottom: 1px solid var(--color-border); background: var(--color-surface); }
    .tab { position: relative; padding: var(--space-3) var(--space-1); text-align: center; color: var(--color-text-muted); }
    .tab-active { color: var(--color-primary); font-weight: 700; }
    .tab-active::after { position: absolute; right: 28%; bottom: 0; left: 28%; height: 3px; border-radius: 3px 3px 0 0; background: var(--color-primary); content: ""; }
    .content-list { display: grid; gap: var(--space-2); padding: var(--space-3); }
    .content-row { display: grid; grid-template-columns: 44px 1fr auto; align-items: center; gap: var(--space-2); border-radius: var(--radius-md); background: var(--color-surface); padding: var(--space-2); }
    .thumb { width: 44px; height: 44px; border-radius: var(--radius-md); background: linear-gradient(135deg, var(--color-surface-muted), var(--color-border)); }
    .bottom-actions { position: absolute; right: var(--space-3); bottom: var(--space-3); left: var(--space-3); display: grid; grid-template-columns: .85fr 1.15fr; gap: var(--space-2); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); padding: var(--space-2); box-shadow: var(--shadow-card); }
    .feedback { min-height: 170px; display: flex; flex-direction: column; justify-content: center; gap: var(--space-2); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); padding: var(--space-3); }
    .feedback-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 50%; font-weight: 700; }
    .success .feedback-icon { background: var(--color-surface-muted); color: var(--color-success); }
    .error .feedback-icon { background: var(--color-surface-muted); color: var(--color-danger); }
    .skeleton { display: grid; gap: var(--space-2); }
    .skeleton i { display: block; height: 11px; border-radius: 999px; background: var(--color-surface-muted); }
    .skeleton i:nth-child(2) { width: 72%; }
    .dialog-demo { display: grid; place-items: center; min-height: 220px; background: linear-gradient(135deg, var(--color-surface-muted), var(--color-background)); }
    .dialog { width: min(300px, 92%); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); padding: var(--space-4); box-shadow: var(--shadow-card); }
    .dialog p { margin-top: var(--space-2); color: var(--color-text-muted); }
    .dialog .action-row { justify-content: end; margin-top: var(--space-4); }
    .source-map { display: flex; flex-wrap: wrap; gap: var(--space-1); padding: var(--space-3); }
    .source-chip { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--space-1) var(--space-2); background: var(--color-surface-muted); color: var(--color-text-muted); font-size: var(--font-size-caption); }
    @media (max-width: 760px) {
      main { padding: var(--space-3); }
      .hero { padding: var(--space-4); }
      .hero-row, .section-head { align-items: start; flex-direction: column; }
      .foundation-grid, .component-grid { grid-template-columns: 1fr; }
      .state-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .palette { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .mobile-canvas { min-height: 370px; }
    }
  </style>
</head>
<body>
  <main>
    <div class="shell">
      <header class="hero" data-design-node-id="kit-identity" data-design-node-kind="block" data-design-node-label="设计体系身份">
        <div class="hero-row"><div><span class="eyebrow">Repository UI Kit · Quick draft</span><h1>{{NAME}}</h1></div><div class="meta"><span class="pill">Commit {{COMMIT}}</span><span class="pill">程序化快速生成</span></div></div>
        <p class="hero-copy">根据固定仓库中的颜色、字体、卡片、导航、弹窗和页面结构生成可视化组件状态。下方内容是可直接检查的移动端 UI，不是文件名清单。</p>
      </header>

      <section data-design-node-id="kit-foundations" data-design-node-kind="block" data-design-node-label="基础视觉">
        <div class="section-head"><div><span class="section-label">Foundation</span><h2>色彩与字体</h2></div><span class="pill">375 / 390 px 移动画布</span></div>
        <div class="foundation-grid">
          <div class="panel panel-body"><div class="palette">
            <div class="swatch swatch-primary"><div class="swatch-color"></div><div class="swatch-copy">主操作<br>{{PRIMARY}}</div></div>
            <div class="swatch swatch-background"><div class="swatch-color"></div><div class="swatch-copy">页面背景<br>{{BACKGROUND}}</div></div>
            <div class="swatch swatch-muted"><div class="swatch-color"></div><div class="swatch-copy">弱化表面<br>{{MUTED}}</div></div>
            <div class="swatch swatch-text"><div class="swatch-color"></div><div class="swatch-copy">正文<br>{{TEXT}}</div></div>
          </div></div>
          <div class="panel type-sample"><div><span class="section-label">Typography</span><div class="type-display">清晰可信的移动体验</div></div><div class="type-scale"><div class="type-line"><span>Title · 32</span><strong>服务与内容一目了然</strong></div><div class="type-line"><span>Body · 14</span><p>正文保持稳定行高，中文信息密度紧凑但不拥挤。</p></div><div class="type-line"><span>Meta · 12</span><p class="muted">辅助信息、标签和状态提示</p></div></div></div>
        </div>
      </section>

      <section data-design-node-id="kit-actions" data-design-node-kind="block" data-design-node-label="按钮与标签状态">
        <div class="section-head"><div><span class="section-label">Components 01</span><h2>按钮与标签状态</h2></div><span class="source-note">来源映射：{{NAV_SOURCE}}</span></div>
        <div class="panel panel-body"><div class="button-row">
          <button class="button button-primary" type="button" data-design-node-id="kit-button-primary" data-design-node-kind="component" data-design-node-label="主按钮">立即咨询</button>
          <button class="button button-secondary" type="button" data-design-node-id="kit-button-secondary" data-design-node-kind="component" data-design-node-label="次按钮">查看详情</button>
          <button class="button button-disabled" type="button" disabled data-design-node-id="kit-button-disabled" data-design-node-kind="component" data-design-node-label="禁用按钮">暂不可用</button>
          <button class="button button-primary button-loading" type="button" data-design-node-id="kit-button-loading" data-design-node-kind="component" data-design-node-label="加载按钮">提交中</button>
        </div><div class="tag-row" data-design-node-id="kit-tags" data-design-node-kind="component" data-design-node-label="标签状态"><span class="tag tag-active">推荐</span><span class="tag">皮肤科</span><span class="tag">可预约</span><span class="tag">视频面诊</span></div></div>
      </section>

      <section data-design-node-id="kit-cards" data-design-node-kind="block" data-design-node-label="卡片与信息层级">
        <div class="section-head"><div><span class="section-label">Components 02</span><h2>卡片与信息层级</h2></div><span class="source-note">{{PROVIDER_SOURCE}} · {{PRODUCT_SOURCE}}</span></div>
        <div class="component-grid">
          <article class="provider-card" data-design-node-id="kit-provider-card" data-design-node-kind="component" data-design-node-label="服务者信息卡"><div class="provider-top"><div class="avatar">医</div><div><h3>专业医生姓名 <span class="verified">已认证</span></h3><p class="muted">主任医师 · 三甲医院 · 从业 16 年</p><div class="rating">★★★★★ <span class="muted">4.9 · 1286 条评价</span></div></div></div><div class="tag-row"><span class="tag">面部年轻化</span><span class="tag">皮肤问题</span></div><div class="card-action"><div><span class="muted">在线咨询</span><div class="price">¥39 起</div></div><button type="button" class="button button-primary">去咨询</button></div></article>
          <article class="product-card" data-design-node-id="kit-product-card" data-design-node-kind="component" data-design-node-label="商品与内容卡"><div class="product-image">PRODUCT</div><h3>真实仓库商品与内容卡片</h3><p class="muted">标题、利益点、价格和操作保持单一视觉焦点。</p><div class="card-action"><div><span class="muted">限时活动价</span><div class="price">¥199</div></div><button type="button" class="button button-secondary">立即查看</button></div></article>
        </div>
      </section>

      <section data-design-node-id="kit-navigation" data-design-node-kind="block" data-design-node-label="导航与固定操作">
        <div class="section-head"><div><span class="section-label">Components 03</span><h2>导航与固定操作</h2></div><span class="source-note">来源映射：{{NAV_SOURCE}}</span></div>
        <div class="panel mobile-canvas"><div class="tabs" data-design-node-id="kit-tabs" data-design-node-kind="component" data-design-node-label="粘性标签导航"><div class="tab tab-active">推荐</div><div class="tab">医生</div><div class="tab">案例</div></div><div class="content-list"><div class="content-row"><div class="thumb"></div><div><strong>高频内容标题</strong><p class="muted">关键信息与简短说明</p></div><span class="verified">查看</span></div><div class="content-row"><div class="thumb"></div><div><strong>服务项目标题</strong><p class="muted">价格、标签与状态</p></div><span class="verified">预约</span></div></div><div class="bottom-actions" data-design-node-id="kit-bottom-actions" data-design-node-kind="component" data-design-node-label="固定底部操作栏"><button type="button" class="button button-secondary">电话咨询</button><button type="button" class="button button-primary">立即预约</button></div></div>
      </section>

      <section data-design-node-id="kit-feedback" data-design-node-kind="block" data-design-node-label="反馈与异常状态">
        <div class="section-head"><div><span class="section-label">Components 04</span><h2>反馈与异常状态</h2></div><span class="source-note">来源映射：{{DIALOG_SOURCE}}</span></div>
        <div class="state-grid">
          <article class="feedback success" data-design-node-id="kit-toast-success" data-design-node-kind="component" data-design-node-label="成功反馈"><div class="feedback-icon">✓</div><h3>操作成功</h3><p class="muted">结果已保存，可继续下一步。</p></article>
          <article class="feedback" data-design-node-id="kit-loading-state" data-design-node-kind="component" data-design-node-label="加载状态"><div class="skeleton"><i></i><i></i><i></i><i></i></div><h3>内容加载中</h3><p class="muted">保留稳定页面骨架。</p></article>
          <article class="feedback" data-design-node-id="kit-empty-state" data-design-node-kind="component" data-design-node-label="空状态"><div class="feedback-icon">＋</div><h3>暂无内容</h3><p class="muted">说明原因并提供明确下一步。</p></article>
          <article class="feedback error" data-design-node-id="kit-error-state" data-design-node-kind="component" data-design-node-label="失败状态"><div class="feedback-icon">!</div><h3>加载失败</h3><p class="muted">保留上下文并允许重新尝试。</p></article>
        </div>
        <div class="panel dialog-demo" data-design-node-id="kit-dialog-stage" data-design-node-kind="block" data-design-node-label="弹窗预览"><div class="dialog" data-design-node-id="kit-dialog" data-design-node-kind="component" data-design-node-label="确认弹窗"><h3>确认提交本次信息？</h3><p>提交后仍可在记录中查看处理进度。</p><div class="action-row"><button type="button" class="button button-secondary">取消</button><button type="button" class="button button-primary">确认提交</button></div></div></div>
      </section>

      <section data-design-node-id="kit-source-map" data-design-node-kind="block" data-design-node-label="仓库来源映射"><div class="section-head"><div><span class="section-label">Repository mapping</span><h2>仓库来源映射</h2></div><span class="source-note">只作为证据，不代替可视化组件</span></div><div class="panel source-map">{{SOURCE_CHIPS}}</div></section>
    </div>
  </main>
</body>
</html>`

	return strings.NewReplacer(
		"{{NAME}}", name,
		"{{COMMIT}}", commit,
		"{{PRIMARY}}", html.EscapeString(theme.primary),
		"{{BACKGROUND}}", html.EscapeString(theme.background),
		"{{MUTED}}", html.EscapeString(theme.surfaceMuted),
		"{{TEXT}}", html.EscapeString(theme.foreground),
		"{{PROVIDER_SOURCE}}", providerSource,
		"{{PRODUCT_SOURCE}}", productSource,
		"{{NAV_SOURCE}}", navigationSource,
		"{{DIALOG_SOURCE}}", dialogSource,
		"{{SOURCE_CHIPS}}", sourceChips,
	).Replace(template)
}

func buildProgrammaticPatternsPreview(input ProgrammaticInput, theme programmaticTheme) string {
	return buildProgrammaticProductPatternsPreview(input, theme)
}

func buildProgrammaticProductPatternsPreview(input ProgrammaticInput, theme programmaticTheme) string {
	name := html.EscapeString(cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120))
	firstPattern := html.EscapeString(programmaticPatternName(theme.pagePatterns, 0, "服务者详情"))
	secondPattern := html.EscapeString(programmaticPatternName(theme.pagePatterns, 1, "内容列表"))
	thirdPattern := html.EscapeString(programmaticPatternName(theme.pagePatterns, 2, "状态反馈"))
	template := `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{NAME}} 页面模式</title><style>
*{box-sizing:border-box}html{background:var(--color-surface-muted)}body{margin:0;background:var(--color-surface-muted);color:var(--color-text);font-family:var(--font-family-body);font-size:var(--font-size-body)}main{min-height:100vh;padding:var(--space-6)}h1,h2,h3,p{margin:0}h1,h2,h3{font-family:var(--font-family-display);line-height:var(--line-height-tight)}header{width:min(1120px,100%);margin:0 auto var(--space-6)}header p{margin-top:var(--space-1);color:var(--color-text-muted)}.grid{width:min(1120px,100%);display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:var(--space-4);margin:0 auto}.phone{overflow:hidden;min-height:650px;border:1px solid var(--color-border);border-radius:24px;background:var(--color-background);box-shadow:var(--shadow-card)}.status{height:24px;background:var(--color-surface);border-bottom:1px solid var(--color-border)}.nav{display:flex;align-items:center;justify-content:space-between;min-height:48px;padding:0 var(--space-3);background:var(--color-surface);border-bottom:1px solid var(--color-border)}.back{color:var(--color-primary);font-size:20px}.hero{padding:var(--space-4);background:linear-gradient(135deg,var(--color-primary),var(--color-success));color:var(--color-primary-contrast)}.hero p{margin-top:var(--space-1);opacity:.82}.body{display:grid;gap:var(--space-3);padding:var(--space-3)}.card{border:1px solid var(--color-border);border-radius:var(--radius-lg);background:var(--color-surface);padding:var(--space-3)}.avatar-row{display:grid;grid-template-columns:52px 1fr;gap:var(--space-2)}.avatar{width:52px;height:52px;display:grid;place-items:center;border-radius:50%;background:var(--color-primary);color:var(--color-primary-contrast);font-weight:700}.meta{color:var(--color-text-muted);font-size:var(--font-size-caption)}.tags{display:flex;flex-wrap:wrap;gap:var(--space-1);margin-top:var(--space-2)}.tag{border-radius:999px;padding:3px var(--space-2);background:var(--color-surface-muted);color:var(--color-text-muted);font-size:var(--font-size-caption)}.cta{display:grid;grid-template-columns:.8fr 1.2fr;gap:var(--space-2);margin-top:var(--space-2)}button{min-height:var(--control-height);border-radius:var(--radius-md);font:inherit;font-weight:600}.secondary{border:1px solid var(--color-primary);background:var(--color-surface);color:var(--color-primary)}.primary{border:1px solid var(--color-primary);background:var(--color-primary);color:var(--color-primary-contrast)}.tabs{display:grid;grid-template-columns:repeat(3,1fr);background:var(--color-surface);border-bottom:1px solid var(--color-border)}.tab{padding:var(--space-3) var(--space-1);text-align:center;color:var(--color-text-muted)}.active{color:var(--color-primary);font-weight:700;border-bottom:3px solid var(--color-primary)}.list{display:grid;gap:var(--space-2);padding:var(--space-3)}.item{display:grid;grid-template-columns:58px 1fr;gap:var(--space-2);border-radius:var(--radius-lg);background:var(--color-surface);padding:var(--space-2);box-shadow:var(--shadow-card)}.image{height:58px;border-radius:var(--radius-md);background:linear-gradient(135deg,var(--color-primary),var(--color-success))}.state-stack{display:grid;gap:var(--space-3);padding:var(--space-4)}.state{display:grid;place-items:center;min-height:150px;border:1px solid var(--color-border);border-radius:var(--radius-lg);background:var(--color-surface);padding:var(--space-3);text-align:center}.state strong{display:block;margin-top:var(--space-2)}.state p{margin-top:var(--space-1);color:var(--color-text-muted)}.icon{width:42px;height:42px;display:grid;place-items:center;border-radius:50%;background:var(--color-surface-muted);color:var(--color-primary);font-size:20px;font-weight:700}.source{padding:0 var(--space-3) var(--space-3);color:var(--color-text-muted);font-size:var(--font-size-caption)}@media(max-width:900px){main{padding:var(--space-3)}.grid{grid-template-columns:1fr}.phone{min-height:580px}}
</style></head><body><main data-design-node-id="preview-patterns-root" data-design-node-kind="block" data-design-node-label="页面模式预览"><header><h1>{{NAME}} 代表性页面</h1><p>用同一套 Tokens 和组件状态组合移动端详情、列表与异常状态。</p></header><div class="grid">
<article class="phone" data-design-node-id="preview-provider-detail" data-design-node-kind="block" data-design-node-label="服务者详情页"><div class="status"></div><div class="nav"><span class="back">‹</span><strong>医生主页</strong><span>•••</span></div><div class="hero"><h2>专业服务者详情</h2><p>可信身份、擅长方向与核心转化操作</p></div><div class="body"><div class="card"><div class="avatar-row"><div class="avatar">医</div><div><h3>医生姓名</h3><p class="meta">主任医师 · 三甲医院</p><div class="tags"><span class="tag">已认证</span><span class="tag">可预约</span></div></div></div></div><div class="card"><h3>专业介绍</h3><p class="meta">正文内容保持清晰行高，次要信息使用弱化色。</p></div><div class="cta"><button class="secondary" type="button">电话咨询</button><button class="primary" type="button">立即预约</button></div></div><p class="source">来源模式：{{PATTERN_ONE}}</p></article>
<article class="phone" data-design-node-id="preview-content-list" data-design-node-kind="block" data-design-node-label="内容列表页"><div class="status"></div><div class="nav"><span class="back">‹</span><strong>推荐内容</strong><span>⌕</span></div><div class="tabs"><div class="tab active">推荐</div><div class="tab">项目</div><div class="tab">案例</div></div><div class="list"><div class="item"><div class="image"></div><div><h3>核心内容卡片</h3><p class="meta">标签、说明和状态信息</p><div class="tags"><span class="tag">热门</span><span class="tag">真实案例</span></div></div></div><div class="item"><div class="image"></div><div><h3>服务项目卡片</h3><p class="meta">价格与下一步操作</p><div class="tags"><span class="tag">可预约</span></div></div></div><div class="item"><div class="image"></div><div><h3>用户内容卡片</h3><p class="meta">作者、时间与互动信息</p></div></div></div><p class="source">来源模式：{{PATTERN_TWO}}</p></article>
<article class="phone" data-design-node-id="preview-state-page" data-design-node-kind="block" data-design-node-label="加载空失败状态页"><div class="status"></div><div class="nav"><span class="back">‹</span><strong>页面状态</strong><span></span></div><div class="state-stack"><div class="state"><div><div class="icon">⋯</div><strong>正在加载</strong><p>保持页面结构稳定，避免内容跳动。</p></div></div><div class="state"><div><div class="icon">＋</div><strong>暂无内容</strong><p>解释原因并提供明确下一步。</p></div></div><div class="state"><div><div class="icon">!</div><strong>加载失败</strong><p>保留上下文，允许用户重新尝试。</p></div></div></div><p class="source">来源模式：{{PATTERN_THREE}}</p></article>
</div></main></body></html>`
	return strings.NewReplacer(
		"{{NAME}}", name,
		"{{PATTERN_ONE}}", firstPattern,
		"{{PATTERN_TWO}}", secondPattern,
		"{{PATTERN_THREE}}", thirdPattern,
	).Replace(template)
}

func programmaticMappedName(values []string, fallback string, terms ...string) string {
	for _, term := range terms {
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), term) {
				return value
			}
		}
	}
	return fallback
}

func programmaticPatternName(values []string, index int, fallback string) string {
	if index >= 0 && index < len(values) && strings.TrimSpace(values[index]) != "" {
		return values[index]
	}
	return fallback
}

func programmaticSourceChips(values []string, limit int) string {
	var result strings.Builder
	for index, value := range values {
		if index >= limit {
			break
		}
		fmt.Fprintf(&result, `<span class="source-chip">%s</span>`, html.EscapeString(value))
	}
	if result.Len() == 0 {
		result.WriteString(`<span class="source-chip">暂无稳定共享组件名称</span>`)
	}
	return result.String()
}
