import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";

const {
  analyzeMatchesMock,
  analyzeJDMock,
  cancelActiveProviderMock,
  createProfileMock,
  exportMarkdownMock,
  exportPDFMock,
  getPDFWorkspaceMock,
  getJDWorkspaceMock,
  getMatchReviewMock,
  getOverviewMock,
  importMarkdownMock,
  renderPDFMock,
  generateResumeMock,
  getResumeWorkspaceMock,
  getSettingsMock,
  saveJDDraftMock,
  saveEvidenceMock,
  saveProviderMock,
  searchProfileMock,
  selectProfileMock,
  setResumeBlockLockedMock,
  updateResumeMarkdownMock,
} = vi.hoisted(() => ({
  analyzeMatchesMock: vi.fn(),
  analyzeJDMock: vi.fn(),
  cancelActiveProviderMock: vi.fn(),
  createProfileMock: vi.fn(),
  exportMarkdownMock: vi.fn(),
  exportPDFMock: vi.fn(),
  generateResumeMock: vi.fn(),
  getJDWorkspaceMock: vi.fn(),
  getMatchReviewMock: vi.fn(),
  getOverviewMock: vi.fn(),
  getPDFWorkspaceMock: vi.fn(),
  getResumeWorkspaceMock: vi.fn(),
  getSettingsMock: vi.fn(),
  importMarkdownMock: vi.fn(),
  renderPDFMock: vi.fn(),
  saveJDDraftMock: vi.fn(),
  saveEvidenceMock: vi.fn(),
  saveProviderMock: vi.fn(),
  searchProfileMock: vi.fn(),
  selectProfileMock: vi.fn(),
  setResumeBlockLockedMock: vi.fn(),
  updateResumeMarkdownMock: vi.fn(),
}));

vi.mock("../bindings/github.com/ch1lam/autocv/internal/app", () => ({
  HealthService: {
    Check: vi.fn().mockResolvedValue({ status: "ready" }),
  },
  JDService: {
    Analyze: analyzeJDMock,
    GetWorkspace: getJDWorkspaceMock,
    SaveDraft: saveJDDraftMock,
  },
  MatchService: {
    Analyze: analyzeMatchesMock,
    GetReview: getMatchReviewMock,
  },
  PDFService: {
    ExportMarkdown: exportMarkdownMock,
    ExportPDF: exportPDFMock,
    GetWorkspace: getPDFWorkspaceMock,
    Render: renderPDFMock,
  },
  ProfileService: {
    CreateProfile: createProfileMock,
    GetOverview: getOverviewMock,
    ImportMarkdown: importMarkdownMock,
    SaveEvidence: saveEvidenceMock,
    Search: searchProfileMock,
    SelectProfile: selectProfileMock,
  },
  ProviderControlService: {
    CancelActive: cancelActiveProviderMock,
  },
  ResumeService: {
    Generate: generateResumeMock,
    GetWorkspace: getResumeWorkspaceMock,
    SetBlockLocked: setResumeBlockLockedMock,
    UpdateMarkdown: updateResumeMarkdownMock,
  },
  SettingsService: {
    GetSettings: getSettingsMock,
    SaveProvider: saveProviderMock,
  },
}));

const profileOverview = {
  profileId: "profile-1",
  name: "主资料库",
  defaultLanguage: "zh-CN",
  profiles: [
    {
      id: "profile-1",
      name: "主资料库",
      defaultLanguage: "zh-CN",
      active: true,
    },
    {
      id: "profile-2",
      name: "English applications",
      defaultLanguage: "en",
      active: false,
    },
  ],
  documents: [
    {
      id: "document-1",
      originalName: "backend-profile.md",
      kind: "markdown",
      parseStatus: "succeeded",
      importedAt: "2026-06-11T01:00:00Z",
    },
  ],
  evidence: [
    {
      id: "evidence-1",
      kind: "experience",
      title: "负责支付平台核心服务开发",
      content: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
      confidence: 0.75,
      userVerified: false,
      updatedAt: "2026-06-11T01:05:00Z",
      sources: [
        {
          chunkId: "chunk-1",
          documentId: "document-1",
          chunkText: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
          locatorJson:
            '{"heading_path":["李志林","工作经历"],"start":120,"end":198}',
          quoteStart: 0,
          quoteEnd: 31,
        },
      ],
    },
  ],
};

