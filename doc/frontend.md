# 前端设计

本文描述 NovelWorkbench 当前前端（`frontend/`）的工程结构、设计系统、状态管理、路由与页面实现。
所有事实以源码为准，引用格式为 `路径:符号`。

## 1. 工程结构

```
frontend/
├── index.html
├── vite.config.ts      别名、/api 代理、manualChunks
├── vitest.config.ts    单元测试配置
├── uno.config.ts       UnoCSS preset-uno + transformer-directives
├── tsconfig.json       strict + noUnusedLocals/Parameters + paths "@/*" -> "src/*"
├── env.d.ts            vite/client 与 virtual:uno.css 声明
├── nginx.conf.template
├── Dockerfile
└── src/
    ├── main.ts         createApp：Pinia / router / i18n / ElementPlus / UnoCSS
    ├── App.vue         el-config-provider + fade-slide 路由过渡
    ├── api/            http.ts（Axios 层）+ contracts.ts + 各业务模块封装
    │   ├── admin.ts        用户 / 角色 / 审计
    │   ├── chapter.ts      章节 / 快照
    │   ├── character.ts    角色 / 关系图
    │   ├── chat.ts         会话 / 历史
    │   ├── continuity.ts   时间事实 / 伏笔 / 章节摘要
    │   ├── knowledge.ts    知识库文档 / 切片 / 检索 / 原文
    │   ├── lore.ts         世界观设定 / 创作笔记
    │   ├── model.ts        模型供应商 / 用量 / 系统状态
    │   ├── outline.ts      大纲节点 / 详细场景
    │   ├── project.ts      项目 / 卷
    │   └── slash.ts        斜杠命令列表
    ├── components/     NWButton / NWCard / NWEmpty / NWLoading / ProjectPageHeader
    ├── composables/    useSSE.ts（+ useSSE.test.ts）
    ├── i18n/           index.ts + locales/{zh-CN,en-US}{,-common,-workspace,-admin,-extensions}.ts
    ├── layout/         AdminLayout.vue + AdminLayout.css
    ├── router/         index.ts（路由表 + 唯一守卫）
    ├── stores/         app.ts / auth.ts / project.ts
    ├── styles/         variables.css / global.css / button-states.css
    ├── utils/          storage.ts（localStorage 封装）
    └── views/          login / dashboard / settings / slash-admin / workflow /
                        micro-app / models / workspace / knowledge / character /
                        lore / notes / outline / continuity / admin
```

### 1.1 页面样式约定

- 单文件接近 500 行时，样式抽到同目录 `*.css`，页面用 `<style scoped src="./page.css"></style>` 引入
  （例：`views/dashboard/index.vue` → `views/dashboard/dashboard.css`；
  `views/outline/index.vue` → `views/outline/outline.css`）。
- 逻辑复杂的页面拆子组件，放在同目录并由父组件组合
  （例：`views/workspace/` 下 `WorkspaceTree.vue` / `ChapterEditor.vue` / `AiPanel.vue` /
  `ChatComposer.vue` / `ChatMessage.vue` / `SnapshotDialog.vue`；
  `views/knowledge/` 下 `KnowledgeUploadPanel.vue` / `KnowledgeSearchPanel.vue` /
  `ChunkListDialog.vue` / `KnowledgePreviewDrawer.vue`）。
- 全部前端源文件 ≤500 行，当前最大为 `views/outline/index.vue`（487 行），其次
  `views/workspace/WorkspaceTree.vue`（476）、`views/character/CharacterGraphTab.vue`（473）、
  `views/dashboard/index.vue`（469）、`layout/AdminLayout.vue`（441）。

### 1.2 构建与测试配置

| 项 | 值 | 来源 |
| --- | --- | --- |
| 别名 | `@` → `./src` | `vite.config.ts:resolve.alias`；`vitest.config.ts` 复用同一别名 |
| dev 端口 | 5173 | `vite.config.ts:server.port` |
| dev 代理 | `/api` → `process.env.NW_DEV_PROXY_TARGET || 'http://127.0.0.1:8080'`，`changeOrigin: true` | `vite.config.ts:server.proxy` |
| 手动分包 | `element-plus`、`echarts`（含 zrender）、`x6`（含 `@antv/*`） | `vite.config.ts:manualChunks` |
| 插件 | `@vitejs/plugin-vue` + UnoCSS | `vite.config.ts:plugins` |
| 测试环境 | `environment: 'node'` | `vitest.config.ts:test.environment` |
| 测试匹配 | `include: ['src/**/*.test.ts']` | `vitest.config.ts:test.include` |
| 构建脚本 | `npm run build` = `vue-tsc && vite build` | `package.json:scripts.build` |
| 测试脚本 | `npm run test` = `vitest run` | `package.json:scripts.test` |

