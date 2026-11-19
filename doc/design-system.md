# 设计系统与交互规范

本文定义 NovelWorkbench 前端的视觉令牌、布局、组件、按钮五态、动效、通知与可访问性规则。
实现入口：`frontend/src/styles/variables.css`、`global.css`、`button-states.css`，
以及 `frontend/src/components/` 下的无业务组件。

## 1. 设计关键词

**安静的写作桌**：低饱和背景、单一品牌主色、清晰层级、克制的阴影。
**可感知的操作**：按钮五态、卡片微抬、路由过渡、未读徽标、加载与空态。
**信息密度可控**：管理页表格化，创作页分栏，长文编辑不抢焦点。

## 2. 品牌五色

| 令牌 | 色值 | 用途 |
| --- | --- | --- |
| `--saas_dashboard-color-1` | `#6366f1` | 主色：主要按钮、选中项、链接、图表主线 |
| `--saas_dashboard-color-2` | `#818cf8` | 主色提亮：悬停、渐变终点、次要图表 |
| `--saas_dashboard-color-3` | `#e0e7ff` | 主色浅底：选中背景、悬停底色、标签 |
| `--saas_dashboard-color-4` | `#f8fafc` | 页面底色：内容区、输入底 |
| `--saas_dashboard-color-5` | `#1e293b` | 主文字与深色渐变起点 |

派生语义变量集中在 `:root`，禁止在页面里散写十六进制：

| 类别 | 变量 |
| --- | --- |
| 主色 | `--primary` / `--primary-hover` / `--primary-active` / `--primary-light` / `--primary-disabled` |
| 背景 | `--bg-page` / `--bg-card` / `--bg-hover` |
| 文字 | `--text-main` / `--text-secondary` / `--text-placeholder` / `--text-inverse` |
| 边框 | `--border` / `--border-light` |
| 语义 | `--success` / `--warning` / `--error` / `--info` 及各自 `-bg` / `-text` / `-border` |
| 图表 | `--chart-series-1` / `--chart-series-2` |
| 阴影 | `--shadow-card` / `--shadow-card-hover` / `--shadow-button-hover` / `--shadow-button-pressed` |
| 圆角 | `--radius-base` / `--radius-lg` / `--radius-pill` |
| 动效 | `--transition-base: 0.2s ease` |

## 3. 布局

### 3.1 工作台外壳

```
┌──────────────┬────────────────────────────────────────────┐
│ 品牌          │ 折叠  面包屑        搜索 铃铛 语言 头像       │
│ 项目切换器    ├────────────────────────────────────────────┤
│              │                                            │
│ 导航分组      │ 页面内容（最大宽度由页面决定）                │
│  - 页面       │                                            │
│  - 页面       │                                            │
│              │                                            │
└──────────────┴────────────────────────────────────────────┘
```

- 侧边栏宽 232px，折叠后 68px；过渡 `width 0.2s ease`。
- 顶栏高 56px，固定不滚动；内容区独立滚动。
- 页面内容默认 padding 20px；表单页限制最大宽度（个人中心 920px，通知 980px）。
- 管理页统一「卡片 + 工具栏 + 表格 + 分页」四段式。

### 3.2 响应式

| 断点 | 行为 |
| --- | --- |
| `> 1200px` | 完整侧边栏 + 双列卡片 |
| `900–1200px` | 统计卡自动换行；表格横向滚动 |
| `< 900px` | 登录页隐藏品牌叙事；侧边栏可折叠；表单单列 |
| `< 720px` | 个人中心头部纵向排列；工具栏换行 |

## 4. 排版

| 层级 | 字号 | 字重 | 用途 |
| --- | --- | --- | --- |
| 页面标题 | 16px | 600 | 卡片标题、模块名 |
| 区块标题 | 14px | 600 | 表单分组、通知标题 |
| 正文 | 14px | 400 | 表格、表单、正文 |
| 辅助文字 | 12.5–13px | 400 | 说明、时间、提示 |
| 统计数字 | 24–28px | 700 | 仪表盘与任务卡 |
| 品牌标题 | 22–30px | 700 | 登录页、个人中心 |

字体栈：`PingFang SC, Microsoft YaHei, Helvetica Neue, Arial, sans-serif`。
正文行高 1.6–1.8；表格行高由 Element Plus 控制，不使用紧凑模式压信息。

## 5. 按钮五态

所有按钮（Element Plus `.el-button` 与自定义 `.nw-button`）必须覆盖五种状态：

| 状态 | 触发 | 视觉变化 | 实现 |
| --- | --- | --- | --- |
| DEFAULT | 静止 | 品牌色 / 白底描边 / 文字色 | 基础样式 |
| HOVER | 鼠标悬停 | `scale(1.05)` + 提亮 + 投影加深 | `:hover` |
| PRESSED | 鼠标按下 | `scale(0.95)` + 颜色加深 + 投影收敛 | `:active` |
| DISABLED | `disabled` | 透明度 0.5、无缩放、`not-allowed` | `.is-disabled` / `[disabled]` |
| LOADING | `loading` | spinner + 微光扫过 + `pointer-events: none` | `.is-loading` / `.nw-button--loading` |

