# Multica Figma Plugin

将 Figma 画板、分组、图层和静态资源上传到 Multica 设计库。

## 安装

1. 解压 `multica-figma-plugin.zip`。
2. 在 Figma Desktop 中打开 `Plugins -> Development -> Import plugin from manifest...`。
3. 选择解压目录中的 `manifest.json`。
4. 运行 `Multica 设计稿上传`。

## 使用

1. 输入 Multica Server 地址，例如 `https://multica.example.com`。
2. 点击登录，在浏览器中完成 Multica 授权。
3. 选择项目、上传类型和 Figma 画板。
4. 点击上传。画板预览、图片填充和切片会先上传到 Multica 配置的 CDN，再保存设计稿。

插件会在本机保存专用上传凭证；退出登录后凭证会被清除。

## 发布文件

Figma 运行时只依赖以下文件：

- `manifest.json`
- `code.js`
- `ui.html`

运行仓库脚本可重新生成安装包：

```bash
./scripts/package-figma-plugin.sh
```