const jdWorkspace = {
  id: "jd-1",
  title: "Senior Backend Engineer",
  company: "Acme",
  rawText:
    "Senior Backend Engineer\nBuild reliable Go services and improve platform observability.",
  language: "en",
  analysisStatus: "succeeded",
  analysisError: "",
  updatedAt: "2026-06-11T02:00:00Z",
  warnings: [],
  analysis: {
    role: "Senior Backend Engineer",
    company: "Acme",
    level: "senior",
    language: "en",
    responsibilities: [
      {
        id: "responsibility-1",
        text: "Build reliable backend services",
        importance: 5,
        hardConstraint: false,
      },
    ],
    requiredSkills: [
      {
        id: "required-1",
        text: "Strong Go experience",
        importance: 5,
        hardConstraint: true,
      },
    ],
    preferredSkills: [
      {
        id: "preferred-1",
        text: "Experience with observability tooling",
        importance: 3,
        hardConstraint: false,
      },
    ],
    domainSignals: ["Developer infrastructure"],
    screeningConstraints: ["5+ years of backend experience"],
    ambiguities: ["Office location is not specified"],
  },
};

const matchReview = {
  status: "ready",
  message: "匹配分只表示当前资料与 JD 的证据覆盖度。",
  error: "",
  jdTitle: "Senior Backend Engineer",
  company: "Acme",
  totalScore: 69,
  hardCapApplied: true,
  updatedAt: "2026-06-11T03:00:00Z",
  counts: {
    strong: 3,
    partial: 1,
    missing: 1,
    unknown: 1,
  },
  dimensions: [
    {
      category: "required",
      label: "必要技能与硬性条件",
      weight: 40,
      earned: 20,
      requirementCount: 2,
    },
    {
      category: "responsibility",
      label: "主要职责证据",
      weight: 30,
      earned: 15,
      requirementCount: 1,
    },
    {
      category: "level",
      label: "岗位级别与责任范围",
      weight: 15,
      earned: 0,
      requirementCount: 1,
    },
    {
      category: "domain",
      label: "领域与业务经验",
      weight: 10,
      earned: 10,
      requirementCount: 1,
    },
    {
      category: "preferred",
      label: "加分项",
      weight: 5,
      earned: 5,
      requirementCount: 1,
    },
  ],
  requirements: [
    {
      id: "required-go",
      category: "required",
      group: "必要技能与硬性条件",
      text: "熟练使用 Go 开发生产服务",
      importance: 5,
      hardConstraint: false,
      strength: "strong",
      explanation: "Go 经验有多处直接证据。",
      clarificationNeeded: false,
      evidence: [
        {
          id: "evidence-1",
          kind: "experience",
          title: "负责支付平台核心服务开发",
          content: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
          sources: [
            {
              chunkId: "chunk-1",
              documentId: "document-1",
              documentName: "backend-profile.md",
              chunkText:
                "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
              locatorJson:
                '{"heading_path":["李志林","工作经历"],"start":120,"end":198}',
              quoteStart: 0,
              quoteEnd: 31,
            },
          ],
        },
      ],
    },
    {
      id: "screening-english",
      category: "required",
      group: "必要技能与硬性条件",
      text: "能够阅读英文技术文档",
      importance: 5,
      hardConstraint: true,
      strength: "missing",
      explanation: "当前资料中没有找到直接证据。",
      clarificationNeeded: true,
      evidence: [],
    },
    {
      id: "responsibility-performance",
      category: "responsibility",
      group: "主要职责证据",
      text: "持续改善服务性能",
      importance: 4,
      hardConstraint: false,
      strength: "partial",
      explanation: "已有优化线索，但覆盖深度仍需确认。",
      clarificationNeeded: true,
      evidence: [
        {
          id: "evidence-1",
          kind: "experience",
          title: "负责支付平台核心服务开发",
          content: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
          sources: [],
        },
      ],
    },
    {
      id: "level-senior",
      category: "level",
      group: "岗位级别与责任范围",
      text: "Senior",
      importance: 3,
      hardConstraint: false,
      strength: "unknown",
      explanation: "当前资料无法判断责任范围。",
      clarificationNeeded: true,
      evidence: [],
    },
    {
      id: "domain-transaction",
      category: "domain",
      group: "领域与业务经验",
      text: "高并发交易系统",
      importance: 3,
      hardConstraint: false,
      strength: "strong",
      explanation: "交易平台经验有直接证据。",
      clarificationNeeded: false,
      evidence: [
        {
          id: "evidence-1",
          kind: "experience",
          title: "负责支付平台核心服务开发",
          content: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
          sources: [],
        },
      ],
    },
    {
      id: "preferred-messaging",
      category: "preferred",
      group: "加分项",
      text: "消息队列与事件驱动经验",
      importance: 2,
      hardConstraint: false,
      strength: "strong",
      explanation: "资料包含消息队列经验。",
      clarificationNeeded: false,
      evidence: [
        {
          id: "evidence-1",
          kind: "project",
          title: "订单平台",
          content: "使用消息队列解耦订单创建和下游处理。",
          sources: [],
        },
      ],
    },
  ],
};

