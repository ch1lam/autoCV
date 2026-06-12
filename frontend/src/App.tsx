import {
    IconArchive,
    IconCheck,
    IconChevronDown,
    IconEdit,
    IconFileDescription,
    IconFileText,
    IconFileTypePdf,
    IconRefresh,
    IconSettings,
    IconTargetArrow,
    IconUpload,
    IconX,
} from "@tabler/icons-react";
import { useCallback, useEffect, useRef, useState } from "react";

import {
    HealthService,
    JDService,
    MatchService,
    PDFService,
    ProfileService,
    ProviderControlService,
    ResumeService,
    SettingsService,
    type EvidenceSourceSummary,
    type EvidenceSummary,
    type JDWorkspace as JDWorkspaceModel,
    type MatchReview,
    type PDFWorkspace,
    type ProfileOverview,
    type ProviderSettings,
    type ResumeBlockSummary,
    type ResumeWorkspace,
} from "../bindings/github.com/ch1lam/autocv/internal/app";
import JDWorkspace, {
    type JDWorkspaceFeedback,
    type JDWorkspaceStatus,
} from "./JDWorkspace";
import MatchReviewWorkspace, {
    type MatchWorkspaceStatus,
} from "./MatchReviewWorkspace";
import ProfileLibrary, {
    type ProfileFeedback,
    type ProfileStatus,
} from "./ProfileLibrary";
import PDFPreview, {
    type PDFPreviewFeedback,
    type PDFPreviewStatus,
} from "./PDFPreview";
import ResumeStudio, {
    type ResumeStudioFeedback,
    type ResumeStudioStatus,
} from "./ResumeStudio";
import SettingsWorkspace, {
    type SettingsFeedback,
    type SettingsWorkspaceStatus,
} from "./SettingsWorkspace";

type HealthState = "checking" | "ready" | "preview";
type ProviderRequestAction = "profile" | "jd" | "match";

const providerRequestDetails: Record<
  ProviderRequestAction,
  { kicker: string; title: string; items: string[] }
> = {
  profile: {
    kicker: "PROFILE EXTRACTION",
    title: "提取 Markdown 中的可追溯 Evidence",
    items: ["按标题切分的 Source Chunk", "Chunk ID 与本地来源定位信息"],
  },
  jd: {
    kicker: "JD ANALYSIS",
    title: "把岗位原文转换为结构化要求",
    items: ["当前 JD 原文", "语言提示与任务指令"],
  },
  match: {
    kicker: "MATCH SUGGESTION",
    title: "关联 Requirement 与已有 Evidence",
    items: ["结构化 Requirement", "相关 Evidence 内容与来源 ID"],
  },
};

function isProviderCancellation(error: unknown) {
  return (
    error instanceof Error &&
    /context canceled|context cancelled|request cancelled/i.test(error.message)
  );
}

const navItems = [
  { label: "资料库", icon: IconArchive },
  { label: "JD 工作区", icon: IconFileText },
  { label: "匹配审阅", icon: IconTargetArrow },
  { label: "简历工作室", icon: IconEdit },
  { label: "PDF 预览", icon: IconFileTypePdf },
];

