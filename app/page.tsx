"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

type Tab = "overview" | "hardware" | "containers" | "diagnostics" | "audit";
type ContainerAction = "start" | "stop" | "restart";

type User = { username: string; role: "admin" | "operator" | "viewer" };
type Container = {
  id: string;
  name: string;
  image: string;
  status: string;
  state: string;
  ports: string;
  runningFor: string;
};
type Finding = {
  id: string;
  severity: "ok" | "info" | "warning" | "critical";
  title: string;
  detail: string;
  recommendation: string;
  detectedAt: string;
};
type AuditEntry = {
  timestamp: string;
  actor: string;
  action: string;
  target?: string;
  result: string;
  remoteIp?: string;
  detail?: string;
};
type Metrics = {
  hostname: string;
  platform: string;
  cpuCount: number;
  system: {
    osName: string;
    kernelVersion: string;
    architecture: string;
    cpuModel: string;
    cpuSockets: number;
    cpuCores: number;
    cpuThreads: number;
    cpuMaxFrequencyMHz: number;
  };
  cpuPercent: number;
  load: [number, number, number];
  memoryTotalBytes: number;
  memoryUsedBytes: number;
  swapTotalBytes: number;
  swapUsedBytes: number;
  diskTotalBytes: number;
  diskUsedBytes: number;
  uptimeSeconds: number;
  temperatures: Array<{ label: string; celsius: number }>;
  fans: Array<{ label: string; rpm: number; pwmDetected: boolean }>;
  power: {
    governor: string;
    availableGovernors: string[];
    driver: string;
    currentFrequencyMHz: number;
    minimumFrequencyMHz: number;
    maximumFrequencyMHz: number;
    platformProfile: string;
    availableProfiles: string[];
    controlSupported: boolean;
    controlDisabledReason: string;
  };
  network: { receivedBytes: number; transmittedBytes: number };
};

const API_BASE = "/api";

const demoMetrics: Metrics = {
  hostname: "ubuntu-xeon-01",
  platform: "linux",
  cpuCount: 32,
  system: {
    osName: "Ubuntu 26.04 LTS",
    kernelVersion: "7.0.0-generic",
    architecture: "amd64",
    cpuModel: "Intel Xeon E5-2689 0 @ 2.60GHz",
    cpuSockets: 2,
    cpuCores: 16,
    cpuThreads: 32,
    cpuMaxFrequencyMHz: 3600,
  },
  cpuPercent: 34,
  load: [5.18, 4.72, 4.31],
  memoryTotalBytes: 128 * 1024 ** 3,
  memoryUsedBytes: 52.4 * 1024 ** 3,
  swapTotalBytes: 8 * 1024 ** 3,
  swapUsedBytes: 384 * 1024 ** 2,
  diskTotalBytes: 3.64 * 1024 ** 4,
  diskUsedBytes: 2.14 * 1024 ** 4,
  uptimeSeconds: 18 * 86400 + 14 * 3600 + 22 * 60,
  temperatures: [
    { label: "CPU 1 Package", celsius: 58 },
    { label: "CPU 2 Package", celsius: 61 },
  ],
  fans: [],
  power: {
    governor: "schedutil",
    availableGovernors: ["performance", "powersave", "schedutil"],
    driver: "intel_cpufreq",
    currentFrequencyMHz: 2680,
    minimumFrequencyMHz: 1200,
    maximumFrequencyMHz: 3600,
    platformProfile: "",
    availableProfiles: [],
    controlSupported: false,
    controlDisabledReason: "Только безопасное чтение до проверки профилей на этом сервере.",
  },
  network: { receivedBytes: 1.82 * 1024 ** 4, transmittedBytes: 684 * 1024 ** 3 },
};

const demoContainers: Container[] = [
  { id: "c7f86d1a2ef3", name: "llm-inference", image: "server/llm:0.8.2", status: "Up 2 days (healthy)", state: "running", ports: "127.0.0.1:11434→11434", runningFor: "2 days" },
  { id: "6a1c2be90344", name: "postgres-main", image: "postgres:16-alpine", status: "Up 18 days (healthy)", state: "running", ports: "5432/tcp", runningFor: "18 days" },
  { id: "2f42dd841b82", name: "vector-store", image: "qdrant/qdrant:v1.13", status: "Up 18 days", state: "running", ports: "127.0.0.1:6333→6333", runningFor: "18 days" },
  { id: "99e0c0c5ad75", name: "training-worker", image: "server/trainer:nightly", status: "Exited (0) 6 hours ago", state: "exited", ports: "—", runningFor: "6 hours ago" },
];

