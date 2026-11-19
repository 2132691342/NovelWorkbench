# 前端设计

本文描述 NovelWorkbench 当前前端的工程结构、设计系统、状态管理、路由与页面实现。

## 1. 工程结构

```
frontend/
├── src/
│   ├── api/          HTTP 客户端与模块 API 封装
│   ├── components/   无业务组件：NWButton/NWCard/NWEmpty/NWLoading/ProjectPageHeader
│   ├── composables/  useSSE：SSE 流式解析与请求
│   ├── i18n/         zh-CN / en-US 文案
│   ├── layout/       AdminLayout：侧栏、项目切换器、顶栏、内容区
│   ├── router/       路由表与登录守卫
│   ├── stores/       auth / app / project
│   ├── styles/       variables.css / global.css / button-states.css
│   ├── utils/        localStorage 封装
│   └── views/        业务页面
├── vite.config.ts    别名、/api 代理、手动分包
├── vitest.config.ts  单元测试配置（复用 @ 别名）
└── uno.config.ts     UnoCSS 预设
```

页面样式约定：单文件接近 500 行时，样式抽到同目录 `*.css`，通过
`<style scoped src="./page.css"></style>` 引用；逻辑复杂的页面拆分子组件。
所有前端源文件保持 ≤500 行（当前最大为 `views/outline/index.vue`，487 行）。

## 2. 设计系统

### 2.1 品牌色

```css
:root {
  --saas_dashboard-color-1: #6366f1;
  --saas_dashboard-color-2: #818cf8;
  --saas_dashboard-color-3: #e0e7ff;
  --saas_dashboard-color-4: #f8fafc;
  --saas_dashboard-color-5: #1e293b;
}
```

语义变量由五色派生：`--primary`、`--primary-hover`、`--primary-light`、`--primary-active`、
`--primary-disabled`、`--bg-page`、`--bg-card`、`--bg-hover`、`--text-main`、`--text-secondary`、
`--text-placeholder`、`--text-inverse`、`--border`、`--border-light`、`--success`、`--warning`、
`--error`、`--error-hover`、`--error-active`。

组件只引用语义变量，换肤只改变量层。

### 2.2 按钮五态

所有按钮与按钮式控件必须实现五态，统一定义在 `styles/button-states.css`，
覆盖 Element Plus 按钮（`.el-button`）与自定义按钮（`.nw-button`）：

| 状态 | 表现 | 动效 |
| --- | --- | --- |
| DEFAULT | 正常颜色、基础阴影 | `transition: 0.2s ease` |
| HOVER | 颜色提亮、阴影加深、`scale(1.05)` | 平滑放大 |
| PRESSED | 颜色加深、阴影收敛、`scale(0.95)` | 平滑缩小，形成点击感 |
| DISABLED | 降透明、灰化、禁止交互 | 无动效 |
| LOADING | spinner 旋转、微光扫过、禁止重复点击 | 持续动画 |

页面不得自定义按钮状态样式；新增按钮优先使用 `NWButton` 或 Element Plus 组件并挂接全局样式。

### 2.3 交互反馈

- 写操作成功/失败均有 `ElMessage` 提示；错误码映射为可读文案。
- 路由切换使用 fade-slide 过渡；卡片悬停微抬起。
- 列表有加载态（`NWLoading`）与空态（`NWEmpty`）；加载失败有明确错误与重试入口。
- 表单提交期间按钮进入 LOADING，防止重复提交。
- 删除等危险操作二次确认。
- AI 流式输出逐字展示；停止按钮可取消；插入正文需用户显式点击。

## 3. 状态与数据流

| Store | 状态 | 说明 |
| --- | --- | --- |
| `auth` | `access_token`、`user_vo` | 持久化到 localStorage；登录/注册/登出 |
| `app` | `locale_code`、`sidebar_collapsed` | 语言与布局，均持久化 |
| `project` | `current_project_ulid`、`project_list`、`list_loading` | 当前项目上下文与项目列表 |

`auth` store 暴露 `is_logged_in` getter；`logout` 尽力调用登出接口后再清理本地会话，
接口异常不阻塞本地清理。

`project` store 在拉取列表后自动把选中项收敛到仍存在的项目（不存在则取第一项），
避免用户停留在已删除项目上。

Axios 层（`api/http.ts`）：

- baseURL `/api/v1`，超时 30 秒。
- 请求拦截注入 Bearer Token。
- 响应拦截解包 `{resp_code, data}`，非 SUCCESS reject `NWApiError`。
- `ERR_UNAUTHORIZED` 清 token 与用户信息并跳登录页。
- 同一错误只提示一次（`NWApiError.notified_flag` 标记）。
- 网络层异常统一归类为 `ERR_INTERNAL` 并给出本地化文案。
- SSE 走独立 fetch 流（`composables/useSSE.ts`），不走普通响应拦截。

## 4. 路由