function App() {
  const [health, setHealth] = useState<HealthState>("checking");
  const [activeNav, setActiveNav] = useState("匹配审阅");
  const [profileOpen, setProfileOpen] = useState(false);
  const [matchReview, setMatchReview] = useState<MatchReview | null>(null);
  const [matchStatus, setMatchStatus] =
    useState<MatchWorkspaceStatus>("loading");
  const [matchError, setMatchError] = useState("");
  const [isAnalyzingMatch, setIsAnalyzingMatch] = useState(false);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [generationLanguage, setGenerationLanguage] = useState("zh");
  const [generationPackaging, setGenerationPackaging] = useState(0.5);
  const [resumeWorkspace, setResumeWorkspace] =
    useState<ResumeWorkspace | null>(null);
  const [resumeStatus, setResumeStatus] =
    useState<ResumeStudioStatus>("loading");
  const [resumeError, setResumeError] = useState("");
  const [resumeMarkdown, setResumeMarkdown] = useState("");
  const [resumeDirty, setResumeDirty] = useState(false);
  const [resumeFeedback, setResumeFeedback] =
    useState<ResumeStudioFeedback | null>(null);
  const [isGeneratingResume, setIsGeneratingResume] = useState(false);
  const [isSavingResume, setIsSavingResume] = useState(false);
  const [isLockingResume, setIsLockingResume] = useState(false);
  const [pdfWorkspace, setPDFWorkspace] = useState<PDFWorkspace | null>(null);
  const [pdfStatus, setPDFStatus] =
    useState<PDFPreviewStatus>("loading");
  const [pdfError, setPDFError] = useState("");
  const [pdfFeedback, setPDFFeedback] =
    useState<PDFPreviewFeedback | null>(null);
  const [isRenderingPDF, setIsRenderingPDF] = useState(false);
  const [isExportingPDF, setIsExportingPDF] = useState(false);
  const [providerSettings, setProviderSettings] =
    useState<ProviderSettings | null>(null);
  const [settingsStatus, setSettingsStatus] =
    useState<SettingsWorkspaceStatus>("loading");
  const [settingsError, setSettingsError] = useState("");
  const [settingsProvider, setSettingsProvider] = useState("fake");
  const [settingsBaseURL, setSettingsBaseURL] = useState("");
  const [settingsModel, setSettingsModel] = useState("fixture-v1");
  const [settingsAPIKey, setSettingsAPIKey] = useState("");
  const [settingsDirty, setSettingsDirty] = useState(false);
  const [settingsFeedback, setSettingsFeedback] =
    useState<SettingsFeedback | null>(null);
  const [isSavingSettings, setIsSavingSettings] = useState(false);
  const [isCancellingProvider, setIsCancellingProvider] = useState(false);
  const [providerRequestAction, setProviderRequestAction] =
    useState<ProviderRequestAction | null>(null);
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

  const refreshMatch = useCallback(async () => {
    setMatchStatus("loading");
    setMatchError("");
    try {
      const review = await MatchService.GetReview();
      setMatchReview(review);
      setMatchStatus("ready");
    } catch (error) {
      setMatchStatus("error");
      setMatchError(
        error instanceof Error ? error.message : "本地匹配服务暂不可用。",
      );
    }
  }, []);

  const applyResumeWorkspace = useCallback((workspace: ResumeWorkspace) => {
    setResumeWorkspace(workspace);
    setResumeMarkdown(workspace.markdown);
    setResumeDirty(false);
    setResumeStatus("ready");
    setResumeError("");
  }, []);

  const refreshResume = useCallback(async () => {
    setResumeStatus("loading");
    setResumeError("");
    try {
      const workspace = await ResumeService.GetWorkspace();
      applyResumeWorkspace(workspace);
    } catch (error) {
      setResumeStatus("error");
      setResumeError(
        error instanceof Error ? error.message : "本地简历服务暂不可用。",
      );
    }
  }, [applyResumeWorkspace]);

  const refreshPDF = useCallback(async () => {
    setPDFStatus("loading");
    setPDFError("");
    try {
      const workspace = await PDFService.GetWorkspace();
      setPDFWorkspace(workspace);
      setPDFStatus("ready");
    } catch (error) {
      setPDFStatus("error");
      setPDFError(
        error instanceof Error ? error.message : "本地 PDF 服务暂不可用。",
      );
    }
  }, []);

  const applyProviderSettings = useCallback((settings: ProviderSettings) => {
    setProviderSettings(settings);
    setSettingsProvider(settings.provider);
    setSettingsBaseURL(settings.baseUrl);
    setSettingsModel(settings.model);
    setSettingsAPIKey("");
    setSettingsDirty(false);
    setSettingsStatus("ready");
    setSettingsError("");
  }, []);

  const refreshSettings = useCallback(async () => {
    setSettingsStatus("loading");
    setSettingsError("");
    try {
      const settings = await SettingsService.GetSettings();
      applyProviderSettings(settings);
    } catch (error) {
      setSettingsStatus("error");
      setSettingsError(
        error instanceof Error ? error.message : "本地设置服务暂不可用。",
      );
    }
  }, [applyProviderSettings]);

  useEffect(() => {
    HealthService.Check()
      .then((status) => setHealth(status.status === "ready" ? "ready" : "preview"))
      .catch(() => setHealth("preview"));
    void refreshProfile();
    void refreshJD();
    void refreshMatch();
    void refreshResume();
    void refreshPDF();
    void refreshSettings();

    return () => {
      window.clearTimeout(noticeTimer.current);
    };
  }, [
    refreshJD,
    refreshMatch,
    refreshPDF,
    refreshProfile,
    refreshResume,
    refreshSettings,
  ]);

  const showNotice = (message: string) => {
    setNotice(message);
    window.clearTimeout(noticeTimer.current);
    noticeTimer.current = window.setTimeout(() => setNotice(""), 2400);
  };

  const handleNav = (label: string) => {
    if (
      label === "资料库" ||
      label === "JD 工作区" ||
      label === "匹配审阅" ||
      label === "简历工作室" ||
      label === "PDF 预览" ||
      label === "设置"
    ) {
      setActiveNav(label);
      return;
    }
    showNotice(`${label}将在后续垂直切片中接入本地业务数据`);
  };

  const handleSettingsProviderChange = (provider: string) => {
    setSettingsProvider(provider);
    setSettingsFeedback(null);
    setSettingsAPIKey("");
    if (provider === "openai") {
      setSettingsBaseURL("https://api.openai.com/v1");
      setSettingsModel("gpt-5.5");
    } else {
      setSettingsBaseURL("");
      setSettingsModel("fixture-v1");
    }
    setSettingsDirty(true);
  };

  const handleSettingsSave = async () => {
    setIsSavingSettings(true);
    setSettingsFeedback(null);
    try {
      const settings = await SettingsService.SaveProvider({
        provider: settingsProvider,
        baseUrl: settingsBaseURL,
        model: settingsModel,
        apiKey: settingsAPIKey,
      });
      applyProviderSettings(settings);
      setSettingsFeedback({
        tone: "success",
        text:
          settings.provider === "openai"
            ? "OpenAI 配置与 Keychain 引用已保存。"
            : "已切换到离线 Fake Provider。",
      });
    } catch (error) {
      setSettingsFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `保存失败：${error.message}`
            : "Provider 配置保存失败。",
      });
    } finally {
      setIsSavingSettings(false);
    }
  };

  const handleProviderCancel = async () => {
    setIsCancellingProvider(true);
    try {
      const result = await ProviderControlService.CancelActive();
      showNotice(result.message);
    } catch (error) {
      showNotice(
        error instanceof Error
          ? `取消失败：${error.message}`
          : "取消请求失败，请重试。",
      );
    } finally {
      setIsCancellingProvider(false);
    }
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
      await refreshMatch();
      await refreshResume();
      await refreshPDF();
      setProfileFeedback({
        tone: result.warnings.length > 0 ? "warning" : "success",
        text:
          result.warnings.length > 0
            ? `已导入 ${result.document.originalName}，生成 ${result.evidenceCount} 条 Evidence；另有 ${result.warnings.length} 条解析提示。`
            : `已导入 ${result.document.originalName}，生成 ${result.evidenceCount} 条可追溯 Evidence。`,
      });
    } catch (error) {
      const cancelled = isProviderCancellation(error);
      setProfileFeedback({
        tone: cancelled ? "info" : "error",
        text: cancelled
          ? "已取消 Evidence 提取，资料库没有写入半成品，可以重新导入。"
          : error instanceof Error
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
      await refreshMatch();
      await refreshResume();
      await refreshPDF();
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
      await refreshMatch();
      await refreshResume();
      await refreshPDF();
      setJDFeedback({
        tone: "success",
        text: "JD 分析完成，结构化结果已通过 Go 侧校验。",
      });
    } catch (error) {
      const cancelled = isProviderCancellation(error);
      setJDFeedback({
        tone: cancelled ? "info" : "error",
        text: cancelled
          ? "已取消 JD 分析，原文仍保留，可以直接重试。"
          : error instanceof Error
            ? `分析失败：${error.message}`
            : "分析失败，请修改 JD 后重试。",
      });
      try {
        const workspace = await JDService.GetWorkspace();
        applyJDWorkspace(workspace);
      } catch {
        setJDStatus("error");
      }
      await refreshMatch();
    } finally {
      setIsAnalyzingJD(false);
    }
  };

  const handleMatchAnalyze = async () => {
    setIsAnalyzingMatch(true);
    setMatchError("");
    try {
      const review = await MatchService.Analyze();
      setMatchReview(review);
      setMatchStatus("ready");
      if (review.status === "ready") {
        showNotice(
          `匹配已更新，${review.requirements.length} 项要求已完成确定性评分`,
        );
      }
      await refreshResume();
      await refreshPDF();
    } catch (error) {
      const cancelled = isProviderCancellation(error);
      setMatchStatus(cancelled ? "ready" : "error");
      setMatchError(
        cancelled
          ? ""
          : error instanceof Error
            ? error.message
            : "匹配分析失败，请重试。",
      );
      if (cancelled) {
        showNotice("已取消匹配分析，已有结果保持不变，可以直接重试");
      }
      try {
        const review = await MatchService.GetReview();
        setMatchReview(review);
        setMatchStatus("ready");
      } catch {
        setMatchStatus("error");
      }
    } finally {
      setIsAnalyzingMatch(false);
    }
  };

  const handleResumeMarkdownChange = (value: string) => {
    setResumeMarkdown(value);
    setResumeDirty(value !== (resumeWorkspace?.markdown ?? ""));
    setResumeFeedback(null);
  };

  const handleResumeGenerate = async () => {
    setIsGeneratingResume(true);
    setResumeFeedback(null);
    try {
      const workspace = await ResumeService.Generate(
        generationLanguage,
        generationPackaging,
      );
      applyResumeWorkspace(workspace);
      await refreshPDF();
      setGenerateOpen(false);
      setActiveNav("简历工作室");
      setResumeFeedback({
        tone: "success",
        text: `第 ${workspace.version} 版已生成，Block 与来源关系已保存。`,
      });
    } catch (error) {
      const cancelled = isProviderCancellation(error);
      setResumeFeedback({
        tone: cancelled ? "info" : "error",
        text: cancelled
          ? "已取消简历生成，上一版本保持不变，可以直接重试。"
          : error instanceof Error
            ? `生成失败：${error.message}`
            : "生成失败，请检查匹配结果后重试。",
      });
    } finally {
      setIsGeneratingResume(false);
    }
  };

  const handleResumeSave = async () => {
    setIsSavingResume(true);
    setResumeFeedback(null);
    try {
      const workspace = await ResumeService.UpdateMarkdown(resumeMarkdown);
      applyResumeWorkspace(workspace);
      await refreshPDF();
      setResumeFeedback({
        tone: "success",
        text: `Markdown 已保存为第 ${workspace.version} 版。`,
      });
    } catch (error) {
      setResumeFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `保存失败：${error.message}`
            : "保存失败，请检查 Block 标记。",
      });
    } finally {
      setIsSavingResume(false);
    }
  };

  const handleResumeLock = async (block: ResumeBlockSummary) => {
    setIsLockingResume(true);
    setResumeFeedback(null);
    try {
      const workspace = await ResumeService.SetBlockLocked(
        block.id,
        !block.locked,
      );
      applyResumeWorkspace(workspace);
      await refreshPDF();
      setResumeFeedback({
        tone: "success",
        text: block.locked
          ? `已解除“${block.label}”锁定。`
          : `已锁定“${block.label}”，重新生成不会改写该内容。`,
      });
    } catch (error) {
      setResumeFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `锁定失败：${error.message}`
            : "锁定状态保存失败。",
      });
    } finally {
      setIsLockingResume(false);
    }
  };

  const handlePDFRender = async () => {
    setIsRenderingPDF(true);
    setPDFFeedback(null);
    try {
      const workspace = await PDFService.Render();
      setPDFWorkspace(workspace);
      setPDFStatus("ready");
      setPDFError("");
      setPDFFeedback({
        tone: "success",
        text: `Resume v${workspace.version} 已生成新的 PDF Artifact。`,
      });
    } catch (error) {
      setPDFFeedback({
        tone: "error",
        text:
          error instanceof Error
            ? `渲染失败：${error.message}`
            : "渲染失败，上一份成功 PDF 已保留。",
      });
    } finally {
      setIsRenderingPDF(false);
    }
  };

  const handlePDFExport = async () => {
    setIsExportingPDF(true);
    setPDFFeedback(null);
    try {
      const result = await PDFService.ExportPDF();
      setPDFFeedback({
        tone: "success",
        text: result.cancelled
          ? "已取消导出，Artifact 没有变化。"
          : `PDF 已导出到 ${result.path}`,
      });
    } catch (error) {
      setPDFFeedback({
        tone: "error",
        text:
          error instanceof Error ? `导出失败：${error.message}` : "导出失败。",
      });
    } finally {
      setIsExportingPDF(false);
    }
  };

  const handleMarkdownExport = async () => {
    setIsExportingPDF(true);
    setPDFFeedback(null);
    try {
      const result = await PDFService.ExportMarkdown();
      setPDFFeedback({
        tone: "success",
        text: result.cancelled
          ? "已取消导出。"
          : `Markdown 已导出到 ${result.path}`,
      });
    } catch (error) {
      setPDFFeedback({
        tone: "error",
        text:
          error instanceof Error ? `导出失败：${error.message}` : "导出失败。",
      });
    } finally {
      setIsExportingPDF(false);
    }
  };

  const executeProviderAction = (action: ProviderRequestAction) => {
    if (action === "profile") {
      void handleProfileImport();
    } else if (action === "jd") {
      void handleJDAnalyze();
    } else {
      void handleMatchAnalyze();
    }
  };

  const requestProviderAction = (action: ProviderRequestAction) => {
    if (providerSettings?.provider === "openai") {
      setProviderRequestAction(action);
      return;
    }
    executeProviderAction(action);
  };

  const confirmProviderAction = () => {
    if (!providerRequestAction) {
      return;
    }
    const action = providerRequestAction;
    setProviderRequestAction(null);
    executeProviderAction(action);
  };

  return (
    <div className="app-shell" data-health={health}>
      <aside className="sidebar">
        <div className="brand">AutoCV</div>
        <nav className="primary-nav" aria-label="主要导航">
          {navItems.map(({ label, icon: Icon }) => (
            <button
              aria-label={label}
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
          aria-label="设置"
          className={`nav-item nav-item--settings ${
            activeNav === "设置" ? "is-active" : ""
          }`}
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
              {(activeNav === "匹配审阅" &&
                (isAnalyzingMatch || matchStatus === "loading")) ||
              (activeNav === "JD 工作区" && isAnalyzingJD) ||
              (activeNav === "资料库" && profileStatus === "loading") ||
              (activeNav === "简历工作室" &&
                (resumeStatus === "loading" ||
                  isGeneratingResume ||
                  isSavingResume ||
                  isLockingResume)) ||
              (activeNav === "PDF 预览" &&
                (pdfStatus === "loading" ||
                  isRenderingPDF ||
                  isExportingPDF)) ||
              (activeNav === "设置" &&
                (settingsStatus === "loading" || isSavingSettings)) ? (
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
              ) : activeNav === "简历工作室" ? (
                <>
                  <strong>
                    {isGeneratingResume
                      ? "生成中"
                      : isSavingResume
                        ? "保存中"
                        : resumeDirty
                          ? "有未保存修改"
                          : resumeWorkspace?.status === "ready"
                            ? `第 ${resumeWorkspace.version} 版`
                            : "等待生成"}
                  </strong>
                  <small>
                    {resumeWorkspace?.status === "ready"
                      ? `${resumeWorkspace.blocks.length} 个 Block · ${resumeWorkspace.packagingLabel}包装`
                      : resumeWorkspace?.message || "先完成匹配审阅"}
                  </small>
                </>
              ) : activeNav === "PDF 预览" ? (
                <>
                  <strong>
                    {isRenderingPDF
                      ? "渲染中"
                      : isExportingPDF
                        ? "导出中"
                        : pdfWorkspace?.status === "ready"
                          ? "PDF 已就绪"
                          : pdfWorkspace?.status === "stale"
                            ? "需要重新渲染"
                            : "等待渲染"}
                  </strong>
                  <small>
                    {pdfWorkspace?.artifactId
                      ? `Artifact ${pdfWorkspace.artifactId.slice(0, 8)} · Resume v${pdfWorkspace.version}`
                      : pdfWorkspace?.message || "先完成简历版本"}
                  </small>
                </>
              ) : activeNav === "设置" ? (
                <>
                  <strong>
                    {isSavingSettings
                      ? "保存中"
                      : settingsDirty
                        ? "有未保存修改"
                        : providerSettings?.provider === "openai"
                          ? "OpenAI 已配置"
                          : "离线模式"}
                  </strong>
                  <small>
                    {settingsProvider === "openai"
                      ? `${settingsModel || "未选择模型"} · ${
                          providerSettings?.apiKeyConfigured
                            ? "Keychain 已就绪"
                            : "需要 API Key"
                        }`
                      : "Fake Provider · 固定 Fixture"}
                  </small>
                </>
              ) : (
                <>
                  <strong>
                    {isAnalyzingMatch
                      ? "匹配中"
                      : matchReview?.status === "ready"
                        ? "评分完成"
                        : matchReview?.status === "stale"
                          ? "结果已失效"
                          : "等待匹配"}
                  </strong>
                  <small>
                    {isAnalyzingMatch
                      ? "正在重建 Evidence 关联"
                      : matchReview?.status === "ready"
                        ? `${matchReview.requirements.length} 项要求 · ${matchReview.totalScore} 分`
                        : matchReview?.message || "准备资料与 JD"}
                  </small>
                </>
              )}
            </span>
          </div>

          <div className="topbar-actions">
            {activeNav === "资料库" ? (
              <>
                {isImportingProfile &&
                providerSettings?.provider === "openai" ? (
                  <button
                    className="button button--secondary button--cancel"
                    disabled={isCancellingProvider}
                    onClick={() => void handleProviderCancel()}
                    type="button"
                  >
                    <IconX aria-hidden="true" size={18} stroke={1.65} />
                    {isCancellingProvider ? "正在取消" : "取消请求"}
                  </button>
                ) : (
                  <button
                    className="button button--secondary"
                    disabled={profileStatus === "loading"}
                    onClick={() => void refreshProfile()}
                    type="button"
                  >
                    <IconRefresh aria-hidden="true" size={18} stroke={1.65} />
                    刷新资料
                  </button>
                )}
                <button
                  className="button button--primary"
                  disabled={isImportingProfile}
                  onClick={() => requestProviderAction("profile")}
                  type="button"
                >
                  <IconUpload aria-hidden="true" size={18} stroke={1.65} />
                  {isImportingProfile ? "正在导入" : "导入 Markdown"}
                </button>
              </>
            ) : activeNav === "JD 工作区" ? (
              <>
                {isAnalyzingJD &&
                providerSettings?.provider === "openai" ? (
                  <button
                    className="button button--secondary button--cancel"
                    disabled={isCancellingProvider}
                    onClick={() => void handleProviderCancel()}
                    type="button"
                  >
                    <IconX aria-hidden="true" size={18} stroke={1.65} />
                    {isCancellingProvider ? "正在取消" : "取消请求"}
                  </button>
                ) : (
                  <button
                    className="button button--secondary"
                    disabled={
                      !jdDirty || isSavingJD || jdText.trim() === ""
                    }
                    onClick={() => void handleJDSave()}
                    type="button"
                  >
                    {isSavingJD ? "正在保存" : "保存原文"}
                  </button>
                )}
                <button
                  className="button button--primary"
                  disabled={
                    isSavingJD || isAnalyzingJD || jdText.trim() === ""
                  }
                  onClick={() => requestProviderAction("jd")}
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
            ) : activeNav === "简历工作室" ? (
              <>
                {isGeneratingResume &&
                providerSettings?.provider === "openai" ? (
                  <button
                    className="button button--secondary button--cancel"
                    disabled={isCancellingProvider}
                    onClick={() => void handleProviderCancel()}
                    type="button"
                  >
                    <IconX aria-hidden="true" size={18} stroke={1.65} />
                    {isCancellingProvider ? "正在取消" : "取消请求"}
                  </button>
                ) : (
                  <button
                    className="button button--secondary"
                    disabled={isSavingResume}
                    onClick={() => setGenerateOpen(true)}
                    type="button"
                  >
                    <IconRefresh
                      aria-hidden="true"
                      size={18}
                      stroke={1.65}
                    />
                    {resumeWorkspace?.status === "ready"
                      ? "重新生成"
                      : "生成简历"}
                  </button>
                )}
                <button
                  className="button button--primary"
                  disabled={!resumeDirty || isSavingResume}
                  onClick={() => void handleResumeSave()}
                  type="button"
                >
                  {isSavingResume ? "正在保存" : "保存新版本"}
                </button>
              </>
            ) : activeNav === "PDF 预览" ? (
              <>
                <button
                  className="button button--secondary"
                  disabled={isRenderingPDF || pdfStatus === "loading"}
                  onClick={() => void handlePDFRender()}
                  type="button"
                >
                  <IconRefresh
                    aria-hidden="true"
                    className={isRenderingPDF ? "is-spinning" : ""}
                    size={18}
                    stroke={1.65}
                  />
                  {pdfWorkspace?.artifactId ? "重新渲染" : "生成 PDF"}
                </button>
                <button
                  className="button button--primary"
                  disabled={!pdfWorkspace?.canExport || isExportingPDF}
                  onClick={() => void handlePDFExport()}
                  type="button"
                >
                  <IconFileTypePdf
                    aria-hidden="true"
                    size={18}
                    stroke={1.65}
                  />
                  {isExportingPDF ? "正在导出" : "导出 PDF"}
                </button>
              </>
            ) : activeNav === "设置" ? (
              <>
                <button
                  className="button button--secondary"
                  disabled={isSavingSettings}
                  onClick={() => void refreshSettings()}
                  type="button"
                >
                  <IconRefresh aria-hidden="true" size={18} stroke={1.65} />
                  重新读取
                </button>
                <button
                  className="button button--primary"
                  disabled={!settingsDirty || isSavingSettings}
                  onClick={() => void handleSettingsSave()}
                  type="button"
                >
                  {isSavingSettings ? "正在保存" : "保存设置"}
                </button>
              </>
            ) : (
              <>
                {isAnalyzingMatch &&
                providerSettings?.provider === "openai" ? (
                  <button
                    className="button button--secondary button--cancel"
                    disabled={isCancellingProvider}
                    onClick={() => void handleProviderCancel()}
                    type="button"
                  >
                    <IconX aria-hidden="true" size={18} stroke={1.65} />
                    {isCancellingProvider ? "正在取消" : "取消请求"}
                  </button>
                ) : (
                  <button
                    className="button button--secondary"
                    disabled={matchStatus === "loading"}
                    onClick={() => requestProviderAction("match")}
                    type="button"
                  >
                    <IconRefresh
                      aria-hidden="true"
                      size={18}
                      stroke={1.65}
                    />
                    重新匹配
                  </button>
                )}
                <button
                  className="button button--primary"
                  disabled={matchReview?.status !== "ready"}
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
            onImport={() => requestProviderAction("profile")}
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
            onAnalyze={() => requestProviderAction("jd")}
            onChange={handleJDTextChange}
            onRetry={() => void refreshJD()}
            onSave={() => void handleJDSave()}
            rawText={jdText}
            status={jdStatus}
            workspace={jdWorkspace}
          />
        ) : activeNav === "简历工作室" ? (
          <ResumeStudio
            error={resumeError}
            feedback={resumeFeedback}
            isDirty={resumeDirty}
            isLocking={isLockingResume}
            isSaving={isSavingResume}
            markdown={resumeMarkdown}
            onChange={handleResumeMarkdownChange}
            onGenerate={() => setGenerateOpen(true)}
            onOpenMatch={() => setActiveNav("匹配审阅")}
            onRetry={() => void refreshResume()}
            onSave={() => void handleResumeSave()}
            onToggleLock={(block) => void handleResumeLock(block)}
            status={resumeStatus}
            workspace={resumeWorkspace}
          />
        ) : activeNav === "PDF 预览" ? (
          <PDFPreview
            error={pdfError}
            feedback={pdfFeedback}
            isExporting={isExportingPDF}
            isRendering={isRenderingPDF}
            onExportMarkdown={() => void handleMarkdownExport()}
            onExportPDF={() => void handlePDFExport()}
            onOpenResume={() => setActiveNav("简历工作室")}
            onRender={() => void handlePDFRender()}
            onRetry={() => void refreshPDF()}
            status={pdfStatus}
            workspace={pdfWorkspace}
          />
        ) : activeNav === "设置" ? (
          <SettingsWorkspace
            apiKey={settingsAPIKey}
            baseUrl={settingsBaseURL}
            error={settingsError}
            feedback={settingsFeedback}
            isDirty={settingsDirty}
            isSaving={isSavingSettings}
            model={settingsModel}
            onAPIKeyChange={(value) => {
              setSettingsAPIKey(value);
              setSettingsDirty(true);
              setSettingsFeedback(null);
            }}
            onBaseURLChange={(value) => {
              setSettingsBaseURL(value);
              setSettingsDirty(true);
              setSettingsFeedback(null);
            }}
            onModelChange={(value) => {
              setSettingsModel(value);
              setSettingsDirty(true);
              setSettingsFeedback(null);
            }}
            onProviderChange={handleSettingsProviderChange}
            onRetry={() => void refreshSettings()}
            onSave={() => void handleSettingsSave()}
            provider={settingsProvider}
            settings={providerSettings}
            status={settingsStatus}
          />
        ) : (
          <MatchReviewWorkspace
            error={matchError}
            isAnalyzing={isAnalyzingMatch}
            onAnalyze={() => requestProviderAction("match")}
            onOpenJD={() => setActiveNav("JD 工作区")}
            onOpenProfile={() => setActiveNav("资料库")}
            review={matchReview}
            status={matchStatus}
          />
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
            <span className="dialog-kicker">Resume Run</span>
            <h2 id="generate-title">基于当前匹配结果生成简历</h2>
            <p>
              先生成结构化 Resume，再派生 Markdown。缺失要求不会被补写成事实。
            </p>
            <dl className="dialog-summary">
              <div>
                <dt>目标岗位</dt>
                <dd>{matchReview?.jdTitle || "目标岗位"}</dd>
              </div>
              <div>
                <dt>综合匹配</dt>
                <dd>{matchReview?.totalScore ?? 0} / 100</dd>
              </div>
              <div>
                <dt>包装档位</dt>
                <dd>
                  {generationPackaging === 0
                    ? "保守"
                    : generationPackaging === 0.5
                      ? "平衡"
                      : "强化"}
                </dd>
              </div>
            </dl>
            <section
              className="provider-request-inline"
              aria-label="本次生成的 Provider 发送摘要"
            >
              <div>
                <span>Provider</span>
                <strong>
                  {providerSettings?.provider === "openai"
                    ? `OpenAI · ${providerSettings.model}`
                    : "Fake Provider · 本地 Fixture"}
                </strong>
              </div>
              <div>
                <span>发送内容</span>
                <strong>
                  {providerSettings?.provider === "openai"
                    ? "Requirement、相关 Evidence、包装参数"
                    : "不向网络发送用户内容"}
                </strong>
              </div>
            </section>
            <fieldset className="generate-options">
              <legend>简历语言</legend>
              <div>
                {[
                  { label: "中文", value: "zh" },
                  { label: "English", value: "en" },
                ].map((option) => (
                  <button
                    aria-pressed={generationLanguage === option.value}
                    className={
                      generationLanguage === option.value ? "is-selected" : ""
                    }
                    key={option.value}
                    onClick={() => setGenerationLanguage(option.value)}
                    type="button"
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </fieldset>
            <fieldset className="generate-options">
              <legend>包装强度</legend>
              <div>
                {[
                  { label: "保守", value: 0 },
                  { label: "平衡", value: 0.5 },
                  { label: "强化", value: 1 },
                ].map((option) => (
                  <button
                    aria-pressed={generationPackaging === option.value}
                    className={
                      generationPackaging === option.value ? "is-selected" : ""
                    }
                    key={option.value}
                    onClick={() => setGenerationPackaging(option.value)}
                    type="button"
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </fieldset>
            <div className="dialog-actions">
              <button
                className="button button--secondary"
                disabled={isGeneratingResume}
                onClick={() => setGenerateOpen(false)}
                type="button"
              >
                继续审阅
              </button>
              <button
                className="button button--primary"
                disabled={isGeneratingResume}
                onClick={() => void handleResumeGenerate()}
                type="button"
              >
                <IconFileDescription
                  aria-hidden="true"
                  size={18}
                  stroke={1.65}
                />
                {isGeneratingResume ? "正在生成" : "确认生成"}
              </button>
            </div>
          </section>
        </div>
      )}

      {providerRequestAction && (
        <div
          aria-labelledby="provider-request-title"
          aria-modal="true"
          className="modal-backdrop"
          role="dialog"
        >
          <section className="generate-dialog provider-request-dialog">
            <button
              aria-label="关闭"
              className="icon-button modal-close"
              onClick={() => setProviderRequestAction(null)}
              type="button"
            >
              <IconX aria-hidden="true" size={20} stroke={1.6} />
            </button>
            <span className="dialog-kicker">
              {providerRequestDetails[providerRequestAction].kicker}
            </span>
            <h2 id="provider-request-title">即将发送给 OpenAI</h2>
            <p>{providerRequestDetails[providerRequestAction].title}</p>
            <dl className="provider-request-meta">
              <div>
                <dt>Provider</dt>
                <dd>OpenAI</dd>
              </div>
              <div>
                <dt>模型</dt>
                <dd>{providerSettings?.model || "gpt-5.5"}</dd>
              </div>
            </dl>
            <section className="provider-request-content">
              <span>本次发送的数据类型</span>
              <ul>
                {providerRequestDetails[providerRequestAction].items.map(
                  (item) => (
                    <li key={item}>
                      <IconCheck aria-hidden="true" size={15} stroke={1.8} />
                      {item}
                    </li>
                  ),
                )}
              </ul>
              <small>API Key、原始文件和本地 PDF 产物不会发送。</small>
            </section>
            <div className="dialog-actions">
              <button
                className="button button--secondary"
                onClick={() => setProviderRequestAction(null)}
                type="button"
              >
                取消
              </button>
              <button
                className="button button--primary"
                onClick={confirmProviderAction}
                type="button"
              >
                确认并继续
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}

export default App;