## 2. 设计系统

### 2.1 品牌五色

`src/styles/variables.css` 的 `:root` 定义五个品牌色，其余语义变量全部由它们派生：

| 变量 | 值 |
| --- | --- |
| `--saas_dashboard-color-1` | `#6366f1` |
| `--saas_dashboard-color-2` | `#818cf8` |
| `--saas_dashboard-color-3` | `#e0e7ff` |
| `--saas_dashboard-color-4` | `#f8fafc` |
| `--saas_dashboard-color-5` | `#1e293b` |

### 2.2 派生语义变量

| 组 | 变量 → 值 |
| --- | --- |
| 主色 | `--primary` → `--saas_dashboard-color-1`；`--primary-hover` → color-2；`--primary-light` → color-3；`--primary-active` → `#4f46e5`；`--primary-disabled` → `#c7d2fe` |
| 背景 | `--bg-page` → color-4；`--bg-card` → `#ffffff`；`--bg-hover` → `#f1f5f9` |
| 文字 | `--text-main` → color-5；`--text-secondary` → `#64748b`；`--text-placeholder` → `#94a3b8`；`--text-inverse` → `#ffffff` |
| 边框 | `--border` → `#e2e8f0`；`--border-light` → `#f1f5f9` |
| 错误 | `--error` → `#ef4444`；`--error-hover` → `#dc2626`；`--error-active` → `#b91c1c` |
| 其它 | `--success` → `#10b981`；`--warning` → `#f59e0b`；`--info` → `#3b82f6` |

### 2.3 状态色三元组（浅底 / 文字 / 边框）

| 状态 | 背景 | 文字 | 边框 |
| --- | --- | --- | --- |
| success | `--success-bg` → `#ecfdf5` | `--success-text` → `#047857` | `--success-border` → `#d1fae5` |
| warning | `--warning-bg` → `#fffbeb` | `--warning-text` → `#b45309` | `--warning-border` → `#fef3c7` |
| error | `--error-bg` → `#fef2f2` | `--error-text` → `#b91c1c` | `--error-border` → `#fee2e2` |
| info | `--info-bg` → `#eff6ff` | `--info-text` → `#1d4ed8` | `--info-border` → `#dbeafe` |

这组三元组用于状态徽标与提示条，替代页面内的硬编码色值。

### 2.4 图表系列色

| 变量 | 值 |
| --- | --- |
| `--chart-series-1` | `var(--saas_dashboard-color-1)` = `#6366f1` |
| `--chart-series-2` | `var(--saas_dashboard-color-2)` = `#818cf8` |

实际页面仍以字面量取色，与上述值一致：`views/dashboard/index.vue` 的
`NW_TREND_BAR_COLOR = '#6366f1'` / `NW_TREND_LINE_COLOR = '#818cf8'`；
`views/admin/usage.vue` 的 `NW_CHART_CALL_COLOR = '#6366f1'` / `NW_CHART_TOKEN_COLOR = '#818cf8'`。

### 2.5 阴影 / 圆角 / 动效

| 变量 | 值 |
| --- | --- |
| `--shadow-card` | `0 1px 3px rgba(15,23,42,0.06), 0 4px 12px rgba(15,23,42,0.04)` |
| `--shadow-card-hover` | `0 8px 24px rgba(99,102,241,0.16)` |
| `--shadow-button-hover` | `0 6px 16px rgba(99,102,241,0.25)` |
| `--shadow-button-pressed` | `0 1px 2px rgba(15,23,42,0.12)` |
| `--radius-base` | `8px` |
| `--radius-lg` | `12px` |
| `--radius-pill` | `999px` |
| `--transition-base` | `0.2s ease` |

### 2.6 现状缺口（诚实记录）

- **没有间距刻度**：不存在 `--space-*` 之类的间距变量，所有 padding / margin / gap 均为字面量。
- **没有排版刻度**：不存在字体族 / 字号 token。字体栈与基准字号硬编码在
  `src/styles/global.css` 的 `body` 规则：`'PingFang SC', 'Microsoft YaHei', 'Helvetica Neue', Arial, sans-serif`，
  `font-size: 14px`；其余字号（12/13/16/18/22px 等）在各组件与页面样式中直接写死。

## 3. 按钮五态

统一定义在 `src/styles/button-states.css`，同时覆盖 Element Plus 的 `.el-button` 与自定义的 `.nw-button`。