const resumeWorkspace = {
  status: "ready",
  message: "结构化简历、Markdown 与来源引用已保存到本地。",
  canExport: true,
  exportIssues: [],
  runId: "run-1",
  resumeId: "resume-1",
  version: 1,
  language: "zh",
  targetRole: "Senior Backend Engineer",
  packagingLevel: 0.5,
  packagingLabel: "平衡",
  markdown: `# Senior Backend Engineer

## 职业概述

<!-- autocv:block:block-summary:start -->
面向 Senior Backend Engineer 岗位，重点呈现支付平台经验。
<!-- autocv:block:block-summary:end -->

## 工作经历

<!-- autocv:block:block-experience:start -->
- 负责支付平台核心服务开发，使用 Go 构建高并发交易接口。
<!-- autocv:block:block-experience:end -->
`,
  updatedAt: "2026-06-11T04:00:00Z",
  optimizationNotes: [
    "按当前匹配结果选择 1 条来源证据，并按岗位相关度排序。",
    "缺失和未知要求未写入简历。",
  ],
  blocks: [
    {
      id: "block-summary",
      kind: "summary",
      label: "职业概述",
      content:
        "面向 Senior Backend Engineer 岗位，重点呈现支付平台经验。",
      locked: false,
      groundingLevel: "derived",
      optimization: "用最高相关度的来源概括候选人与目标岗位的连接。",
      evidence: [matchReview.requirements[0].evidence[0]],
    },
    {
      id: "block-experience",
      kind: "experience",
      label: "工作经历",
      content: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
      locked: false,
      groundingLevel: "source",
      optimization: "对应 Go 服务开发要求。",
      evidence: [matchReview.requirements[0].evidence[0]],
    },
  ],
};

const pdfWorkspace = {
  status: "ready",
  message: "PDF 已从当前 Resume 版本渲染并保存到本地。",
  exportIssues: [],
  artifactId: "artifact-1234567890",
  resumeId: "resume-1",
  version: 1,
  language: "zh",
  targetRole: "Senior Backend Engineer",
  renderedAt: "2026-06-11T05:00:00Z",
  contentHash: "1234567890abcdef",
  pdfBase64: "JVBERi0xLjcK",
  previewPagesBase64: ["iVBORw0KGgo="],
  canExport: true,
};

const providerSettings = {
  provider: "fake",
  baseUrl: "",
  model: "fixture-v1",
  apiKeyConfigured: false,
  secretBackend: "macOS Keychain",
  sentContentTypes: [
    {
      label: "JD 原文",
      description: "仅在分析岗位时发送当前粘贴的岗位描述。",
    },
    {
      label: "相关 Evidence",
      description: "只发送任务需要的证据内容与来源 ID。",
    },
    {
      label: "Requirement 与匹配上下文",
      description: "发送结构化岗位要求和必要上下文。",
    },
    {
      label: "Resume Block 与包装参数",
      description: "发送当前结构化内容和包装档位。",
    },
  ],
  localOnlyTypes: [
    {
      label: "API Key",
      description: "保存在 macOS Keychain。",
    },
    {
      label: "原始资料文件",
      description: "原文件保留在本地。",
    },
    {
      label: "Typst 与 PDF 产物",
      description: "排版和导出在本机执行。",
    },
  ],
  configurationNote: "Fake Provider 使用固定 Fixture。",
  updatedAt: "2026-06-12T06:00:00Z",
};

