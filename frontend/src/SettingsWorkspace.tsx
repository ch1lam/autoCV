import {
  IconAlertTriangle,
  IconCheck,
  IconKey,
  IconLock,
  IconRefresh,
  IconWorld,
} from "@tabler/icons-react";

import type { ProviderSettings } from "../bindings/github.com/ch1lam/autocv/internal/app";

export type SettingsWorkspaceStatus = "loading" | "ready" | "error";
export type SettingsFeedback = {
  tone: "success" | "error" | "info";
  text: string;
};

type SettingsWorkspaceProps = {
  apiKey: string;
  baseUrl: string;
  error: string;
  feedback: SettingsFeedback | null;
  isDirty: boolean;
  isSaving: boolean;
  model: string;
  onAPIKeyChange: (value: string) => void;
  onBaseURLChange: (value: string) => void;
  onModelChange: (value: string) => void;
  onProviderChange: (value: string) => void;
  onRetry: () => void;
  onSave: () => void;
  provider: string;
  settings: ProviderSettings | null;
  status: SettingsWorkspaceStatus;
};

function SettingsWorkspace({
  apiKey,
  baseUrl,
  error,
  feedback,
  isDirty,
  isSaving,
  model,
  onAPIKeyChange,
  onBaseURLChange,
  onModelChange,
  onProviderChange,
  onRetry,
  onSave,
  provider,
  settings,
  status,
}: SettingsWorkspaceProps) {
  if (status === "loading") {
    return (
      <section className="match-stage-state" aria-label="正在读取 Provider 设置">
        <IconRefresh className="is-spinning" size={28} stroke={1.5} />
        <span>Provider Settings</span>
        <h1>正在读取本地 AI 配置</h1>
        <p>Go 服务正在检查 SQLite 配置与系统 Keychain 状态。</p>
      </section>
    );
  }

  if (status === "error") {
    return (
      <section className="match-stage-state match-stage-state--error">
        <IconAlertTriangle size={28} stroke={1.5} />
        <span>设置服务不可用</span>
        <h1>无法读取 Provider 配置</h1>
        <p>{error || "请检查本地数据库与系统 Keychain 后重试。"}</p>
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

  const isOpenAI = provider === "openai";
  const configured = Boolean(settings?.apiKeyConfigured);

  return (
    <div className="settings-workspace-layout">
      <main className="settings-editor">
        <header className="settings-heading">
          <div>
            <span className="section-kicker">PROVIDER SETTINGS</span>
            <h1>AI Provider 与密钥</h1>
            <p>
              业务规则仍由 Go 控制。这里只决定结构化任务由本地 Fixture
              还是 OpenAI 执行。
            </p>
          </div>
          <span className="settings-security-badge">
            <IconLock size={16} stroke={1.7} />
            密钥不进入数据库
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

        <section className="settings-form-section" aria-labelledby="provider-title">
          <div className="settings-section-heading">
            <span>01</span>
            <div>
              <h2 id="provider-title">选择执行方式</h2>
              <p>Fake Provider 可离线运行；OpenAI 使用独立结构化 Schema。</p>
            </div>
          </div>
          <div className="provider-choice" role="group" aria-label="AI Provider">
            <button
              aria-pressed={provider === "fake"}
              className={provider === "fake" ? "is-selected" : ""}
              onClick={() => onProviderChange("fake")}
              type="button"
            >
              <strong>Fake Provider</strong>
              <span>固定 Fixture · 离线可用</span>
            </button>
            <button
              aria-pressed={isOpenAI}
              className={isOpenAI ? "is-selected" : ""}
              onClick={() => onProviderChange("openai")}
              type="button"
            >
              <strong>OpenAI</strong>
              <span>Responses API · Structured Outputs</span>
            </button>
          </div>
        </section>

        <section className="settings-form-section" aria-labelledby="model-title">
          <div className="settings-section-heading">
            <span>02</span>
            <div>
              <h2 id="model-title">模型与端点</h2>
              <p>保留可配置端点，默认使用官方 API 地址。</p>
            </div>
          </div>
          <div className="settings-field-grid">
            <label className="settings-field">
              <span>模型</span>
              <input
                aria-label="模型"
                onChange={(event) => onModelChange(event.target.value)}
                placeholder={isOpenAI ? "gpt-5.5" : "fixture-v1"}
                spellCheck={false}
                type="text"
                value={model}
              />
              <small>
                {isOpenAI
                  ? "按任务使用独立 Schema，不提供通用 Chat 入口。"
                  : "Fixture 版本用于稳定回归测试。"}
              </small>
            </label>
            <label className="settings-field">
              <span>Base URL</span>
              <span className="settings-input-with-icon">
                <IconWorld aria-hidden="true" size={17} stroke={1.6} />
                <input
                  aria-label="Base URL"
                  disabled={!isOpenAI}
                  onChange={(event) => onBaseURLChange(event.target.value)}
                  placeholder="https://api.openai.com/v1"
                  spellCheck={false}
                  type="url"
                  value={baseUrl}
                />
              </span>
              <small>仅 OpenAI 模式使用；不填写时采用官方默认端点。</small>
            </label>
          </div>
        </section>

        <section className="settings-form-section" aria-labelledby="key-title">
          <div className="settings-section-heading">
            <span>03</span>
            <div>
              <h2 id="key-title">系统密钥</h2>
              <p>保存后只保留 Keychain 引用，不允许前端回读明文。</p>
            </div>
          </div>
          <label className="settings-field settings-key-field">
            <span>OpenAI API Key</span>
            <span className="settings-input-with-icon">
              <IconKey aria-hidden="true" size={17} stroke={1.6} />
              <input
                aria-label="OpenAI API Key"
                autoComplete="off"
                disabled={!isOpenAI}
                onChange={(event) => onAPIKeyChange(event.target.value)}
                placeholder={
                  configured
                    ? "已保存在 Keychain；留空可继续使用"
                    : "sk-..."
                }
                type="password"
                value={apiKey}
              />
            </span>
            <small className={configured ? "is-configured" : ""}>
              {configured ? (
                <>
                  <IconCheck aria-hidden="true" size={14} stroke={1.8} />
                  已保存在 {settings?.secretBackend || "macOS Keychain"}
                </>
              ) : isOpenAI ? (
                "首次启用时必须填写；已保存的 Keychain 密钥会自动复用。"
              ) : (
                "Fake Provider 不需要 API Key。"
              )}
            </small>
          </label>
        </section>

        <footer className="settings-editor-footer">
          <p>{settings?.configurationNote}</p>
          <button
            className="button button--primary"
            disabled={!isDirty || isSaving}
            onClick={onSave}
            type="button"
          >
            {isSaving ? "正在保存" : "保存 Provider"}
          </button>
        </footer>
      </main>

      <aside className="settings-privacy-ledger" aria-label="AI 数据发送边界">
        <header>
          <span className="section-kicker">PRIVACY LEDGER</span>
          <h2>每次调用前的发送边界</h2>
          <p>
            界面列出可能发送的数据类型。实际请求只取当前任务所需的最小上下文。
          </p>
        </header>

        <section>
          <div className="privacy-ledger-title">
            <span>发送给当前 Provider</span>
            <strong>{settings?.sentContentTypes.length ?? 0} 类</strong>
          </div>
          <ol className="privacy-ledger-list">
            {settings?.sentContentTypes.map((item, index) => (
              <li key={item.label}>
                <span>{String(index + 1).padStart(2, "0")}</span>
                <div>
                  <strong>{item.label}</strong>
                  <p>{item.description}</p>
                </div>
              </li>
            ))}
          </ol>
        </section>

        <section className="privacy-local-only">
          <div className="privacy-ledger-title">
            <span>始终留在本机</span>
            <IconLock aria-hidden="true" size={16} stroke={1.7} />
          </div>
          <ul>
            {settings?.localOnlyTypes.map((item) => (
              <li key={item.label}>
                <IconCheck aria-hidden="true" size={15} stroke={1.8} />
                <div>
                  <strong>{item.label}</strong>
                  <p>{item.description}</p>
                </div>
              </li>
            ))}
          </ul>
        </section>
      </aside>
    </div>
  );
}

export default SettingsWorkspace;
