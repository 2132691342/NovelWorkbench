# 前端设计

## 1. 工程结构

```
frontend/
├── src/
│   ├── api/          http.ts（axios 实例 + 包络解包）与各模块 api 封装
│   ├── components/   NWButton / NWCard / NWEmpty / NWLoading 等无业务组件
│   ├── layout/       AdminLayout（顶栏 + 侧边菜单 + 用户区）
│   ├── router/       路由表 + 登录守卫
│   ├── stores/       auth（token/用户）、app（locale/布局）
│   ├── styles/       variables.css（五色主题）、button-states.css（按钮五态）
│   ├── i18n/         zh-CN / en-US 文案
│   └── views/        login / dashboard / workspace / character / knowledge / notes / settings / slash-admin
├── vite.config.ts    别名 @→src、/api 代理、手动分包
└── uno.config.ts     UnoCSS（presetUno + transformerDirectives）
```

## 2. 设计系统

### 2.1 色彩

| 变量 | 值 | 用途 |
| --- | --- | --- |
| --saas_dashboard-color-1 | #6366f1 | 主色（品牌、主按钮、选中态） |
| --saas_dashboard-color-2 | #818cf8 | 主色浅阶（悬停、次强调） |
| --saas_dashboard-color-3 | #e0e7ff | 背景点缀（浅底、标签） |
| --saas_dashboard-color-4 | #f8fafc | 页面底色 |
| --saas_dashboard-color-5 | #1e293b | 主文字 |

语义派生变量（`--primary/--primary-hover/--bg-page/--text-main/--border/--success/--warning/--error` 等）由五色推导，组件一律引用语义变量，换肤只改变量层。

### 2.2 按钮五态（核心体验规范）

| 状态 | 触发 | 表现 |
| --- | --- | --- |
| DEFAULT | 常态 | 主色底/白字，阴影 1 级 |
| HOVER | 悬停 | 放大 `scale(1.05)`，颜色过渡到 --primary-hover，阴影加深 |
| PRESSED | 按下 | 缩小 `scale(0.95)`，颜色变深，阴影收敛（点击感） |
| DISABLED | 禁用 | 透明度降低、灰化、`pointer-events: none`、无动效 |
| LOADING | 加载 | 转圈 spinner、禁止点击、微光扫过，保持按下前的尺寸 |

所有状态共享 `transition: 0.2s ease`（颜色/变换/阴影）；实现为全局样式（覆盖 Element Plus `.el-button`）+ `NWButton` 组件复用同一套类，任何页面不得另写按钮状态样式。

### 2.3 交互反馈

- 操作成功/失败一律 `ElMessage` 提示（含错误码对应的可读文案）。
- 路由切换 fade-slide 页面过渡；卡片悬停微抬起；列表有加载骨架与空态（NWEmpty）。
- 表单提交期间按钮进入 LOADING 态并禁用重复提交；危险操作（删除）二次确认。

## 3. 状态管理与 API 层

| 模块 | 职责 |
| --- | --- |
| stores/auth | access_token 与 user_vo 持久化 localStorage；login/register/logout action |
| stores/app | locale（zh-CN/en-US）、侧边栏折叠 |
| api/http | baseURL `/api/v1`；请求拦截注入 Bearer；响应拦截按 `{resp_code, data}` 解包，非 SUCCESS reject(`resp_message`)；`ERR_UNAUTHORIZED` 清 token 跳登录 |

页面组件只消费解包后的业务数据；错误提示统一在拦截器与页面 catch 里出，杜绝静默失败。

## 4. 路由与守卫

| 路径 | 页面 | 说明 |
| --- | --- | --- |
| /login | 登录/注册 | 免鉴权 |
| / | dashboard | 统计总览 |
| /workspace/:project_ulid | 写作工作台 | 目录树 + 编辑器 + AI 面板 |
| /character/:project_ulid | 角色管理 | 角色卡 + X6 关系图 |
| /knowledge/:project_ulid | 知识库 | 上传/检索/预览（docx/pdf） |
| /notes/:project_ulid | 创作笔记 | WangEditor 富文本 |
| /settings | 设置 | 运行时配置 |
| /slash-admin | 斜杠命令 | 技能资产管理 |

守卫：无 token 访问受保护路由 → 跳 `/login`（带 redirect 回跳）；已登录访问 `/login` → 重定向 `/`。

## 5. 关键页面交互

| 页面 | 交互要点 |
| --- | --- |
| 登录 | 登录/注册双 Tab、字段校验、提交 LOADING、成功提示跳转 |
| 仪表盘 | 4 统计卡 + ECharts 近 30 天双系列（人工/AI 字数）+ 项目列表（新建/编辑/删除，编辑支持状态与概要清空） |
| 工作台 | 左：卷/章树（下拉菜单增删改、XLSX 大纲导出）；中：CodeMirror 编辑器（自动保存、字数实时统计、快照/回滚）；右：AI 面板（流式打字、斜杠自动补全、@引用选择器、历史条数设置、召回依据展示、"插入到章节"） |
| 角色 | 卡片列表 + 编辑抽屉 + X6 关系图（拖拽节点布局、表单保存关系边） |
| 知识库 | 上传（拖拽）、索引状态徽标（pending/indexing 3s 轮询）、混合检索（高亮命中）；txt/md 原文预览、docx/pdf 原文渲染 |
| 斜杠命令 | 表格 + 弹窗表单（提示词模板编辑器）、内置技能只读徽标 |

## 6. 可视化与工具库分工

| 库 | 用途 |
| --- | --- |
| ECharts 5.5 | 字数趋势、统计图 |
| AntV X6 2.18 | 角色关系图（节点=角色、边=关系） |
| CodeMirror 6 | 章节正文编辑（轻量、可控、中文友好） |
| WangEditor 5 | 创作笔记富文本 |
| XLSX | 写作台目录导出（卷/章/排序/字数/状态，按需加载） |
| docx-preview / vue-pdf-embed | 知识库文档在线预览 |

未采用：Qiankun（无微前端子应用，引之徒增运行时复杂度）、BPMN-JS（剧情流程用目录树 + 时间线已覆盖）、Mavon-Editor（Markdown 编辑由 CodeMirror 承担）。当前无正式用户，宁可少一个依赖也不留死代码。

## 7. 国际化

- Vue I18n 9.10，locale 文件按页面组织（`login.*`、`dashboard.*`…），键名语义化。
- 默认 zh-CN，顶栏可切换 en-US；数字/日期用毫秒时间戳本地化格式化。
