# Gallery Native 操作流程：从 Figma 插件到本地 Agent 还原

本文用于说明当前测试链路的最小操作流程，目标是验证：设计稿上传到 Multica 后，可以创建还原任务，并交给本地 Agent 在绑定仓库中生成结构化还原产物。

## 0. 启动本地服务

在项目根目录启动前后端与 daemon：

```bash
make dev
```

如果只需要手动启动当前测试环境：

```bash
# backend
cd server
set -a && source ../.env && set +a
go run ./cmd/server

# frontend
pnpm --filter @multica/web dev --port 3031

# daemon，建议使用当前仓库构建出的 dev binary
cd server
go build -o bin/multica ./cmd/multica
./bin/multica daemon start
```

访问：

```text
http://localhost:3031
```

## 1. 安装 Figma 插件

当前插件目录：

```text
apps/figma-plugin
```

在 Figma 中选择：

```text
Plugins → Development → Import plugin from manifest...
```

选择插件目录中的 `manifest.json`。

插件用于把 Figma/设计稿数据上传到 Multica。上传前确认本地 `.env` 中允许 Figma 来源：

```text
CORS_ALLOWED_ORIGINS=http://localhost:3031,https://www.figma.com,null
```

## 2. 在 Multica 中准备项目

进入 workspace：

```text
http://localhost:3031/amc
```

准备一个测试项目，例如：

```text
项目：exap
```

在项目里可以准备一个测试 Issue，例如：

```text
还原 Gallery Native 测试页面 Hero 区域
```

## 3. 绑定本地仓库资源

本地 Agent 要能落地代码，需要把测试项目绑定到当前仓库。

示例：

```bash
./server/bin/multica project resource add <project-id> \
  --type local_directory \
  --local-path "/Users/fengyujie/Documents/soyoung/multica" \
  --daemon-id "<daemon-id>" \
  --label "本地测试仓库（仅允许 fengchenDoc）" \
  --ref-label "multica-local-fengchenDoc-test" \
  --output json
```

当前测试约定：Agent 只允许写入：

```text
fengchenDoc/gallery-native-agent-test/
```

不要让测试 Agent 直接修改真实业务代码目录。

## 4. 上传设计稿

通过 Figma 插件上传设计稿到 Multica。

上传后在设计库中查看：

```text
/amc/designs/{designFileId}
```

设计文件会保存为：

```text
design_file
design_revision.native_json
design_asset
```

其中 `design_revision.native_json` 是还原的唯一设计数据源。

## 5. 创建 Restore Task

在设计详情页或画板详情页选择目标 frame，保存还原任务。

任务页面形如：

```text
/amc/designs/restore-tasks/{restoreTaskId}
```

当前 restore task 会包含：

```text
design_file_id
revision_id
frame_id
restore item
```

## 6. 派发给 Agent

在 restore task 详情页右侧选择本地 Agent，例如：

```text
Local UI Restore Agent
```

填写可选 Issue ID 和执行提示，然后点击：

```text
交给 Agent / 重新派发
```

系统会创建一个 Agent task。当前此类任务的 kind 为：

```text
design_restore
```

## 7. Agent 还原规则

当前策略为：

```text
strict-structure
```

含义：

- 只能使用 Multica 的 `item_contexts` 作为设计来源。
- 禁止使用外部 `sy-gallery_*` 当前会话作为设计来源。
- 禁止把整张 `frame_preview` / `frame_thumbnail` / full-frame slice 当作还原结果。
- 如果无法结构化还原，应返回 blocked，或生成明确标注的“缺少可结构化 UI 稿”占位页。
- Agent 最终必须输出 `RESTORE_RESULT_JSON`。

示例 JSON：

```json
{
  "status": "completed",
  "summary": "基于 item_contexts 完成结构化还原",
  "files": ["fengchenDoc/gallery-native-agent-test/hero-restore.html"],
  "checks": ["pnpm --filter @multica/views exec tsc --noEmit --pretty false"],
  "blockers": [],
  "restoreMapping": [
    {
      "itemId": "local-xxx",
      "frameId": "frame-1",
      "targetFile": "fengchenDoc/gallery-native-agent-test/hero-restore.html"
    }
  ],
  "usedLayerIds": ["0-659", "0-680"],
  "usedAssetIds": [],
  "usedFullFramePreview": false,
  "policyViolation": ""
}
```

## 8. 查看结果

回到 restore task 详情页查看：

- 执行摘要
- 策略校验
- 变更文件
- 检查命令
- 使用图层
- 使用资产
- Restore Mapping
- 原始 JSON

当前测试产物位置：

```text
fengchenDoc/gallery-native-agent-test/hero-restore.html
```

该文件仅用于本地链路验证，不是业务页面。

## 9. 常见问题

### 1）Agent 说找不到仓库

检查项目是否绑定了 `local_directory` 资源，并且 daemon 正在运行。

### 2）Agent 生成的是整张图片

这是不合格结果。当前服务端会校验：

```text
frame_preview / frame_thumbnail / full-frame slice => failed
```

### 3）Agent 读取到了错误的 Gallery 画板

当前已禁止 `sy-gallery_*` 作为 restore 来源。设计来源必须是 Multica 的 `design_file/revision/item_contexts`。

### 4）前端页面打不开

确认前端端口：

```bash
lsof -ti tcp:3031
```

重新启动：

```bash
set -a && source .env && set +a
pnpm --filter @multica/web dev --port 3031
```