describe("Paper Trail match review", () => {
  beforeEach(() => {
    analyzeMatchesMock.mockReset().mockResolvedValue(matchReview);
    analyzeJDMock.mockReset().mockResolvedValue(jdWorkspace);
    cancelActiveProviderMock.mockReset().mockResolvedValue({
      cancelled: true,
      task: "jd_analysis",
      message: "已发送取消请求；当前步骤结束后可以直接重试。",
    });
    createProfileMock.mockReset();
    getJDWorkspaceMock.mockReset().mockResolvedValue(jdWorkspace);
    getMatchReviewMock.mockReset().mockResolvedValue(matchReview);
    getOverviewMock.mockReset().mockResolvedValue(profileOverview);
    getPDFWorkspaceMock.mockReset().mockResolvedValue(pdfWorkspace);
    getResumeWorkspaceMock.mockReset().mockResolvedValue(resumeWorkspace);
    getSettingsMock.mockReset().mockResolvedValue(providerSettings);
    generateResumeMock.mockReset().mockResolvedValue(resumeWorkspace);
    updateResumeMarkdownMock.mockReset().mockImplementation((markdown) =>
      Promise.resolve({
        ...resumeWorkspace,
        version: 2,
        markdown,
      }),
    );
    setResumeBlockLockedMock.mockReset().mockResolvedValue({
      ...resumeWorkspace,
      version: 2,
      blocks: resumeWorkspace.blocks.map((block, index) =>
        index === 0 ? { ...block, locked: true } : block,
      ),
    });
    importMarkdownMock.mockReset().mockResolvedValue({
      cancelled: false,
      duplicate: false,
      document: profileOverview.documents[0],
      chunkCount: 1,
      evidenceCount: 1,
      warnings: [],
    });
    renderPDFMock.mockReset().mockResolvedValue(pdfWorkspace);
    exportPDFMock.mockReset().mockResolvedValue({
      cancelled: false,
      kind: "pdf",
      path: "/tmp/AutoCV-resume.pdf",
    });
    exportMarkdownMock.mockReset().mockResolvedValue({
      cancelled: false,
      kind: "markdown",
      path: "/tmp/AutoCV-resume.md",
    });
    saveJDDraftMock.mockReset().mockResolvedValue({
      ...jdWorkspace,
      analysisStatus: "pending",
      analysis: null,
    });
    saveEvidenceMock.mockReset().mockResolvedValue(profileOverview);
    saveProviderMock.mockReset().mockImplementation((input) =>
      Promise.resolve({
        ...providerSettings,
        provider: input.provider,
        baseUrl: input.baseUrl,
        model: input.model,
        apiKeyConfigured: input.provider === "openai",
        configurationNote:
          input.provider === "openai"
            ? "OpenAI 配置已保存。"
            : providerSettings.configurationNote,
      }),
    );
    searchProfileMock.mockReset().mockResolvedValue([]);
    selectProfileMock.mockReset();
  });

  it("filters requirement rows by match status", async () => {
    const user = userEvent.setup();
    render(<App />);

    const partialTab = await screen.findByRole("tab", { name: /部分匹配/ });
    await user.click(partialTab);

    expect(partialTab).toHaveAttribute("aria-selected", "true");

    const review = within(screen.getByRole("main"));
    expect(
      review.queryByText("熟练使用 Go 开发生产服务"),
    ).not.toBeInTheDocument();
    expect(
      review.getByText("持续改善服务性能"),
    ).toBeInTheDocument();
    expect(
      await within(
        screen.getByRole("complementary", { name: "来源证据" }),
      ).findByRole("heading", { name: "持续改善服务性能" }),
    ).toBeInTheDocument();
  });

  it("shows deterministic dimensions, hard-cap notice, and source evidence", async () => {
    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Senior Backend Engineer" }),
    ).toBeInTheDocument();
    expect(screen.getByText("69")).toBeInTheDocument();
    expect(
      screen.getByText(/明确硬性条件.*最高 69/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "匹配分项得分" }),
    ).toBeInTheDocument();

    const inspector = screen.getByRole("complementary", {
      name: "来源证据",
    });
    expect(within(inspector).getByText("backend-profile.md")).toBeInTheDocument();
    expect(
      within(inspector).getByText("Go 经验有多处直接证据。"),
    ).toBeInTheDocument();
  });

  it("rebuilds stale match results through the Go service", async () => {
    const user = userEvent.setup();
    getMatchReviewMock.mockResolvedValue({
      ...matchReview,
      status: "stale",
      message: "资料或 JD 已变化，旧匹配结果已失效。",
      requirements: [],
      dimensions: [],
    });
    render(<App />);

    expect(
      await screen.findByRole("heading", {
        name: "资料发生变化，需要重新建立证据关联",
      }),
    ).toBeInTheDocument();
    await user.click(screen.getAllByRole("button", { name: "开始匹配" })[0]);

    expect(analyzeMatchesMock).toHaveBeenCalledOnce();
    expect(
      await screen.findByRole("heading", { name: "Senior Backend Engineer" }),
    ).toBeInTheDocument();
  });

  it("opens and closes the resume generation confirmation", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole("button", { name: "生成简历" }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "基于当前匹配结果生成简历",
    });
    expect(dialog).toBeInTheDocument();
    expect(
      within(dialog).getByText(/缺失要求不会被补写成事实/),
    ).toBeInTheDocument();
    expect(
      within(
        within(dialog).getByRole("region", {
          name: "本次生成的 Provider 发送摘要",
        }),
      ).getByText("Fake Provider · 本地 Fixture"),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "继续审阅" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("generates a structured resume and opens Resume Studio", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole("button", { name: "生成简历" }),
    );
    const dialog = screen.getByRole("dialog", {
      name: "基于当前匹配结果生成简历",
    });
    await user.click(
      within(dialog).getByRole("button", { name: "确认生成" }),
    );

    expect(generateResumeMock).toHaveBeenCalledWith("zh", 0.5);
    expect(
      await screen.findByRole("textbox", { name: "简历 Markdown" }),
    ).toHaveValue(resumeWorkspace.markdown);
    expect(
      screen.getByRole("complementary", { name: "简历 Block 检查器" }),
    ).toBeInTheDocument();
  });

  it("saves constrained Markdown and appends a lock version", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "简历工作室" }));
    const editor = await screen.findByRole("textbox", {
      name: "简历 Markdown",
    });
    const editedMarkdown = resumeWorkspace.markdown.replace(
      "重点呈现支付平台经验。",
      "重点呈现支付平台经验与稳定性治理。",
    );
    fireEvent.change(editor, { target: { value: editedMarkdown } });
    await user.click(screen.getByRole("button", { name: "保存新版本" }));

    expect(updateResumeMarkdownMock).toHaveBeenCalledWith(editedMarkdown);
    expect(
      await screen.findByText("Markdown 已保存为第 2 版。"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "锁定内容" }));
    expect(setResumeBlockLockedMock).toHaveBeenCalledWith(
      "block-summary",
      true,
    );
    expect(
      await screen.findByRole("button", { name: "解除锁定" }),
    ).toBeInTheDocument();
  });

  it("previews and exports the same PDF artifact", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "PDF 预览" }));

    expect(
      await screen.findByRole("heading", {
        name: "Senior Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("complementary", { name: "PDF Artifact 检查器" }),
    ).toBeInTheDocument();
    expect(
      screen.getByAltText("Senior Backend Engineer PDF 第 1 页预览"),
    ).toHaveAttribute(
      "src",
      `data:image/png;base64,${pdfWorkspace.previewPagesBase64[0]}`,
    );

    await user.click(
      within(
        screen.getByRole("complementary", {
          name: "PDF Artifact 检查器",
        }),
      ).getByRole("button", { name: "导出 PDF" }),
    );
    expect(exportPDFMock).toHaveBeenCalledOnce();
    expect(
      await screen.findByText("PDF 已导出到 /tmp/AutoCV-resume.pdf"),
    ).toBeInTheDocument();
  });

  it("shows unconfirmed content and disables final exports", async () => {
    const user = userEvent.setup();
    const exportIssue =
      "职业概述内容“适合承担目标岗位相关职责。”没有来源，也未经用户确认";
    getResumeWorkspaceMock.mockResolvedValue({
      ...resumeWorkspace,
      canExport: false,
      exportIssues: [exportIssue],
      blocks: [
        {
          ...resumeWorkspace.blocks[0],
          content: "适合承担目标岗位相关职责。",
          evidence: [],
        },
        resumeWorkspace.blocks[1],
      ],
    });
    getPDFWorkspaceMock.mockResolvedValue({
      ...pdfWorkspace,
      message: "当前 PDF 可以预览，但存在未确认内容，暂不能导出。",
      canExport: false,
      exportIssues: [exportIssue],
    });
    render(<App />);

    await user.click(screen.getByRole("button", { name: "简历工作室" }));
    expect(
      await screen.findByText("最终导出已阻止"),
    ).toBeInTheDocument();
    expect(screen.getByText(exportIssue)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "PDF 预览" }));
    expect(
      await screen.findByText("预览可用，最终导出已阻止"),
    ).toBeInTheDocument();
    expect(
      within(
        screen.getByRole("complementary", {
          name: "PDF Artifact 检查器",
        }),
      ).getByRole("button", { name: "导出 PDF" }),
    ).toBeDisabled();
    expect(
      within(
        screen.getByRole("complementary", {
          name: "PDF Artifact 检查器",
        }),
      ).getByRole("button", { name: "导出 Markdown" }),
    ).toBeDisabled();
  });

  it("opens the Profile Library and shows source traceability", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "资料库" }));

    expect(
      await screen.findByRole("heading", { name: "主资料库" }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("main")).getByText("backend-profile.md"),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", {
        name: /负责支付平台核心服务开发/,
      }),
    );

    const inspector = screen.getByRole("complementary", {
      name: "Evidence 来源检查器",
    });
    expect(
      within(inspector).getAllByText("李志林 / 工作经历"),
    ).toHaveLength(2);
    expect(within(inspector).getByText("120–198")).toBeInTheDocument();
    expect(
      within(inspector).getAllByText(
        "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
      ),
    ).toHaveLength(2);
  });

  it("imports Markdown and refreshes the Profile Library", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "资料库" }));
    await within(screen.getByRole("main")).findByText("backend-profile.md");
    await user.click(
      screen.getAllByRole("button", { name: "导入 Markdown" })[0],
    );

    expect(importMarkdownMock).toHaveBeenCalledOnce();
    expect(getOverviewMock).toHaveBeenCalledTimes(2);
    expect(
      await screen.findByText(
        "已导入 backend-profile.md，生成 1 条可追溯 Evidence。",
      ),
    ).toBeInTheDocument();
  });

  it("searches Profile sources and shows document snippets", async () => {
    const user = userEvent.setup();
    searchProfileMock.mockResolvedValue([
      {
        entityType: "evidence",
        entityId: "evidence-1",
        documentId: "document-1",
        sourceChunkId: "chunk-1",
        documentName: "backend-profile.md",
        title: "负责支付平台核心服务开发",
        snippet: "使用 Go 构建高并发交易接口。",
      },
      {
        entityType: "source_chunk",
        entityId: "chunk-1",
        documentId: "document-1",
        sourceChunkId: "chunk-1",
        documentName: "backend-profile.md",
        title: "backend-profile.md",
        snippet: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
      },
    ]);
    render(<App />);

    await user.click(screen.getByRole("button", { name: "资料库" }));
    const search = await screen.findByRole("searchbox", {
      name: "搜索资料库",
    });
    await user.type(search, "Go");
    await user.click(
      within(screen.getByRole("region", { name: "资料检索" })).getByRole(
        "button",
        { name: "搜索" },
      ),
    );

    expect(searchProfileMock).toHaveBeenCalledWith("Go");
    expect(await screen.findByText("2 条结果")).toBeInTheDocument();
    expect(
      screen.getByText("使用 Go 构建高并发交易接口。"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("backend-profile.md").length).toBeGreaterThan(1);
  });

  it("confirms an extracted Evidence from the source inspector", async () => {
    const user = userEvent.setup();
    const confirmedOverview = {
      ...profileOverview,
      evidence: profileOverview.evidence.map((evidence) => ({
        ...evidence,
        userVerified: true,
        updatedAt: "2026-06-15T01:00:00Z",
      })),
    };
    saveEvidenceMock.mockResolvedValue(confirmedOverview);
    render(<App />);

    await user.click(screen.getByRole("button", { name: "资料库" }));
    await user.click(
      await screen.findByRole("button", { name: "确认此 Evidence" }),
    );

    expect(saveEvidenceMock).toHaveBeenCalledWith({
      evidenceId: "evidence-1",
      title: "负责支付平台核心服务开发",
      content: "负责支付平台核心服务开发，使用 Go 构建高并发交易接口。",
      userVerified: true,
    });
    expect(await screen.findByText("用户已确认")).toBeInTheDocument();
    expect(
      screen.getByText("Evidence 已保存并标记为用户确认。"),
    ).toBeInTheDocument();
  });

  it("edits Evidence while preserving its source inspector", async () => {
    const user = userEvent.setup();
    const updatedOverview = {
      ...profileOverview,
      evidence: profileOverview.evidence.map((evidence) => ({
        ...evidence,
        title: "支付平台后端交付",
        content: "负责支付平台核心服务交付，并使用 Go 改善接口稳定性。",
        userVerified: true,
        updatedAt: "2026-06-15T02:00:00Z",
      })),
    };
    saveEvidenceMock.mockResolvedValue(updatedOverview);
    render(<App />);

    await user.click(screen.getByRole("button", { name: "资料库" }));
    const inspector = screen.getByRole("complementary", {
      name: "Evidence 来源检查器",
    });
    await user.click(within(inspector).getByRole("button", { name: "编辑" }));
    const title = within(inspector).getByRole("textbox", {
      name: "Evidence 标题",
    });
    const content = within(inspector).getByRole("textbox", {
      name: "Evidence 内容",
    });
    await user.clear(title);
    await user.type(title, "支付平台后端交付");
    await user.clear(content);
    await user.type(
      content,
      "负责支付平台核心服务交付，并使用 Go 改善接口稳定性。",
    );
    expect(
      within(inspector).getAllByText("李志林 / 工作经历"),
    ).toHaveLength(2);
    await user.click(
      within(inspector).getByRole("button", { name: "保存并确认" }),
    );

    expect(saveEvidenceMock).toHaveBeenCalledWith({
      evidenceId: "evidence-1",
      title: "支付平台后端交付",
      content: "负责支付平台核心服务交付，并使用 Go 改善接口稳定性。",
      userVerified: true,
    });
    expect(
      await within(inspector).findByRole("heading", {
        name: "支付平台后端交付",
      }),
    ).toBeInTheDocument();
    expect(getMatchReviewMock).toHaveBeenCalledTimes(2);
    expect(getResumeWorkspaceMock).toHaveBeenCalledTimes(2);
    expect(getPDFWorkspaceMock).toHaveBeenCalledTimes(2);
  });

  it("switches the active Profile and refreshes dependent workspaces", async () => {
    const user = userEvent.setup();
    selectProfileMock.mockResolvedValue({
      ...profileOverview,
      profileId: "profile-2",
      name: "English applications",
      defaultLanguage: "en",
      profiles: profileOverview.profiles.map((profile) => ({
        ...profile,
        active: profile.id === "profile-2",
      })),
      documents: [],
      evidence: [],
    });
    render(<App />);

    await user.click(
      await screen.findByRole("button", {
        name: /李志林.*主资料库/,
      }),
    );
    await user.click(
      screen.getByRole("button", {
        name: "选择 English applications",
      }),
    );

    expect(selectProfileMock).toHaveBeenCalledWith("profile-2");
    expect(
      await screen.findByRole("button", {
        name: /李志林.*English applications/,
      }),
    ).toBeInTheDocument();
    expect(getMatchReviewMock).toHaveBeenCalledTimes(2);
    expect(getResumeWorkspaceMock).toHaveBeenCalledTimes(2);
    expect(getPDFWorkspaceMock).toHaveBeenCalledTimes(2);
  });

  it("creates a Profile from the existing topbar workflow", async () => {
    const user = userEvent.setup();
    createProfileMock.mockResolvedValue({
      ...profileOverview,
      profileId: "profile-3",
      name: "海外岗位",
      defaultLanguage: "en",
      profiles: [
        ...profileOverview.profiles.map((profile) => ({
          ...profile,
          active: false,
        })),
        {
          id: "profile-3",
          name: "海外岗位",
          defaultLanguage: "en",
          active: true,
        },
      ],
      documents: [],
      evidence: [],
    });
    render(<App />);

    await user.click(
      await screen.findByRole("button", {
        name: /李志林.*主资料库/,
      }),
    );
    await user.click(screen.getByRole("button", { name: "新建资料库" }));
    const dialog = screen.getByRole("dialog", { name: "创建独立资料库" });
    await user.type(
      within(dialog).getByRole("textbox", { name: "资料库名称" }),
      "海外岗位",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "English" }),
    );
    await user.click(
      within(dialog).getByRole("button", { name: "创建并切换" }),
    );

    expect(createProfileMock).toHaveBeenCalledWith("海外岗位", "en");
    expect(
      await screen.findByRole("heading", { name: "海外岗位" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("已创建 海外岗位，可以开始导入 Markdown 资料。"),
    ).toBeInTheDocument();
  });

  it("shows the raw JD and validated analysis together", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "JD 工作区" }));

    const editor = await screen.findByRole("textbox", {
      name: "岗位 JD 原始文本",
    });
    expect(editor).toHaveValue(jdWorkspace.rawText);

    const analysis = screen.getByRole("complementary", {
      name: "JD 结构化分析",
    });
    expect(
      within(analysis).getByRole("heading", {
        name: "Senior Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(within(analysis).getByText("Strong Go experience")).toBeInTheDocument();
    expect(
      within(analysis).getByText("5+ years of backend experience"),
    ).toBeInTheDocument();
  });

  it("invalidates stale analysis and analyzes the edited JD", async () => {
    const user = userEvent.setup();
    const updatedText = "Staff Backend Engineer\nOwn the Go platform roadmap.";
    analyzeJDMock.mockResolvedValue({
      ...jdWorkspace,
      title: "Staff Backend Engineer",
      rawText: updatedText,
      analysis: {
        ...jdWorkspace.analysis,
        role: "Staff Backend Engineer",
      },
    });
    render(<App />);

    await user.click(screen.getByRole("button", { name: "JD 工作区" }));
    const editor = await screen.findByRole("textbox", {
      name: "岗位 JD 原始文本",
    });
    await user.clear(editor);
    await user.type(editor, updatedText);

    expect(screen.getByText("当前结果已失效")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Senior Backend Engineer" }),
    ).not.toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "分析 JD" })[0]);

    expect(analyzeJDMock).toHaveBeenCalledWith(updatedText);
    expect(
      await screen.findByRole("heading", { name: "Staff Backend Engineer" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("JD 分析完成，结构化结果已通过 Go 侧校验。"),
    ).toBeInTheDocument();
  });

  it("saves the edited raw JD without keeping the previous analysis", async () => {
    const user = userEvent.setup();
    const updatedText = "Backend Engineer\nMaintain local-first Go services.";
    saveJDDraftMock.mockResolvedValue({
      ...jdWorkspace,
      title: "Backend Engineer",
      rawText: updatedText,
      analysisStatus: "pending",
      analysis: null,
    });
    render(<App />);

    await user.click(screen.getByRole("button", { name: "JD 工作区" }));
    const editor = await screen.findByRole("textbox", {
      name: "岗位 JD 原始文本",
    });
    await user.clear(editor);
    await user.type(editor, updatedText);
    await user.click(screen.getAllByRole("button", { name: "保存原文" })[0]);

    expect(saveJDDraftMock).toHaveBeenCalledWith(updatedText);
    expect(
      await screen.findByText("原始 JD 已保存；旧分析结果已失效。"),
    ).toBeInTheDocument();
    expect(screen.getByText("等待第一份分析结果")).toBeInTheDocument();
  });

  it("shows a persisted stage error when JD analysis fails validation", async () => {
    const user = userEvent.setup();
    const failedWorkspace = {
      ...jdWorkspace,
      analysisStatus: "failed",
      analysisError: "requiredSkills must not be empty",
      analysis: null,
    };
    analyzeJDMock.mockRejectedValue(
      new Error("validate JD analysis: requiredSkills must not be empty"),
    );
    getJDWorkspaceMock
      .mockResolvedValueOnce(jdWorkspace)
      .mockResolvedValueOnce(failedWorkspace);
    render(<App />);

    await user.click(screen.getByRole("button", { name: "JD 工作区" }));
    await screen.findByRole("textbox", { name: "岗位 JD 原始文本" });
    await user.click(screen.getAllByRole("button", { name: "分析 JD" })[0]);

    expect(await screen.findByText("JD 分析未通过")).toBeInTheDocument();
    expect(
      screen.getByText("requiredSkills must not be empty"),
    ).toBeInTheDocument();
  });

  it("shows Provider settings and the privacy ledger without a secret", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "设置" }));

    expect(
      await screen.findByRole("heading", { name: "AI Provider 与密钥" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("complementary", { name: "AI 数据发送边界" }),
    ).toBeInTheDocument();
    expect(screen.getByText("JD 原文")).toBeInTheDocument();
    expect(screen.getByText("Typst 与 PDF 产物")).toBeInTheDocument();
    expect(screen.getByLabelText("OpenAI API Key")).toHaveValue("");
    expect(screen.queryByDisplayValue(/sk-/)).not.toBeInTheDocument();
  });

  it("saves OpenAI settings and clears the API key field", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "设置" }));
    await screen.findByRole("heading", { name: "AI Provider 与密钥" });
    await user.click(screen.getByRole("button", { name: /OpenAI/ }));
    const apiKey = screen.getByLabelText("OpenAI API Key");
    await user.type(apiKey, "sk-test-secret");
    await user.click(screen.getByRole("button", { name: "保存设置" }));

    expect(saveProviderMock).toHaveBeenCalledWith({
      provider: "openai",
      baseUrl: "https://api.openai.com/v1",
      model: "gpt-5.5",
      apiKey: "sk-test-secret",
    });
    expect(
      await screen.findByText("OpenAI 配置与 Keychain 引用已保存。"),
    ).toBeInTheDocument();
    expect(apiKey).toHaveValue("");
    expect(screen.getByText(/已保存在 macOS Keychain/)).toBeInTheDocument();
  });

  it("confirms the OpenAI data summary before analyzing a JD", async () => {
    const user = userEvent.setup();
    getSettingsMock.mockResolvedValue({
      ...providerSettings,
      provider: "openai",
      baseUrl: "https://api.openai.com/v1",
      model: "gpt-5.5",
      apiKeyConfigured: true,
    });
    render(<App />);

    await user.click(screen.getByRole("button", { name: "设置" }));
    await screen.findByText(/已保存在 macOS Keychain/);
    await user.click(screen.getByRole("button", { name: "JD 工作区" }));
    await screen.findByRole("textbox", { name: "岗位 JD 原始文本" });
    await user.click(screen.getAllByRole("button", { name: "分析 JD" })[0]);

    const dialog = screen.getByRole("dialog", {
      name: "即将发送给 OpenAI",
    });
    expect(within(dialog).getByText("当前 JD 原文")).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "API Key、原始文件和本地 PDF 产物不会发送。",
      ),
    ).toBeInTheDocument();
    expect(analyzeJDMock).not.toHaveBeenCalled();

    await user.click(
      within(dialog).getByRole("button", { name: "确认并继续" }),
    );
    expect(analyzeJDMock).toHaveBeenCalledWith(jdWorkspace.rawText);
  });

  it("cancels an active OpenAI request and leaves the action retryable", async () => {
    const user = userEvent.setup();
    let rejectAnalysis: ((error: Error) => void) | undefined;
    getSettingsMock.mockResolvedValue({
      ...providerSettings,
      provider: "openai",
      baseUrl: "https://api.openai.com/v1",
      model: "gpt-5.5",
      apiKeyConfigured: true,
    });
    analyzeJDMock.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectAnalysis = reject;
        }),
    );
    cancelActiveProviderMock.mockImplementation(() => {
      rejectAnalysis?.(new Error("context canceled"));
      return Promise.resolve({
        cancelled: true,
        task: "jd_analysis",
        message: "已发送取消请求；当前步骤结束后可以直接重试。",
      });
    });
    render(<App />);

    await user.click(screen.getByRole("button", { name: "设置" }));
    await screen.findByText(/已保存在 macOS Keychain/);
    await user.click(screen.getByRole("button", { name: "JD 工作区" }));
    await user.click(screen.getAllByRole("button", { name: "分析 JD" })[0]);
    await user.click(
      within(
        screen.getByRole("dialog", { name: "即将发送给 OpenAI" }),
      ).getByRole("button", { name: "确认并继续" }),
    );

    const cancelButton = await screen.findByRole("button", {
      name: "取消请求",
    });
    await user.click(cancelButton);

    expect(cancelActiveProviderMock).toHaveBeenCalledOnce();
    expect(
      await screen.findByText(
        "已取消 JD 分析，原文仍保留，可以直接重试。",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "分析 JD" })[0],
    ).toBeEnabled();
  });
});