| 状态 | `.el-button` | `.nw-button` | 动效 |
| --- | --- | --- | --- |
| DEFAULT | `transform: scale(1)`，保留 Element Plus 主题 | `scale(1)`，背景/文字/边框由 `--nw-btn-*` 变量决定 | `transition: all 0.2s ease` |
| HOVER | `scale(1.05)` + `box-shadow: var(--shadow-button-hover)`；文字/边框转 `--primary` / `--primary-hover`；`--primary` 变体转 `--primary-hover`；`--danger` 变体转 `--error-hover` | `scale(1.05)` + 同一 hover 阴影；`--primary` → `--primary-hover`；`--default` → 底 `--primary-light`、字 `--primary`；`--danger` → `--error-hover`；`--text` → 底 `--primary-light` | 平滑放大 |
| PRESSED | `scale(0.95)` + `box-shadow: var(--shadow-button-pressed)`；`--primary` 变体转 `--primary-active`（`#4f46e5`）；`--danger` 变体转 `--error-active`（`#b91c1c`） | `scale(0.95)` + 同一 pressed 阴影；`--primary` → `--primary-active`；`--default` / `--text` → 底 `--primary-light`、字 `--primary-active`；`--danger` → `--error-active` | 平滑缩小 |
| DISABLED | `opacity: 0.5`、`cursor: not-allowed`、`transform: none`、`box-shadow: none`、`transition: none` | 同左（选择器含 `.nw-button--disabled` 与 `[disabled]`） | 无动效 |
| LOADING | `cursor: wait`、`transform: none`、`box-shadow: none`、`pointer-events: none`、`overflow: hidden`；`::before` 亮带自左向右扫过（`nw-btn-shimmer`，1.2s） | 同左；额外渲染 `.nw-button__spinner` 边框旋转圈（`nw-btn-spin`，0.8s）；`--primary` 变体底色为 `--primary-hover` | 持续动画 |

焦点可达性：`.nw-button:focus-visible, .el-button:focus-visible` 统一 `outline: 2px solid var(--primary); outline-offset: 2px`
（`src/styles/global.css`）。

## 4. 基础组件

| 组件 | Props | Slots / 行为 |
| --- | --- | --- |
| `components/NWButton.vue` | `type: 'primary' \| 'default' \| 'text' \| 'danger'`（默认 `default`）、`size: 'large' \| 'default' \| 'small'`（默认 `default`）、`loading: boolean`、`disabled: boolean` | 默认 slot 为按钮文字；`loading` 或 `disabled` 时不派发 `click`，同时置 `disabled` 属性；`loading` 时渲染 `.nw-button__spinner`。变体类名 `nw-button--{type}`、`nw-button--{size}`，另有页面自用的 `nw-button--icon` / `nw-button--tab` 修饰类 |
| `components/NWCard.vue` | `title: string`、`hoverable: boolean`（默认 `true`）、`loading: boolean` | `title` 或存在 `header` slot 时渲染 `.nw-card__header`；`header` slot 优先于 `title`；`loading` 为真时 body 内渲染 `NWLoading`，否则渲染默认 slot |
| `components/NWEmpty.vue` | `description_text: string`（默认空） | 无描述时回落 `t('common.empty')`；内联 SVG 占位图标 |
| `components/NWLoading.vue` | 无 | 旋转圈 + `t('common.loading')` 文案，`role="status"` |
| `components/ProjectPageHeader.vue` | `projectUlid: string` | 显示返回工作台按钮（跳 `/workspace/{projectUlid}`）与项目名；项目名由 `getProjectDetail` 拉取，`projectUlid` 变化时 watch 重载；项目不在 store 列表且列表已加载完时不再发请求（视为已删除/无权），交由布局守卫跳转 |

## 5. 状态与数据流

### 5.1 三个 Store

| Store | State | Getters | Actions | 持久化键 |
| --- | --- | --- | --- | --- |
| `stores/app.ts`（`useAppStore`） | `locale_code`、`sidebar_collapsed` | — | `toggleSidebar()`、`setLocale(localeCode)`（同步 vue-i18n 并持久化） | 语言：`nw_locale`；侧栏折叠：`nw_sidebar_collapsed`（`utils/storage.ts:NW_LOCALE_KEY` / `NW_SIDEBAR_COLLAPSED_KEY`） |
| `stores/auth.ts`（`useAuthStore`） | `access_token`、`user_vo` | `is_logged_in` | `login()`、`register()`、`fetchProfile()`、`logout()`、`resetSession()` | token：`nw_access_token`；用户信息：`nw_user_vo`（JSON，读取时做结构校验，非法回 `null`） |
| `stores/project.ts`（`useProjectStore`） | `project_list`、`current_project_ulid`、`list_loading` | `current_project_vo`、`hasProject(projectUlid)` | `loadProjectList()`（`POST /project/list`，page 1/size 100；当前项不在列表时自动收敛到第一项）、`setCurrentProject(projectUlid)` | `nw_current_project_ulid`（模块内常量 `NW_CURRENT_PROJECT_KEY`） |

