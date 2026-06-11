import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";

const { getOverviewMock, importMarkdownMock } = vi.hoisted(() => ({
  getOverviewMock: vi.fn(),
  importMarkdownMock: vi.fn(),
}));

vi.mock("../bindings/github.com/ch1lam/autocv/internal/app", () => ({
  HealthService: {
    Check: vi.fn().mockResolvedValue({ status: "ready" }),
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
            '{"heading_path":["黎智林","工作经历"],"start":120,"end":198}',
          quoteStart: 0,
          quoteEnd: 31,
        },
      ],
    },
  ],
};

describe("Paper Trail match review", () => {
  beforeEach(() => {
    getOverviewMock.mockReset().mockResolvedValue(profileOverview);
    importMarkdownMock.mockReset().mockResolvedValue({
      cancelled: false,
      duplicate: false,
      document: profileOverview.documents[0],
      chunkCount: 1,
      evidenceCount: 1,
      warnings: [],
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
      within(inspector).getAllByText("黎智林 / 工作经历"),
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
});
