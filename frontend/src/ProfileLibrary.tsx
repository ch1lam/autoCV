import {
  IconAlertCircle,
  IconArchive,
  IconChevronRight,
  IconCircleCheck,
  IconEdit,
  IconFileText,
  IconInfoCircle,
  IconLink,
  IconRefresh,
  IconSearch,
  IconUpload,
  IconX,
} from "@tabler/icons-react";
import { useEffect, useState } from "react";

import type {
  EvidenceSourceSummary,
  EvidenceSummary,
  ProfileOverview,
  ProfileSearchResult,
  SaveEvidenceInput,
} from "../bindings/github.com/ch1lam/autocv/internal/app";

export type ProfileStatus = "loading" | "ready" | "error";
export type ProfileFeedback = {
  tone: "success" | "warning" | "info" | "error";
  text: string;
};
export type ProfileSearchStatus = "idle" | "loading" | "ready" | "error";

type ProfileLibraryProps = {
  error: string;
  feedback: ProfileFeedback | null;
  isImporting: boolean;
  isSavingEvidence: boolean;
  onImport: () => void;
  onRefresh: () => void;
  onSearch: () => void;
  onSearchChange: (value: string) => void;
  onSearchClear: () => void;
  onSaveEvidence: (input: SaveEvidenceInput) => Promise<boolean>;
  onSelectSearchResult: (result: ProfileSearchResult) => void;
  onSelectEvidence: (evidence: EvidenceSummary) => void;
  onSelectSource: (source: EvidenceSourceSummary) => void;
  overview: ProfileOverview | null;
  searchError: string;
  searchQuery: string;
  searchResults: ProfileSearchResult[];
  searchedQuery: string;
  searchStatus: ProfileSearchStatus;
  selectedEvidenceId: string;
  selectedSourceId: string;
  status: ProfileStatus;
};

type SourceLocator = {
  heading_path?: string[];
  start?: number;
  end?: number;
};

const evidenceKindLabels: Record<string, string> = {
  achievement: "成果",
  certification: "认证",
  education: "教育",
  experience: "经历",
  project: "项目",
  skill: "技能",
};

function parseLocator(locatorJSON: string): SourceLocator {
  try {
    return JSON.parse(locatorJSON) as SourceLocator;
  } catch {
    return {};
  }
}

