# 前端设计

本文说明 NovelWorkbench 前端（`frontend/`）的工程结构、设计系统、状态与数据流、路由与权限、页面实现与构建发布。

## 1. 工程结构与构建

### 1.1 目录职责

前端是 pnpm workspace 单仓：根应用 + 4 个内部包。内部包只承载与业务无关的可复用能力，业务代码全部留在 `src/`。

| 路径 | 职责 |
| --- | --- |
| `src/main.ts` | 入口：Pinia → 导航/鉴权 → i18n → 全局组件 → Element Plus → 路由 → 挂载 |
| `src/App.vue` | 根组件：`ConfigGlobal`（Element Plus 语言包 + 响应式断点）包裹 `RouterView` |
| `src/api/` | `http.ts`（get/post 薄封装）、`contracts.ts`（后端 VO 类型）与各业务模块 |
| `src/request/` | 请求实例：注入令牌、解包包络、401 会话过期 |
| `src/layout/` | `Layout.vue`（外壳）、`components/`（顶栏、导航、项目切换、通知铃铛）、`useNavMenu.ts`、`menuIcons.ts` |
| `src/router/` | 静态路由表（全部页面组件在此声明） |
| `src/permission.ts` | 唯一全局守卫：登录态、导航加载、页面权限码 |
| `src/store/modules/` | `user`、`menu`、`project`、`app`、`locale`、`tagsView`、`notification` |
| `src/components/` | 通用组件（Logo、Breadcrumb、TagsView、ContextMenu、Echart、ConfigGlobal、UserInfo、Backtop） |
| `src/views/` | 业务页面目录 |
| `src/composables/` | `usePageQuery.ts`（分页列表状态）、`useSSE.ts`（流式请求） |
| `src/utils/` | `storage.ts`、`time.ts`、`table.ts`（表格行类型还原）、`routerHelper.ts` |
| `src/locales/` | 界面壳文案（`zh-CN.ts`），业务文案由后端数据提供 |
| `packages/components` | `Icon`、`ThemeSwitch`、`LocaleDropdown` 与图标注册表 |
| `packages/hooks` | `useForm` 与校验器（表单状态机） |
| `packages/request` | `createRequest`：axios 实例工厂，含前置钩子、响应解包、错误转交与并发取消 |
| `packages/styles` | 品牌令牌、Element Plus 变量桥接、按钮五态与页面骨架类（唯一改造点） |

### 1.2 样式加载顺序

`src/main.ts` 与 `packages/styles/src/index.less` 决定层叠优先级，后加载者覆盖先加载者：

1. `element-plus/dist/index.css`：组件库基础样式。
2. `virtual:uno.css`：UnoCSS 原子类（`header-action` 等快捷方式在 `uno.config.ts`）。
3. `@novelworkbench/styles` → `var.css`：品牌五色 + Element Plus 变量桥接 + 语义令牌 + 明暗两套。
4. `@novelworkbench/styles` → `button.css`：按钮五态体系。
5. `@novelworkbench/styles` → 其余：`index.less` 内配置项重置与 `.nw-page` / `.nw-toolbar` / `.nw-badge` 骨架类。
6. `element-plus/theme-chalk/dark/css-vars.css`：暗色变量。

### 1.3 构建配置

| 项 | 值 | 来源 |
| --- | --- | --- |
| 包管理 | pnpm 10 workspace（`packages/*`） | `pnpm-workspace.yaml` |
| 依赖版本 | `vue` 3.5.41、`element-plus` 2.14.4、`typescript` | `pnpm-workspace.yaml` 的 `catalog` |
| 别名 | `@` → `./src` | `vite.config.ts` |
| 插件 | `@vitejs/plugin-vue`、`-jsx`、`UnoCSS`、`@intlify/unplugin-vue-i18n` | `vite.config.ts` |
| dev 端口 | 4000 | `vite.config.ts` |
| dev 代理 | `/api` → `NW_DEV_PROXY_TARGET`（默认 `http://127.0.0.1:8080`） | `vite.config.ts` |
| 构建分包 | `vendor-vue`、`vendor-element`、`vendor-editor`、`vendor-canvas`、`vendor-doc` | `vite.config.ts` 的 `codeSplitting.groups` |
| 构建产物 | `dist` | `.env.pro` 的 `VITE_OUT_DIR` |
| 构建脚本 | `vue-tsc --noEmit && vite build --mode pro` | `package.json` 的 `scripts.build` |
| 类型严格度 | `strict`、`noUnusedLocals`、`noUnusedParameters`、`noEmit` | `tsconfig.json` |

