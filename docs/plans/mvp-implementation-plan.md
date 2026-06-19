# AutoCV MVP 实施计划

> 状态：可执行 v0.1
>
> 日期：2026-06-10
>
> 产品规格：[`docs/product/autocv-mvp-product-spec.md`](../product/autocv-mvp-product-spec.md)
>
> 技术架构：[`docs/architecture/mvp-architecture.md`](../architecture/mvp-architecture.md)

## 1. 目标

按垂直闭环开发 AutoCV，而不是先分别建设完整资料库、完整 Agent 系统或完整编辑器。

第一个可演示版本必须完整走通：

```text
Markdown 职业资料
  -> 粘贴 JD
  -> JD 分析
  -> 资料匹配
  -> 生成 Markdown 简历
  -> Typst PDF
  -> 应用内预览和导出
```

随后再补齐追问、来源追溯、锁定、DOCX/PDF 导入和恢复能力。

## 2. 实施原则

- 每个里程碑都交付可运行的用户流程。
- 先使用合成样本和 Fake Provider，之后再接真实 AI。
- Go 持有业务状态，前端不复制工作流规则。
- AI 输出先进入结构化 Schema，再生成 Markdown 和 PDF。
- 所有数据库变化使用迁移。
- 私有简历和 JD 不进入 Git。
- 不为第二阶段 Job Radar 提前建设模块。
- 不在 MVP 中引入 Agent 框架、向量数据库或富文本编辑器。

## 3. 完成定义

一个任务只有同时满足以下条件才算完成：

- 行为符合产品规格。
- 有适当的单元、合约或集成测试。
- 错误状态在界面可理解。
- 不在日志中泄露完整用户内容。
- 数据重启后仍然存在。
- 相关文档或 ADR 已更新。
- `go test ./...`、前端检查和构建通过。

## 4. 里程碑总览

| 里程碑 | 用户可见结果 |
| --- | --- |
| M0 工程基线 | 桌面应用可启动，数据库和测试框架可用 |
| M1 首个垂直闭环 | Markdown + JD 可以生成并预览 PDF |
| M2 可复用资料库 | 多文档 Profile、Evidence 和本地检索可用 |
| M3 可审阅生成 | 追问、三档包装、来源追溯和锁定可用 |
| M4 输入与输出完整 | DOCX/PDF 导入、稳定 PDF 和本地导出删除可用 |
| M5 MVP 验收 | 中英文真实样本通过产品成功标准 |

## 5. M0：工程基线

状态：已完成（2026-06-10）

### 5.1 目标

建立能够持续开发和验证的 Wails 桌面项目，不实现业务闭环。

### 5.2 任务

- [x] 初始化 Go module 和 Wails 3 React/TypeScript 项目。
- [x] 锁定 Go、Node、Wails 和前端包版本。
- [x] 建立 `internal/app`、`domain`、`ports`、`adapters` 和 `workflow` 目录。
- [x] 建立 SQLite 连接和迁移执行器。
- [x] 添加首个 schema migration。
- [x] 实现操作系统应用数据目录解析。
- [x] 建立结构化日志和默认脱敏规则。
- [x] 建立配置读取、保存和版本字段。
- [x] 建立 Fake Provider。
- [x] 建立测试使用的临时数据库和合成样本。
- [x] 建立 Go、TypeScript、格式化和测试命令。
- [x] 添加 `.gitignore`，排除私有样本、数据库、日志、Run 和导出目录。

### 5.3 最小数据表

- `profiles`
- `source_documents`
- `job_descriptions`
- `resume_runs`
- `stage_results`
- `resumes`
- `artifacts`
- `provider_configs`

其他表在对应能力进入开发时通过后续 migration 添加。

### 5.4 验收

- 开发模式能够启动 Wails 窗口。
- 前端可以通过 Binding 调用 Go 健康检查。
- 数据库首次启动自动创建，第二次启动不重复迁移。
- Fake Provider 能返回固定的 JD Analysis。
- `go test ./...` 和前端类型检查通过。

