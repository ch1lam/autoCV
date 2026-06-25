import {
  IconAlertTriangle,
  IconCheck,
  IconDownload,
  IconFileText,
  IconFileTypePdf,
  IconRefresh,
} from "@tabler/icons-react";

import type { PDFWorkspace } from "../bindings/github.com/ch1lam/autocv/internal/app";

export type PDFPreviewStatus = "loading" | "ready" | "error";
export type PDFPreviewFeedback = {
  tone: "success" | "error" | "info";
  text: string;
};

type PDFPreviewProps = {
  error: string;
  feedback: PDFPreviewFeedback | null;
  isExporting: boolean;
  isRendering: boolean;
  onExportMarkdown: () => void;
  onExportPDF: () => void;
  onOpenResume: () => void;
  onRender: () => void;
  onRetry: () => void;
  status: PDFPreviewStatus;
  workspace: PDFWorkspace | null;
};

function formatRenderedAt(value: string) {
  if (!value) {
    return "尚未渲染";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function PDFPreview({
  error,
  feedback,
  isExporting,
  isRendering,
  onExportMarkdown,
  onExportPDF,
  onOpenResume,
  onRender,
  onRetry,
  status,
  workspace,
}: PDFPreviewProps) {
  if (status === "loading") {
    return (
      <section className="match-stage-state" aria-label="正在读取 PDF 产物">
        <IconRefresh className="is-spinning" size={28} stroke={1.5} />
        <span>PDF ARTIFACT</span>
        <h1>正在恢复上一次成功渲染</h1>
        <p>Go 服务正在读取 Resume 版本、Artifact 记录和本地 PDF。</p>
      </section>
    );
  }

  if (status === "error") {
    return (
      <section className="match-stage-state match-stage-state--error">
        <IconAlertTriangle size={28} stroke={1.5} />
        <span>PDF 服务不可用</span>
        <h1>无法读取本地 PDF Artifact</h1>
        <p>{error || "请检查本地 HTML/PDF 渲染器与数据目录后重试。"}</p>
        <button
          className="button button--primary"
          onClick={onRetry}
          type="button"
        >
          <IconRefresh size={18} stroke={1.6} />
          重新读取
        </button>
      </section>
    );
  }

  const hasPreview = Boolean(workspace?.previewPagesBase64.length);
  if (!workspace || !hasPreview) {
    const blocked = workspace?.status === "blocked";
    return (
      <section className="match-stage-state">
        <IconFileTypePdf size={30} stroke={1.45} />
        <span>PDF ARTIFACT</span>
        <h1>{blocked ? "先完成当前简历版本" : "生成第一份可预览 PDF"}</h1>
        <p>
          {workspace?.message ||
            "HTML 渲染器将从当前结构化 Resume 生成 Kami 风格、可复制文本的 PDF。"}
        </p>
        <div className="match-stage-actions">
          <button
            className="button button--primary"
            disabled={blocked || isRendering}
            onClick={onRender}
            type="button"
          >
            <IconFileTypePdf size={18} stroke={1.6} />
            {isRendering ? "正在渲染" : "生成 PDF"}
          </button>
          <button
            className="button button--secondary"
            onClick={onOpenResume}
            type="button"
          >
            返回简历工作室
          </button>
        </div>
      </section>
    );
  }

  const stale = workspace.status === "stale";

  return (
    <div className="pdf-preview-layout">
      <main className="pdf-preview-stage">
        <header className="pdf-preview-heading">
          <div>
            <span className="section-kicker">PDF ARTIFACT</span>
            <h1>{workspace.targetRole}</h1>
            <p>
              Resume v{workspace.version} ·{" "}
              {workspace.language === "en" ? "English" : "中文"} · ATS
              单栏模板
            </p>
          </div>
          <span className={`pdf-artifact-state ${stale ? "is-stale" : ""}`}>
            {stale ? (
              <IconAlertTriangle size={16} stroke={1.7} />
            ) : (
              <IconCheck size={16} stroke={1.8} />
            )}
            {stale ? "上一份成功产物" : "当前版本"}
          </span>
        </header>

        {feedback && (
          <div
            className={`profile-feedback profile-feedback--${feedback.tone}`}
            role="status"
          >
            {feedback.tone === "error" ? (
              <IconAlertTriangle aria-hidden="true" size={18} stroke={1.7} />
            ) : (
              <IconCheck aria-hidden="true" size={18} stroke={1.7} />
            )}
            <span>{feedback.text}</span>
          </div>
        )}

        {stale && (
          <div className="pdf-stale-notice" role="status">
            <IconAlertTriangle aria-hidden="true" size={18} stroke={1.7} />
            <span>{workspace.message}</span>
            <button onClick={onRender} type="button">
              渲染当前版本
            </button>
          </div>
        )}

        {!stale && !workspace.canExport && workspace.exportIssues.length > 0 && (
          <div className="pdf-export-gate" role="alert">
            <IconAlertTriangle aria-hidden="true" size={18} stroke={1.7} />
            <div>
              <strong>预览可用，最终导出已阻止</strong>
              <p>{workspace.exportIssues.join("；")}</p>
            </div>
            <button onClick={onOpenResume} type="button">
              返回确认内容
            </button>
          </div>
        )}

        {!stale && workspace.warnings.length > 0 && (
          <div className="pdf-export-warning" role="status">
            <IconAlertTriangle aria-hidden="true" size={18} stroke={1.7} />
            <div>
              <strong>PDF 篇幅提醒</strong>
              <p>{workspace.warnings.join("；")}</p>
            </div>
          </div>
        )}

        <section className="pdf-canvas" aria-label="PDF 简历预览">
          {workspace.previewPagesBase64.map((page, index) => (
            <figure key={`${workspace.artifactId}-page-${index + 1}`}>
              <img
                alt={`${workspace.targetRole} PDF 第 ${index + 1} 页预览`}
                src={`data:image/png;base64,${page}`}
              />
              <figcaption>第 {index + 1} 页</figcaption>
            </figure>
          ))}
        </section>
      </main>

      <aside className="pdf-inspector" aria-label="PDF Artifact 检查器">
        <header>
          <span className="section-kicker">RENDER LEDGER</span>
          <h2>产物记录</h2>
          <p>{workspace.message}</p>
        </header>

        <dl className="pdf-artifact-meta">
          <div>
            <dt>Resume 版本</dt>
            <dd>v{workspace.version}</dd>
          </div>
          <div>
            <dt>渲染时间</dt>
            <dd>{formatRenderedAt(workspace.renderedAt)}</dd>
          </div>
          <div>
            <dt>Artifact ID</dt>
            <dd title={workspace.artifactId}>
              {workspace.artifactId.slice(0, 12)}
            </dd>
          </div>
          <div>
            <dt>内容摘要</dt>
            <dd title={workspace.contentHash}>
              {workspace.contentHash.slice(0, 12)}
            </dd>
          </div>
        </dl>

        <section className="pdf-check-list" aria-label="PDF 输出检查">
          <h3>输出检查</h3>
          <p>
            <IconCheck aria-hidden="true" size={16} stroke={1.8} />
            中英文固定字体回退
          </p>
          <p>
            <IconCheck aria-hidden="true" size={16} stroke={1.8} />
            可选择文本与 ATS 单栏结构
          </p>
          <p>
            <IconCheck aria-hidden="true" size={16} stroke={1.8} />
            预览与导出共用同一 Artifact
          </p>
        </section>

        <div className="pdf-inspector-actions">
          <button
            className="button button--primary"
            disabled={!workspace.canExport || isExporting}
            onClick={onExportPDF}
            type="button"
          >
            <IconDownload aria-hidden="true" size={18} stroke={1.65} />
            {isExporting ? "正在导出" : "导出 PDF"}
          </button>
          <button
            className="button button--secondary"
            disabled={!workspace.canExport || isExporting}
            onClick={onExportMarkdown}
            type="button"
          >
            <IconFileText aria-hidden="true" size={18} stroke={1.65} />
            导出 Markdown
          </button>
          <button
            className="pdf-render-again"
            disabled={isRendering}
            onClick={onRender}
            type="button"
          >
            <IconRefresh
              aria-hidden="true"
              className={isRendering ? "is-spinning" : ""}
              size={16}
              stroke={1.7}
            />
            {isRendering ? "正在渲染" : "重新渲染当前版本"}
          </button>
        </div>
      </aside>
    </div>
  );
}

export default PDFPreview;