const demoFindings: Finding[] = [
  { id: "updates", severity: "warning", title: "Доступно 7 обновлений безопасности", detail: "Обновления установлены не полностью; перезагрузка пока не требуется.", recommendation: "Установите обновления в ближайшее окно обслуживания и проверьте контейнеры после операции.", detectedAt: new Date().toISOString() },
  { id: "docker-policy", severity: "info", title: "Docker-действия работают в ограниченном режиме", detail: "Панель разрешает только start, stop и restart для валидного ID контейнера.", recommendation: "Оставьте произвольные команды и доступ к Docker socket недоступными из браузера.", detectedAt: new Date().toISOString() },
  { id: "thermal", severity: "ok", title: "Температуры в норме", detail: "Максимальное значение датчиков — 61°C.", recommendation: "Продолжайте наблюдать тренд под длительной нагрузкой.", detectedAt: new Date().toISOString() },
];

const demoAudit: AuditEntry[] = [
  { timestamp: new Date(Date.now() - 8 * 60_000).toISOString(), actor: "admin", action: "docker.restart", target: "llm-inference", result: "allowed", remoteIp: "10.0.0.24" },
  { timestamp: new Date(Date.now() - 42 * 60_000).toISOString(), actor: "admin", action: "auth.login", result: "allowed", remoteIp: "10.0.0.24" },
  { timestamp: new Date(Date.now() - 3 * 3600_000).toISOString(), actor: "unknown", action: "auth.login", result: "denied", remoteIp: "10.0.0.19" },
];

const initialHistory = [28, 31, 26, 42, 39, 34, 37, 51, 46, 41, 38, 34];