分包目标：首屏只加载 `index` + `vendor-vue` + `vendor-element`；CodeMirror、ECharts/X6、docx/pdf/marked 各自独立，进入对应页面才下载。

### 1.4 组件注册约定

- **Element Plus 全量注册**（`app.use(ElementPlus)`）：业务页直接写 `<ElButton>`、`<ElTable>`，不逐页维护 import 清单，也不会因漏注册而渲染失败。全量样式在入口统一引入。
- **图标按需注册**：`src/icons.ts` 登记实际用到的 `mdi:*` 图标，`Icon` 组件全局可用；新增图标在此追加一行 import + 一条映射。
- **业务菜单图标**：后端只存语义编码（`dashboard` / `book` / `tree`…），前端在 `src/layout/menuIcons.ts` 映射为具体图标，未登记编码回落到通用文件图标。

## 2. 设计系统

### 2.1 品牌五色

品牌五色是唯一色源，页面内禁止硬编码替代色值（`packages/styles/src/var.css`）：

| 变量 | 值 | 角色 |
| --- | --- | --- |
| `--saas_dashboard-color-1` | `#6366f1` | 主色（按钮描边、激活态、图表主序列） |
| `--saas_dashboard-color-2` | `#818cf8` | 主色亮阶（悬停、暗色主色） |
| `--saas_dashboard-color-3` | `#e0e7ff` | 主色浅阶（按下底、选中底） |
| `--saas_dashboard-color-4` | `#f8fafc` | 页面底色 |
| `--saas_dashboard-color-5` | `#1e293b` | 正文字色 |

### 2.2 语义令牌

业务样式只使用语义令牌，令牌再指回品牌色，换肤只需改一处：

| 组 | 令牌 |
| --- | --- |
| 主色 | `--nw-primary`、`--nw-primary-soft`、`--nw-primary-light` |
| 文本 | `--nw-ink`、`--nw-text-secondary`、`--nw-text-muted` |
| 表面 | `--nw-surface`、`--nw-surface-subtle`、`--el-fill-color-light` |
| 描边 | `--nw-border`、`--nw-border-soft` |
| 状态 | `--nw-success`、`--nw-warning`、`--nw-danger`、`--nw-info` |
| 圆角 | `--nw-radius-sm`(4) / `base`(6) / `md`(8) / `lg`(10) / `xl`(12) |

圆角只有五档，控件 6、输入与按钮 8、卡片 10、弹层 12，不做大圆角设计。

### 2.3 Element Plus 变量桥接与暗色

`html:root` 把 Element Plus 的主色、背景、描边、文本变量指向品牌色；`html:root.dark` 提供整组暗色值（主色切到 `--saas_dashboard-color-2`，表面切到 `#151e30` / `#0b1120`）。
主题切换由 `useAppStore().isDark` 驱动 `useTheme()` 切换根节点的 `dark` class，`ConfigGlobal` 的 `ElConfigProvider` 保证组件语言包与断点一致。

### 2.4 按钮五态（镂空线条风格）

`packages/styles/src/button.css` 同时覆盖 `.el-button` 与业务自定义按钮 `.nw-button`。默认态是「透明底 + 1px 描边 + 主色字」的镂空形态，只有悬停与按下才填底色，一屏几十个按钮不会糊成色块。

| 状态 | 表现 | 实现 |
| --- | --- | --- |
| DEFAULT | 透明底、描边、主色字 | `--nw-btn-*` 变量组 |
| HOVER | `scale(1.05)` 放大 + 淡底 + 描边提亮 + 浮起投影 | `transform: scale(var(--nw-btn-hover-scale))` |
| PRESSED | `scale(0.95)` 缩小 + 底色加深 + 投影收敛 | `:active`，过渡压到 0.08s |
| DISABLED | 透明度 0.45、`not-allowed`、无缩放 | `.is-disabled` / `:disabled` |
| LOADING | 转圈 + 微光扫过、保持底色不闪烁 | `.is-loading::after` 渐变扫动 |

图标按钮与小尺寸按钮放大到 `1.08` / 按下 `0.92`，小尺寸需要更大相对位移才可感知。全局 `prefers-reduced-motion` 下关闭缩放与扫光。

### 2.5 页面骨架类

业务页统一用全局类，避免每页各写一套间距：`.nw-page`（纵向栅格 + 入场动画）、`.nw-toolbar`（工具条）、`.nw-grid`、`.nw-hint`、`.nw-badge` 及四个色调变体。

## 3. 状态与数据流

### 3.1 七个 Store