function formatImportedAt(value: string) {
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

function ProfileLibrary({
  error,
  feedback,
  isImporting,
  isSavingEvidence,
  onImport,
  onRefresh,
  onSearch,
  onSearchChange,
  onSearchClear,
  onSaveEvidence,
  onSelectSearchResult,
  onSelectEvidence,
  onSelectSource,
  overview,
  searchError,
  searchQuery,
  searchResults,
  searchedQuery,
  searchStatus,
  selectedEvidenceId,
  selectedSourceId,
  status,
}: ProfileLibraryProps) {
  const [editingEvidenceID, setEditingEvidenceID] = useState("");
  const [draftEvidenceTitle, setDraftEvidenceTitle] = useState("");
  const [draftEvidenceContent, setDraftEvidenceContent] = useState("");
  const selectedEvidence =
    overview?.evidence.find((item) => item.id === selectedEvidenceId) ??
    overview?.evidence[0];
  const selectedSource =
    selectedEvidence?.sources.find((item) => item.chunkId === selectedSourceId) ??
    selectedEvidence?.sources[0];
  const selectedDocument = overview?.documents.find(
    (document) => document.id === selectedSource?.documentId,
  );
  const locator = selectedSource
    ? parseLocator(selectedSource.locatorJson)
    : {};
  const isEditingEvidence = editingEvidenceID === selectedEvidence?.id;

  useEffect(() => {
    setEditingEvidenceID("");
    setDraftEvidenceTitle(selectedEvidence?.title ?? "");
    setDraftEvidenceContent(selectedEvidence?.content ?? "");
  }, [selectedEvidence?.id]);

  const beginEvidenceEdit = () => {
    if (!selectedEvidence) {
      return;
    }
    setDraftEvidenceTitle(selectedEvidence.title);
    setDraftEvidenceContent(selectedEvidence.content);
    setEditingEvidenceID(selectedEvidence.id);
  };

  const cancelEvidenceEdit = () => {
    setEditingEvidenceID("");
    setDraftEvidenceTitle(selectedEvidence?.title ?? "");
    setDraftEvidenceContent(selectedEvidence?.content ?? "");
  };

  const saveEvidence = async () => {
    if (!selectedEvidence) {
      return;
    }
    const saved = await onSaveEvidence({
      evidenceId: selectedEvidence.id,
      title: draftEvidenceTitle,
      content: draftEvidenceContent,
      userVerified: true,
    });
    if (saved) {
      setEditingEvidenceID("");
    }
  };

  const confirmEvidence = async () => {
    if (!selectedEvidence) {
      return;
    }
    await onSaveEvidence({
      evidenceId: selectedEvidence.id,
      title: selectedEvidence.title,
      content: selectedEvidence.content,
      userVerified: true,
    });
  };

  return (
    <div className="profile-workspace-layout">
      <main className="profile-library">
        <section className="profile-heading">
          <div>
            <span className="section-kicker">PROFILE LIBRARY</span>
            <h1>{overview?.name ?? "主资料库"}</h1>
            <p>导入 Markdown 职业资料，检查提取结果并追溯每条证据的来源。</p>
          </div>
          <button
            className="button button--primary profile-heading-action"
            disabled={isImporting}
            onClick={onImport}
            type="button"
          >
            {isImporting ? (
              <IconRefresh
                aria-hidden="true"
                className="is-spinning"
                size={18}
                stroke={1.65}
              />
            ) : (
              <IconUpload aria-hidden="true" size={18} stroke={1.65} />
            )}
            {isImporting ? "正在导入" : "导入 Markdown"}
          </button>
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

        {status === "loading" && !overview && (
          <div className="profile-state">
            <IconRefresh
              aria-hidden="true"
              className="is-spinning"
              size={22}
              stroke={1.6}
            />
            正在读取本地资料库
          </div>
        )}

        {status === "error" && !overview && (
          <div className="profile-state profile-state--error">
            <IconAlertCircle aria-hidden="true" size={23} stroke={1.6} />
            <strong>无法读取本地资料库</strong>
            <p>{error || "请确认 AutoCV 本地服务已经启动。"}</p>
            <button
              className="button button--secondary"
              onClick={onRefresh}
              type="button"
            >
              <IconRefresh aria-hidden="true" size={17} stroke={1.65} />
              重新读取
            </button>
          </div>
        )}

        {overview && (
          <>
            <dl className="profile-metrics">
              <div>
                <dt>来源文档</dt>
                <dd>{overview.documents.length}</dd>
              </div>
              <div>
                <dt>可追溯证据</dt>
                <dd>{overview.evidence.length}</dd>
              </div>
              <div>
                <dt>默认语言</dt>
                <dd>{overview.defaultLanguage}</dd>
              </div>
            </dl>

            <section aria-label="资料检索" className="profile-search">
              <header>
                <div>
                  <h2>搜索资料</h2>
                  <p>同时检索原始 Markdown 片段和已提取的 Evidence。</p>
                </div>
                {searchedQuery && searchStatus !== "loading" && (
                  <span>{searchResults.length} 条结果</span>
                )}
              </header>
              <form
                className="profile-search-form"
                onSubmit={(event) => {
                  event.preventDefault();
                  onSearch();
                }}
              >
                <div className="profile-search-input">
                  <IconSearch aria-hidden="true" size={18} stroke={1.55} />
                  <input
                    aria-label="搜索资料库"
                    maxLength={200}
                    onChange={(event) => onSearchChange(event.target.value)}
                    placeholder="搜索技能、项目或原文片段"
                    type="search"
                    value={searchQuery}
                  />
                  {searchQuery && (
                    <button
                      aria-label="清除搜索"
                      onClick={onSearchClear}
                      type="button"
                    >
                      <IconX aria-hidden="true" size={16} stroke={1.6} />
                    </button>
                  )}
                </div>
                <button
                  className="button button--secondary"
                  disabled={
                    searchStatus === "loading" || searchQuery.trim() === ""
                  }
                  type="submit"
                >
                  {searchStatus === "loading" ? (
                    <IconRefresh
                      aria-hidden="true"
                      className="is-spinning"
                      size={17}
                      stroke={1.6}
                    />
                  ) : (
                    <IconSearch aria-hidden="true" size={17} stroke={1.6} />
                  )}
                  {searchStatus === "loading" ? "检索中" : "搜索"}
                </button>
              </form>

              {searchStatus === "error" && (
                <div className="profile-search-message profile-search-message--error">
                  <IconAlertCircle aria-hidden="true" size={17} stroke={1.6} />
                  <span>{searchError || "搜索失败，请重试。"}</span>
                </div>
              )}

              {searchStatus === "ready" &&
                searchedQuery &&
                searchResults.length === 0 && (
                  <div className="profile-search-message">
                    没有找到“{searchedQuery}”，可以尝试更短的关键词。
                  </div>
                )}

              {searchResults.length > 0 && (
                <div className="profile-search-results">
                  {searchResults.map((result, index) => (
                    <button
                      className="profile-search-result"
                      key={`${result.entityType}-${result.entityId}-${result.sourceChunkId}-${index}`}
                      onClick={() => onSelectSearchResult(result)}
                      type="button"
                    >
                      <span className="profile-search-result-type">
                        {result.entityType === "evidence"
                          ? "Evidence"
                          : "原文片段"}
                      </span>
                      <span className="profile-search-result-copy">
                        <strong>{result.title}</strong>
                        <small>{result.snippet}</small>
                      </span>
                      <span className="profile-search-result-source">
                        <IconFileText
                          aria-hidden="true"
                          size={14}
                          stroke={1.55}
                        />
                        {result.documentName}
                      </span>
                      <IconChevronRight
                        aria-hidden="true"
                        size={17}
                        stroke={1.55}
                      />
                    </button>
                  ))}
                </div>
              )}
            </section>

            <section className="library-section">
              <header className="library-section-heading">
                <div>
                  <h2>来源文档</h2>
                  <p>原始文件会复制到 AutoCV 管理目录，后续解析只读取受管理副本。</p>
                </div>
                <span>{overview.documents.length} 个文档</span>
              </header>

              {overview.documents.length === 0 ? (
                <div className="library-empty">
                  <IconArchive aria-hidden="true" size={28} stroke={1.45} />
                  <strong>还没有职业资料</strong>
                  <p>从一份 Markdown 简历开始建立可追溯的 Profile。</p>
                  <button
                    className="button button--secondary"
                    disabled={isImporting}
                    onClick={onImport}
                    type="button"
                  >
                    <IconUpload aria-hidden="true" size={17} stroke={1.65} />
                    选择 Markdown
                  </button>
                </div>
              ) : (
                <div className="document-list">
                  <div className="document-list-head" aria-hidden="true">
                    <span>文档</span>
                    <span>类型</span>
                    <span>解析状态</span>
                    <span>导入时间</span>
                  </div>
                  {overview.documents.map((document) => (
                    <div className="document-row" key={document.id}>
                      <span className="document-name">
                        <IconFileText
                          aria-hidden="true"
                          size={19}
                          stroke={1.55}
                        />
                        <strong>{document.originalName}</strong>
                      </span>
                      <span>Markdown</span>
                      <span className="parse-status">
                        <IconCircleCheck
                          aria-hidden="true"
                          size={15}
                          stroke={1.8}
                        />
                        {document.parseStatus === "succeeded"
                          ? "已完成"
                          : document.parseStatus}
                      </span>
                      <time dateTime={document.importedAt}>
                        {formatImportedAt(document.importedAt)}
                      </time>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className="library-section evidence-library">
              <header className="library-section-heading">
                <div>
                  <h2>提取证据</h2>
                  <p>选择一条 Evidence，在右侧核对原始内容与 Markdown 定位信息。</p>
                </div>
                <span>{overview.evidence.length} 条证据</span>
              </header>

              {overview.evidence.length === 0 ? (
                <div className="library-empty library-empty--compact">
                  <IconLink aria-hidden="true" size={27} stroke={1.45} />
                  <strong>没有可显示的 Evidence</strong>
                  <p>导入包含工作经历、项目或技能内容的 Markdown 文件。</p>
                </div>
              ) : (
                <div className="profile-evidence-list">
                  <div className="profile-evidence-head" aria-hidden="true">
                    <span>Evidence</span>
                    <span>类型</span>
                    <span>置信度</span>
                    <span>来源</span>
                    <span />
                  </div>
                  {overview.evidence.map((evidence) => (
                    <button
                      className={`profile-evidence-row ${
                        evidence.id === selectedEvidence?.id
                          ? "is-selected"
                          : ""
                      }`}
                      key={evidence.id}
                      onClick={() => onSelectEvidence(evidence)}
                      type="button"
                    >
                      <span className="profile-evidence-title">
                        <span className="profile-evidence-title-line">
                          <strong>{evidence.title}</strong>
                          {evidence.userVerified && (
                            <span>
                              <IconCircleCheck
                                aria-hidden="true"
                                size={12}
                                stroke={1.9}
                              />
                              已确认
                            </span>
                          )}
                        </span>
                        <small>{evidence.content}</small>
                      </span>
                      <span className="evidence-kind">
                        {evidenceKindLabels[evidence.kind] ?? evidence.kind}
                      </span>
                      <span>{Math.round(evidence.confidence * 100)}%</span>
                      <span>{evidence.sources.length}</span>
                      <IconChevronRight
                        aria-hidden="true"
                        size={18}
                        stroke={1.55}
                      />
                    </button>
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </main>

      <aside
        aria-label="Evidence 来源检查器"
        className="evidence-panel profile-source-panel"
      >
        <header className="evidence-header">
          <h2>{isEditingEvidence ? "编辑 Evidence" : "Evidence 来源"}</h2>
          {selectedEvidence && (
            <button
              className="profile-inspector-action"
              disabled={isSavingEvidence}
              onClick={
                isEditingEvidence ? cancelEvidenceEdit : beginEvidenceEdit
              }
              type="button"
            >
              {isEditingEvidence ? (
                <IconX aria-hidden="true" size={16} stroke={1.6} />
              ) : (
                <IconEdit aria-hidden="true" size={16} stroke={1.6} />
              )}
              {isEditingEvidence ? "取消" : "编辑"}
            </button>
          )}
        </header>

        {!selectedEvidence ? (
          <div className="profile-inspector-empty">
            <IconLink aria-hidden="true" size={27} stroke={1.45} />
            <strong>选择一条 Evidence</strong>
            <p>来源文本和 Markdown 定位信息会显示在这里。</p>
          </div>
        ) : (
          <>
            <section className="profile-evidence-summary">
              {isEditingEvidence ? (
                <form
                  className="profile-evidence-editor"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void saveEvidence();
                  }}
                >
                  <label>
                    <span>Evidence 标题</span>
                    <input
                      aria-label="Evidence 标题"
                      maxLength={240}
                      onChange={(event) =>
                        setDraftEvidenceTitle(event.target.value)
                      }
                      type="text"
                      value={draftEvidenceTitle}
                    />
                  </label>
                  <label>
                    <span>事实内容</span>
                    <textarea
                      aria-label="Evidence 内容"
                      maxLength={8000}
                      onChange={(event) =>
                        setDraftEvidenceContent(event.target.value)
                      }
                      rows={7}
                      value={draftEvidenceContent}
                    />
                  </label>
                  <p>
                    保存代表你确认这条信息准确。来源引用会继续保留，后续提取不能静默覆盖。
                  </p>
                  <div className="profile-evidence-editor-actions">
                    <button
                      className="button button--secondary"
                      disabled={isSavingEvidence}
                      onClick={cancelEvidenceEdit}
                      type="button"
                    >
                      取消
                    </button>
                    <button
                      className="button button--primary"
                      disabled={
                        isSavingEvidence ||
                        draftEvidenceTitle.trim() === "" ||
                        draftEvidenceContent.trim() === ""
                      }
                      type="submit"
                    >
                      {isSavingEvidence ? (
                        <IconRefresh
                          aria-hidden="true"
                          className="is-spinning"
                          size={16}
                          stroke={1.6}
                        />
                      ) : (
                        <IconCircleCheck
                          aria-hidden="true"
                          size={16}
                          stroke={1.7}
                        />
                      )}
                      {isSavingEvidence ? "保存中" : "保存并确认"}
                    </button>
                  </div>
                </form>
              ) : (
                <>
                  <div className="profile-evidence-meta">
                    <span>
                      {evidenceKindLabels[selectedEvidence.kind] ??
                        selectedEvidence.kind}
                    </span>
                    <span
                      className={
                        selectedEvidence.userVerified
                          ? "is-verified"
                          : "is-unverified"
                      }
                    >
                      {selectedEvidence.userVerified ? (
                        <IconCircleCheck
                          aria-hidden="true"
                          size={13}
                          stroke={1.9}
                        />
                      ) : (
                        <IconInfoCircle
                          aria-hidden="true"
                          size={13}
                          stroke={1.8}
                        />
                      )}
                      {selectedEvidence.userVerified
                        ? "用户已确认"
                        : "AI 提取，待确认"}
                    </span>
                  </div>
                  <h3>{selectedEvidence.title}</h3>
                  <p>{selectedEvidence.content}</p>
                  <dl>
                    <div>
                      <dt>置信度</dt>
                      <dd>
                        {Math.round(selectedEvidence.confidence * 100)}%
                      </dd>
                    </div>
                    <div>
                      <dt>来源数量</dt>
                      <dd>{selectedEvidence.sources.length}</dd>
                    </div>
                    {selectedEvidence.updatedAt && (
                      <div>
                        <dt>最后更新</dt>
                        <dd>{formatImportedAt(selectedEvidence.updatedAt)}</dd>
                      </div>
                    )}
                  </dl>
                  {!selectedEvidence.userVerified && (
                    <div className="profile-evidence-confirmation">
                      <span>
                        确认后，这条 Evidence 可作为用户认可的事实继续参与生成。
                      </span>
                      <button
                        className="button button--secondary"
                        disabled={isSavingEvidence}
                        onClick={() => void confirmEvidence()}
                        type="button"
                      >
                        {isSavingEvidence ? (
                          <IconRefresh
                            aria-hidden="true"
                            className="is-spinning"
                            size={16}
                            stroke={1.6}
                          />
                        ) : (
                          <IconCircleCheck
                            aria-hidden="true"
                            size={16}
                            stroke={1.8}
                          />
                        )}
                        {isSavingEvidence ? "确认中" : "确认此 Evidence"}
                      </button>
                    </div>
                  )}
                </>
              )}
            </section>

            <section className="sources profile-sources">
              <h3>来源定位（{selectedEvidence.sources.length}）</h3>
              {selectedEvidence.sources.map((source, index) => {
                const sourceLocator = parseLocator(source.locatorJson);
                const document = overview?.documents.find(
                  (item) => item.id === source.documentId,
                );
                const location =
                  sourceLocator.heading_path?.join(" / ") ||
                  `内容片段 ${index + 1}`;

                return (
                  <button
                    className={`profile-source-row ${
                      source.chunkId === selectedSource?.chunkId
                        ? "is-current"
                        : ""
                    }`}
                    key={source.chunkId}
                    onClick={() => onSelectSource(source)}
                    type="button"
                  >
                    <IconFileText
                      aria-hidden="true"
                      size={16}
                      stroke={1.55}
                    />
                    <span>
                      <strong>{location}</strong>
                      <small>{document?.originalName ?? "Markdown 来源"}</small>
                    </span>
                    <IconChevronRight
                      aria-hidden="true"
                      size={16}
                      stroke={1.5}
                    />
                  </button>
                );
              })}
            </section>

            {selectedSource && (
              <>
                <section className="source-content profile-source-content">
                  <header>
                    <h3>来源内容</h3>
                    <span>{selectedDocument?.originalName}</span>
                  </header>
                  <div className="profile-source-quote">
                    <code>{selectedSource.chunkText}</code>
                  </div>
                </section>

                <section className="source-locator">
                  <h3>Markdown 定位</h3>
                  <dl>
                    <div>
                      <dt>标题路径</dt>
                      <dd>
                        {locator.heading_path?.join(" / ") || "文档正文"}
                      </dd>
                    </div>
                    <div>
                      <dt>字节范围</dt>
                      <dd>
                        {locator.start ?? 0}–{locator.end ?? 0}
                      </dd>
                    </div>
                    <div>
                      <dt>引用范围</dt>
                      <dd>
                        {selectedSource.quoteStart}–
                        {selectedSource.quoteEnd}
                      </dd>
                    </div>
                  </dl>
                </section>
              </>
            )}
          </>
        )}
      </aside>
    </div>
  );
}

export default ProfileLibrary;