| 路径 | 页面 | 说明 |
| --- | --- | --- |
| `/login` | 登录/注册 | 免鉴权 |
| `/` | 仪表盘 | 统计卡、字数趋势、项目管理 |
| `/workspace/:project_ulid` | 写作台 | 目录树、编辑器、AI 面板 |
| `/outline/:project_ulid` | 创作规划 | 大纲树与场景卡 |
| `/continuity/:project_ulid` | 连续性看板 | 时序事实、伏笔、章节摘要 |
| `/character/:project_ulid` | 角色管理 | 角色卡、X6 关系图 |
| `/lore/:project_ulid` | 世界观设定 | 分类、搜索、编辑抽屉 |
| `/knowledge/:project_ulid` | 知识库 | 上传、检索、预览 |
| `/notes/:project_ulid` | 创作笔记 | 富文本编辑 |
| `/models` | 模型供应商 | 双协议、测试、默认切换 |
| `/settings` | 系统设置 | 运行时配置 |
| `/slash-admin` | 斜杠技能 | 技能模板管理 |
| `/admin/users` · `/admin/roles` · `/admin/audit` | 系统管理 | 用户/角色/审计 |
| `/admin/usage` | 用量分析 | 调用与 token 趋势 |
| `/admin/status` | 运行状态 | 依赖健康与 AI 能力 |

守卫：无 token 跳 `/login`；已登录访问登录页回 `/`；未匹配路径重定向 `/`。
路由按页面懒加载，Element Plus、ECharts、X6 独立分包（`vite.config.ts` 的 `manualChunks`）。

**项目上下文导航**：侧栏顶部是项目切换器，下面固定七个项目内入口：写作台、创作规划、
连续性看板、角色管理、世界观设定、知识库、素材笔记。URL 中的 `project_ulid` 优先级最高
（`routeProjectUlid`），路由变化反写 store；URL 中项目不存在时提示并回到仪表盘。

## 5. 页面实现

### 5.1 登录

登录/注册双 Tab，字段校验，提交 LOADING，成功提示后进入仪表盘。

### 5.2 仪表盘

四张统计卡：项目总数、累计字数、今日新增、AI 生成字数。ECharts 近 30 天双系列折线
（人工/AI）。项目列表支持新建、编辑、删除；删除二次确认。

### 5.3 写作台

三栏布局：

- 左：卷/章目录树（`WorkspaceTree.vue`）。下拉菜单增删改；XLSX 导出大纲。
- 中：CodeMirror 编辑器（`ChapterEditor.vue`）。自动保存、字数实时统计、快照创建与回滚。
- 右：AI 面板（`AiPanel.vue`）。流式打字、斜杠补全、@引用选择器、历史条数、模型选择、
  召回依据展示、插入章节。

AI 面板会话可创建/删除/切换，停止生成按钮取消 SSE；所有动作有 toast 反馈。

### 5.3.1 创作规划

左侧大纲树按层级展示卷/幕/故事块/章节，支持新建、编辑与删除（有子节点不可删）；
右侧展示选中节点的场景卡列表，支持新建、编辑、删除。表单覆盖角色、地点、冲突、节奏、
目标、关键选择、结果与状态字段。

### 5.3.2 连续性看板

三个标签页分别展示时序事实、伏笔钩子与章节摘要。事实列表支持候选/确认/取代/失效状态与审核操作；
伏笔列表支持状态流转；摘要列表展示内容哈希与 stale 标记。

### 5.4 角色与关系

角色卡片列表 + 编辑抽屉；关系图用 AntV X6：节点是角色，边是关系；支持拖拽布局与表单保存关系。
删除角色二次确认。

### 5.5 世界观设定

左侧分类（全部/世界背景/规则/地点/物品/事件，带计数）+ 关键词搜索；
右侧设定卡片展示类型、摘要、更新时间；编辑抽屉支持名称/类型/内容/排序。

### 5.6 知识库

上传区支持拖拽；列表显示索引状态（pending/indexing 轮询）；检索支持关键词与词法混合，
无向量选项；召回结果展示命中内容；原文明细可预览：txt/md 直接渲染，
docx/pdf 用组件渲染。检索结果展示 query trace，让 AI 依据可见。

### 5.7 模型供应商

顶部能力提示：对话是否就绪、如何配置。卡片展示协议、地址、模型、密钥掩码、默认标记、
测试状态。抽屉表单可新增/编辑；一键测试与设为默认。

### 5.8 管理页

- 用户：分页列表、创建/编辑/删除、重置密码、角色分配。
- 角色：列表、权限码勾选、内置角色只读。
- 审计：按操作者/动作/时间筛选。
- 用量：7/14/30 天切换、指标卡、ECharts 双轴趋势、供应商与模型分布表。
- 运行状态：进程信息与依赖健康表；失败时显示错误页与重试入口，绝不空白。

### 5.9 设置与技能

设置页展示可修改的运行时配置白名单（`chat_history_count`、`rag_top_k`）。
斜杠技能页展示内置/自定义技能表格与提示词模板编辑器。

## 6. 可视化与编辑器分工

| 库 | 用途 |
| --- | --- |
| ECharts 5.5 | 字数趋势、用量趋势 |
| AntV X6 2.18 | 角色关系图 |
| CodeMirror 6 | 章节正文编辑 |
| WangEditor 5 | 创作笔记 |
| XLSX | 大纲导出 |
| docx-preview | docx 预览 |
| vue-pdf-embed | PDF 预览 |

## 7. 国际化

Vue I18n 9.10，默认 zh-CN，顶栏切换 en-US。文案按模块组织：`common.*`、`layout.*`、
`dashboard.*`、`workspace.*`、`knowledge.*` 等。时间使用毫秒时间戳或日期字符串本地化格式化。

## 8. 前端测试

Vitest 2.1，只覆盖纯逻辑且易回归的核心链路，当前保留 SSE 事件解析一条用例
（跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、尾部半行留存）。

```bash
cd frontend
npm run test
```

组件渲染、样式存在性一类低信息量断言不保留；类型正确性与构建产物由
`npm run build`（`vue-tsc && vite build`）保证。