| Store | 职责 | 持久化 |
| --- | --- | --- |
| `user` | 令牌、当前用户、权限码；`login` / `logout` / `fetchProfile` / `clearSession` | 仅 `token`（`nw-session-v1`） |
| `menu` | 后端导航树与权限码；`loadNav` / `reset`；getter `flatPages` | 否 |
| `project` | 当前项目、项目选项；`selectProject` / `syncFromRoute` / `loadProjectOptions` | 仅 `currentProjectUlid`（`nw-project-v1`） |
| `app` | 折叠、暗色、移动端断点、页面 loading | `collapse` / `isDark` |
| `locale` | 界面语言（当前仅 zh-CN） | `lang` |
| `tagsView` | 多页签与 keep-alive 缓存名单 | 否 |
| `notification` | 未读数与最近一条通知 | 否 |

### 3.2 请求层

`src/request/index.ts` 基于 `@novelworkbench/request` 的 `createRequest` 构造实例：

1. `beforeRequest` 注入 `Authorization: Bearer <token>`（令牌来自 `localStorage`，SSE 与普通请求同源）。
2. `transformResponse` 解包后端包络：`resp_code === 'SUCCESS'` 时直接返回 `data`，否则抛 `ApiError`（保留 `resp_code` / `resp_message` / `trace_id` / `http_status`）。
3. 统一出口 `send()`：401 视为会话过期，清本地会话并跳登录（登录接口自身的 401 是凭据错误，单独放行）；其余失败按 `silent` 决定是否弹 `ElMessage`。

`src/api/http.ts` 是业务模块唯一依赖的薄封装：`get<T>` / `post<T>` 返回解包后的数据，`silent_error: true` 静默失败，支持 `timeout` / `responseType` / `onUploadProgress`。

### 3.3 SSE

`composables/useSSE.ts` 用 `fetch` + `ReadableStream`（axios 不支持流式响应）：`parseSseBuffer` 只消费完整行，跨 chunk 断行与多事件粘包都能处理；`useSSE()` 暴露 `streamingFlag` / `startStream` / `stopStream`（AbortController 中止）。

### 3.4 写作台 AI 面板

- **斜杠命令**：输入 `/` 唤起技能候选（按编码与名称过滤，↑↓ 选择，Enter/Tab 确认，Esc 关闭），选中后写成 `/skill_code` 徽标并清掉触发词。
- **@ 引用**：输入 `@` 唤起选择器，五个页签——文档、章节、摘要、提示词档案、历史消息；多选后以标签形式挂到输入区，发送时装配为 `ref_*_ulid_list`。
- **上下文开关**：「附带当前章节」开启时，当前章自动并入 `ref_chapter_ulid_list`。
- **流式渲染**：`thinking` 事件累加思考过程（可折叠），`chunk` 累加正文，`done` 用服务端全量覆盖并补召回依据，`error` 落错误文案并提供「重试」（保留原载荷重发）。
- **等待态**：每 200ms 刷新已等待时长，界面始终显示仍在工作。
- **结果处置**：助手回复可复制、插入编辑器光标处、或替换整章正文（编辑器同一自动保存通道）。

## 4. 路由与权限

### 4.1 静态路由表

页面组件在 `src/router/index.ts` 静态声明，后端菜单**不**下发组件路径——菜单只决定「侧边栏显示什么、点去哪」。这样后端加/改菜单不需要前端发版（新增页面除外）。

路由分组与菜单编码一一对应：`dashboard`、`workspace`/`outline`/`continuity`/`character`/`lore`/`knowledge`/`notes`（项目作用域，路径带 `:project_ulid`）、`models`/`skills`/`prompts`/`notifications`、`admin/*`、`settings`。

### 4.2 唯一全局守卫

`src/permission.ts`：

1. 未登录 → `/login?redirect=…`（`/login` 在白名单）。
2. 已登录访问 `/login` → 回首页。
3. 首次进入拉导航（`/menu/nav`），失败即清理会话回登录。
4. `meta.permCode` 存在且未命中用户权限码 → 回首页（直接输 URL 的兜底门禁）。

### 4.3 项目上下文

项目作用域页面的路径带 `:project_ulid`。`Layout.vue` 监听该参数并调用 `projectStore.syncFromRoute()`，把 URL 上的项目同步为当前项目；顶栏项目切换器改选项目时只替换 URL 中的 ULID，停留在同一功能页。

### 4.4 后端驱动导航

`layout/useNavMenu.ts` 把 `/menu/nav` 的树转成侧边栏节点：项目作用域菜单用当前项目替换 `:project_ulid`，未选项目时**置灰而非隐藏**（点击提示先选项目）；当前高亮按路径前缀最长匹配计算，因此带 ULID 的项目页也能正确高亮。

## 5. 页面实现

