import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";

const {
  analyzeJDMock,
  getJDWorkspaceMock,
  getOverviewMock,
  importMarkdownMock,
  saveJDDraftMock,
} = vi.hoisted(() => ({
  analyzeJDMock: vi.fn(),
  getJDWorkspaceMock: vi.fn(),
  getOverviewMock: vi.fn(),
  importMarkdownMock: vi.fn(),
  saveJDDraftMock: vi.fn(),
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
  ProfileService: {
    GetOverview: getOverviewMock,
    ImportMarkdown: importMarkdownMock,
  },
}));

const profileOverview = {
  profileId: "profile-1",
  name: "主资料库",
  defaultLanguage: "zh-CN",
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

describe("Paper Trail match review", () => {
  beforeEach(() => {
    analyzeJDMock.mockReset().mockResolvedValue(jdWorkspace);
    getJDWorkspaceMock.mockReset().mockResolvedValue(jdWorkspace);
    getOverviewMock.mockReset().mockResolvedValue(profileOverview);
    importMarkdownMock.mockReset().mockResolvedValue({
      cancelled: false,
      duplicate: false,
      document: profileOverview.documents[0],
      chunkCount: 1,
      evidenceCount: 1,
      warnings: [],
    });
    saveJDDraftMock.mockReset().mockResolvedValue({
      ...jdWorkspace,
      analysisStatus: "pending",
      analysis: null,
    });
  });

  it("filters requirement rows by match status", async () => {
    const user = userEvent.setup();
    render(<App />);

    const partialTab = screen.getByRole("tab", { name: /部分匹配/ });
    await user.click(partialTab);

    expect(partialTab).toHaveAttribute("aria-selected", "true");

    const review = within(screen.getByRole("main"));
    expect(
      review.queryByText("精通 Go 语言，理解并发模型与内存模型"),
    ).not.toBeInTheDocument();
    expect(
      review.getByText("有 Redis 使用经验，了解缓存策略与高可用方案"),
    ).toBeInTheDocument();
  });

  it("opens and closes the resume generation confirmation", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "生成简历" }));

    const dialog = screen.getByRole("dialog", {
      name: "基于当前匹配结果生成简历",
    });
    expect(dialog).toBeInTheDocument();
    expect(
      within(dialog).getByText(/缺失要求不会被补写成事实/),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "继续审阅" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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
});