### 5.2 Axios 层规则（`src/api/http.ts`）

| 规则 | 实现 |
| --- | --- |
| baseURL | `/api/v1` |
| 超时 | `timeout: 30000`（单接口可覆盖，如知识库上传 `timeout: 120000`） |
| 鉴权注入 | 请求拦截器读 `readToken()`，非空时设 `config.headers.Authorization = 'Bearer ' + token` |
| 包络解包 | 响应拦截器校验 `envelope.resp_code === 'SUCCESS'`，成功后返回 `envelope.data`；`get<T>` / `post<T>` 直接返回 `T` |
| 包络字段 | `{ resp_code, resp_message, trace_id, data }`（`NWResponseEnvelope`） |
| 业务码常量 | `NW_RESP_CODE_SUCCESS='SUCCESS'`、`NW_RESP_CODE_UNAUTHORIZED='ERR_UNAUTHORIZED'`、`NW_RESP_CODE_NOT_FOUND='ERR_NOT_FOUND'` |
| 未授权处理 | `ERR_UNAUTHORIZED` → `clearToken()` + `clearUserVo()` → `ElMessage.error` → 动态 `import('@/router')` 后 `push('/login')` |
| 单次提示 | 统一出口 `rejectWithError` 用 `NWApiError.notified_flag` 保证同一错误只弹一次；`NWRequestConfig.silent_error = true` 时不弹提示 |
| 网络错误 | 无 `error.response` → `resp_code='ERR_INTERNAL'`，`resp_message = t('http.errorNetwork')`，`http_status=0`；有响应但无合法包络 → `resp_code='ERR_INTERNAL'`，`resp_message = axiosError.message` |
| 错误对象 | `NWApiError`（含 `resp_code` / `resp_message` / `trace_id` / `http_status` / `notified_flag`） |
| 辅助 | `isApiNotReadyError(error)`：包络码为 `ERR_NOT_FOUND` 或 `http_status===404` 时为真 |

### 5.3 SSE

`src/composables/useSSE.ts` 不走 axios（axios 不支持流式响应），改用 `fetch` + `ReadableStream`：

- `parseSseBuffer(bufferText)` 为纯函数：只消费以 `\n` 结尾的完整行，`\r` 结尾兼容；`data:` 前缀行解析 JSON，非法 JSON 行跳过；末尾半行留在 `restText` 供下段拼接。
- `streamSsePost(urlPath, body, handlers, signal)`：`POST /api/v1{urlPath}`，带 `Authorization: Bearer`，`TextDecoder` 流式解码；流结束后补一个换行再解析一次，兜住末尾无换行的 `data` 行；`AbortError` 视为主动取消，静默结束，其余异常走 `handlers.onError`。
- `useSSE()` 返回 `{ streamingFlag, startStream, stopStream }`；`startStream` 先中止上一条流。

## 6. 路由

### 6.1 路由表（`src/router/index.ts`）

| path | name | component | `meta.menu_key` | 项目内 |
| --- | --- | --- | --- | --- |
| `/login` | `LoginView` | `views/login/index.vue` | —（无 meta） | 否 |
| `/`（子路由 `''`） | `DashboardView` | `views/dashboard/index.vue` | `dashboard` | 否 |
| `/settings` | `SettingsView` | `views/settings/index.vue` | `settings` | 否 |
| `/slash-admin` | `SlashAdminView` | `views/slash-admin/index.vue` | `slashAdmin` | 否 |
| `/workflow` | `WorkflowView` | `views/workflow/index.vue` | `workflow` | 否 |
| `/micro-app` | `MicroAppView` | `views/micro-app/index.vue` | `microApp` | 否 |
| `/admin/users` | `AdminUsersView` | `views/admin/users.vue` | `adminUsers` | 否 |
| `/admin/roles` | `AdminRolesView` | `views/admin/roles.vue` | `adminRoles` | 否 |
| `/admin/usage` | `AdminUsageView` | `views/admin/usage.vue` | `adminUsage` | 否 |
| `/admin/status` | `AdminStatusView` | `views/admin/status.vue` | `adminStatus` | 否 |
| `/admin/audit` | `AdminAuditView` | `views/admin/audit.vue` | `adminAudit` | 否 |
| `/models` | `ModelProviderView` | `views/models/index.vue` | `models` | 否 |
| `/lore/:project_ulid` | `LoreView` | `views/lore/index.vue` | `lore` | 是 |
| `/workspace/:project_ulid` | `WorkspaceView` | `views/workspace/index.vue` | `workspace` | 是 |
| `/knowledge/:project_ulid` | `KnowledgeView` | `views/knowledge/index.vue` | `knowledge` | 是 |
| `/character/:project_ulid` | `CharacterView` | `views/character/index.vue` | `character` | 是 |
| `/notes/:project_ulid` | `NotesView` | `views/notes/index.vue` | `notes` | 是 |
| `/outline/:project_ulid` | `OutlineView` | `views/outline/index.vue` | `outline` | 是 |
| `/continuity/:project_ulid` | `ContinuityView` | `views/continuity/index.vue` | `continuity` | 是 |

