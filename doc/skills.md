# 提示词资产：流程域蒸馏矩阵

本文是一份**资产清单**：系统内置的提示词资产从哪些方法论蒸馏而来、每个流程域交付什么、编码如何对应到界面与 `/命令`。
它回答"为什么这些提示词长这样"，实现细节见 [ai.md](ai.md)。

## 1. 组织方式：一个流程域 = 1 份大提示词 + 2 个技能

写作是一条流水线，每个环节要的东西不同：立项要判断，起草要产出，打磨要克制，质检要狠。
所以内置资产按**流程域**组织，而不是按"提示词长什么样"组织：

| 层 | 落库位置 | 回答什么 | 什么时候用 |
| --- | --- | --- | --- |
| 大提示词（档案） | `prompt_profiles` | 我是谁、我的底线、我绝不写什么 | 整个会话期间持续生效，可临时切换 |
| 技能 | `skills` | 这一次具体做什么、按什么格式交付 | 由 `/编码` 或界面按钮单次触发 |

一个域内两者是配套的：档案定"以什么身份说话"，技能定"这一单活怎么干"。
配合上下文装配（当前章节、@引用、召回、角色卡、设定、历史），一次请求就构成完整作业单。

### 1.1 域清单

按创作作业顺序排列；`sort_order` 即界面与 `/` 菜单的排序。

| # | 流程域 | 大提示词（档案编码 → 名称） | 技能 1 | 技能 2 |
| --- | --- | --- | --- | --- |
| 1 | 正文起草 `draft` | `novelist` → 长篇连载作者（默认档案） | `/continue` 续写正文 | `/conflict` 冲突加码 |
| 2 | 故事架构 `architect` | `architect` → 故事架构师 | `/outline` 搭建大纲 | `/worldrule` 世界观规则卡 |
| 3 | 文字打磨 `style` | `stylist` → 文字打磨师 | `/polish` 润色文字 | `/dialogue` 对白打磨 |
| 4 | 去 AI 味 `deai` | `deai` → 去 AI 味修订 | `/deai` 去 AI 味 | `/humanness` 活人感改写 |
| 5 | 发布质检 `review` | `editor` → 连载责任编辑 | `/review` 章节质检 | `/hitscan` 量化门禁 |
| 6 | 连续性 `continuity` | `continuity` → 设定守门人 | `/foreshadow` 伏笔台账 | `/ledger` 连续性台账 |
| 7 | 定稿归档 `finalize` | `final_editor` → 定稿编辑 | `/summary` 章节梗概 | `/finalcheck` 终审对账 |
| 8 | 上架运营 `operation` | `serial_editor` → 连载平台主编 | `/golden3` 开篇体检 | `/platform` 平台适配 |
| 9 | 立项评估 `pitch` | `pitch_analyst` → 选题策划 | `/pitch` 立项评估 | `/deconstruct` 拆书 |

合计 **9 份档案 + 18 个技能**。默认档案是 `novelist`（`sort_order=10`，最小者），未指定档案的对话都用它。

### 1.2 技能编码与前端入口的契约

`/continue`、`/polish`、`/summary`、`/deai`、`/review` 这五个编码被写作台快捷按钮与 `Skills` 页示例直接引用，
`internal/ai/builtin_assets_test.go` 会断言它们始终存在。改编码等于改前端契约，必须同步改前端。

## 2. 蒸馏来源与方法

资产来自 `C:\Users\LIKX\.workbuddy\skills` 下 38 个中文写作技能包的蒸馏，**不是原文照搬**：

| 处理 | 做法 |
| --- | --- |
| 合并同类 | 番茄系（`tomato-*`、`fanqie-*`、`qimao-*`）合并为"平台运营"一条口径，不再区分平台名 |
| 保留判据 | 只保留**可验收**的判据（填不满卖点即卖点不清、没有边界的规则不算规则）；删掉口号与自我描述 |
| 去掉工具依赖 | 依赖脚本、文件写入、多轮 Agent 的流程（如批量推进、插章重编号）不进提示词 |
| 收敛长度 | 单个技能模板控制在 800–1600 字；超长方法论拆成"技能 + 参考资料"两层 |
| 去人格化 | 删除"你是全网第一""封神"等角色包装，只留作业纪律 |

### 2.1 来源对照表