## 6. M1：首个垂直闭环

### 6.1 目标

用最少功能证明 AutoCV 的完整价值链和架构。

限制：

- 一个 Profile。
- 只导入 Markdown。
- 只粘贴 JD。
- 只支持平衡包装档。
- 只提供一个 PDF 模板。
- 暂不实现追问、锁定和 DOCX/PDF 导入。

### 6.2 Slice 1：Markdown 资料导入

任务：

- [x] 创建默认 Profile。
- [x] 通过原生文件选择器选择 Markdown。
- [x] 保存受管理副本、Hash 和导入记录。
- [x] 按标题和段落切分 Source Chunk。
- [x] 使用 Fake Provider 提取 Evidence。
- [x] 提供导入结果和警告界面。

验收：

- 重复导入同一文件能通过 Hash 识别。
- 重新启动应用后仍可查看文档和 Evidence。
- Evidence 可以跳转到来源 Chunk。

测试：

- Markdown 标题、列表、中英文混合内容。
- 空文件和超大段落。
- 导入中断不会产生半条业务记录。

### 6.3 Slice 2：JD 分析

任务：

- [x] 提供 JD 粘贴工作区。
- [x] 保存原始 JD。
- [x] 定义并校验 JD Analysis Schema。
- [x] 接入 Fake Provider 分析。
- [x] 展示职责、必要技能、加分项和筛选条件。

验收：

- 原始 JD 和结构化结果同时可见。
- Schema 错误有明确阶段错误。
- 编辑 JD 后旧分析结果失效。

### 6.4 Slice 3：匹配与分数

任务：

- [x] 定义 Requirement、Match 和 Evidence 关联。
- [x] 让 Provider 提供语义匹配建议。
- [x] 在 Go 中计算确定性总分和分项分数。
- [x] 展示强匹配、弱匹配、缺失和未知项。
- [x] 实现硬性条件缺失时的 69 分上限。

验收：

- 每个得分项都能找到 Requirement 和 Evidence。
- 修改 Evidence 或 JD 后分数重新计算。
- Provider 直接返回的任意总分不会被采用。

### 6.5 Slice 4：Resume 生成

任务：

- [x] 定义 Resume 和 Resume Block Schema。
- [x] 基于匹配结果生成结构化 Resume。
- [x] 验证每个关键 Block 的来源。
- [x] 从 Resume 派生 Markdown。
- [x] 提供 Markdown 查看和简单编辑。
- [x] 展示主要优化说明。

验收：

- 生成内容不存在无来源的具体成果数字。
- Markdown 可以重新打开。
- 编辑后能够保存一个新 Resume 版本。

### 6.6 Slice 5：Typst PDF

任务：

- [x] 建立 Resume 到 Typst View Model。
- [x] 创建单栏 ATS 友好模板。
- [x] 配置中英文字体。
- [x] 实现本地 Typst 执行、超时和错误捕获。
- [x] 保存 PDF Artifact。
- [x] 在应用中预览，并导出 Markdown/PDF。

验收：

- 合成中文和英文简历均能生成 PDF。
- PDF 文本可复制。
- 预览和导出使用同一个 Artifact。
- 渲染失败不覆盖上一次成功 PDF。

### 6.7 Slice 6：真实 OpenAI Provider

任务：

- [x] 实现 Provider 配置界面。
- [x] API Key 写入系统 Keychain。
- [x] 接入 OpenAI 官方 Go SDK。
- [x] 为每个任务使用独立结构化 Schema。
- [x] 实现超时、取消、有限重试和用量元数据。
- [x] 实现 Schema 修复一次的策略。
- [x] 在发起请求前显示 Provider 和发送内容类型摘要。

验收：

- Fake Provider 和 OpenAI Provider 可以通过配置切换。
- 数据库和日志不出现明文 API Key。
- 取消请求后 Run 可继续重试。
- 模型返回非结构化内容时不会直接进入业务数据。

