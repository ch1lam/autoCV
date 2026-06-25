# AutoCV PDF Kami Quality Review

> 日期：2026-06-19
>
> 范围：M4 PDF 质量，AutoCV 旧 Typst 简历模板与 `kami` 参考设计语言的人工对照。
>
> 2026-06-25 更新：产品方向已从“Typst 接近 Kami”切换为“内化 Kami-style HTML -> PDF 流程”。本文保留为旧基线复盘，新的实现不再接受“白底、Typst、仅视觉参考”作为目标差异。

## 1. 对照依据

本次复盘使用仓库内已实现的 Typst 模板和合成测试样本，不使用私有简历、真实 JD、导出 PDF 或本地数据库。

- 旧 AutoCV 模板：`internal/adapters/typst/templates/resume.typ`（已移除）
- 旧 AutoCV 渲染器：`internal/adapters/typst/renderer.go`（已移除）
- 旧合成验证：`internal/adapters/typst/renderer_test.go`（已移除）
- 产品约束：`docs/product/autocv-mvp-product-spec.md`
- 架构边界：`docs/architecture/mvp-architecture.md`
- kami 参考语言：温暖纸感、ink-blue accent、serif-led hierarchy、紧凑 editorial rhythm。

当前仓库没有可提交的 `kami` 参考 PDF 示例，因此本次对照以 `kami` skill 的设计语言说明作为质量参照，不提交生成产物。

## 2. 通过项

- **Serif-led hierarchy**：英文正文优先 `Charter`，中文正文使用 `Charter` + `Songti SC` fallback，符合 serif-led 的简历阅读气质。
- **Ink-blue accent**：标题、分隔线和链接使用蓝色系，保持克制的重点提示。
- **Editorial rhythm**：单栏结构、紧凑段落 leading、列表缩进和 section spacing 已统一，不依赖卡片化装饰。
- **ATS 友好**：保持白底、可选择文本、单栏结构和 ToUnicode 映射，不用背景纹理、图片文本或复杂装饰覆盖可读性。
- **中英文覆盖**：中文和英文共用数据结构，但正文和标题字体栈按语言分开。
- **链接质量**：旧实现会把 Markdown 链接和裸 URL 渲染为 PDF 链接，PDF 中保留真实 URL 目标。
- **分页质量**：Section 标题与首条内容不可拆分，单条内容也不跨页拆分，降低标题悬空和孤行风险。
- **篇幅控制**：超过两页时产生非阻塞提醒，不通过缩小到不可读字号强行压页。

## 3. 接受差异

- **不使用暖纸背景**：`kami` 的 warm parchment 适合成品文档气质，但 AutoCV MVP PDF 优先 ATS、打印和复制稳定性，因此继续使用白底。
- **不引入装饰性版式**：AutoCV 当前只提供一个 ATS 友好的单栏模板，不做头像、侧栏、图标组或复杂网格。
- **不固定 kami 字体依赖**：`kami` 中文偏向 TsangerJinKai02；AutoCV 不把该字体作为运行时依赖，改用 macOS 和常见 Linux fallback。
- **不把 kami 作为运行时组件**：`kami` 仅作为设计和工作流参考；AutoCV 运行时使用自有 HTML/PDF renderer。

## 4. 结论

旧 MVP PDF 模板已经达到 `kami` 参考语言中与简历投递兼容的部分：serif-led、ink-blue、紧凑节奏和克制层级。这个结论只适用于旧 Typst 实现。

新的目标是 AutoCV 自有的 Kami-style resume 模板：HTML 是排版源文件，独立排版 Agent 填充 body，Go 校验结构/事实/安全，WeasyPrint 生成 PDF，PDFium 生成 PNG 预览。`kami` skill 仍不作为运行时组件。

M5 真实样本验收时仍需使用至少一份中文和一份英文真实脱敏样本复查视觉质量，特别是多页经历、长项目列表和链接密集内容。

## 5. 验证

- `GOCACHE=/tmp/autocv-go-cache go test ./internal/adapters/htmlresume`
- `GOCACHE=/tmp/autocv-go-cache go test ./internal/app`
- `GOCACHE=/tmp/autocv-go-cache go test ./...`
- `git diff --check`
