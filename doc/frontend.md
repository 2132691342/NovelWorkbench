# 前端设计

本文说明 NovelWorkbench 前端（`frontend/`）的工程结构、设计系统、状态与数据流、路由与权限、页面实现与构建发布。

工程骨架直接取自 **vue-element-plus-admin**（`apps/admin` + `packages/*`）：布局外壳、`Menu` 的 ElMenu 渲染、
TagsView、Breadcrumb、ContentWrap、CountTo、LocaleDropdown、ContextMenu、Backtop 都保留上游实现，
只替换品牌令牌与业务页面。因此升级上游组件时可以整文件覆盖，不必逐个重写。

## 1. 工程结构与构建

### 1.1 目录职责

前端是 pnpm workspace 单仓：根应用 + 4 个内部包。内部包只承载与业务无关的可复用能力，业务代码全部留在 `src/`。

| 路径 | 职责 |
| --- | --- |
| `src/main.ts` | 入口：Pinia → 导航/鉴权 → i18n → 全局组件 → Element Plus → 路由 → 挂载 |
| `src/App.vue` | 根组件：`ConfigGlobal`（Element Plus 语言包 + 响应式断点）包裹 `RouterView` |
| `src/api/` | `http.ts`（get/post 薄封装）、`contracts.ts`（后端 VO 类型）与各业务模块 |
| `src/request/` | 请求实例：注入令牌、解包包络、401 会话过期 |
| `src/layout/` | `Layout.vue`（四种布局外壳）、`components/`（`Menu`、`PrimaryNav`、`LayoutSwitcher`、顶栏、项目切换、通知铃铛）、`menuIcons.ts` |
| `src/router/` | 常量路由表（登录 / 404 / 业务外壳）；业务页面由后端菜单动态注册 |
| `src/permission.ts` | 唯一全局守卫：登录态、动态路由注册、项目作用域补全、页面权限码 |
| `src/store/modules/` | `user`（含导航树与权限码）、`permission`（路由表）、`project`、`app`、`locale`、`tagsView`、`notification` |
| `src/components/` | 通用组件（`Menu`、`ContentWrap`、`CountTo`、`Footer`、`LocaleDropdown`、Logo、Breadcrumb、TagsView、ContextMenu、Collapse、Echart、ConfigGlobal、UserInfo、Backtop、Error） |
| `src/views/` | 业务页面目录 |
| `src/composables/` | `usePageQuery.ts`（分页列表状态）、`useSSE.ts`（流式请求） |
| `src/utils/` | `storage.ts`、`time.ts`、`table.ts`（表格行类型还原）、`routerHelper.ts` |
| `src/locales/` | 界面壳文案（`zh-CN.ts`），业务文案由后端数据提供 |
| `packages/components` | `Icon`（按名称渲染已注册图标）与 `ThemeSwitch`（明暗切换）、图标注册表 |
| `packages/hooks` | `useForm` 与校验器（表单状态机） |
| `packages/request` | `createRequest`：axios 实例工厂，含前置钩子、响应解包与错误转交（只实现业务用到的 GET / POST） |
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
| `user` | 令牌、当前用户、导航树、权限码；`login` / `logout` / `fetchProfile` / `loadNav` / `clearSession` | 仅 `token`（`nw-session-v2`） |
| `permission` | 后端导航树生成的路由表；`generateRoutes` / `setIsAddRouters` / `reset` | 否 |
| `project` | 当前项目、项目选项；`selectProject` / `syncFromRoute` / `loadProjectOptions` | 仅 `currentProjectUlid`（`nw-project-v1`） |
| `app` | 折叠、暗色、移动端断点、页面 loading、布局模式 | `collapse` / `layout` / `isDark` |
| `locale` | 界面语言（当前仅 zh-CN） | `lang` |
| `tagsView` | 多页签与 keep-alive 缓存名单 | 否 |
| `notification` | 未读数与最近一条通知 | 否 |

`menu` store 已并入 `user`：导航树与权限码只在登录态存在，拆成两个 store 会导致登出时清理顺序耦合。

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
- **工具结果**：正文命中工具关键词时，服务端在装配上下文前先执行工具，结果以「工具结果」块注入；回复的「AI 依据」区展示召回片段。
- **体检联动**：写作台「体检」打开 `ChapterCheckPanel`，跑完可点「让 AI 修」，把问题清单写入 AI 输入框并预选 `/deai` 与当前章节引用。

## 4. 路由与权限

### 4.1 动态路由：后端菜单即路由

页面组件不再静态声明在路由表里。`src/router/index.ts` 只保留登录、404 与业务外壳（`Layout` + `redirect` 子路由）；
业务页面全部由 `src/utils/routerHelper.ts` 从 `/menu/nav` 生成：

```
菜单节点 { menu_code, route_path, component_path, icon_code, perm_code, scope_code, child_list }
   ↓ buildRoutes / toRouteRecord
vue-router 记录 { path, name: PascalCase(menu_code), meta: { title, icon, permCode, projectScope }, component: resolveView(component_path) }
   ↓ router.addRoute('Root', …)
侧边栏 / TagsView / 面包屑全部读 permissionStore.routers
```

- 组件解析：`import.meta.glob('../views/**/*.{vue,tsx}')` 构建期收集，`component_path`（如 `views/Workspace/index`）按字符串取懒加载函数；
  找不到组件时路由仍注册但页面显示 404，管理端改错路径不会让整个前端白屏。
- 分组节点（`menu_type_code=group`）只生成 children 与 `redirect`，不渲染自身页面。
- 新增页面：写 `.vue` → 在「菜单管理」登记 `component_path` → 刷新即生效，**不需要前端发版**。

### 4.2 唯一全局守卫

`src/permission.ts`：