### 6.8 M1 退出条件

- 从 Markdown 和 JD 到 PDF 的完整流程可以在一个窗口内完成。
- 关闭应用后可以重新查看 Profile、JD、Resume 和 PDF。
- 使用一组真实资料本地演示成功。
- 核心流程不依赖手工运行额外脚本。

## 7. M2：可复用资料库

状态：进行中（2026-06-13）

### 7.1 目标

让 Profile 从“一份 Markdown”升级为可长期维护的本地资料库。

### 7.2 任务

- [x] 支持多个 Profile。
- [x] 支持一个 Profile 中的多份 Source Document。
- [x] 增加 `source_chunks`、`evidence` 和 `evidence_sources` 表。
- [x] 建立 FTS5 索引。
- [x] 支持 Evidence 的查看、编辑和用户确认。
- [x] 处理冲突信息和重复 Evidence。
- [x] 允许为一次 Run 选择资料范围。
- [x] 增加 Profile 导出。

### 7.3 验收

- 新 JD 可以复用已有 Evidence，不重复导入资料。
- 用户修正后的 Evidence 不被后续提取静默覆盖。
- 搜索结果能返回来源文档和片段。
- 删除文档后相关 Evidence 和引用按规则处理。

## 8. M3：可审阅生成

### 8.1 Slice 1：追问

- [x] 增加 Clarification Question Schema 和表。
- [x] 根据产品规则生成最多 5 个问题。
- [x] 支持回答、跳过和第二轮追问。
- [x] 把用户确认写入 Evidence 或 Run Confirmation。
- [x] 跳过后自动降低对应表述强度。

验收：

- 已有资料中存在答案时不重复提问。
- 最多两轮，用户可以随时继续生成。
- 未确认数字不会进入简历。

### 8.2 Slice 2：三档包装

- [x] 把保守、平衡、强化定义为结构化策略。
- [x] Prompt 和 Review 同时接收策略。
- [x] 为三档建立固定测试样本。
- [x] 展示不同档位的影响说明。

验收：

- 三档输出有可解释差异。
- 强化档仍不会新增技术、职责或数字。

### 8.3 Slice 3：来源追溯

- [x] 增加 `block_sources`。
- [x] 在 Resume Studio 显示关键 Block 的来源。
- [x] 支持跳转到 Source Chunk。
- [x] 标识来源、归纳和用户确认三种 Grounding Level。
- [x] 高风险无来源 Block 阻止最终导出。

### 8.4 Slice 4：内容锁定

- [x] 支持 Block 级锁定。
- [x] 重新生成时把锁定块作为不可修改输入。
- [x] Go 侧比较并拒绝被修改的锁定块。
- [x] 上游变化与锁定块冲突时给出提示。

验收：

- 单条 bullet 锁定后重新生成保持逐字不变。
- 锁定不阻止其他未锁定内容更新。

### 8.5 Slice 5：工作流恢复

- [x] 完成完整 Stage 状态机。
- [x] 增加输入 Hash 和下游失效规则。
- [x] 支持取消、失败重试和从指定阶段重跑。
- [x] 启动后恢复未完成 Run。
- [x] 前端通过事件显示实时状态，通过查询恢复状态。

## 9. M4：输入与输出完整

### 9.1 DOCX

- [x] 选定并锁定 OOXML 解析方案。
- [x] 提取正文、标题、列表和表格文本。
- [x] 输出解析警告。
- [x] 使用至少一份中文和一份英文 DOCX 验证。

### 9.2 文本型 PDF

- [x] 选定并锁定 PDF 文本提取方案。
- [x] 保留页码定位。
- [x] 检测扫描件或低质量文本层。
- [x] 明确提示不支持 OCR。
- [x] 使用至少三种不同来源 PDF 验证。

### 9.3 PDF 质量

