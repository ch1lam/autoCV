import {
    IconArchive,
    IconCheck,
    IconChevronDown,
    IconChevronRight,
    IconChevronUp,
    IconCopy,
    IconDatabase,
    IconEdit,
    IconFileDescription,
    IconFileText,
    IconFileTypePdf,
    IconInfoCircle,
    IconPointFilled,
    IconRefresh,
    IconSettings,
    IconTargetArrow,
    IconUpload,
    IconX,
} from "@tabler/icons-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
    HealthService,
    JDService,
    ProfileService,
    type EvidenceSourceSummary,
    type EvidenceSummary,
    type JDWorkspace as JDWorkspaceModel,
    type ProfileOverview,
} from "../bindings/github.com/ch1lam/autocv/internal/app";
import JDWorkspace, {
    type JDWorkspaceFeedback,
    type JDWorkspaceStatus,
} from "./JDWorkspace";
import {
    allRequirements,
    requirementGroups,
    type MatchStatus,
    type Requirement,
} from "./mockData";
import ProfileLibrary, {
    type ProfileFeedback,
    type ProfileStatus,
} from "./ProfileLibrary";

type HealthState = "checking" | "ready" | "preview";
type Filter = "all" | MatchStatus;
type SortMode = "importance" | "status" | "evidence";

const navItems = [
  { label: "资料库", icon: IconArchive },
  { label: "JD 工作区", icon: IconFileText },
  { label: "匹配审阅", icon: IconTargetArrow },
  { label: "简历工作室", icon: IconEdit },
  { label: "PDF 预览", icon: IconFileTypePdf },
];

const statusMeta: Record<
  MatchStatus,
  { label: string; className: string; rank: number }
> = {
  strong: { label: "强匹配", className: "strong", rank: 0 },
  partial: { label: "部分匹配", className: "partial", rank: 1 },
  missing: { label: "缺失", className: "missing", rank: 2 },
};

const statusCounts = allRequirements.reduce(
  (counts, requirement) => {
    counts[requirement.status] += 1;
    return counts;
  },
  { strong: 0, partial: 0, missing: 0 },
);