export default function Home() {
  const [tab, setTab] = useState<Tab>("overview");
  const [authState, setAuthState] = useState<"checking" | "signed-out" | "signed-in">("checking");
  const [demoMode, setDemoMode] = useState(false);
  const [user, setUser] = useState<User>({ username: "admin", role: "admin" });
  const [csrfToken, setCsrfToken] = useState("");
  const [metrics, setMetrics] = useState<Metrics>(demoMetrics);
  const [containers, setContainers] = useState<Container[]>(demoContainers);
  const [findings, setFindings] = useState<Finding[]>(demoFindings);
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>(demoAudit);
  const [history, setHistory] = useState(initialHistory);
  const [lastUpdated, setLastUpdated] = useState(new Date());
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState("");
  const [pendingAction, setPendingAction] = useState<{ container: Container; action: ContainerAction } | null>(null);

  const loadDashboard = useCallback(async () => {
    if (demoMode) {
      setLastUpdated(new Date());
      return;
    }
    const [metricResponse, containerResponse, diagnosticsResponse, auditResponse] = await Promise.all([
      fetch(`${API_BASE}/metrics`, { credentials: "include" }),
      fetch(`${API_BASE}/containers`, { credentials: "include" }),
      fetch(`${API_BASE}/diagnostics`, { credentials: "include" }),
      fetch(`${API_BASE}/audit`, { credentials: "include" }),
    ]);
    if (metricResponse.status === 401) {
      setAuthState("signed-out");
      return;
    }
    if (metricResponse.ok) {
      const data = (await metricResponse.json()) as { metrics: Metrics };
      setMetrics(data.metrics);
      setHistory((current) => [...current.slice(-23), Math.round(data.metrics.cpuPercent)]);
    }
    if (containerResponse.ok) {
      const data = (await containerResponse.json()) as { containers: Container[] };
      setContainers(data.containers);
    }
    if (diagnosticsResponse.ok) {
      const data = (await diagnosticsResponse.json()) as { findings: Finding[] };
      setFindings(data.findings);
    }
    if (auditResponse.ok) {
      const data = (await auditResponse.json()) as { entries: AuditEntry[] };
      setAuditEntries(data.entries.reverse());
    }
    setLastUpdated(new Date());
  }, [demoMode]);

  useEffect(() => {
    const localDemo = process.env.NODE_ENV === "development"
      && (window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1");
    if (localDemo) {
      window.setTimeout(() => {
        setDemoMode(true);
        setAuthState("signed-in");
      }, 0);
      return;
    }
    fetch(`${API_BASE}/session`, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("signed out");
        const data = (await response.json()) as { user: User; csrfToken: string };
        setUser(data.user);
        setCsrfToken(data.csrfToken);
        setAuthState("signed-in");
      })
      .catch(() => setAuthState("signed-out"));
  }, []);

  useEffect(() => {
    if (authState !== "signed-in") return;
    const initial = window.setTimeout(() => void loadDashboard(), 0);
    if (demoMode) return () => window.clearTimeout(initial);
    const interval = window.setInterval(() => void loadDashboard(), 5000);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(interval);
    };
  }, [authState, demoMode, loadDashboard]);

  useEffect(() => {
    if (!toast) return;
    const timeout = window.setTimeout(() => setToast(""), 4200);
    return () => window.clearTimeout(timeout);
  }, [toast]);

  const usage = useMemo(() => ({
    memory: ratio(metrics.memoryUsedBytes, metrics.memoryTotalBytes),
    swap: ratio(metrics.swapUsedBytes, metrics.swapTotalBytes),
    disk: ratio(metrics.diskUsedBytes, metrics.diskTotalBytes),
    maxTemperature: Math.max(0, ...metrics.temperatures.map((item) => item.celsius)),
  }), [metrics]);

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const username = String(form.get("username") || "");
    const password = String(form.get("password") || "");
    if (!username || !password) return;
    setBusy(true);
    try {
      if (demoMode) {
        setUser({ username, role: "admin" });
        setAuthState("signed-in");
        setToast("Локальный демо-режим открыт");
        return;
      }
      const response = await fetch(`${API_BASE}/auth/login`, {
        method: "POST", credentials: "include", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      const data = await response.json() as { user?: User; csrfToken?: string; error?: string };
      if (!response.ok || !data.user || !data.csrfToken) throw new Error(data.error || "Не удалось войти");
      setUser(data.user);
      setCsrfToken(data.csrfToken);
      setAuthState("signed-in");
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Не удалось войти");
    } finally {
      setBusy(false);
    }
  }

  async function signOut() {
    if (!demoMode) {
      await fetch(`${API_BASE}/auth/logout`, { method: "POST", credentials: "include", headers: { "X-CSRF-Token": csrfToken } });
    }
    setAuthState("signed-out");
  }

  async function runContainerAction() {
    if (!pendingAction) return;
    const { container, action } = pendingAction;
    setBusy(true);
    try {
      if (!demoMode) {
        const response = await fetch(`${API_BASE}/containers/${encodeURIComponent(container.id)}/${action}`, {
          method: "POST", credentials: "include", headers: { "X-CSRF-Token": csrfToken },
        });
        const data = await response.json() as { error?: string };
        if (!response.ok) throw new Error(data.error || "Операция не выполнена");
      } else {
        setContainers((current) => current.map((item) => item.id === container.id ? {
          ...item,
          state: action === "stop" ? "exited" : "running",
          status: action === "stop" ? "Exited (0) just now" : "Up a few seconds",
        } : item));
      }
      setToast(`${labelAction(action)}: ${container.name} — выполнено`);
      setPendingAction(null);
      await loadDashboard();
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Операция не выполнена");
    } finally {
      setBusy(false);
    }
  }

  if (authState === "checking") return <LoadingScreen />;
  if (authState === "signed-out") return <LoginScreen busy={busy} onSubmit={signIn} demoMode={demoMode} />;

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">Перейти к содержимому</a>
      <aside className="sidebar" aria-label="Основная навигация">
        <div className="brand"><span className="brand-mark">SP</span><span>ServerPanel<small>Control plane · 0.1</small></span></div>
        <nav className="nav-list">
          <NavButton active={tab === "overview"} icon="overview" label="Обзор" badge="live" onClick={() => setTab("overview")} />
          <NavButton active={tab === "hardware"} icon="hardware" label="Питание" badge={String(metrics.temperatures.length)} onClick={() => setTab("hardware")} />
          <NavButton active={tab === "containers"} icon="containers" label="Контейнеры" badge={String(containers.length)} onClick={() => setTab("containers")} />
          <NavButton active={tab === "diagnostics"} icon="diagnostics" label="Диагностика" badge={String(findings.filter((item) => item.severity !== "ok").length)} onClick={() => setTab("diagnostics")} />
          <NavButton active={tab === "audit"} icon="audit" label="Журнал" onClick={() => setTab("audit")} />
        </nav>
        <div className="sidebar-footer">
          <div className="future-card"><span>Следующий этап</span><strong>Codex и профили питания</strong><p>После проверки MVP и сервера по SSH.</p></div>
          <button className="account-button" type="button" onClick={signOut}><span className="avatar">{user.username.slice(0, 2).toUpperCase()}</span><span><strong>{user.username}</strong><small>{roleLabel(user.role)}</small></span><span className="logout-label">Выйти</span></button>
        </div>
      </aside>

      <main id="main-content" className="main-content">
        <header className="topbar">
          <div className="topbar-copy"><p className="eyebrow">ServerPanel / {metrics.hostname}</p><h1>{tabTitle(tab)}</h1><p className="page-description">{tabDescription(tab)}</p></div>
          <div className="topbar-actions">
            <div className="connection-state"><span className="status-dot" />Сервер на связи</div>
            <button className="button secondary" type="button" disabled={busy} onClick={() => void loadDashboard()}>{busy ? "Обновление…" : "Обновить"}</button>
          </div>
        </header>

        {demoMode && <div className="demo-banner"><strong>Режим предпросмотра.</strong> Показаны безопасные демонстрационные данные; на Ubuntu здесь появятся реальные метрики агента.</div>}

        {tab === "overview" && <Overview metrics={metrics} usage={usage} history={history} findings={findings} containers={containers} lastUpdated={lastUpdated} />}
        {tab === "hardware" && <Hardware metrics={metrics} />}
        {tab === "containers" && <Containers containers={containers} onAction={(container, action) => setPendingAction({ container, action })} />}
        {tab === "diagnostics" && <Diagnostics findings={findings} />}
        {tab === "audit" && <Audit entries={auditEntries} />}
      </main>

      {pendingAction && <ConfirmDialog pending={pendingAction} busy={busy} onCancel={() => setPendingAction(null)} onConfirm={runContainerAction} />}
      {toast && <div className="toast" role="status" aria-live="polite">{toast}</div>}
    </div>
  );
}