| 流程域 | 主要来源技能包 | 蒸馏进资产的部分 |
| --- | --- | --- |
| 正文起草 | `conflict-max-web-fiction`、`cn-novel-craft`、`my-novel-writer` | 冲突核与代价递增链、MRU 刺激-反应单元、章节前后状态 |
| 故事架构 | `novel-outline`、`serial-novel-architect`、`cn-novel-craft` | 六层结构（卖点/世界观/主线/人物/分卷/章纲）、规则卡三栏、Save the Cat 节拍 |
| 文字打磨 | `novel-polish`、`fiction-polish-distill`、`human-writing` | 职责边界（只改怎么说）、术语锁定、比喻三原则、真实人声 |
| 去 AI 味 | `de-ai-novel`、`deai-promax`、`deai-rewriter`、`human-writing` | AI 味四定义、三层扫描（词面/语义/统计）、34 模式与 4 大死亡气味 |
| 发布质检 | `webnovel-review`、`cn-novel-craft`、`final-review` | 六道检查、16 维框架、P0/P1/P2 分级与改法要求 |
| 连续性 | `serial-novel-architect`、`cn-novel-craft`、`de-ai-novel` | 四张管理表（主线锚点/知识状态/时间线/未收悬念）、伏笔登记册 |
| 定稿归档 | `final-review`、`cn-novel-craft`、`novel-deai-tomato-pass` | 两阶段终审顺序、章节 spec 前后状态、发布前自检清单 |
| 上架运营 | `tomato-web-novel-master`、`fanqie-bestseller`、`web-novel-writer`、`fanqie-publish` | 黄金三章硬指标、情绪需求金字塔、平台形态与字数门槛 |
| 立项评估 | `novel-outline`、`fanqie-bestseller`、`tomato-web-novel-master`、`novel-deconstruction` | 卖点判据、赛道与差异化、八师拆书法（溯源与事实/推测分离） |

### 2.2 未纳入的资产

以下技能包的能力**刻意不做进提示词**，因为它们是工具流程而非写作判断：

| 未纳入 | 原因 | 现有替代 |
| --- | --- | --- |
| 批量推进 / 插章重编号 | 需要文件读写与多轮调度，本系统不做 Agent 循环 | 作者逐章触发；连续性看板记台账 |
| 平台投稿与签约操作 | 依赖平台后台人工操作与账号 | `/platform` 只给适配方案与核对清单 |
| 数据复盘与上榜诊断 | 需要平台真实后台数据 | `/golden3` 与 `/pitch` 给可自查的硬指标 |
| 短剧/剧本/分镜改编 | 不在本产品范围 | — |

## 3. 版本与作者改动

内置资产是**产品资产**，同时允许作者改：两者靠内容指纹区分。

| 字段 | 含义 |
| --- | --- |
| `asset_hash_text` | 该行对应的出厂内容指纹（sha256，64 位十六进制） |
| `customized_flag` | 作者是否改过出厂内容 |

启动时 `SeedDefaults` 的判断顺序：

1. 编码不存在 → 插入，写入出厂指纹。
2. `customized_flag = true` → **不动**（尊重作者改稿）。
3. `asset_hash_text` 与当前代码指纹一致 → **不动**（已是最新）。
4. 其余 → 覆盖名称、说明、模板、排序号，并写入新指纹（出厂内容升级）。

保存时（`SkillService.Save` / `PromptProfileService.Save`）如果提交内容与当前代码指纹不同，
`customized_flag` 置 `true`，此后代码升级不再覆盖这一行；把内容改回出厂版本会重新置回 `false`。

下线机制：内置编码从清单移除后，启动时连带删除库中该编码的内置行（技能还会级联删除参考资料）。
用户自建的技能与档案永不被清理。

## 4. 维护这份资产

1. 改提示词 → 只改 `backend/internal/ai/domain_<域>.go` 里对应字段，一个域一个文件（单文件 ≤500 行）。
2. 加一个域 → 新建 `domain_<域>.go`，在 `builtin_assets.go` 的 `builtinDomainList` 注册；**必须正好一份档案 + 两个技能**。
3. 改完跑 `go test ./internal/ai/`：形状、编码唯一、占位符合法、指纹长度都会被断言。
4. 上线后无需清库：指纹不同即自动升级，作者改过的行保持原样。
5. 本文的域清单与来源对照表必须同步更新，否则视为文档缺失。
