import {
  IconAlertTriangle,
  IconCheck,
  IconChevronRight,
  IconFileDescription,
  IconInfoCircle,
  IconLink,
  IconLock,
  IconLockOpen,
  IconRefresh,
} from "@tabler/icons-react";
import { useEffect, useMemo, useState } from "react";

import type {
  MatchEvidenceSourceSummary,
  MatchEvidenceSummary,
  ResumeBlockSummary,
  ResumeWorkspace,
} from "../bindings/github.com/ch1lam/autocv/internal/app";

export type ResumeStudioStatus = "loading" | "ready" | "error";
export type ResumeStudioFeedback = {
  tone: "success" | "error" | "info";
  text: string;
};

type ResumeStudioProps = {
  error: string;
  feedback: ResumeStudioFeedback | null;
  isDirty: boolean;
  isLocking: boolean;
  isSaving: boolean;
  markdown: string;
  onChange: (value: string) => void;
  onGenerate: () => void;
  onOpenMatch: () => void;
  onRetry: () => void;
  onSave: () => void;
  onToggleLock: (block: ResumeBlockSummary) => void;
  status: ResumeStudioStatus;
  workspace: ResumeWorkspace | null;
};

const groundingLabels: Record<string, string> = {
  derived: "来源推导",
  source: "来源原文",
  user_confirmed: "用户确认",
};