function Overview({ metrics, usage, history, findings, containers, lastUpdated }: { metrics: Metrics; usage: { memory: number; swap: number; disk: number; maxTemperature: number }; history: number[]; findings: Finding[]; containers: Container[]; lastUpdated: Date }) {
  const critical = findings.filter((item) => item.severity === "critical").length;
  const running = containers.filter((item) => item.state === "running").length;
  return <div className="page-stack">
    <section className="server-hero" aria-label="Краткое состояние сервера">
      <div className="server-identity">
        <div className="server-orbit" aria-hidden="true"><span>SP</span></div>
        <div><p className="eyebrow">Основной узел</p><h2>{metrics.hostname}</h2><p>{metrics.system.osName || "Linux"} · {metrics.system.cpuModel || "локальный агент"}</p></div>
      </div>
      <dl className="server-facts">
        <div><dt>Состояние</dt><dd className="online-value"><span className="status-dot" />В сети</dd></div>
        <div><dt>Uptime</dt><dd>{formatUptime(metrics.uptimeSeconds)}</dd></div>
        <div><dt>CPU</dt><dd>{metrics.system.cpuCores || metrics.cpuCount}C / {metrics.system.cpuThreads || metrics.cpuCount}T</dd></div>
        <div><dt>Load 1m</dt><dd>{metrics.load[0]?.toFixed(2) || "0.00"}</dd></div>
      </dl>
    </section>

    <section className="metric-grid" aria-label="Текущие показатели">
      <MetricCard code="CPU" label="Процессоры" value={`${metrics.cpuPercent.toFixed(0)}%`} detail={`${metrics.system.cpuSockets || 1} сок. · ${metrics.system.cpuCores || metrics.cpuCount} ядер · ${metrics.system.cpuThreads || metrics.cpuCount} потоков`} progress={metrics.cpuPercent} tone="blue" />
      <MetricCard code="RAM" label="Оперативная память" value={`${usage.memory.toFixed(0)}%`} detail={`${formatBytes(metrics.memoryUsedBytes)} из ${formatBytes(metrics.memoryTotalBytes)}`} progress={usage.memory} tone="violet" />
      <MetricCard code="SWAP" label="Подкачка" value={metrics.swapTotalBytes ? `${usage.swap.toFixed(0)}%` : "Отключена"} detail={metrics.swapTotalBytes ? `${formatBytes(metrics.swapUsedBytes)} из ${formatBytes(metrics.swapTotalBytes)}` : "Swap-раздел не настроен"} progress={usage.swap} tone="blue" />
      <MetricCard code="NVME" label="Хранилище" value={`${usage.disk.toFixed(0)}%`} detail={`${formatBytes(metrics.diskUsedBytes)} из ${formatBytes(metrics.diskTotalBytes)}`} progress={usage.disk} tone="amber" />
      <MetricCard code="TEMP" label="Макс. температура" value={usage.maxTemperature ? `${usage.maxTemperature.toFixed(0)}°C` : "Нет данных"} detail={metrics.temperatures.length ? `${metrics.temperatures.length} активных датчика` : "Датчики не обнаружены"} progress={usage.maxTemperature} tone="green" />
    </section>

    <section className="dashboard-grid">
      <article className="panel chart-panel">
        <div className="panel-header"><div><p className="eyebrow">Последние измерения</p><h2>Нагрузка CPU</h2></div><span className="live-pill"><span className="status-dot" /> live</span></div>
        <div className="chart-kpi"><strong>{metrics.cpuPercent.toFixed(0)}%</strong><span>сейчас</span></div>
        <div className="bar-chart" aria-label={`График загрузки CPU, текущее значение ${metrics.cpuPercent.toFixed(0)} процентов`}>
          {history.map((value, index) => <span key={`${index}-${value}`} style={{ height: `${Math.max(8, value)}%` }}><i>{value}%</i></span>)}
        </div>
        <div className="chart-axis"><span>раньше</span><span>сейчас</span></div>
      </article>

      <article className="panel health-panel">
        <div className="panel-header"><div><p className="eyebrow">Состояние</p><h2>{critical ? "Требуется внимание" : "Система стабильна"}</h2></div><span className={`health-score ${critical ? "danger" : "healthy"}`}>{critical ? critical : "A"}</span></div>
        <dl className="health-list">
          <div><dt>Операционная система</dt><dd>{metrics.system.osName || metrics.platform}</dd></div>
          <div><dt>Ядро</dt><dd>{metrics.system.kernelVersion || "—"}</dd></div>
          <div><dt>Архитектура</dt><dd>{metrics.system.architecture || "—"}</dd></div>
          <div><dt>Модель CPU</dt><dd>{metrics.system.cpuModel || "—"}</dd></div>
          <div><dt>Макс. частота</dt><dd>{metrics.system.cpuMaxFrequencyMHz ? `${(metrics.system.cpuMaxFrequencyMHz / 1000).toFixed(2)} ГГц` : "—"}</dd></div>
          <div><dt>Uptime</dt><dd>{formatUptime(metrics.uptimeSeconds)}</dd></div>
          <div><dt>Docker</dt><dd>{running} из {containers.length} запущено</dd></div>
          <div><dt>Входящий трафик</dt><dd>{formatBytes(metrics.network.receivedBytes)}</dd></div>
          <div><dt>Последнее обновление</dt><dd>{lastUpdated.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</dd></div>
        </dl>
      </article>
    </section>

    <section className="dashboard-grid lower-grid">
      <article className="panel">
        <div className="panel-header"><div><p className="eyebrow">Активные замечания</p><h2>Диагностика</h2></div><span className="count-pill">{findings.length}</span></div>
        <div className="finding-compact-list">{findings.slice(0, 3).map((finding) => <div className="finding-compact" key={finding.id}><span className={`severity-marker ${finding.severity}`} /><div><strong>{finding.title}</strong><p>{finding.detail}</p></div></div>)}</div>
      </article>
      <article className="panel capability-panel">
        <div><p className="eyebrow">Безопасный rollout</p><h2>Питание и вентиляторы</h2><p>Недоступно в MVP до определения модели сервера, BMC/IPMI и допустимых температурных лимитов.</p></div>
        <div className="segmented disabled" aria-label="Профили питания пока недоступны"><span>Эко</span><span className="selected">Баланс</span><span>Турбо</span></div>
        <small>Будет включено только после аппаратной проверки и автоматического отката.</small>
      </article>
    </section>
  </div>;
}

function Hardware({ metrics }: { metrics: Metrics }) {
  const maximumTemperature = Math.max(0, ...metrics.temperatures.map((sensor) => sensor.celsius));
  return <div className="page-stack">
    <section className="summary-strip four" aria-label="Сводка питания и охлаждения">
      <div><span className="summary-number">{metrics.power.governor || "—"}</span><span>CPU governor</span></div>
      <div><span className="summary-number">{formatFrequency(metrics.power.currentFrequencyMHz)}</span><span>средняя частота</span></div>
      <div><span className="summary-number">{maximumTemperature ? `${maximumTemperature.toFixed(0)}°C` : "—"}</span><span>макс. температура</span></div>
      <div><span className="summary-number">{metrics.fans.length || "—"}</span><span>датчиков RPM</span></div>
    </section>

    <section className="dashboard-grid lower-grid">
      <article className="panel capability-panel power-panel">
        <div className="panel-header"><div><p className="eyebrow">CPU frequency policy</p><h2>Профиль энергопотребления</h2></div><span className="live-pill">только чтение</span></div>
        <dl className="health-list">
          <div><dt>Драйвер</dt><dd>{metrics.power.driver || "недоступно"}</dd></div>
          <div><dt>Governor</dt><dd>{metrics.power.governor || "недоступно"}</dd></div>
          <div><dt>Диапазон</dt><dd>{formatFrequency(metrics.power.minimumFrequencyMHz)} — {formatFrequency(metrics.power.maximumFrequencyMHz)}</dd></div>
          <div><dt>Доступные governor</dt><dd>{metrics.power.availableGovernors.join(", ") || "не объявлены ядром"}</dd></div>
          <div><dt>ACPI-профиль</dt><dd>{metrics.power.platformProfile || "не поддерживается"}</dd></div>
        </dl>
        <div className="segmented disabled" aria-label="Переключение профилей пока заблокировано"><span>Эко</span><span className="selected">Баланс</span><span>Турбо</span></div>
        <small>{metrics.power.controlDisabledReason || "Переключение будет доступно после аппаратной проверки и настройки отката."}</small>
      </article>

      <article className="panel">
        <div className="panel-header"><div><p className="eyebrow">hwmon / sysfs</p><h2>Температуры</h2></div><span className="count-pill">{metrics.temperatures.length}</span></div>
        <div className="telemetry-list">
          {metrics.temperatures.length ? metrics.temperatures.map((sensor, index) => <div className="telemetry-row" key={`${sensor.label}-${index}`}><span>{sensor.label}</span><strong className={sensor.celsius >= 85 ? "critical-value" : sensor.celsius >= 75 ? "warning-value" : "healthy-value"}>{sensor.celsius.toFixed(1)}°C</strong></div>) : <p className="empty-telemetry">Температурные датчики не обнаружены или недоступны агенту.</p>}
        </div>
      </article>
    </section>

    <section className="panel">
      <div className="panel-header"><div><p className="eyebrow">Охлаждение</p><h2>Вентиляторы</h2><p className="panel-description">RPM отображается только при наличии hwmon/IPMI-датчиков. Наличие PWM-файла ещё не означает, что запись безопасна.</p></div><span className="count-pill">{metrics.fans.length}</span></div>
      <div className="telemetry-list fan-list">
        {metrics.fans.length ? metrics.fans.map((fan, index) => <div className="telemetry-row" key={`${fan.label}-${index}`}><span>{fan.label}<small>{fan.pwmDetected ? "PWM обнаружен · управление заблокировано" : "только датчик оборотов"}</small></span><strong>{fan.rpm.toFixed(0)} RPM</strong></div>) : <p className="empty-telemetry">ОС не видит датчики оборотов. Для этого сервера потребуется определить наличие BMC/IPMI или поддержку платы до добавления управления.</p>}
      </div>
    </section>
  </div>;
}

function Containers({ containers, onAction }: { containers: Container[]; onAction: (container: Container, action: ContainerAction) => void }) {
  return <section className="panel table-panel">
    <div className="panel-header table-heading"><div><p className="eyebrow">Docker Engine</p><h2>{containers.length} контейнера</h2><p>Только разрешённые действия без произвольных консольных команд.</p></div><div className="legend"><span><i className="status-dot" /> работает</span><span><i className="status-dot stopped" /> остановлен</span></div></div>
    <div className="table-wrap"><table><thead><tr><th>Контейнер</th><th>Образ</th><th>Состояние</th><th>Порты</th><th><span className="visually-hidden">Действия</span></th></tr></thead><tbody>
      {containers.map((container) => <tr key={container.id}><td><div className="container-name"><span className={`container-state ${container.state}`} /><span><strong>{container.name}</strong><small>{container.id}</small></span></div></td><td><code>{container.image}</code></td><td><strong>{container.state === "running" ? "Работает" : "Остановлен"}</strong><small className="cell-subtitle">{container.status}</small></td><td><code>{container.ports || "—"}</code></td><td><div className="row-actions">{container.state === "running" ? <><button type="button" onClick={() => onAction(container, "restart")}>Перезапустить</button><button className="danger-link" type="button" onClick={() => onAction(container, "stop")}>Остановить</button></> : <button type="button" onClick={() => onAction(container, "start")}>Запустить</button>}</div></td></tr>)}
    </tbody></table></div>
  </section>;
}

function Diagnostics({ findings }: { findings: Finding[] }) {
  return <div className="page-stack"><section className="summary-strip"><div><span className="summary-number">{findings.filter((f) => f.severity === "critical").length}</span><span>критических</span></div><div><span className="summary-number">{findings.filter((f) => f.severity === "warning").length}</span><span>предупреждений</span></div><div><span className="summary-number">{findings.filter((f) => f.severity === "ok").length}</span><span>в норме</span></div></section><section className="finding-grid">{findings.map((finding) => <article className={`finding-card ${finding.severity}`} key={finding.id}><div className="finding-card-top"><span className={`severity-label ${finding.severity}`}>{severityLabel(finding.severity)}</span><time>{new Date(finding.detectedAt).toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" })}</time></div><h2>{finding.title}</h2><p>{finding.detail}</p><div className="recommendation"><strong>Что делать</strong><p>{finding.recommendation}</p></div></article>)}</section></div>;
}

function Audit({ entries }: { entries: AuditEntry[] }) {
  return <section className="panel table-panel"><div className="panel-header table-heading"><div><p className="eyebrow">Последние 100 событий</p><h2>Журнал действий</h2><p>Входы и привилегированные операции фиксируются агентом.</p></div><button className="button secondary" type="button" disabled>Экспорт — далее</button></div><div className="table-wrap"><table><thead><tr><th>Время</th><th>Пользователь</th><th>Событие</th><th>Цель</th><th>Результат</th><th>IP</th></tr></thead><tbody>{entries.map((entry, index) => <tr key={`${entry.timestamp}-${index}`}><td>{new Date(entry.timestamp).toLocaleString("ru-RU")}</td><td><strong>{entry.actor}</strong></td><td><code>{entry.action}</code></td><td>{entry.target || "—"}</td><td><span className={`result-pill ${entry.result}`}>{entry.result === "allowed" ? "разрешено" : "отклонено"}</span></td><td><code>{entry.remoteIp || "—"}</code></td></tr>)}</tbody></table></div></section>;
}

function MetricCard({ code, label, value, detail, progress, tone }: { code: string; label: string; value: string; detail: string; progress: number; tone: string }) {
  return <article className={`metric-card ${tone}`}><div className="metric-label"><span><i />{label}</span><small>{code}</small></div><strong className="metric-value">{value}</strong><p>{detail}</p><div className="progress-track"><span style={{ width: `${Math.min(100, Math.max(0, progress))}%` }} /></div></article>;
}

type NavIconName = "overview" | "hardware" | "containers" | "diagnostics" | "audit";

function NavButton({ active, icon, label, badge, onClick }: { active: boolean; icon: NavIconName; label: string; badge?: string; onClick: () => void }) {
  return <button type="button" className={active ? "active" : ""} aria-current={active ? "page" : undefined} onClick={onClick}><span className="nav-label"><NavIcon name={icon} />{label}</span>{badge && <small>{badge}</small>}</button>;
}

function NavIcon({ name }: { name: NavIconName }) {
  const paths = {
    overview: <><rect x="3" y="3" width="7" height="7" rx="1.5" /><rect x="14" y="3" width="7" height="7" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /><rect x="14" y="14" width="7" height="7" rx="1.5" /></>,
    hardware: <><path d="M13 2 5.5 13h5L9.8 22 18.5 10h-5L13 2Z" /><path d="M4 4v5" /><path d="M2 6.5h4" /></>,
    containers: <><path d="m12 3 8 4.5-8 4.5-8-4.5L12 3Z" /><path d="m4 12 8 4.5 8-4.5" /><path d="m4 16.5 8 4.5 8-4.5" /></>,
    diagnostics: <><path d="M12 3 4.5 6v5.5c0 4.5 3 7.7 7.5 9.5 4.5-1.8 7.5-5 7.5-9.5V6L12 3Z" /><path d="m8.5 12 2.2 2.2 4.8-5" /></>,
    audit: <><path d="M6 4h12" /><path d="M6 9h12" /><path d="M6 14h8" /><path d="M6 19h5" /><circle cx="18" cy="18" r="3" /></>,
  };
  return <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

function ConfirmDialog({ pending, busy, onCancel, onConfirm }: { pending: { container: Container; action: ContainerAction }; busy: boolean; onCancel: () => void; onConfirm: () => void }) {
  const dangerous = pending.action === "stop";
  return <div className="modal-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onCancel()}><div className="modal" role="dialog" aria-modal="true" aria-labelledby="confirm-title"><span className={`modal-symbol ${dangerous ? "danger" : "warning"}`}>{dangerous ? "!" : "↻"}</span><p className="eyebrow">Подтверждение операции</p><h2 id="confirm-title">{labelAction(pending.action)} «{pending.container.name}»?</h2><p>Агент выполнит только команду <code>docker {pending.action} {pending.container.id}</code>. Событие попадёт в журнал.</p><div className="modal-actions"><button className="button secondary" type="button" onClick={onCancel} disabled={busy}>Отмена</button><button className={`button ${dangerous ? "danger" : "primary"}`} type="button" onClick={onConfirm} disabled={busy}>{busy ? "Выполняется…" : "Подтвердить"}</button></div></div></div>;
}

function LoginScreen({ busy, onSubmit, demoMode }: { busy: boolean; onSubmit: (event: FormEvent<HTMLFormElement>) => void; demoMode: boolean }) {
  return <main className="login-page"><section className="login-intro"><div className="brand large"><span className="brand-mark">SP</span><span>ServerPanel<small>Ubuntu control plane</small></span></div><div><p className="eyebrow">Сервер под контролем</p><h1>Главное состояние — без терминального шума.</h1><p>Метрики, Docker и диагностика в одной защищённой панели. Привилегированные действия ограничены и записываются в журнал.</p></div><div className="login-trust"><span>Loopback agent</span><span>CSRF protection</span><span>No shell endpoint</span></div></section><section className="login-form-wrap"><form className="login-form" onSubmit={onSubmit}><div><p className="eyebrow">Авторизация</p><h2>Войти в ServerPanel</h2><p>Используйте аккаунт, созданный командой на Ubuntu.</p></div>{demoMode && <div className="form-note">Локальный демо-режим: введите любые непустые значения.</div>}<label htmlFor="username">Логин</label><input id="username" name="username" autoComplete="username" required defaultValue={demoMode ? "admin" : ""} /><label htmlFor="password">Пароль</label><input id="password" name="password" type="password" autoComplete="current-password" required defaultValue={demoMode ? "demo-password" : ""} /><button className="button primary full" type="submit" disabled={busy}>{busy ? "Проверка…" : "Войти"}</button><small>После 5 неудачных попыток вход временно блокируется.</small></form></section></main>;
}

function LoadingScreen() { return <main className="loading-screen"><div className="brand-mark pulse">SP</div><p>Проверяем защищённую сессию…</p></main>; }

function ratio(used: number, total: number) { return total > 0 ? used / total * 100 : 0; }
function formatBytes(bytes: number) { if (!bytes) return "0 Б"; const units = ["Б", "КБ", "МБ", "ГБ", "ТБ"]; const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** power).toLocaleString("ru-RU", { maximumFractionDigits: power > 2 ? 1 : 0 })} ${units[power]}`; }
function formatUptime(seconds: number) { const days = Math.floor(seconds / 86400); const hours = Math.floor((seconds % 86400) / 3600); return `${days} д ${hours} ч`; }
function tabTitle(tab: Tab) { return ({ overview: "Обзор сервера", hardware: "Питание и охлаждение", containers: "Docker-контейнеры", diagnostics: "Диагностика", audit: "Журнал действий" })[tab]; }
function tabDescription(tab: Tab) { return ({ overview: "Живое состояние, ресурсы и ключевые риски узла.", hardware: "Частоты CPU, governor, swap, температуры и доступность датчиков вентиляторов.", containers: "Контроль жизненного цикла разрешённых контейнеров.", diagnostics: "Проблемы, рекомендации и состояние защиты.", audit: "Проверяемая история входов и привилегированных операций." })[tab]; }
function roleLabel(role: User["role"]) { return ({ admin: "Администратор", operator: "Оператор", viewer: "Наблюдатель" })[role]; }
function severityLabel(severity: Finding["severity"]) { return ({ ok: "Норма", info: "Информация", warning: "Внимание", critical: "Критично" })[severity]; }
function labelAction(action: ContainerAction) { return ({ start: "Запустить", stop: "Остановить", restart: "Перезапустить" })[action]; }
function formatFrequency(megahertz: number) { return megahertz > 0 ? `${(megahertz / 1000).toFixed(2)} ГГц` : "—"; }
