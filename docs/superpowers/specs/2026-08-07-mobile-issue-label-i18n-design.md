# 移动端状态 / 优先级标签 i18n

日期：2026-08-07
范围：`apps/mobile/`（不含 web / desktop / server）

## 问题

`apps/mobile/lib/issue-status.ts` 把 `STATUS_LABEL` / `PRIORITY_LABEL` 写成硬编码英文常量。mobile 已经有完整的 i18n 运行时（i18next + react-i18next，`locales/{en,zh-Hans}/`），但这两张表绕过了它——所以中文设备上 mobile 显示 "In Progress" / "Todo"，web 和 desktop 显示「进行中」/「待办」。

My Issues 看板上线后这个问题被显著放大：这些标签现在是看板列头和状态条 tab 的文字。

同一份标签在 mobile 里目前有 **四处**副本：

| 位置 | 形态 | 中文设备表现 |
| --- | --- | --- |
| `lib/issue-status.ts` | 硬编码英文常量 | 英文 |
| `lib/format-activity.ts` | 模块私有的英文副本 | 英文 |
| `app/(app)/[workspace]/issues-filter.tsx:39` | 又一份局部 `PRIORITY_LABEL` | 英文 |
| `locales/{en,zh-Hans}/inbox.json` 的 `status_label` / `priority_label` | 已走 i18n，但 **zh-Hans 的值是未翻译的英文** | 英文 |

第四处是调查中新发现的：收件箱行已经接入 i18n，但中文文案从未填写，所以中文用户在收件箱里同样看到 "In Progress"。

`lib/project-status.ts` 对项目状态 / 优先级有完全相同的问题。

## 决策

1. **取值 API**：`lib/issue-status.ts` 保持 React-free 的纯函数，另起文件放 hook。
2. **收件箱副本**：合并到共享 key，删除 `inbox.json` 里的 `status_label` / `priority_label`。
3. **`format-activity.ts`**：整模块 i18n（不只是两张标签表），文案逐字对齐 web。
4. **动词文案**：中英文都照搬 web，同一事件在三端描述完全一致。
5. **日期本地化**：**本次排除**，见下方「明确排除项」。

## 术语来源

所有中文文案逐字取自 `packages/views/locales/zh-Hans/`，不自行发明术语。

`apps/docs/content/docs/developers/conventions.zh.mdx` 是术语表的权威来源。本次涉及的两条硬规则：

- `issue` → **任务**（第 125 行）
- `task`（智能体的一次执行）→ **`task`**，保留小写英文（第 126 行）

第 130 行说明了原因：一个任务下面可以跑多次执行，两者用户都看得到，任何语言都不能写成同一个词。因此 web 的 `"完成了 task（{{count}} 次）"` 是**正确**的，不是缺陷，mobile 逐字照搬。

### ⚠️ 未解决的冲突：issue 状态到底翻不翻

`conventions.mdx:203-211`（中英文版本一致）把 issue 状态列为 schema-level 标识符，规定中文环境也保持小写英文：

> - Issue status: `backlog` / `todo` / `in_progress` / `in_review` / `done` / `blocked` / `cancelled`
> - In UI, surface them in English: "已切换到 in_progress"

根 `CLAUDE.md` 指定 `conventions.mdx` 是 i18n 术语表的 source of truth，并明确要求不要拿 locale 文件当术语表。但 `packages/views/locales/zh-Hans/issues.json` 实际发的是「待办 / 进行中」——**web 自身违反了这条规则**。

本次取舍：**跟 web 一致（中文标签）**。理由是这个任务的目标就是消除三端显示差异；让 mobile 单独合规只会把不一致换个方向。

这条冲突没有被修复，只是被记录。真正的收敛需要人类裁决：要么修 `conventions.mdx`（承认状态标签该翻译），要么另开 PR 把 web/desktop 改成小写英文。在裁决之前，mobile 会跟着 web 走。

不受影响：**优先级**和**项目状态**都不在「不翻」清单里（清单只有角色名和 issue 状态），照搬 web 无争议。

## locale key 布局

### `locales/{en,zh-Hans}/issues.json` —— 新增顶层小节

`status`（7 个）、`priority`（5 个），与 `packages/views/locales/*/issues.json` 同名同结构：

| key | en | zh-Hans |
| --- | --- | --- |
| `status.backlog` | Backlog | 待规划 |
| `status.todo` | Todo | 待办 |
| `status.in_progress` | In Progress | 进行中 |
| `status.in_review` | In Review | 审核中 |
| `status.done` | Done | 已完成 |
| `status.blocked` | Blocked | 已阻塞 |
| `status.cancelled` | Cancelled | 已取消 |
| `priority.urgent` | Urgent | 紧急 |
| `priority.high` | High | 高 |
| `priority.medium` | Medium | 中 |
| `priority.low` | Low | 低 |
| `priority.none` | No priority | 无优先级 |

无冲突：mobile 的 `issues.json` 已有的 `picker_body.status` 和 `activity.run_row.status` 都是嵌套 key，与新的顶层 `status` 不在同一层。

### `locales/{en,zh-Hans}/issues.json` —— 新增 `activity.verb.*`

