import {
  IconAlertCircle,
  IconCircleCheck,
  IconFileText,
  IconInfoCircle,
  IconPointFilled,
  IconRefresh,
} from "@tabler/icons-react";

import type {
  JDAnalysisSummary,
  JDRequirementSummary,
  JDWorkspace as JDWorkspaceModel,
} from "../bindings/github.com/ch1lam/autocv/internal/app";

export type JDWorkspaceStatus = "loading" | "ready" | "error";
export type JDWorkspaceFeedback = {
  tone: "success" | "warning" | "info" | "error";
  text: string;
};

type JDWorkspaceProps = {
  error: string;
  feedback: JDWorkspaceFeedback | null;
  isAnalyzing: boolean;
  isDirty: boolean;
  isSaving: boolean;
  onAnalyze: () => void;
  onChange: (value: string) => void;
  onRetry: () => void;
  onSave: () => void;
  rawText: string;
  status: JDWorkspaceStatus;
  workspace: JDWorkspaceModel | null;
};

type RequirementSectionProps = {
  emptyText: string;
  items: JDRequirementSummary[];
  title: string;
};

function formatUpdatedAt(value: string) {
  if (!value) {
    return "尚未保存";
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

function RequirementSection({
  emptyText,
  items,
  title,
}: RequirementSectionProps) {
  return (
    <section className="jd-analysis-section">
      <header>
        <h3>{title}</h3>
        <span>{items.length}</span>
      </header>
      {items.length === 0 ? (
        <p className="jd-analysis-empty-copy">{emptyText}</p>
      ) : (
        <ol className="jd-requirement-list">
          {items.map((item, index) => (
            <li key={item.id}>
              <span className="jd-requirement-index">
                {String(index + 1).padStart(2, "0")}
              </span>
              <span className="jd-requirement-copy">
                <strong>{item.text}</strong>
                <small>
                  重要性 {item.importance}/5
                  {item.hardConstraint ? " · 硬性条件" : ""}
                </small>
              </span>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

function TextList({
  emptyText,
  items,
  title,
}: {
  emptyText: string;
  items: string[];
  title: string;
}) {
  return (
    <section className="jd-analysis-section jd-analysis-section--text">
      <header>
        <h3>{title}</h3>
        <span>{items.length}</span>
      </header>
      {items.length === 0 ? (
        <p className="jd-analysis-empty-copy">{emptyText}</p>
      ) : (
        <ul>
          {items.map((item) => (
            <li key={item}>
              <IconPointFilled aria-hidden="true" size={10} />
              <span>{item}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function AnalysisResult({ analysis }: { analysis: JDAnalysisSummary }) {
  return (
    <>
      <section className="jd-role-summary">
        <span className="section-kicker">ROLE PROFILE</span>
        <h2>{analysis.role}</h2>
        <dl>
          <div>
            <dt>公司</dt>
            <dd>{analysis.company || "未识别"}</dd>
          </div>
          <div>
            <dt>级别</dt>
            <dd>{analysis.level || "未识别"}</dd>
          </div>
          <div>
            <dt>语言</dt>
            <dd>{analysis.language || "未识别"}</dd>
          </div>
        </dl>
      </section>

      <RequirementSection
        emptyText="没有提取到明确职责。"
        items={analysis.responsibilities}
        title="主要职责"
      />
      <RequirementSection
        emptyText="没有提取到必要技能。"
        items={analysis.requiredSkills}
        title="必要技能"
      />
      <RequirementSection
        emptyText="没有提取到加分技能。"
        items={analysis.preferredSkills}
        title="加分技能"
      />
      <TextList
        emptyText="没有识别到领域信号。"
        items={analysis.domainSignals}
        title="领域信号"
      />
      <TextList
        emptyText="没有识别到筛选条件。"
        items={analysis.screeningConstraints}
        title="筛选条件"
      />
      <TextList
        emptyText="没有需要人工确认的歧义。"
        items={analysis.ambiguities}
        title="模糊与冲突"
      />
    </>
  );
}

function JDWorkspace({
  error,
  feedback,
  isAnalyzing,
  isDirty,
  isSaving,
  onAnalyze,
  onChange,
  onRetry,
  onSave,
  rawText,
  status,
  workspace,
}: JDWorkspaceProps) {
  const hasAnalysis =
    workspace?.analysisStatus === "succeeded" && workspace.analysis;
  const isFailed = workspace?.analysisStatus === "failed";

  return (
    <div className="jd-workspace-layout">
      <main className="jd-editor">
        <section className="jd-heading">
          <div>
            <span className="section-kicker">JD WORKSPACE</span>
            <h1>岗位原始文本</h1>
            <p>粘贴完整 JD，保存原文后生成经过 Go 侧校验的结构化要求。</p>
          </div>
          <div className="jd-document-state" aria-live="polite">
            <span className={isDirty ? "is-dirty" : ""}>
              {isDirty ? "有未保存修改" : "本地已同步"}
            </span>
            <time dateTime={workspace?.updatedAt}>
              {formatUpdatedAt(workspace?.updatedAt ?? "")}
            </time>
          </div>
        </section>

        {feedback && (
          <div
            className={`profile-feedback profile-feedback--${feedback.tone}`}
            role="status"
          >
            {feedback.tone === "success" ? (
              <IconCircleCheck aria-hidden="true" size={18} stroke={1.7} />
            ) : feedback.tone === "error" ? (
              <IconAlertCircle aria-hidden="true" size={18} stroke={1.7} />
            ) : (
              <IconInfoCircle aria-hidden="true" size={18} stroke={1.7} />
            )}
            <span>{feedback.text}</span>
          </div>
        )}

        {status === "error" && !workspace ? (
          <div className="profile-state profile-state--error">
            <IconAlertCircle aria-hidden="true" size={23} stroke={1.6} />
            <strong>无法读取 JD 工作区</strong>
            <p>{error || "请确认 AutoCV 本地服务已经启动。"}</p>
            <button
              className="button button--secondary"
              onClick={onRetry}
              type="button"
            >
              <IconRefresh aria-hidden="true" size={17} stroke={1.65} />
              重新读取
            </button>
          </div>
        ) : (
          <>
            <div className="jd-editor-meta">
              <span>
                <IconFileText aria-hidden="true" size={17} stroke={1.6} />
                {workspace?.title || "待分析职位"}
              </span>
              <span>{rawText.length.toLocaleString("zh-CN")} 字符</span>
            </div>
            <label className="jd-textarea-field">
              <span>岗位 JD 原始文本</span>
              <textarea
                aria-label="岗位 JD 原始文本"
                disabled={status === "loading" && !workspace}
                onChange={(event) => onChange(event.target.value)}
                placeholder={`在这里粘贴完整职位描述……

建议保留岗位名称、职责、必要技能、加分项、地点与年限等原始信息。`}
                spellCheck={false}
                value={rawText}
              />
            </label>

            <div className="jd-editor-footer">
              <p>
                <IconInfoCircle aria-hidden="true" size={16} stroke={1.6} />
                编辑后，旧分析会立即失效；重新分析会先保存当前原文。
              </p>
              <div>
                <button
                  className="button button--secondary"
                  disabled={
                    !isDirty ||
                    isSaving ||
                    isAnalyzing ||
                    rawText.trim() === ""
                  }
                  onClick={onSave}
                  type="button"
                >
                  {isSaving ? "正在保存" : "保存原文"}
                </button>
                <button
                  className="button button--primary"
                  disabled={isSaving || isAnalyzing || rawText.trim() === ""}
                  onClick={onAnalyze}
                  type="button"
                >
                  {isAnalyzing ? "正在分析" : "分析 JD"}
                </button>
              </div>
            </div>

            {(workspace?.warnings.length ?? 0) > 0 && !isDirty && (
              <section className="jd-warning-list" aria-label="JD 解析提示">
                {workspace?.warnings.map((warning) => (
                  <p key={warning}>
                    <IconInfoCircle
                      aria-hidden="true"
                      size={16}
                      stroke={1.65}
                    />
                    {warning}
                  </p>
                ))}
              </section>
            )}
          </>
        )}
      </main>

      <aside className="jd-analysis-panel" aria-label="JD 结构化分析">
        <header className="evidence-header">
          <div>
            <h2>结构化分析</h2>
            <span>Schema 校验后写入本地数据库</span>
          </div>
          <span
            className={`jd-analysis-status jd-analysis-status--${
              isDirty ? "stale" : workspace?.analysisStatus || "empty"
            }`}
          >
            {isDirty
              ? "已失效"
              : isAnalyzing
                ? "分析中"
                : workspace?.analysisStatus === "succeeded"
                ? "已完成"
                : workspace?.analysisStatus === "failed"
                  ? "失败"
                  : "等待分析"}
          </span>
        </header>

        <div className="jd-analysis-scroll">
          {status === "loading" && !workspace ? (
            <div className="jd-analysis-state">
              <IconRefresh
                aria-hidden="true"
                className="is-spinning"
                size={22}
                stroke={1.6}
              />
              正在读取本地 JD
            </div>
          ) : isAnalyzing ? (
            <div className="jd-analysis-state">
              <IconRefresh
                aria-hidden="true"
                className="is-spinning"
                size={24}
                stroke={1.6}
              />
              <strong>正在分析岗位要求</strong>
              <p>Fake Provider 返回后还会经过 Schema 与业务规则校验。</p>
            </div>
          ) : isDirty ? (
            <div className="jd-analysis-state jd-analysis-state--warning">
              <IconAlertCircle aria-hidden="true" size={26} stroke={1.55} />
              <strong>当前结果已失效</strong>
              <p>原始 JD 已发生变化。保存原文或重新分析后再使用结构化结果。</p>
            </div>
          ) : isFailed ? (
            <div className="jd-analysis-state jd-analysis-state--error">
              <IconAlertCircle aria-hidden="true" size={26} stroke={1.55} />
              <strong>JD 分析未通过</strong>
              <p>
                {workspace?.analysisError ||
                  "结构化结果未通过 Go 侧校验，请修改 JD 后重试。"}
              </p>
            </div>
          ) : hasAnalysis ? (
            <AnalysisResult analysis={workspace.analysis as JDAnalysisSummary} />
          ) : (
            <div className="jd-analysis-state">
              <IconFileText aria-hidden="true" size={27} stroke={1.45} />
              <strong>等待第一份分析结果</strong>
              <p>粘贴 JD 后点击“分析 JD”，职责、技能与筛选条件会显示在这里。</p>
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}

export default JDWorkspace;