兜底路由 `/:pathMatch(.*)*` 重定向到 `/`。除 `/login` 外全部挂在 `AdminLayout.vue` 之下，组件均为懒加载
`() => import(...)`。

### 6.2 meta 与守卫的实际边界

- `meta` 中**只有** `menu_key` 一个字段，没有 `title`，也没有 `permission`。页面标题文案统一由 i18n 的
  `layout.menu.{menu_key}` 提供。
- 全局**只有一个**路由守卫：`router.beforeEach`（`src/router/index.ts`），只做登录态判断。
  - 目标在 `PUBLIC_PAGE_PATH_LIST = ['/login']` 且本地有 token → 重定向 `/`；
  - 目标非公开页且无 token → 重定向 `/login`；
  - 其余放行。
- 没有基于角色的路由守卫、没有 `meta.permission` 校验。系统管理菜单在 `AdminLayout.vue` 中按
  `isAdminUser`（`(authStore.user_vo?.role_codes ?? []).includes('admin')`）做**隐藏**处理，
  这只是界面可见性控制；真正的权限判定在后端接口侧，前端路由不构成访问边界。

### 6.3 项目上下文同步（位于 AdminLayout，不在路由守卫里）

`src/layout/AdminLayout.vue` 中的 `watch([routeProjectUlid, () => projectStore.list_loading], ...)`：

1. 路由带 `project_ulid` 且与 `projectStore.current_project_ulid` 不一致时，回写 store
   （URL 是项目上下文的最高优先级来源）；
2. 列表仍在加载时等待；
3. 列表中不存在该项目时，若列表为空先 `loadProjectList()` 再判定；仍缺失则
   `ElMessage.warning(t('layout.projectMissing'))` 并 `router.replace('/')`。

## 7. 项目上下文导航

### 7.1 固定的 7 个项目内入口

`AdminLayout.vue` 的 `NW_PROJECT_PAGE_LIST`：

| menu_key | 路径前缀 | 菜单文案 key |
| --- | --- | --- |
| `workspace` | `/workspace/` | `layout.menu.workspace` |
| `outline` | `/outline/` | `layout.menu.outline` |
| `continuity` | `/continuity/` | `layout.menu.continuity` |
| `character` | `/character/` | `layout.menu.character` |
| `lore` | `/lore/` | `layout.menu.lore` |
| `knowledge` | `/knowledge/` | `layout.menu.knowledge` |
| `notes` | `/notes/` | `layout.menu.notes` |

菜单项 index 为 `${path_prefix}${effectiveProjectUlid}`；无当前项目时菜单项禁用。

### 7.2 项目 ULID 优先级

`effectiveProjectUlid = routeProjectUlid || projectStore.current_project_ulid`：

1. **URL 的 `project_ulid` 最高优先**；
2. 其次是 Pinia `projectStore.current_project_ulid`（持久化于 `nw_current_project_ulid`）；
3. 两者都空时项目内菜单禁用。

各项目内页面的取值方式不完全一致：

| 页面 | 取值方式 |
| --- | --- |
| `views/outline/index.vue`、`views/continuity/index.vue` | `route.params.project_ulid || projectStore.current_project_ulid`（路由优先，缺省回退 store） |
| `views/character/index.vue`、`views/lore/index.vue`、`views/knowledge/index.vue`、`views/notes/index.vue`、`views/workspace/index.vue` | 仅取 `route.params.project_ulid` |

### 7.3 路由 → store 回写与切换行为

- 回写：`AdminLayout.vue` 的 watch 在 URL 项目变化时调用 `projectStore.setCurrentProject(projectUlid)`，
  同步写入 `nw_current_project_ulid`。