- [x] 校准中文和英文字体。
- [x] 检查链接、分页、孤行和标题断页。
- [x] 增加两页警告。
- [x] 与 `kami` 参考样式做人工比较。
- [x] 固定 Typst 和模板版本。

### 9.4 数据控制

- [x] Profile 完整导出。
- [ ] Profile、JD、Run 和 Artifact 删除。
- [ ] 删除前影响范围预览。
- [ ] 数据库备份和恢复的最小能力。
- [ ] 诊断包默认脱敏。

## 10. M5：MVP 验收与发布

### 10.1 合成测试

- [x] 仓库内提供不包含真实个人信息的中英文样本。
- [ ] CI 完成 Go 单元测试、前端检查和 Typst 模板测试。
- [x] Fake Provider 端到端流程稳定通过。

### 10.2 私有真实样本

建立本地且被 Git 忽略的样本目录，至少准备：

- 一组中文简历 + JD。
- 一组英文简历 + JD。
- 一组资料不足或容易过度包装的简历 + JD。

记录：

- 首稿生成时间。
- 审阅时间。
- 修改的 Resume Block 比例。
- 发现的事实风险。
- PDF 问题。
- 匹配分是否有帮助。

### 10.3 发布门

必须满足产品规格第 12 节成功标准，并完成：

- [ ] macOS 开发构建和发布构建。
- [ ] 新安装启动测试。
- [ ] 数据迁移测试。
- [ ] 无 API Key 状态测试。
- [ ] 网络失败和 Provider 失败测试。
- [ ] 私有数据删除测试。
- [ ] README 安装、配置和限制说明。
- [ ] 第三方依赖和 Typst 许可检查。

## 11. 初始工作项顺序

建议按以下顺序创建首批 Issue：

1. 初始化 Wails 3 + React/TypeScript 工程。
2. 建立目录、构建命令和 CI 基线。
3. 建立 SQLite migration 和应用数据目录。
4. 定义核心 Domain 类型和 Run 状态。
5. 实现 Fake Provider 和结构化 Schema 校验。
6. 实现 Markdown 导入和 Source Chunk。
7. 实现 JD 粘贴和分析。
8. 实现 Match 与确定性评分。
9. 实现 Resume Schema 和 Markdown 输出。
10. 实现 Typst 模板、PDF 预览和导出。
11. 实现 OpenAI Provider 和 Keychain。
12. 使用首组真实资料验收 M1。

不要在第 12 项完成前开始：

- DOCX/PDF 解析。
- 多模板。
- Job Radar。
- 长期记忆。
- 向量检索。

## 12. 风险与控制

| 风险 | 控制 |
| --- | --- |
| AI 输出不稳定 | 结构化 Schema、一次修复、Fixture 合约测试 |
| 内容过度包装 | Go 侧内容政策、来源引用、用户确认、真实样本 |
| 匹配分误导 | 确定性计算、分项解释、硬条件上限 |
| PDF 质量不足 | Typst 模板、固定字体、Artifact 测试、人工对照 |
| PDF/DOCX 解析差异大 | 后置到 M4，使用真实格式样本选择解析器 |
| Wails/Typst 打包问题 | M0 和 M1 提前做发布构建，不等到最后 |
| 私有资料泄露 | `.gitignore`、Keychain、脱敏日志、诊断包检查 |
| 工作流状态复杂 | 显式状态机、阶段结果持久化、输入 Hash |
| 范围膨胀 | 以 M1 闭环和产品规格的排除清单为准 |

## 13. 暂不阻塞开发的待定项

以下内容在对应里程碑前完成 Spike 即可，不阻塞 M0：

- SQLite Go Driver 的最终选择。
- DOCX/PDF 解析库。
- OpenAI 具体模型。
- Keychain 封装库。
- Typst 字体组合。

Spike 必须输出：

- 选择结论。
- 最小验证代码或测试。
- 打包影响。
- 许可和维护风险。
- 对现有接口是否有影响。