文案逐字取自 web 的 `activity.*`。**结构上刻意与 web 不同**：mobile 的 `activity` 对象已经有 `section_title` / `agent_row` / `run_row` / `new_chip` / `unread_divider`，把动词平铺进去会和这些界面文案混在一起，所以收进 `verb` 子对象。key 名与 web 保持 1:1（`activity.verb.created` ↔ web 的 `activity.created`），便于对照。

需要的 key：`created`、`self_assigned`、`assigned_to`、`removed_assignee`、`changed_assignee`、`status_changed`、`priority_changed`、`start_date_set`、`start_date_removed`、`due_date_set`、`due_date_removed`、`title_renamed`、`description_updated`、`task_completed_one`/`_other`、`task_failed_one`/`_other`、`squad_leader_evaluated`、`squad_leader_action`/`_reason`、`squad_leader_no_action`/`_reason`、`squad_leader_failed`/`_reason`。

**两个 web 没有的 key**：web 的 zh-Hans 只有 `task_completed_other` / `task_failed_other`，没有 `_one`。mobile 的 `lib/i18n/parity.test.ts` 要求两个语言 key 集合完全一致，且 `apps/mobile/CLAUDE.md` §Pluralization 明确要求 zh-Hans 也带 `_one`（即使 `Intl.PluralRules` 对中文没有 "one" 分类、运行时永不命中）。补：

- `task_completed_one` → `"完成了 task"`
- `task_failed_one` → `"task 失败"`

### `locales/{en,zh-Hans}/issues.json` —— 看板空态

`my_issues.board.empty_column`：en `"Nothing in {{status}}"`，zh-Hans `"暂无{{status}}任务"`。

**与 web 的刻意差异**：web 的 `board.empty_column` 是无插值的 `"No issues"` / `"无任务"`。mobile 的看板是左右滑动的 pager，一屏只有一个状态，说明文字里带上状态名信息量更高。这是 UI 层面的合理差异（`apps/mobile/CLAUDE.md` §Behavioral parity 允许），需在调用点注释说明。

中文措辞用「暂无{{status}}任务」而非「{{status}}中暂无任务」，因为后者在 `in_progress` 上会得到「进行中中暂无任务」。

### `locales/{en,zh-Hans}/projects.json` —— 新增顶层小节

| key | en | zh-Hans |
| --- | --- | --- |
| `status.planned` | Planned | 计划中 |
| `status.in_progress` | In Progress | 进行中 |
| `status.paused` | Paused | 已暂停 |
| `status.completed` | Completed | 已完成 |
| `status.cancelled` | Cancelled | 已取消 |
| `priority.*` | 同 issues | 同 issues |

### `locales/{en,zh-Hans}/inbox.json` —— 删除

删掉 `status_label` 和 `priority_label` 两个小节。`detail-label.tsx` 改用共享 key。

## 取值层

### `lib/issue-status.ts`（改）

**必须保持 React-free**：`lib/board-columns.ts` 从这里 import `BOARD_STATUSES`，而 `lib/board-columns.test.ts` 跑在 Node-only 的 vitest 通道（`vitest.config.ts` 的 `include: ["lib/**/*.test.ts", ...]`，`environment: "node"`）。

- 保留 `BOARD_STATUSES`（它是排序常量，不是标签）
- 删除 `STATUS_LABEL` / `PRIORITY_LABEL`
- 新增：

```ts
type TranslateFn = (key: string, opts?: { defaultValue?: string }) => string;

export function issueStatusLabel(t: TranslateFn, value: string): string;
export function issuePriorityLabel(t: TranslateFn, value: string): string;
```

`t` 需要绑定在 `issues` 命名空间上（JSDoc 注明）。两个函数都用 `{ defaultValue: value }` 兜底未知枚举值。

这是一处**行为升级**而不仅是搬迁：现在的 `STATUS_LABEL[x]` 遇到服务端新增的枚举值返回 `undefined`（渲染成空白），新写法返回原始值。这正是根 `CLAUDE.md` 「API Compatibility」和 `apps/mobile/CLAUDE.md` 「State enums / transitions —— 渲染每一个状态，未知值要有合理兜底」要求的行为，且只需在一处实现，而不是 17 个调用点各写一遍。

### `lib/use-issue-labels.ts`（新）

```ts
export function useIssueLabels(): {
  statusLabel: (value: string) => string;
  priorityLabel: (value: string) => string;
};
```

内部 `useTranslation("issues")` + `useMemo`，返回已绑定 `t` 的两个函数。`useMemo` 保证引用稳定（根 `CLAUDE.md` UI 规则），避免下游 memo 组件无谓重渲染。

hook 单独成文件而非放进 `issue-status.ts`，正是为了守住上面的 React-free 约束。

这个 hook 也是 `detail-label.tsx` 能同时用两个命名空间的关键：该文件保留 `useTranslation("inbox")` 渲染收件箱文案，另外调 `useIssueLabels()` 取共享标签，不需要 `useTranslation(["inbox", "issues"])` 加 `issues:` 前缀那套写法。

