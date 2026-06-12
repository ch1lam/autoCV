import {
  IconAlertTriangle,
  IconArchive,
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconChevronUp,
  IconCopy,
  IconDatabase,
  IconInfoCircle,
  IconPointFilled,
  IconRefresh,
  IconTargetArrow,
} from "@tabler/icons-react";
import { useEffect, useMemo, useState } from "react";

import type {
  MatchEvidenceSummary,
  MatchEvidenceSourceSummary,
  MatchReview,
  RequirementMatchSummary,
} from "../bindings/github.com/ch1lam/autocv/internal/app";

export type MatchWorkspaceStatus = "loading" | "ready" | "error";
type MatchStrength = "strong" | "partial" | "missing" | "unknown";
type Filter = "all" | MatchStrength;
type SortMode = "importance" | "status" | "evidence";

type MatchReviewWorkspaceProps = {
  error: string;
  isAnalyzing: boolean;
  onAnalyze: () => void;
  onOpenJD: () => void;
  onOpenProfile: () => void;
  review: MatchReview | null;
  status: MatchWorkspaceStatus;
};

const statusMeta: Record<
  MatchStrength,
  { label: string; rank: number }
> = {
  strong: { label: "强匹配", rank: 0 },
  partial: { label: "部分匹配", rank: 1 },
  unknown: { label: "信息不足", rank: 2 },
  missing: { label: "缺失", rank: 3 },
};