- 项目切换器（顶栏下拉）：`handleProjectCommand` 调用 `setCurrentProject` 后，
  **停留在同类页面**（当前命中的项目内页面前缀），当前不在项目内页面时进 `/workspace/{ulid}`；
  下拉里的 `dashboard` 命令直接跳 `/`。
- 项目缺失：`/workspace`、`/knowledge` 等页面的项目 ULID 若在列表中不存在，`AdminLayout` 提示
  `layout.projectMissing` 并 `router.replace('/')` 回到仪表盘。

## 8. 页面实现

| 路由 | 页面 | 职责 | 关键集成 |
| --- | --- | --- | --- |
| `/login` | `views/login/index.vue` | 登录 / 注册双 Tab 表单 | `useAuthStore.login/register`；Element Plus `el-form` 校验（密码 ≥6 位、邮箱格式）；`NWButton` 提交态；成功后跳 `/` |
| `/` | `views/dashboard/index.vue` | 统计卡 + 30 天趋势图 + 项目分页管理（新建/编辑/删除） | ECharts 双系列（柱 word_delta + 折线 ai_word_delta），配色 `#6366f1`/`#818cf8`；`GET /stats/overview`；`listProjects/createProject/updateProject/deleteProject`；封面色按书名散列从 6 色调色板取色；删除走 `ElMessageBox.confirm` |
| `/settings` | `views/settings/index.vue` | 运行时配置表展示与逐条保存 | `GET /setting/list` + `POST /setting/update`；每行独立保存按钮，保存中按 `setting_key` 置 loading |
| `/slash-admin` | `views/slash-admin/index.vue` | 内置与自定义斜杠技能列表、提示词模板编辑 | `GET /slash/list`（`enabled_only`）、保存/删除走 `post`；接口未就绪时用 `isApiNotReadyError` 判定并展示降级提示 |
| `/workflow` | `views/workflow/index.vue` | BPMN 创作流程设计器 | `bpmn-js/lib/Modeler`；草稿**只存浏览器**：`localStorage` 键 `nw_workflow_draft_list_v1`；支持新建/载入/保存/导入/导出 BPMN XML，不入库 |
| `/micro-app` | `views/micro-app/index.vue` | 微前端入口登记与按需装载 | `qiankun` 的 `loadMicroApp`（`sandbox: true, singular: true`）；登记表**只存浏览器**：`localStorage` 键 `nw_micro_app_registry_v1`；入口必须为合法 URL；装载/卸载子应用不入库 |
| `/models` | `views/models/index.vue` | 模型供应商卡片列表、新建编辑抽屉、连通性测试 | `listModelProviders/saveModelProvider/deleteModelProvider/testModelProvider/getModelCapability`；协议仅 `openai`/`anthropic`；系统只用对话协议，不接向量模型 |
| `/admin/users` | `views/admin/users.vue` | 用户分页表格、状态与角色标签、新建弹窗、编辑抽屉、重置密码、删除 | `api/admin.ts` 用户接口 + `listAdminRoles` 供角色多选；子组件 `UserFormDialogs.vue` |
| `/admin/roles` | `views/admin/roles.vue` | 角色分页表格、权限多选保存、删除 | `listAdminRoles/saveAdminRole/deleteAdminRole`；内置角色（`builtin_flag`）删除按钮禁用并以 tooltip 说明；权限码可自定义输入，已知项含 `*`、`system:user:manage`、`system:role:manage`、`system:audit:read` |
| `/admin/audit` | `views/admin/audit.vue` | 审计日志筛选与只读分页表 | `listAdminAudits`，按 `action_code` / `user_ulid` 过滤 |
| `/admin/usage` | `views/admin/usage.vue` | 区间选择 + 4 项指标 + 按日趋势 + 按供应商/按模型分布 | `GET /model/usage/overview`；ECharts 双轴（调用量柱状 + token 折线）；区间预设 7/14/30 天 |
| `/admin/status` | `views/admin/status.vue` | 进程信息、依赖健康、AI 能力就绪结论 | `GET /system/status`；依赖项含 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore / 模型；手动刷新，不自动轮询 |
| `/workspace/:project_ulid` | `views/workspace/index.vue` + 子组件 | 三栏写作台：左目录树 / 中章节编辑器 / 右 AI 面板，左右可折叠 | `getProjectTree` + `getChapterDetail`；`WorkspaceTree.vue` 负责卷章增删改与 **XLSX 导出**（`await import('xlsx')` 动态加载）；`ChapterEditor.vue` 用 CodeMirror 6（markdown）+ 2s 防抖自动保存 `POST /chapter/update`，字数用本地 `countWords`（与后端同口径：汉字逐字、其余按空白分词）；`AiPanel.vue` 管理会话与消息流，发送走 SSE `POST /chat/send`；`SnapshotDialog.vue` 管理章节快照与回滚 |
| `/knowledge/:project_ulid` | `views/knowledge/index.vue` + 子组件 | 知识库文档上传、列表、状态徽标、重投索引、删除、切片预览、原文预览、检索 | `KnowledgeUploadPanel.vue`（multipart 上传，扩展名 txt/md/pdf/docx，上限 20MB，上传进度回调）；`KnowledgePreviewDrawer.vue`：txt/md 走切片文本 `<pre>`，docx 走 `docx-preview` 的 `renderAsync`，pdf 走 `vue-pdf-embed`，原文经 `GET /knowledge/content` 取二进制（不走包络解包）；`KnowledgeSearchPanel.vue`：keyword / hybrid 两种检索模式 + `top_k_count` + 命中高亮（按片段切分，不整体 `v-html`）；`ChunkListDialog.vue` 分页看切片；`pending`/`indexing` 状态文档按 3s 轮询刷新，离开页面停止 |
| `/character/:project_ulid` | `views/character/index.vue` + 子组件 | 角色卡片网格（类型/关键词筛选）+ 编辑抽屉 + 删除 | `listCharacters/saveCharacter/deleteCharacter`；`CharacterGraphTab.vue` 关系图用 **AntV X6**，通过 `await import('@antv/x6')` 懒加载，卸载时 `dispose`；关系边走 `saveCharacterRelation/deleteCharacterRelation` |
| `/lore/:project_ulid` | `views/lore/index.vue` | 世界观设定分类（全部/世界规则/地点/物品/事件）+ 关键词搜索 + 卡片列表 + 编辑抽屉 | `saveLore/deleteLore` + `POST /lore/list`；`lore_type_code` 为 `note` 的创作笔记不在本页展示；设定按 `sort_order` 升序 |
| `/notes/:project_ulid` | `views/notes/index.vue` + `NoteEditor.vue` + `MarkdownNoteEditor.vue` | 创作笔记：左列表（标题/更新时间/搜索）+ 右编辑区 | 存储复用 lore，`lore_type_code='note'`（`NW_LORE_TYPE_NOTE`）；标题与正文统一 2s 防抖自动保存 `POST /lore/save`，切换笔记前 `flushSave`；保存状态 idle/saving/saved/failed（失败可重试）；**双模式**：`rich` 用 WangEditor（`@wangeditor/editor-for-vue` 的 `Editor`/`Toolbar`，`shallowRef` 持实例、卸载 `destroy`），`markdown` 用 MavonEditor（`mavon-editor` 的 `mavonEditor` 组件），两个编辑器共享同一字符串契约 |
| `/outline/:project_ulid` | `views/outline/index.vue` | 左大纲节点树（卷/篇章/情节块/章节）+ 右详细场景卡片列表 | `api/outline.ts`：`listOutlineTree/saveOutlineNode/deleteOutlineNode`、`listDetailedScenes/saveDetailedScene/deleteDetailedScene`；节点类型 `volume`/`arc`/`storyBlock`/`chapter`，状态 `draft`/`planning`/`ready`/`done`，场景节奏 `slow`/`medium`/`fast`/`climax`；删除含子节点的行由后端拒绝 |
| `/continuity/:project_ulid` | `views/continuity/index.vue` + `ContinuityDialogs.vue` | 连续性看板三页签：时间事实、伏笔钩子、章节摘要 | `api/continuity.ts`：事实 `candidate/confirmed/superseded/stale`，复核走 `reviewFact`；钩子 `planned/planted/echoed/resolved/abandoned`，重要度 `high/medium/low`；摘要 `pending/verified/stale`；各页签独立筛选与加载态 |