| 页面 | 关键实现 |
| --- | --- |
| 仪表盘 | 统计卡（项目数/总字数/今日新增/AI 产出）+ 近 30 天趋势（ECharts）+ 项目卡片网格（进入写作台/大纲/角色/知识库、编辑、删除）+ 分页 |
| 写作台 | 三栏：目录树（卷/章增删改、搜索、折叠）/ CodeMirror 正文编辑器（2s 防抖自动保存、字数实时、梗概折叠）/ AI 面板（模型与会话切换、斜杠、@、流式、插入/替换） |
| 创作规划 | 大纲树 + 场景卡（左侧节点、右侧场景列表与编辑器） |
| 连续性看板 | 三个页签：时间事实（主体+谓词+客体+来源章节，复核确认/推翻）、伏笔台账（计划→埋设→呼应→回收）、章节摘要（可由事实一键起草）；顶部统计待确认事实与未回收伏笔。事实与伏笔的表单在 `views/Continuity/components/EntityDialogs.vue` |
| 角色管理 | 角色表（姓名/类型/描述/目标动机）+ 抽屉编辑 + X6 关系图谱页签（节点拖拽、关系新建/删除） |
| 世界观设定 | 设定卡片列表 + 搜索 + 抽屉编辑 + 删除 |
| 知识库 | 统计卡 + 文档表（上传进度、重索引、删除）+ 词法检索面板（关键词/混合、命中片段高亮）+ 切片弹窗 + 原文预览抽屉（md/txt/docx/pdf） |
| 创作笔记 | 左侧列表 + 右侧双栏编辑预览（Markdown / HTML 模式切换、2s 防抖自动保存、保存状态与重试） |
| 模型供应商 | 能力探测横幅 + 供应商表（协议、密钥掩码、连通性测试）+ 模型目录表（默认、授权角色）+ 用量面板（ECharts 双轴） |
| 技能管理 | 按流程域分组 + 过滤器 + 内置/已改标记 + 抽屉编辑（模板、变量、参考资料）+ 试用弹窗（解析技能并展示缺参） |
| 提示词档案 | 按域分组 + 抽屉编辑 + 只读预览 |
| 管理端 | 用户（角色分配、重置密码）、角色（权限目录勾选）、菜单（树形父级、路由、权限码）、权限目录（只读分组）、审计、登录日志、任务中心（统计与重试）、用量分析、运行状态（依赖健康） |
| 系统设置 | 配置项分组编辑（布尔项用开关）+ 整体保存 |
| 个人中心 | 账号信息 + 资料编辑 + 改密 |
| 通知中心 | 全部/未读/已读筛选 + 标记已读 + 删除，联动顶栏铃铛计数 |

## 6. 编辑器与可视化分工

| 场景 | 选型 | 理由 |
| --- | --- | --- |
| 章节正文 | CodeMirror 6（markdown） | 体积小、可控、行号与历史栈够用 |
| 笔记 | 原生 textarea + `marked` 预览（双栏） | 两种模式（HTML / Markdown）共用同一自动保存通道，不需要富文本工具栏 |
| 关系图谱 | AntV X6（动态加载） | 节点拖拽与边标签开箱可用；卸载时 `dispose` 防泄漏 |
| 趋势与用量 | ECharts（按需注册） | 折线/柱状足以表达字数与调用量 |
| 文档预览 | `docx-preview` / `vue-pdf-embed`（动态加载） | 只在预览 docx/pdf 时下载 |

## 7. 国际化

界面壳文案在 `src/locales/zh-CN.ts`（`common` / `error` / `layout` / `login` / `router` 五组），业务文案（菜单名、技能名、提示词内容）来自数据库。当前只交付简体中文：`config/locale.ts` 的注册表只登记 zh-CN，新增语言需补齐键位后再开放切换入口。

## 8. 构建与发布

| 场景 | 命令 | 产物 |
| --- | --- | --- |
| 开发 | `pnpm dev`（代理到本机 8080） | — |
| 类型检查 | `pnpm ts:check` | — |
| 单元测试 | `pnpm test` | — |
| 生产构建 | `pnpm build` | `dist/` |
| 容器镜像 | `frontend/Dockerfile`（node22 + pnpm → nginx） | 仅静态资源 + nginx 模板 |

`frontend/Dockerfile` 先复制 `package.json` / `pnpm-lock.yaml` / `pnpm-workspace.yaml` / `packages` 再 `pnpm install --frozen-lockfile`，最后拷源码构建，最大化层缓存命中。nginx 模板把 `/api/` 反代到后端并对 SSE 关闭缓冲（`proxy_buffering off`、`proxy_read_timeout 3600s`）。