function MatchReviewWorkspace({
  error,
  isAnalyzing,
  onAnalyze,
  onOpenJD,
  onOpenProfile,
  review,
  status,
}: MatchReviewWorkspaceProps) {
  const [filter, setFilter] = useState<Filter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("importance");
  const [sortOpen, setSortOpen] = useState(false);
  const [selectedId, setSelectedId] = useState("");
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const [expandedEvidence, setExpandedEvidence] = useState<Set<string>>(
    new Set(),
  );
  const [selectedEvidenceId, setSelectedEvidenceId] = useState("");
  const [selectedSourceId, setSelectedSourceId] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!review || review.status !== "ready") {
      return;
    }
    const firstRequirement = review.requirements[0];
    setSelectedId((current) =>
      review.requirements.some((item) => item.id === current)
        ? current
        : (firstRequirement?.id ?? ""),
    );
    setExpandedGroups(new Set(review.dimensions.map((item) => item.label)));
  }, [review]);

  const selectedRequirement =
    review?.requirements.find((item) => item.id === selectedId) ??
    review?.requirements[0] ??
    null;

  useEffect(() => {
    const firstEvidence = selectedRequirement?.evidence[0];
    const firstSource = firstEvidence?.sources[0];
    setSelectedEvidenceId(firstEvidence?.id ?? "");
    setSelectedSourceId(firstSource?.chunkId ?? "");
    setExpandedEvidence(new Set(firstEvidence ? [firstEvidence.id] : []));
    setCopied(false);
  }, [selectedRequirement]);

  const groups = useMemo(() => {
    if (!review) {
      return [];
    }
    return review.dimensions
      .map((dimension) => {
        const requirements = review.requirements
          .filter((item) => item.group === dimension.label)
          .filter(
            (item) =>
              filter === "all" ||
              item.strength === filter,
          )
          .slice()
          .sort((left, right) => {
            if (sortMode === "status") {
              return (
                statusMeta[left.strength as MatchStrength].rank -
                statusMeta[right.strength as MatchStrength].rank
              );
            }
            if (sortMode === "evidence") {
              return right.evidence.length - left.evidence.length;
            }
            return right.importance - left.importance;
          });
        const summaryStrength = requirements.reduce<MatchStrength>(
          (current, requirement) => {
            const strength = requirement.strength as MatchStrength;
            return statusMeta[strength].rank > statusMeta[current].rank
              ? strength
              : current;
          },
          "strong",
        );
        return {
          id: dimension.category,
          label: dimension.label,
          requirements,
          summaryStrength,
          evidenceCount: requirements.reduce(
            (total, item) => total + item.evidence.length,
            0,
          ),
        };
      })
      .filter((group) => group.requirements.length > 0);
  }, [filter, review, sortMode]);

  useEffect(() => {
    const visibleRequirements = groups.flatMap((group) => group.requirements);
    if (
      visibleRequirements.length > 0 &&
      !visibleRequirements.some((item) => item.id === selectedId)
    ) {
      setSelectedId(visibleRequirements[0].id);
    }
  }, [groups, selectedId]);

  const selectedEvidence =
    selectedRequirement?.evidence.find(
      (item) => item.id === selectedEvidenceId,
    ) ??
    selectedRequirement?.evidence[0] ??
    null;
  const selectedSource =
    selectedEvidence?.sources.find(
      (item) => item.chunkId === selectedSourceId,
    ) ??
    selectedEvidence?.sources[0] ??
    null;

  if (status === "loading") {
    return (
      <section className="match-stage-state" aria-label="正在读取匹配结果">
        <IconRefresh className="is-spinning" size={28} stroke={1.5} />
        <span>匹配审阅</span>
        <h1>正在读取 Requirement 与 Evidence</h1>
        <p>Go 服务正在恢复最近一次结构化匹配结果。</p>
      </section>
    );
  }

  if (status === "error") {
    return (
      <section className="match-stage-state match-stage-state--error">
        <IconAlertTriangle size={28} stroke={1.5} />
        <span>匹配服务不可用</span>
        <h1>无法读取本地匹配结果</h1>
        <p>{error || "请检查本地数据库后重试。"}</p>
        <button
          className="button button--primary"
          onClick={onAnalyze}
          type="button"
        >
          <IconRefresh size={18} stroke={1.6} />
          重试匹配
        </button>
      </section>
    );
  }

  if (!review || review.status !== "ready") {
    const isFailed = review?.status === "failed";
    return (
      <section
        className={`match-stage-state ${
          isFailed ? "match-stage-state--error" : ""
        }`}
      >
        {isFailed ? (
          <IconAlertTriangle size={28} stroke={1.5} />
        ) : (
          <IconTargetArrow size={28} stroke={1.5} />
        )}
        <span>
          {review?.status === "stale"
            ? "结果已失效"
            : isFailed
              ? "阶段错误"
              : "匹配审阅"}
        </span>
        <h1>
          {review?.status === "stale"
            ? "资料发生变化，需要重新建立证据关联"
            : isFailed
              ? "上一次匹配分析未完成"
              : "准备 Requirement 与 Evidence"}
        </h1>
        <p>{review?.error || review?.message || "请先准备资料和目标 JD。"}</p>
        <div className="match-stage-actions">
          <button
            className="button button--primary"
            disabled={isAnalyzing}
            onClick={onAnalyze}
            type="button"
          >
            <IconRefresh
              className={isAnalyzing ? "is-spinning" : ""}
              size={18}
              stroke={1.6}
            />
            {isAnalyzing ? "正在匹配" : "开始匹配"}
          </button>
          {review?.status === "blocked" && (
            <>
              <button
                className="button button--secondary"
                onClick={onOpenProfile}
                type="button"
              >
                <IconArchive size={18} stroke={1.6} />
                打开资料库
              </button>
              <button
                className="button button--secondary"
                onClick={onOpenJD}
                type="button"
              >
                打开 JD 工作区
              </button>
            </>
          )}
        </div>
      </section>
    );
  }

  const counts = review.counts;
  const filters: Array<{ id: Filter; label: string; count: number }> = [
    { id: "all", label: "全部", count: review.requirements.length },
    { id: "strong", label: "强匹配", count: counts.strong },
    { id: "partial", label: "部分匹配", count: counts.partial },
    { id: "missing", label: "缺失", count: counts.missing },
    { id: "unknown", label: "信息不足", count: counts.unknown },
  ];

  const toggleGroup = (group: string) => {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(group)) {
        next.delete(group);
      } else {
        next.add(group);
      }
      return next;
    });
  };

  const selectRequirement = (requirement: RequirementMatchSummary) => {
    setSelectedId(requirement.id);
  };

  const toggleEvidence = (evidence: MatchEvidenceSummary) => {
    setExpandedEvidence((current) => {
      const next = new Set(current);
      if (next.has(evidence.id)) {
        next.delete(evidence.id);
      } else {
        next.add(evidence.id);
      }
      return next;
    });
    setSelectedEvidenceId(evidence.id);
    setSelectedSourceId(evidence.sources[0]?.chunkId ?? "");
  };

  const selectSource = (
    evidence: MatchEvidenceSummary,
    source: MatchEvidenceSourceSummary,
  ) => {
    setSelectedEvidenceId(evidence.id);
    setSelectedSourceId(source.chunkId);
  };

  const handleCopy = async () => {
    if (!selectedSource) {
      return;
    }
    await navigator.clipboard.writeText(selectedSource.chunkText);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  const sortLabel = {
    importance: "按重要性",
    status: "按匹配状态",
    evidence: "按证据数量",
  }[sortMode];

  return (
    <div className="review-layout">
      <main className="match-review">
        <section className="review-heading">
          <div>
            <span className="review-kicker">Evidence coverage</span>
            <h1>{review.jdTitle}</h1>
            <p>
              匹配审阅 · {review.company || "目标岗位"} ·
              只使用可定位到来源的 Evidence 计分
            </p>
          </div>
          <div className="score">
            <span>综合匹配</span>
            <strong>{review.totalScore}</strong>
            <small>/ 100</small>
          </div>
        </section>

        <section className="score-ledger" aria-label="匹配分项得分">
          {review.dimensions.map((dimension) => (
            <div key={dimension.category}>
              <span>{dimension.label}</span>
              <strong>
                {dimension.earned.toFixed(1)}
                <small> / {dimension.weight}</small>
              </strong>
              <em>
                {dimension.requirementCount > 0
                  ? `${dimension.requirementCount} 项`
                  : "无要求"}
              </em>
            </div>
          ))}
        </section>

        {review.hardCapApplied && (
          <p className="score-cap-notice">
            <IconAlertTriangle size={17} stroke={1.65} />
            存在未满足的明确硬性条件，综合分已按产品规则限制为最高 69。
          </p>
        )}

        <section className="review-controls">
          <div className="filter-tabs" role="tablist" aria-label="匹配筛选">
            {filters.map((tab) => (
              <button
                aria-selected={filter === tab.id}
                className={filter === tab.id ? "is-active" : ""}
                key={tab.id}
                onClick={() => setFilter(tab.id)}
                role="tab"
                type="button"
              >
                {tab.label}
                <span>{tab.count}</span>
              </button>
            ))}
          </div>
          <div className="sort-picker">
            <button
              aria-expanded={sortOpen}
              className="sort-trigger"
              onClick={() => setSortOpen((current) => !current)}
              type="button"
            >
              {sortLabel}
              <IconChevronDown size={16} stroke={1.5} />
            </button>
            {sortOpen && (
              <div className="sort-menu">
                {[
                  ["importance", "按重要性"],
                  ["status", "按匹配状态"],
                  ["evidence", "按证据数量"],
                ].map(([value, label]) => (
                  <button
                    className={sortMode === value ? "is-selected" : ""}
                    key={value}
                    onClick={() => {
                      setSortMode(value as SortMode);
                      setSortOpen(false);
                    }}
                    type="button"
                  >
                    {sortMode === value && (
                      <IconCheck size={15} stroke={1.8} />
                    )}
                    <span>{label}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </section>

        <div className="requirement-table">
          <div className="table-head" aria-hidden="true">
            <span>要求</span>
            <span>匹配状态</span>
            <span>证据数量</span>
            <span />
          </div>
          <div className="table-body">
            {groups.map((group) => {
              const expanded = expandedGroups.has(group.label);
              return (
                <section className="requirement-group" key={group.id}>
                  <button
                    aria-expanded={expanded}
                    className="group-row"
                    onClick={() => toggleGroup(group.label)}
                    type="button"
                  >
                    <span className="group-label">
                      {expanded ? (
                        <IconChevronDown size={18} stroke={1.65} />
                      ) : (
                        <IconChevronRight size={18} stroke={1.65} />
                      )}
                      <strong>{group.label}</strong>
                      <small>({group.requirements.length})</small>
                    </span>
                    {!expanded && (
                      <>
                        <span
                          className={`status-badge status-badge--${group.summaryStrength}`}
                        >
                          <IconPointFilled size={15} />
                          {statusMeta[group.summaryStrength].label}
                        </span>
                        <span className="group-evidence">
                          {group.evidenceCount}
                        </span>
                      </>
                    )}
                  </button>
                  {expanded &&
                    group.requirements.map((requirement) => {
                      const strength =
                        requirement.strength as MatchStrength;
                      return (
                        <button
                          className={`requirement-row ${
                            requirement.id === selectedRequirement?.id
                              ? "is-selected"
                              : ""
                          }`}
                          key={requirement.id}
                          onClick={() => selectRequirement(requirement)}
                          type="button"
                        >
                          <span className="requirement-text">
                            <small>
                              {review.requirements.indexOf(requirement) + 1}
                            </small>
                            <span>
                              {requirement.text}
                              {requirement.hardConstraint && (
                                <em>硬性</em>
                              )}
                            </span>
                          </span>
                          <span
                            className={`status-badge status-badge--${strength}`}
                          >
                            <IconPointFilled size={15} />
                            {statusMeta[strength].label}
                          </span>
                          <span className="evidence-count">
                            {requirement.evidence.length}
                          </span>
                          <IconChevronRight
                            className="row-chevron"
                            size={18}
                            stroke={1.55}
                          />
                        </button>
                      );
                    })}
                </section>
              );
            })}
            {groups.length === 0 && (
              <div className="empty-filter">
                当前筛选没有匹配要求。
                <button onClick={() => setFilter("all")} type="button">
                  查看全部
                </button>
              </div>
            )}
          </div>
        </div>
        <p className="review-hint">
          <IconInfoCircle size={17} stroke={1.6} />
          分数由 Go 按固定权重计算，不采用 Provider 返回的主观总分
        </p>
      </main>

      {selectedRequirement && (
        <aside className="evidence-panel" aria-label="来源证据">
          <header className="evidence-header">
            <h2>来源证据</h2>
            <span className="evidence-stage-label">
              {selectedRequirement.group}
            </span>
          </header>

          <section className="evidence-summary">
            <span>对应要求</span>
            <h3>{selectedRequirement.text}</h3>
            <dl>
              <div>
                <dt>匹配状态</dt>
                <dd
                  className={`status-badge status-badge--${selectedRequirement.strength}`}
                >
                  <IconPointFilled size={15} />
                  {
                    statusMeta[
                      selectedRequirement.strength as MatchStrength
                    ].label
                  }
                </dd>
              </div>
              <div>
                <dt>重要性</dt>
                <dd>{selectedRequirement.importance} / 5</dd>
              </div>
            </dl>
          </section>

          <section className="sources">
            <h3>Evidence（{selectedRequirement.evidence.length}）</h3>
            {selectedRequirement.evidence.length === 0 ? (
              <div className="empty-source">
                <IconDatabase size={24} stroke={1.5} />
                <strong>
                  {selectedRequirement.strength === "unknown"
                    ? "当前信息不足"
                    : "当前资料中没有直接证据"}
                </strong>
                <p>
                  {selectedRequirement.clarificationNeeded
                    ? "后续追问阶段会确认用户是否具备该项能力。"
                    : "该要求不会增加当前匹配分。"}
                </p>
              </div>
            ) : (
              selectedRequirement.evidence.map((evidence) => {
                const expanded = expandedEvidence.has(evidence.id);
                return (
                  <article className="source-item" key={evidence.id}>
                    <button
                      aria-expanded={expanded}
                      className="source-trigger"
                      onClick={() => toggleEvidence(evidence)}
                      type="button"
                    >
                      {expanded ? (
                        <IconChevronUp size={17} stroke={1.5} />
                      ) : (
                        <IconChevronRight size={17} stroke={1.5} />
                      )}
                      <span>{evidence.title}</span>
                    </button>
                    {expanded && (
                      <div className="source-snippets">
                        {evidence.sources.map((source) => (
                          <button
                            className={
                              source.chunkId === selectedSource?.chunkId
                                ? "is-current"
                                : ""
                            }
                            key={source.chunkId}
                            onClick={() => selectSource(evidence, source)}
                            type="button"
                          >
                            <IconPointFilled size={12} />
                            <span>
                              {source.quoteStart}–{source.quoteEnd}
                            </span>
                            <strong>
                              {source.documentName || source.documentId}
                            </strong>
                          </button>
                        ))}
                      </div>
                    )}
                  </article>
                );
              })
            )}
          </section>

          {selectedEvidence && selectedSource && (
            <section className="source-content">
              <header>
                <h3>来源内容</h3>
                <span>
                  来自 {selectedSource.documentName || selectedSource.documentId}
                </span>
              </header>
              <div className="source-code">
                {selectedSource.chunkText.split("\n").map((line, index) => (
                  <div className="source-line" key={`${line}-${index}`}>
                    <span>{index + 1}</span>
                    <code>{line || " "}</code>
                  </div>
                ))}
                <button
                  className="copy-button"
                  onClick={() => void handleCopy()}
                  type="button"
                >
                  {copied ? (
                    <IconCheck size={15} stroke={1.7} />
                  ) : (
                    <IconCopy size={15} stroke={1.7} />
                  )}
                  {copied ? "已复制" : "复制"}
                </button>
              </div>
            </section>
          )}

          <section className="match-explanation">
            <h3>匹配说明</h3>
            <p>{selectedRequirement.explanation}</p>
          </section>
        </aside>
      )}
    </div>
  );
}

export default MatchReviewWorkspace;
