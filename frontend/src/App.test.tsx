import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import App from "./App";

vi.mock("../bindings/github.com/ch1lam/autocv/internal/app", () => ({
  HealthService: {
    Check: vi.fn().mockResolvedValue({ status: "ready" }),
  },
}));

describe("Paper Trail match review", () => {
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
});