function formatUpdatedAt(value: string) {
  if (!value) {
    return "尚未生成";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function ResumeStudio({
  error,
  feedback,
  isDirty,
  isLocking,
  isSaving,
  markdown,
  onChange,
  onGenerate,
  onOpenMatch,
  onRetry,
  onSave,
  onToggleLock,
  status,
  workspace,
}: ResumeStudioProps) {
  const [selectedBlockId, setSelectedBlockId] = useState("");
  const [selectedEvidenceId, setSelectedEvidenceId] = useState("");
  const [selectedSourceId, setSelectedSourceId] = useState("");

  useEffect(() => {
    if (!workspace || workspace.blocks.length === 0) {
      setSelectedBlockId("");
      return;
    }
    setSelectedBlockId((current) =>
      workspace.blocks.some((block) => block.id === current)
        ? current
        : workspace.blocks[0].id,
    );
  }, [workspace]);

  const selectedBlock = useMemo(
    () =>
      workspace?.blocks.find((block) => block.id === selectedBlockId) ??
      workspace?.blocks[0] ??
      null,
    [selectedBlockId, workspace],
  );

  useEffect(() => {
    const evidence = selectedBlock?.evidence[0];
    setSelectedEvidenceId(evidence?.id ?? "");
    setSelectedSourceId(evidence?.sources[0]?.chunkId ?? "");
  }, [selectedBlock]);

  const selectedEvidence =
    selectedBlock?.evidence.find(
      (evidence) => evidence.id === selectedEvidenceId,
    ) ??
    selectedBlock?.evidence[0] ??
    null;
  const selectedSource =
    selectedEvidence?.sources.find(
      (source) => source.chunkId === selectedSourceId,
    ) ??
    selectedEvidence?.sources[0] ??
    null;

  const selectEvidence = (evidence: MatchEvidenceSummary) => {
    setSelectedEvidenceId(evidence.id);
    setSelectedSourceId(evidence.sources[0]?.chunkId ?? "");
  };

  const selectSource = (source: MatchEvidenceSourceSummary) => {
    setSelectedSourceId(source.chunkId);
  };

  if (status === "loading") {
    return (
      <section className="match-stage-state" aria-label="正在读取简历工作室">
        <IconRefresh className="is-spinning" size={28} stroke={1.5} />
        <span>Resume Studio</span>
        <h1>正在恢复结构化简历与 Markdown</h1>
        <p>Go 服务正在读取最新版本、锁定状态和来源关系。</p>
      </section>
    );
  }

  if (status === "error") {
    return (
      <section className="match-stage-state match-stage-state--error">
        <IconAlertTriangle size={28} stroke={1.5} />
        <span>简历服务不可用</span>
        <h1>无法读取本地简历版本</h1>
        <p>{error || "请检查本地数据库后重试。"}</p>
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

  if (!workspace || workspace.status !== "ready") {
    const stale = workspace?.status === "stale";
    return (
      <section className="match-stage-state">
        <IconFileDescription size={28} stroke={1.5} />
        <span>Resume Studio</span>
        <h1>
          {stale ? "输入已变化，需要重新生成未锁定内容" : "生成第一版针对性简历"}
        </h1>
        <p>
          {workspace?.message ||
            "完成资料导入、JD 分析和匹配后，从结构化 Resume 派生 Markdown。"}
        </p>
        <div className="match-stage-actions">
          <button
            className="button button--primary"
            onClick={onGenerate}
            type="button"
          >
            <IconFileDescription size={18} stroke={1.6} />
            {stale ? "重新生成" : "生成简历"}
          </button>
          <button
            className="button button--secondary"
            onClick={onOpenMatch}
            type="button"
          >
            返回匹配审阅
          </button>
        </div>
      </section>
    );
  }

  return (
    <div className="resume-studio-layout">
      <main className="resume-editor">
        <section className="resume-heading">
          <div>
            <span className="section-kicker">RESUME STUDIO</span>
            <h1>{workspace.targetRole}</h1>
            <p>
              第 {workspace.version} 版 · {workspace.packagingLabel}包装 ·
              {workspace.language === "en" ? " English" : " 中文"}
            </p>
          </div>
          <div className="resume-sync-state">
            <span className={isDirty ? "is-dirty" : ""}>
              {isDirty ? "有未保存修改" : "本地已同步"}
            </span>
            <time dateTime={workspace.updatedAt}>
              {formatUpdatedAt(workspace.updatedAt)}
            </time>
          </div>
        </section>

        {feedback && (
          <div
            className={`profile-feedback profile-feedback--${feedback.tone}`}
            role="status"
          >
            {feedback.tone === "error" ? (
              <IconAlertTriangle aria-hidden="true" size={18} stroke={1.7} />
            ) : feedback.tone === "success" ? (
              <IconCheck aria-hidden="true" size={18} stroke={1.7} />
            ) : (
              <IconInfoCircle aria-hidden="true" size={18} stroke={1.7} />
            )}
            <span>{feedback.text}</span>
          </div>
        )}

        {!workspace.canExport && workspace.exportIssues.length > 0 && (
          <section className="resume-export-gate" role="alert">
            <IconAlertTriangle aria-hidden="true" size={19} stroke={1.7} />
            <div>
              <strong>最终导出已阻止</strong>
              <p>以下内容需要补充来源或由用户明确确认：</p>
              <ul>
                {workspace.exportIssues.map((issue) => (
                  <li key={issue}>{issue}</li>
                ))}
              </ul>
            </div>
          </section>
        )}

        <section
          aria-label="本次优化说明"
          className="resume-optimization-ledger"
        >
          <header>
            <span>本次优化说明</span>
            <strong>{workspace.optimizationNotes.length}</strong>
          </header>
          <ol>
            {workspace.optimizationNotes.map((note, index) => (
              <li key={`${index}-${note}`}>
                <span>{String(index + 1).padStart(2, "0")}</span>
                {note}
              </li>
            ))}
          </ol>
        </section>

        <label className="resume-markdown-field">
          <span>受约束 Markdown</span>
          <textarea
            aria-label="简历 Markdown"
            onChange={(event) => onChange(event.target.value)}
            spellCheck={false}
            value={markdown}
          />
        </label>

        <footer className="resume-editor-footer">
          <p>
            <IconInfoCircle aria-hidden="true" size={16} stroke={1.6} />
            只修改 Block 标记之间的正文；标题、顺序或标记变化会被 Go
            侧拒绝。
          </p>
          <button
            className="button button--primary"
            disabled={!isDirty || isSaving}
            onClick={onSave}
            type="button"
          >
            {isSaving ? "正在保存" : "保存为新版本"}
          </button>
        </footer>
      </main>

      <aside aria-label="简历 Block 检查器" className="resume-inspector">
        <header className="evidence-header">
          <h2>Block 与来源</h2>
          <span className="evidence-stage-label">
            {workspace.blocks.length} 个内容块
          </span>
        </header>

        <nav aria-label="简历内容块" className="resume-block-nav">
          {workspace.blocks.map((block, index) => (
            <button
              className={block.id === selectedBlock?.id ? "is-selected" : ""}
              key={block.id}
              onClick={() => setSelectedBlockId(block.id)}
              type="button"
            >
              <span>{String(index + 1).padStart(2, "0")}</span>
              <strong>{block.label}</strong>
              {block.locked ? (
                <IconLock aria-label="已锁定" size={15} stroke={1.7} />
              ) : (
                <IconChevronRight aria-hidden="true" size={16} stroke={1.6} />
              )}
            </button>
          ))}
        </nav>

        {selectedBlock && (
          <>
            <section className="resume-block-detail">
              <div className="resume-block-detail-heading">
                <span>
                  {selectedBlock.groundingLevel === "derived" &&
                  selectedBlock.evidence.length === 0
                    ? "待确认推导"
                    : groundingLabels[selectedBlock.groundingLevel] ??
                      selectedBlock.groundingLevel}
                </span>
                <button
                  className="button button--secondary resume-lock-button"
                  disabled={isLocking}
                  onClick={() => onToggleLock(selectedBlock)}
                  type="button"
                >
                  {selectedBlock.locked ? (
                    <IconLockOpen size={16} stroke={1.7} />
                  ) : (
                    <IconLock size={16} stroke={1.7} />
                  )}
                  {selectedBlock.locked ? "解除锁定" : "锁定内容"}
                </button>
              </div>
              <h3>{selectedBlock.label}</h3>
              <p>{selectedBlock.content}</p>
              <div className="resume-block-optimization">
                <span>调整原因</span>
                {selectedBlock.optimization}
              </div>
            </section>

            <section className="resume-source-list">
              <header>
                <h3>来源 Evidence</h3>
                <span>{selectedBlock.evidence.length}</span>
              </header>
              {selectedBlock.evidence.length === 0 ? (
                <p className="empty-source">
                  {selectedBlock.groundingLevel === "user_confirmed"
                    ? "该内容已由用户确认，不依赖来源引用。"
                    : "该内容尚无来源，最终导出前需要确认或补充 Evidence。"}
                </p>
              ) : (
                selectedBlock.evidence.map((evidence) => (
                  <div className="resume-source-item" key={evidence.id}>
                    <button
                      className={
                        evidence.id === selectedEvidence?.id
                          ? "is-selected"
                          : ""
                      }
                      onClick={() => selectEvidence(evidence)}
                      type="button"
                    >
                      <IconLink aria-hidden="true" size={15} stroke={1.65} />
                      <span>
                        <strong>{evidence.title}</strong>
                        <small>{evidence.content}</small>
                      </span>
                    </button>
                    {evidence.id === selectedEvidence?.id &&
                      evidence.sources.map((source) => (
                        <button
                          className={`resume-source-location ${
                            source.chunkId === selectedSource?.chunkId
                              ? "is-selected"
                              : ""
                          }`}
                          key={source.chunkId}
                          onClick={() => selectSource(source)}
                          type="button"
                        >
                          {source.documentName || "来源文档"}
                        </button>
                      ))}
                  </div>
                ))
              )}
            </section>

            {selectedSource && (
              <section className="resume-source-preview">
                <span>{selectedSource.documentName || "来源片段"}</span>
                <p>{selectedSource.chunkText}</p>
              </section>
            )}
          </>
        )}
      </aside>
    </div>
  );
}

export default ResumeStudio;