规则：

- 过渡统一 `all 0.2s ease`；禁用态关闭过渡，避免闪动。
- 悬停放大与按下缩小必须同时存在，形成明确点击感。
- 图标按钮同样缩放，但幅度更小（1.06 / 0.94）。
- 加载态保留按钮宽度，禁止布局跳动。
- 危险操作使用 danger / danger-text 变体；表格内次要危险操作用 danger-text。

## 6. 组件规范

### 6.1 基础组件

| 组件 | 职责 | 约束 |
| --- | --- | --- |
| `NWButton` | 统一按钮外观与五态 | 不承载业务判断；`loading` 与 `disabled` 互斥 |
| `NWCard` | 卡片容器 | 默认悬停微抬；配置表单用 `hoverable=false` |
| `NWEmpty` | 空态 | 必须提供可读描述 |
| `NWLoading` | 加载态 | 居中 spinner + 文案 |
| `NWIcon` | 后端图标编码渲染 | 24×24 网格、线性描边、`currentColor` |
| `ProjectPageHeader` | 项目页标题与动作区 | 项目名 + 描述 + 右侧按钮 |

### 6.2 Element Plus 使用约定

- 表格：`el-table` + `stripe`；空态用 `#empty` 插槽放 `NWEmpty`。
- 分页：`layout="total, prev, pager, next, sizes"`，右对齐。
- 表单：`label-position="top"`；必填与格式规则走 `FormRules`。
- 弹窗：短表单用 `el-dialog`，复杂编辑用 `el-drawer`；危险操作前 `ElMessageBox.confirm`。
- 标签：状态用 `el-tag effect="light"`；类型色遵循语义（success / warning / danger / info）。
- 描述列表：账号信息用 `el-descriptions border`。

### 6.3 反馈

| 场景 | 反馈 |
| --- | --- |
| 保存成功 | `ElMessage.success` + 列表刷新 |
| 参数错误 | 表单内联校验 + 后端 `resp_message` |
| 网络失败 | Axios 层统一 `ElMessage.error`，页面保留旧数据 |
| 未授权 | 清理 token、提示、跳登录 |
| 删除 | 二次确认，说明级联影响 |
| 异步任务 | 通知中心 + 任务中心状态标签 |
| 空列表 | 空态插画/图标 + 一句可行动文案 |

## 7. 动效

| 场景 | 动效 | 时长 |
| --- | --- | --- |
| 路由切换 | fade + 上下位移 12px | 0.2s |
| 按钮悬停/按下 | scale 1.05 / 0.95 | 0.2s |
| 卡片悬停 | translateY(-4px) + 阴影加深 | 0.2s |
| 导航项悬停 | 背景变化 + scale 1.015 | 0.2s |
| 通知浮层项 | 背景变化 + translateX(2px) | 0.2s |
| 加载 spinner | 0.8s 线性旋转 | 循环 |
| 按钮加载微光 | 1.2s 往复扫过 | 循环 |

禁止：大幅弹跳、长时间阻塞动画、影响布局的动画。

## 8. 通知与未读

- 顶栏铃铛显示未读数，超过 99 显示 `99+`。
- 点击铃铛打开最近 5 条通知浮层；未读项左侧圆点高亮。
- 浮层内可全部已读；点击单条标记已读并跳转 `link_url`。
- 通知中心页支持全部 / 未读 / 已读、类型筛选、关键词搜索、删除。
- 未读数由顶栏 `NotificationBell` 每 60 秒轮询；标记已读后立即本地扣减并刷新。

## 9. 可访问性

- 所有可交互元素可键盘聚焦，`:focus-visible` 显示 2px 品牌色描边。
- 图标按钮必须有 `aria-label` 或可见文字。
- 颜色不是唯一信息载体：状态同时有文字标签。
- 对比度：正文文字与背景 ≥ 4.5:1；辅助文字 ≥ 3:1。
- 加载态使用 `aria-hidden` 装饰 spinner，文案说明正在发生什么。

## 10. 国际化

- 语言包分片：common / workspace / admin / extensions，中英各一份。
- 新增文案必须同时补齐 `zh-CN-*` 与 `en-US-*`。
- 键名使用业务命名空间：`profile.*`、`notifications.*`、`admin.jobs.*`。
- 后端返回的业务文案保持中文；前端负责界面标签与提示。

## 11. 禁止事项

- 禁止在页面内硬编码品牌色十六进制；必须使用 CSS 变量。
- 禁止只做悬停态而缺失按下态。
- 禁止用 `alert` / `confirm` 浏览器原生弹窗。
- 禁止静默吞掉失败；至少保留旧数据并给出提示。
- 禁止在列表页一次性拉全量数据；统一分页。
- 禁止在表格里放超过两个主操作；次要操作收进更多菜单。