function App() {
  const [health, setHealth] = useState<HealthState>("checking");
  const [activeNav, setActiveNav] = useState("匹配审阅");
  const [selectedId, setSelectedId] = useState("go-concurrency");
  const [filter, setFilter] = useState<Filter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("importance");
  const [sortOpen, setSortOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [inspectorOpen, setInspectorOpen] = useState(true);
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(
    new Set(["technical"]),
  );
  const [expandedSources, setExpandedSources] = useState<Set<string>>(
    new Set(["source-backend"]),
  );
  const [isAnalysing, setIsAnalysing] = useState(false);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [notice, setNotice] = useState("");
  const [profileOverview, setProfileOverview] =
    useState<ProfileOverview | null>(null);
  const [profileStatus, setProfileStatus] =
    useState<ProfileStatus>("loading");
  const [profileError, setProfileError] = useState("");
  const [profileFeedback, setProfileFeedback] =
    useState<ProfileFeedback | null>(null);
  const [isImportingProfile, setIsImportingProfile] = useState(false);
  const [jdWorkspace, setJDWorkspace] =
    useState<JDWorkspaceModel | null>(null);
  const [jdStatus, setJDStatus] = useState<JDWorkspaceStatus>("loading");
  const [jdError, setJDError] = useState("");
  const [jdText, setJDText] = useState("");
  const [jdDirty, setJDDirty] = useState(false);
  const [jdFeedback, setJDFeedback] =
    useState<JDWorkspaceFeedback | null>(null);
  const [isSavingJD, setIsSavingJD] = useState(false);
  const [isAnalyzingJD, setIsAnalyzingJD] = useState(false);
  const [selectedProfileEvidenceId, setSelectedProfileEvidenceId] =
    useState("");
  const [selectedProfileSourceId, setSelectedProfileSourceId] = useState("");
  const analyseTimer = useRef<number>();
  const noticeTimer = useRef<number>();

  const refreshProfile = useCallback(async () => {
    setProfileStatus("loading");
    setProfileError("");
    try {
      const overview = await ProfileService.GetOverview();
      setProfileOverview(overview);
      setSelectedProfileEvidenceId((current) => {
        if (overview.evidence.some((item) => item.id === current)) {
          return current;
        }
        return overview.evidence[0]?.id ?? "";
      });
      setSelectedProfileSourceId((current) => {
        if (
          overview.evidence.some((item) =>
            item.sources.some((source) => source.chunkId === current),
          )
        ) {
          return current;
        }
        return overview.evidence[0]?.sources[0]?.chunkId ?? "";
      });
      setProfileStatus("ready");
    } catch (error) {
      setProfileStatus("error");
      setProfileError(
        error instanceof Error ? error.message : "本地资料服务暂不可用。",
      );
    }
  }, []);

  const applyJDWorkspace = useCallback((workspace: JDWorkspaceModel) => {
    setJDWorkspace(workspace);
    setJDText(workspace.rawText);
    setJDDirty(false);
    setJDStatus("ready");
    setJDError("");
  }, []);

  const refreshJD = useCallback(async () => {
    setJDStatus("loading");
    setJDError("");
    try {
      const workspace = await JDService.GetWorkspace();
      applyJDWorkspace(workspace);
    } catch (error) {
      setJDStatus("error");
      setJDError(
        error instanceof Error ? error.message : "本地 JD 服务暂不可用。",
      );
    }
  }, [applyJDWorkspace]);

  useEffect(() => {
    HealthService.Check()
      .then((status) => setHealth(status.status === "ready" ? "ready" : "preview"))
      .catch(() => setHealth("preview"));
    void refreshProfile();
    void refreshJD();

    return () => {
      window.clearTimeout(analyseTimer.current);
      window.clearTimeout(noticeTimer.current);
    };
  }, [refreshJD, refreshProfile]);

  const selectedRequirement =
    allRequirements.find((requirement) => requirement.id === selectedId) ??
    allRequirements[0];

  const visibleGroups = useMemo(() => {
    return requirementGroups
      .map((group) => {
        const requirements = group.requirements
          .filter(
            (requirement) =>
              filter === "all" || requirement.status === filter,
          )
          .slice()
          .sort((left, right) => {
            if (sortMode === "status") {
              return (
                statusMeta[left.status].rank - statusMeta[right.status].rank
              );
            }
            if (sortMode === "evidence") {
              return right.evidenceCount - left.evidenceCount;
            }
            return (
              allRequirements.indexOf(left) - allRequirements.indexOf(right)
            );
          });

        return { ...group, requirements };
      })
      .filter((group) => group.requirements.length > 0);
  }, [filter, sortMode]);

  const showNotice = (message: string) => {
    setNotice(message);
    window.clearTimeout(noticeTimer.current);
    noticeTimer.current = window.setTimeout(() => setNotice(""), 2400);
  };

  const handleNav = (label: string) => {
    if (
      label === "资料库" ||
      label === "JD 工作区" ||
      label === "匹配审阅"
    ) {
      setActiveNav(label);
      return;
    }
    showNotice(`${label}将在后续垂直切片中接入本地业务数据`);
  };

  const handleProfileImport = async () => {
    setIsImportingProfile(true);
    setProfileFeedback(null);
    try {
      const result = await ProfileService.ImportMarkdown();
      if (result.cancelled) {
        setProfileFeedback({
          tone: "info",
          text: "已取消导入，资料库没有发生变化。",
        });
        return;
      }
      if (result.duplicate) {
        setProfileFeedback({
          tone: "warning",
          text:
            result.warnings[0] ?? "相同内容已导入，未创建重复资料。",
        });
        return;
      }

      await refreshProfile();
      setProfileFeedback({
        tone: result.warnings.length > 0 ? "warning" : "success",
        text:
          result.warnings.length > 0
            ? `已导入 ${result.document.originalName}，生成 ${result.evidenceCount} 条 Evidence；另有 ${result.warnings.length} 条解析提示。`
            : `已导入 ${result.document.originalName}，生成 ${result.evidenceCount} 条可追溯 Evidence。`,
      });
    } catch (error) {
      setProfileFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `导入失败：${error.message}`
            : "导入失败，请重试。",
      });
    } finally {
      setIsImportingProfile(false);
    }
  };

  const handleSelectProfileEvidence = (evidence: EvidenceSummary) => {
    setSelectedProfileEvidenceId(evidence.id);
    setSelectedProfileSourceId(evidence.sources[0]?.chunkId ?? "");
  };

  const handleSelectProfileSource = (source: EvidenceSourceSummary) => {
    setSelectedProfileSourceId(source.chunkId);
  };

  const handleJDTextChange = (value: string) => {
    setJDText(value);
    setJDDirty(value.trim() !== (jdWorkspace?.rawText ?? ""));
    setJDFeedback(null);
  };

  const handleJDSave = async () => {
    if (jdText.trim() === "") {
      setJDFeedback({
        tone: "error",
        text: "请先粘贴岗位 JD，再保存原文。",
      });
      return;
    }

    setIsSavingJD(true);
    setJDFeedback(null);
    try {
      const workspace = await JDService.SaveDraft(jdText);
      applyJDWorkspace(workspace);
      setJDFeedback({
        tone: "success",
        text: "原始 JD 已保存；旧分析结果已失效。",
      });
    } catch (error) {
      setJDFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `保存失败：${error.message}`
            : "保存失败，请重试。",
      });
    } finally {
      setIsSavingJD(false);
    }
  };

  const handleJDAnalyze = async () => {
    if (jdText.trim() === "") {
      setJDFeedback({
        tone: "error",
        text: "请先粘贴岗位 JD，再开始分析。",
      });
      return;
    }

    setIsAnalyzingJD(true);
    setJDFeedback(null);
    try {
      const workspace = await JDService.Analyze(jdText);
      applyJDWorkspace(workspace);
      setJDFeedback({
        tone: "success",
        text: "JD 分析完成，结构化结果已通过 Go 侧校验。",
      });
    } catch (error) {
      setJDFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `分析失败：${error.message}`
            : "分析失败，请修改 JD 后重试。",
      });
      try {
        const workspace = await JDService.GetWorkspace();
        applyJDWorkspace(workspace);
      } catch {
        setJDStatus("error");
      }
    } finally {
      setIsAnalyzingJD(false);
    }
  };

  const toggleGroup = (groupId: string) => {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(groupId)) {
        next.delete(groupId);
      } else {
        next.add(groupId);
      }
      return next;
    });
  };

  const selectRequirement = (requirement: Requirement) => {
    setSelectedId(requirement.id);
    setInspectorOpen(true);
    setExpandedSources(
      new Set(requirement.sources[0] ? [requirement.sources[0].id] : []),
    );
    setCopied(false);
  };

  const toggleSource = (sourceId: string) => {
    setExpandedSources((current) => {
      const next = new Set(current);
      if (next.has(sourceId)) {
        next.delete(sourceId);
      } else {
        next.add(sourceId);
      }
      return next;
    });
  };

  const handleAnalyse = () => {
    setIsAnalysing(true);
    window.clearTimeout(analyseTimer.current);
    analyseTimer.current = window.setTimeout(() => {
      setIsAnalysing(false);
      showNotice("分析已更新，19 项要求的证据关联保持有效");
    }, 1100);
  };

  const handleCopy = async () => {
    const source = selectedRequirement.sources[0];
    if (!source) {
      return;
    }
    await navigator.clipboard.writeText(source.excerpt.join("\n"));
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  const sortLabel = {
    importance: "按重要性",
    status: "按匹配状态",
    evidence: "按证据数量",
  }[sortMode];

  return (
    <div className="app-shell" data-health={health}>
      <aside className="sidebar">
        <div className="brand">AutoCV</div>
        <nav className="primary-nav" aria-label="主要导航">
          {navItems.map(({ label, icon: Icon }) => (
            <button
              className={`nav-item ${activeNav === label ? "is-active" : ""}`}
              key={label}
              onClick={() => handleNav(label)}
              type="button"
            >
              <Icon aria-hidden="true" size={21} stroke={1.65} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <button
          className="nav-item nav-item--settings"
          onClick={() => handleNav("设置")}
          type="button"
        >
          <IconSettings aria-hidden="true" size={21} stroke={1.65} />
          <span>设置</span>
        </button>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div className="profile-picker">
            <button
              aria-expanded={profileOpen}
              className="profile-trigger"
              onClick={() => setProfileOpen((current) => !current)}
              type="button"
            >
              <span className="avatar" aria-hidden="true">
                LZ
              </span>
              <span>
                <strong>李志林</strong>
                <small>/ {profileOverview?.name ?? "主资料库"}</small>
              </span>
              <IconChevronDown aria-hidden="true" size={17} stroke={1.6} />
            </button>
            {profileOpen && (
              <div className="profile-menu">
                <button
                  onClick={() => {
                    setProfileOpen(false);
                    showNotice("已选择主资料库");
                  }}
                  type="button"
                >
                  <IconCheck aria-hidden="true" size={16} stroke={1.7} />
                  主资料库
                </button>
                <button
                  onClick={() => {
                    setProfileOpen(false);
                    showNotice("英文资料库将在 M2 支持");
                  }}
                  type="button"
                >
                  <span className="menu-spacer" />
                  英文资料库
                </button>
              </div>
            )}
          </div>

          <div className="analysis-state" aria-live="polite">
            <span className="analysis-icon">
              {isAnalysing ||
              (activeNav === "JD 工作区" && isAnalyzingJD) ||
              (activeNav === "资料库" && profileStatus === "loading") ? (
                <IconRefresh
                  aria-hidden="true"
                  className="is-spinning"
                  size={21}
                  stroke={1.7}
                />
              ) : (
                <IconCheck aria-hidden="true" size={19} stroke={1.8} />
              )}
            </span>
            <span>
              {activeNav === "资料库" ? (
                <>
                  <strong>
                    {profileStatus === "loading" ? "读取资料" : "资料已同步"}
                  </strong>
                  <small>
                    {profileStatus === "loading"
                      ? "正在读取本地数据库"
                      : `${profileOverview?.documents.length ?? 0} 个文档 · ${
                          profileOverview?.evidence.length ?? 0
                        } 条证据`}
                  </small>
                </>
              ) : activeNav === "JD 工作区" ? (
                <>
                  <strong>
                    {isAnalyzingJD
                      ? "分析中"
                      : jdDirty
                        ? "结果已失效"
                        : jdWorkspace?.analysisStatus === "succeeded"
                          ? "分析完成"
                          : "等待分析"}
                  </strong>
                  <small>
                    {isAnalyzingJD
                      ? "正在校验结构化结果"
                      : jdDirty
                        ? "原始 JD 已修改"
                        : jdWorkspace?.analysis
                          ? `${jdWorkspace.analysis.requiredSkills.length} 项必要技能`
                          : "粘贴岗位描述开始"}
                  </small>
                </>
              ) : (
                <>
                  <strong>{isAnalysing ? "分析中" : "分析完成"}</strong>
                  <small>
                    {isAnalysing ? "正在重建证据关联" : "19 个要求已分析"}
                  </small>
                </>
              )}
            </span>
          </div>

          <div className="topbar-actions">
            {activeNav === "资料库" ? (
              <>
                <button
                  className="button button--secondary"
                  disabled={profileStatus === "loading" || isImportingProfile}
                  onClick={() => void refreshProfile()}
                  type="button"
                >
                  <IconRefresh aria-hidden="true" size={18} stroke={1.65} />
                  刷新资料
                </button>
                <button
                  className="button button--primary"
                  disabled={isImportingProfile}
                  onClick={() => void handleProfileImport()}
                  type="button"
                >
                  <IconUpload aria-hidden="true" size={18} stroke={1.65} />
                  {isImportingProfile ? "正在导入" : "导入 Markdown"}
                </button>
              </>
            ) : activeNav === "JD 工作区" ? (
              <>
                <button
                  className="button button--secondary"
                  disabled={
                    !jdDirty ||
                    isSavingJD ||
                    isAnalyzingJD ||
                    jdText.trim() === ""
                  }
                  onClick={() => void handleJDSave()}
                  type="button"
                >
                  {isSavingJD ? "正在保存" : "保存原文"}
                </button>
                <button
                  className="button button--primary"
                  disabled={
                    isSavingJD || isAnalyzingJD || jdText.trim() === ""
                  }
                  onClick={() => void handleJDAnalyze()}
                  type="button"
                >
                  <IconRefresh
                    aria-hidden="true"
                    className={isAnalyzingJD ? "is-spinning" : ""}
                    size={18}
                    stroke={1.65}
                  />
                  {isAnalyzingJD ? "正在分析" : "分析 JD"}
                </button>
              </>
            ) : (
              <>
                <button
                  className="button button--secondary"
                  disabled={isAnalysing}
                  onClick={handleAnalyse}
                  type="button"
                >
                  <IconRefresh aria-hidden="true" size={18} stroke={1.65} />
                  重新分析
                </button>
                <button
                  className="button button--primary"
                  onClick={() => setGenerateOpen(true)}
                  type="button"
                >
                  <IconFileDescription
                    aria-hidden="true"
                    size={19}
                    stroke={1.65}
                  />
                  生成简历
                </button>
              </>
            )}
          </div>
        </header>

        {activeNav === "资料库" ? (
          <ProfileLibrary
            error={profileError}
            feedback={profileFeedback}
            isImporting={isImportingProfile}
            onImport={() => void handleProfileImport()}
            onRefresh={() => void refreshProfile()}
            onSelectEvidence={handleSelectProfileEvidence}
            onSelectSource={handleSelectProfileSource}
            overview={profileOverview}
            selectedEvidenceId={selectedProfileEvidenceId}
            selectedSourceId={selectedProfileSourceId}
            status={profileStatus}
          />
        ) : activeNav === "JD 工作区" ? (
          <JDWorkspace
            error={jdError}
            feedback={jdFeedback}
            isAnalyzing={isAnalyzingJD}
            isDirty={jdDirty}
            isSaving={isSavingJD}
            onAnalyze={() => void handleJDAnalyze()}
            onChange={handleJDTextChange}
            onRetry={() => void refreshJD()}
            onSave={() => void handleJDSave()}
            rawText={jdText}
            status={jdStatus}
            workspace={jdWorkspace}
          />
        ) : (
          <div className="review-layout">
          <main className="match-review">
            <section className="review-heading">
              <div>
                <h1>Senior Backend Engineer</h1>
                <p>匹配审阅 · 审查岗位要求与个人证据的匹配情况</p>
              </div>
              <div className="score">
                <span>综合匹配</span>
                <strong>82</strong>
                <small>/ 100</small>
              </div>
            </section>

            <section className="review-controls">
              <div className="filter-tabs" role="tablist" aria-label="匹配筛选">
                {[
                  { id: "all" as const, label: "全部", count: allRequirements.length },
                  { id: "strong" as const, label: "强匹配", count: statusCounts.strong },
                  { id: "partial" as const, label: "部分匹配", count: statusCounts.partial },
                  { id: "missing" as const, label: "缺失", count: statusCounts.missing },
                ].map((tab) => (
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
                  <IconChevronDown aria-hidden="true" size={16} stroke={1.5} />
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
                          <IconCheck aria-hidden="true" size={15} stroke={1.8} />
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
                {visibleGroups.map((group) => {
                  const expanded = expandedGroups.has(group.id);

                  return (
                    <section className="requirement-group" key={group.id}>
                      <button
                        aria-expanded={expanded}
                        className="group-row"
                        onClick={() => toggleGroup(group.id)}
                        type="button"
                      >
                        <span className="group-label">
                          {expanded ? (
                            <IconChevronDown
                              aria-hidden="true"
                              size={18}
                              stroke={1.65}
                            />
                          ) : (
                            <IconChevronRight
                              aria-hidden="true"
                              size={18}
                              stroke={1.65}
                            />
                          )}
                          <strong>{group.label}</strong>
                          <small>({group.requirements.length})</small>
                        </span>
                        {!expanded && (
                          <>
                            <span
                              className={`status-badge status-badge--${group.summaryStatus}`}
                            >
                              <IconPointFilled aria-hidden="true" size={15} />
                              {statusMeta[group.summaryStatus].label}
                            </span>
                            <span className="group-evidence">
                              {group.summaryEvidenceCount}
                            </span>
                          </>
                        )}
                      </button>
                      {expanded &&
                        group.requirements.map((requirement) => {
                          const selected = requirement.id === selectedRequirement.id;
                          return (
                            <button
                              className={`requirement-row ${selected ? "is-selected" : ""}`}
                              key={requirement.id}
                              onClick={() => selectRequirement(requirement)}
                              type="button"
                            >
                              <span className="requirement-text">
                                <small>
                                  {allRequirements.indexOf(requirement) + 1}
                                </small>
                                <span>{requirement.text}</span>
                              </span>
                              <span
                                className={`status-badge status-badge--${requirement.status}`}
                              >
                                <IconPointFilled aria-hidden="true" size={15} />
                                {statusMeta[requirement.status].label}
                              </span>
                              <span className="evidence-count">
                                {requirement.evidenceCount}
                              </span>
                              <IconChevronRight
                                aria-hidden="true"
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
                {visibleGroups.length === 0 && (
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
              <IconInfoCircle aria-hidden="true" size={17} stroke={1.6} />
              点击要求行查看右侧的来源证据与匹配说明
            </p>
          </main>

          <aside
            className={`evidence-panel ${inspectorOpen ? "" : "is-closed"}`}
            key={selectedRequirement.id}
          >
            <header className="evidence-header">
              <h2>来源证据</h2>
              <button
                aria-label="关闭来源证据"
                className="icon-button evidence-close"
                onClick={() => setInspectorOpen(false)}
                type="button"
              >
                <IconX aria-hidden="true" size={19} stroke={1.6} />
              </button>
            </header>

            <section className="evidence-summary">
              <span>对应要求</span>
              <h3>{selectedRequirement.text}</h3>
              <dl>
                <div>
                  <dt>匹配状态</dt>
                  <dd
                    className={`status-badge status-badge--${selectedRequirement.status}`}
                  >
                    <IconPointFilled aria-hidden="true" size={15} />
                    {statusMeta[selectedRequirement.status].label}
                  </dd>
                </div>
                <div>
                  <dt>证据数量</dt>
                  <dd>{selectedRequirement.evidenceCount}</dd>
                </div>
              </dl>
            </section>

            <section className="sources">
              <h3>证据来源（{selectedRequirement.sources.length}）</h3>
              {selectedRequirement.sources.length === 0 ? (
                <div className="empty-source">
                  <IconDatabase aria-hidden="true" size={24} stroke={1.5} />
                  <strong>当前资料中没有直接证据</strong>
                  <p>后续追问阶段会确认用户是否具备该项能力。</p>
                </div>
              ) : (
                selectedRequirement.sources.map((source) => {
                  const expanded = expandedSources.has(source.id);
                  return (
                    <article className="source-item" key={source.id}>
                      <button
                        aria-expanded={expanded}
                        className="source-trigger"
                        onClick={() => toggleSource(source.id)}
                        type="button"
                      >
                        {expanded ? (
                          <IconChevronUp
                            aria-hidden="true"
                            size={17}
                            stroke={1.5}
                          />
                        ) : (
                          <IconChevronRight
                            aria-hidden="true"
                            size={17}
                            stroke={1.5}
                          />
                        )}
                        <span>{source.document}</span>
                      </button>
                      {expanded && (
                        <div className="source-snippets">
                          {source.snippets.map((snippet, index) => (
                            <button
                              className={index === 0 ? "is-current" : ""}
                              key={`${source.id}-${snippet.location}`}
                              onClick={() =>
                                showNotice(
                                  `已定位到 ${source.document} ${snippet.location}`,
                                )
                              }
                              type="button"
                            >
                              <IconPointFilled
                                aria-hidden="true"
                                size={12}
                              />
                              <span>{snippet.location}</span>
                              <strong>{snippet.label}</strong>
                            </button>
                          ))}
                        </div>
                      )}
                    </article>
                  );
                })
              )}
            </section>

            {selectedRequirement.sources[0] && (
              <>
                <section className="source-content">
                  <header>
                    <h3>来源内容</h3>
                    <span>
                      来自 {selectedRequirement.sources[0].document}
                    </span>
                  </header>
                  <div className="source-code">
                    {selectedRequirement.sources[0].excerpt.map((line, index) => (
                      <div className="source-line" key={`${line}-${index}`}>
                        <span>{42 + index}</span>
                        <code>{line || " "}</code>
                      </div>
                    ))}
                    <button
                      className="copy-button"
                      onClick={handleCopy}
                      type="button"
                    >
                      {copied ? (
                        <IconCheck aria-hidden="true" size={15} stroke={1.7} />
                      ) : (
                        <IconCopy aria-hidden="true" size={15} stroke={1.7} />
                      )}
                      {copied ? "已复制" : "复制"}
                    </button>
                  </div>
                </section>
                <section className="match-explanation">
                  <h3>匹配说明</h3>
                  <p>{selectedRequirement.sources[0].explanation}</p>
                </section>
              </>
            )}
          </aside>
          </div>
        )}
      </section>

      {notice && (
        <div className="toast" role="status">
          <IconCheck aria-hidden="true" size={17} stroke={1.8} />
          {notice}
        </div>
      )}

      {generateOpen && (
        <div
          aria-labelledby="generate-title"
          aria-modal="true"
          className="modal-backdrop"
          role="dialog"
        >
          <section className="generate-dialog">
            <button
              aria-label="关闭"
              className="icon-button modal-close"
              onClick={() => setGenerateOpen(false)}
              type="button"
            >
              <IconX aria-hidden="true" size={20} stroke={1.6} />
            </button>
            <span className="dialog-kicker">下一阶段</span>
            <h2 id="generate-title">基于当前匹配结果生成简历</h2>
            <p>
              将使用平衡包装档，并保留所有来源引用。缺失要求不会被补写成事实。
            </p>
            <dl className="dialog-summary">
              <div>
                <dt>目标岗位</dt>
                <dd>Senior Backend Engineer</dd>
              </div>
              <div>
                <dt>综合匹配</dt>
                <dd>82 / 100</dd>
              </div>
              <div>
                <dt>包装档位</dt>
                <dd>平衡</dd>
              </div>
            </dl>
            <div className="dialog-actions">
              <button
                className="button button--secondary"
                onClick={() => setGenerateOpen(false)}
                type="button"
              >
                继续审阅
              </button>
              <button
                className="button button--primary"
                onClick={() => {
                  setGenerateOpen(false);
                  showNotice("生成请求已进入 Resume Studio");
                }}
                type="button"
              >
                <IconFileDescription
                  aria-hidden="true"
                  size={18}
                  stroke={1.65}
                />
                确认生成
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}

export default App;