## 9. 可视化与编辑器分工

| 库 | 版本（package.json） | 用途 |
| --- | --- | --- |
| `element-plus` | 2.9.0 | 全局 UI 组件库（表格、表单、弹窗、菜单、下拉、分页、消息）；主题样式在 `main.ts` 引入 `element-plus/dist/index.css`，并由 `styles/button-states.css` 覆写按钮五态 |
| `echarts` | 5.5.0 | 仪表盘 30 天字数趋势（柱+折线）、用量分析按日双轴图 |
| `@antv/x6` | 2.18.1 | 角色关系图（节点 + 边），`await import('@antv/x6')` 懒加载 |
| `bpmn-js` | 16.4.0 | 创作流程设计器（BPMN 建模），引入 `diagram-js.css` 与 `bpmn-font/css/bpmn.css` |
| `qiankun` | 2.10.16 | 微前端宿主装载（`loadMicroApp`） |
| `@codemirror/state` / `view` / `commands` / `language` / `lang-markdown` | 6.7.6 / 6.43.15 / 6.11.1 / 6.13.1 / 6.5.2 | 章节正文编辑器（markdown 语法高亮、历史、缩进） |
| `@wangeditor/editor` + `@wangeditor/editor-for-vue` | 5.1.23 / 5.1.12 | 创作笔记富文本模式 |
| `mavon-editor` | 3.0.2 | 创作笔记 Markdown 模式 |
| `docx-preview` | 0.3.4 | 知识库 docx 原文预览（`renderAsync`） |
| `vue-pdf-embed` | 2.1.7 | 知识库 pdf 原文预览 |
| `xlsx` | 0.18.5 | 写作台目录树导出 Excel（`await import('xlsx')` 懒加载） |
| `axios` | 1.6.8 | HTTP 客户端（`src/api/http.ts`） |
| `pinia` | 2.1.7 | 状态管理（app / auth / project 三个 store） |
| `vue` / `vue-router` / `vue-i18n` | 3.4.34 / 4.3.2 / 9.10.2 | 框架、路由、国际化 |
| `unocss` + `@unocss/preset-uno` + `@unocss/transformer-directives` | 0.58.6 | 原子化工具类与 `@apply`/`@variants` 指令支持（`uno.config.ts`） |