### `lib/project-status.ts`（改）+ `lib/use-project-labels.ts`（新）

同构处理：

- 保留 `PROJECT_STATUSES`、`PROJECT_PRIORITIES`、`PROJECT_STATUS_COLOR`、`PROJECT_PRIORITY_BARS`、`projectStatusColor()`、`projectPriorityBars()`
- 删除 `PROJECT_STATUS_LABEL` / `PROJECT_PRIORITY_LABEL`
- `projectStatusLabel(value)` → `projectStatusLabel(t, value)`，`projectPriorityLabel` 同
- `useProjectLabels()` 走 `projects` 命名空间
- 更新文件头注释——现在写的是「i18n lands later when mobile picks an i18n lib」，已经过时

`project-status.ts` 目前没有 Node 通道的测试传递依赖，但仍按同一形状拆分，避免以后有人加 `project-status.test.ts` 时踩到同一个坑。

### `lib/format-activity.ts`（改）

- 删除模块私有的 `STATUS_LABEL` / `PRIORITY_LABEL`
- 签名改为 `formatActivity(entry, resolveActorName, t)`，`t` 绑定 `issues` 命名空间
- 全部约 15 条英文散文迁到 `activity.verb.*`
- 未知 action 仍然落到 `entry.action ?? ""`，绝不抛错、绝不丢行（保持现状）

唯一调用点 `components/issue/activity-row.tsx` 负责传入 `t`。

## 调用点（17 个文件）

路由页（5）：

- `app/(app)/[workspace]/issues-filter.tsx` —— 同时删掉第 39 行那份局部 `PRIORITY_LABEL`
- `app/(app)/[workspace]/search.tsx` —— 同时用到任务状态和项目状态，两个 hook 都要
- `app/(app)/[workspace]/(tabs)/my-issues.tsx`
- `app/(app)/[workspace]/more/issues.tsx`
- `app/(app)/[workspace]/project/new.tsx`

组件（12）：

- `components/issue/issue-board.tsx` —— 状态条 + 空态插值 key；该文件目前完全没有 i18n，是首次引入 `useTranslation`
- `components/issue/attribute-row.tsx`
- `components/issue/create-form-attribute-row.tsx`
- `components/issue/activity-row.tsx` —— 向 `formatActivity` 传 `t`
- `components/issue/pickers/status-picker-body.tsx`
- `components/issue/pickers/priority-picker-body.tsx`
- `components/inbox/detail-label.tsx`
- `components/project/project-row.tsx`
- `components/project/project-properties-section.tsx`
- `components/project/project-related-issues.tsx`
- `components/project/pickers/project-status-picker-body.tsx`
- `components/project/pickers/project-priority-picker-body.tsx`

`search.tsx` 里 `STATUS_LABEL[item.status as IssueStatus] ?? item.status` 这类写法可以简化为 `statusLabel(item.status)`——兜底已内置。

## 测试

| 测试 | 状态 | 作用 |
| --- | --- | --- |
| `lib/i18n/parity.test.ts` | 无需改动 | 只在单个语言里加 key 会自动失败 |
| `lib/issue-status.test.ts` | **新增** | 每个 `IssueStatus` / `IssuePriority` 在 en 和 zh-Hans 两个 bundle 里都能取到值；未知枚举返回原值 |
| `lib/project-status.test.ts` | **新增** | 同上，项目侧对应物。设计阶段没列，实现时补——项目标签有完全相同的漂移风险，缺了就是不对称 |
| `lib/format-activity.test.ts` | **新增** | 该模块目前零覆盖；注入 `t` 后可用假 `t` 在 Node 通道里测 |

`issue-status.test.ts` 补上了这次改动会丢掉的一层保障：原来的 `Record<IssueStatus, string>` 让 TypeScript 保证穷尽，改成 `(t, value: string)` 之后类型层面不再检查。这个测试比原来更强——它同时守住 zh-Hans，能抓到「web 加了新状态、mobile 的 JSON 没跟上」。

不写组件测试：mobile 的 vitest 是 Node-only、没有 RN renderer，`vitest.config.ts` 的注释明确了这条边界。

## 明确排除项

**日期本地化。** `lib/format-activity.ts` 的 `shortDate()` 和 `components/inbox/detail-label.tsx` 的 `shortDate()` 都硬编码 `"en-US"`，所以中文设备上日期仍显示 "Aug 6" 而非「8月6日」。

排除理由：日期本地化要动 `format-activity`、`detail-label` 和截止日期 chip 至少三处，且需要把当前 locale（`i18n.language`）沿调用链传下去——与标签取值是两条独立的改造路径。混在一起会让本次 diff 难以 review。

这是本次改动后**已知的残留缺口**，应作为紧接的后续 PR。

**web 侧文案。** 本次不改 `packages/views/locales/`，避免牵连 web / desktop 的回归面。

## 验证

```
pnpm --filter @multica/mobile typecheck
pnpm --filter @multica/mobile lint
pnpm --filter @multica/mobile test
```

改完后应满足：`grep -rn "STATUS_LABEL\|PRIORITY_LABEL" apps/mobile --include='*.ts' --include='*.tsx'` 无结果。