1. 未登录 → `/login?redirect=…`（`/login` 在白名单）。
2. 已登录访问 `/login` → 回首页。
3. `ensureDynamicRoutes()`：首次拉 `/menu/nav`，生成并注册路由；注册成功后重新导航一次（否则首帧会落到 Fallback）。
4. 项目作用域路由路径仍带 `:project_ulid` 占位时，替换为当前项目（或首个项目）；已含具体 ULID 时原样放行，避免自我重定向死循环。
5. `meta.permCode` 存在且未命中用户权限码 → 跳到第一个有权限的页面。
6. 命中 `Fallback` → `/404`。

导航、路由与页面门禁三者同源：`perm_code` 只写在菜单表里，改权限不需要同时改前后端两处常量。

### 4.3 项目上下文

项目作用域菜单的路由形如 `/workspace/:project_ulid`。`Layout.vue` 用 `substituteProject()` 把菜单树里的占位符替换成
`projectStore.currentProjectUlid` 后再交给 `Menu`，因此菜单 index 与当前 URL 能对上；`watch(route.params.project_ulid)`
调 `projectStore.syncFromRoute()` 把 URL 上的项目同步为当前项目。顶栏项目切换器改选项目时只替换 URL 中的 ULID，停留在同一功能页。

### 4.4 后端驱动导航与四种布局

`components/Menu/index.vue` 用 Element Plus `ElMenu` / `ElSubMenu` / `ElMenuItem`（模板来自 vue-element-plus-admin）：
折叠、二级展开、弹层与键盘可达性都沿用组件库行为；`mode="horizontal"` 用于顶栏，`mode="vertical"` 用于侧栏/二级栏。
`meta.icon` 存 `mdi:*` 名称，由 `layout/menuIcons.ts` 把后端短编码（`book`、`tree`…）映射成图标表键。

`layout/Layout.vue` 支持四种布局（`useAppStore().layout`，顶栏「切换布局」即时生效并持久化）：

| 布局 | 结构 | 说明 |
| --- | --- | --- |
| `sidebar` | 顶栏 + 左侧完整菜单 | 默认，层级全部在侧栏展开 |
| `top` | 顶栏横向主菜单（分组下拉）+ 内容 | 适合宽屏，纵向空间全给内容 |
| `mixed` | 顶栏横向一级 + 左侧二级 | 一级切分组、左侧只显示该分组页面 |
| `dual` | 左侧图标栏 + 二级菜单栏 | 一级只显示图标，二级常驻 |

`dual` 与 `mixed` 的一级点击只切换二级菜单（`PrimaryNav` 的 `select` 事件），不跳转；`dual` 默认跟随当前路由的分组。
窄屏（≤767px）强制回落 `sidebar` 并以抽屉形式展开。

## 5. 页面实现

| 页面 | 关键实现 |
| --- | --- |
| 仪表盘 | 统计卡（项目数/总字数/今日新增/AI 产出）+ 近 30 天趋势（ECharts）+ 项目卡片网格（进入写作台/大纲/角色/知识库、编辑、删除）+ 分页 + **AI 开书向导**（`components/BootstrapDialog.vue`：创意 → 方案预览与逐字段修改 → 一次性落库并进入写作台） |
| 写作台 | 三栏：目录树（卷/章增删改、搜索、折叠）/ CodeMirror 正文编辑器（2s 防抖自动保存、字数实时、梗概折叠）/ AI 面板（模型与会话切换、斜杠、@、流式、插入/替换）；顶栏「体检」打开 `ChapterCheckPanel`（量化门禁结果 + 问题清单 + 一键交给 AI 修） |
| 创作规划 | 大纲树 + 场景卡（左侧节点、右侧场景列表与编辑器） |
| 连续性看板 | 三个页签：时间事实（主体+谓词+客体+来源章节，复核确认/推翻）、伏笔台账（计划→埋设→呼应→回收）、章节摘要（可由事实一键起草）；顶部统计待确认事实与未回收伏笔。事实与伏笔的表单在 `views/Continuity/components/EntityDialogs.vue` |
| 角色管理 | 角色表（姓名/类型/描述/目标动机）+ 抽屉编辑 + X6 关系图谱页签（节点拖拽、关系新建/删除） |
| 世界观设定 | 设定卡片列表 + 搜索 + 抽屉编辑 + 删除 |
| 知识库 | 统计卡 + 文档表（上传进度、重索引、删除）+ 词法检索面板（关键词/混合、命中片段高亮）+ 切片弹窗 + 原文预览抽屉（md/txt/docx/pdf） |
| 创作笔记 | 左侧列表 + 右侧双栏编辑预览（Markdown / HTML 模式切换、2s 防抖自动保存、保存状态与重试） |
| 模型供应商 | 能力探测横幅 + 供应商表（协议、密钥掩码、连通性测试）+ 模型目录表（默认、授权角色/用户）+ 授权弹窗（按角色与按用户两个页签）+ 用量面板（ECharts 双轴） |
| 技能管理 | 按流程域分组 + 过滤器 + 内置/已改标记 + 抽屉编辑（模板、变量、参考资料）+ 试用弹窗（解析技能并展示缺参） |
| 工具管理 | 内置/自建标记 + 触发关键词标签 + 启停开关 + 抽屉编辑（说明、关键词、参数声明）+ 试运行抽屉（结果文本 + 指标） |
| 提示词档案 | 按域分组 + 抽屉编辑 + 只读预览 |
| 管理端 | 用户（角色分配、重置密码）、角色（权限目录勾选）、菜单（树形父级、路由、组件路径、权限码）、权限目录（只读分组）、审计（级别/链路/目标/日期筛选 + 详情抽屉）、登录日志、任务中心（统计与重试）、用量分析、运行状态（依赖健康） |
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