## 10. 国际化

### 10.1 语言包分片

每种语言 5 个分片文件，`locales/{lang}.ts` 只做聚合：

| 文件 | 内容 |
| --- | --- |
| `locales/{lang}.ts` | 聚合导出：`common` + `workspace` + `admin` + `extensions` |
| `locales/{lang}-common.ts` | 通用命名空间 |
| `locales/{lang}-workspace.ts` | 写作台与项目内页面 |
| `locales/{lang}-admin.ts` | 模型、用量、状态、系统管理 |
| `locales/{lang}-extensions.ts` | 工作流、Markdown 笔记、微前端 |

支持语言：`zh-CN`（简体中文）、`en-US`（English），见 `src/i18n/index.ts:NW_LOCALE_OPTIONS`；
`fallbackLocale` 为 `zh-CN`，`legacy: false`。

### 10.2 20 个命名空间

| 分片 | 命名空间 |
| --- | --- |
| common | `common`、`http`、`login`、`layout`、`dashboard`、`settings`、`slashAdmin`（7） |
| workspace | `workspace`、`projectPage`、`character`、`knowledge`、`notes`、`lore`（6） |
| admin | `models`、`usage`、`status`、`admin`（4） |
| extensions | `workflow`、`markdownNote`、`microApp`（3） |

合计 20 个。

### 10.3 键名与同步

- 键名一律 camelCase，例：`layout.menu.dashboard`、`projectPage.backToWorkspace`、
  `knowledge.search.modeKeyword`、`notes.saveStatus.saveFailed`、`markdownNote.modeRich`。
- 语言切换：`useAppStore.setLocale` → `i18n/index.ts:applyLocale`，同时写 `nw_locale`。
- Element Plus 语言同步：`src/App.vue` 用 `el-config-provider` 包裹根节点，
  `:locale` 由 `appStore.locale_code` 计算——`en-US` 用 `element-plus/es/locale/lang/en`，
  否则用 `element-plus/es/locale/lang/zh-cn`。

## 11. 前端测试

- 现有**仅 1 个测试用例**：`src/composables/useSSE.test.ts`，`describe('parseSseBuffer 事件解析链路')`
  下的单个 `it`。
- 覆盖内容（一条用例内串联断言）：
  1. 跨 chunk 断行：完整事件被解析，末尾半行留在 `restText`；
  2. 半行补全 + 多事件粘包：两条事件按序解析，内容字段正确；
  3. CRLF 换行（`\r\n`）兼容；
  4. 非法 JSON 行被跳过且不中断后续解析；
  5. 非 `data:` 前缀行（心跳注释等）不产生事件；
  6. 末尾半行必须留存供下一段拼接。
- 组件渲染与样式断言**有意不保留**：测试环境为 `environment: 'node'`，不引入 jsdom，也不做快照。
- 类型正确性由 `npm run build`（`vue-tsc && vite build`）保证。
- `tsconfig.json` 开启 `strict`、`noUnusedLocals`、`noUnusedParameters`、`noFallthroughCasesInSwitch`。
