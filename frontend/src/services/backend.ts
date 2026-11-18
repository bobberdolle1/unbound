import * as WailsApp from '../../wailsjs/go/main/App';
import { engine, main } from '../../wailsjs/go/models';

export interface AutoTuneVNextHistorySummary {
  record_count: number;
  verified_count: number;
  negative_count: number;
  updated_at: string;
  context_ready: boolean;
}

export interface AutoTuneVNextServicePreset {
  id: string;
  label: string;
  description: string;
  hosts: string[];
}

export interface AutoTuneVNextServiceGraphResult {
  status: string;
  preset_id: string;
  hosts: string[];
  verified_hosts: string[];
  applied: boolean;
  summary: string;
  errors: string[];
}

interface ExtendedWailsGoApp {
  CancelAutoTuneVNext?: () => Promise<boolean>;
  GetAutoTuneVNextHistorySummary?: () => Promise<AutoTuneVNextHistorySummary>;
  ClearAutoTuneVNextHistory?: () => Promise<void>;
  GetAutoTuneVNextServicePresets?: () => Promise<AutoTuneVNextServicePreset[]>;
  AutoTuneVNextServiceGraph?: (presetID: string) => Promise<AutoTuneVNextServiceGraphResult>;
  RevertAutoTuneVNextServiceGraph?: () => Promise<void>;
  GetAutoTuneVNextManagedStatus?: () => Promise<main.AutoTuneVNextManagedStatus>;
  StartDoctor?: (mode: string) => Promise<engine.DoctorRunStart>;
  SaveSettings?: (settings: engine.Settings) => Promise<void>;
}

function getGoApp(): ExtendedWailsGoApp | undefined {
  if (typeof window === 'undefined') return undefined;
  return (window as unknown as { go?: { main?: { App?: ExtendedWailsGoApp } } }).go?.main?.App;
}

export const DEFAULT_SERVICE_GRAPH_PRESETS: AutoTuneVNextServicePreset[] = [
  {
    id: 'youtube',
    label: 'YouTube',
    description: 'Веб-интерфейс, API и медиа-узлы YouTube',
    hosts: ['www.youtube.com', 'youtubei.googleapis.com', 'i.ytimg.com'],
  },
  {
    id: 'discord',
    label: 'Discord',
    description: 'Основной домен, шлюз и CDN Discord',
    hosts: ['discord.com', 'gateway.discord.gg', 'cdn.discordapp.com'],
  },
];

export const backendService = {
  // Engine APIs
  getEngineNames: () => WailsApp.GetEngineNames(),
  getProfiles: (engineName: string) => WailsApp.GetProfiles(engineName),
  startEngine: (engineName: string, profileName: string) => WailsApp.StartEngine(engineName, profileName),
  stopEngine: () => WailsApp.StopEngine(),
  autoTune: () => WailsApp.AutoTune(),
  cancelAutoTune: () => WailsApp.CancelAutoTune(),
  killWinws2: () => WailsApp.KillWinws2(),
  getAutoTuneVNextExperimentConfig: () => WailsApp.GetAutoTuneVNextExperimentConfig(),
  runExperimentalAutoTuneVNext: (presetId: string, customTarget: string) =>
    WailsApp.RunExperimentalAutoTuneVNext(presetId, customTarget),
  applyAutoTuneVNextSelection: (token: string) => WailsApp.ApplyAutoTuneVNextSelection(token),
  revertAutoTuneVNext: () => WailsApp.RevertAutoTuneVNext(),
  getAutoTuneVNextManagedStatus: (): Promise<main.AutoTuneVNextManagedStatus> => {
    const goApp = getGoApp();
    if (typeof goApp?.GetAutoTuneVNextManagedStatus === 'function') {
      return WailsApp.GetAutoTuneVNextManagedStatus();
    }
    return Promise.resolve(
      new main.AutoTuneVNextManagedStatus({ state: 'DIRECT', active: false, needs_revalidation: false })
    );
  },

  // New vNext Contracts (Progress / Cancel / History Ledger / ServiceGraph)
  cancelAutoTuneVNext: (): Promise<boolean> => {
    const goApp = getGoApp();
    if (typeof goApp?.CancelAutoTuneVNext === 'function') {
      return goApp.CancelAutoTuneVNext();
    }
    return Promise.resolve(false);
  },
  getAutoTuneVNextHistorySummary: (): Promise<AutoTuneVNextHistorySummary> => {
    const goApp = getGoApp();
    if (typeof goApp?.GetAutoTuneVNextHistorySummary === 'function') {
      return goApp.GetAutoTuneVNextHistorySummary();
    }
    return Promise.resolve({
      record_count: 0,
      verified_count: 0,
      negative_count: 0,
      updated_at: '',
      context_ready: false,
    });
  },
  clearAutoTuneVNextHistory: (): Promise<void> => {
    const goApp = getGoApp();
    if (typeof goApp?.ClearAutoTuneVNextHistory === 'function') {
      return goApp.ClearAutoTuneVNextHistory();
    }
    return Promise.resolve();
  },
  getAutoTuneVNextServicePresets: (): Promise<AutoTuneVNextServicePreset[]> => {
    const goApp = getGoApp();
    if (typeof goApp?.GetAutoTuneVNextServicePresets === 'function') {
      return goApp.GetAutoTuneVNextServicePresets();
    }
    return Promise.resolve(DEFAULT_SERVICE_GRAPH_PRESETS.map((p) => ({ ...p, hosts: [...p.hosts] })));
  },
  autoTuneVNextServiceGraph: (presetID: string): Promise<AutoTuneVNextServiceGraphResult> => {
    const goApp = getGoApp();
    if (typeof goApp?.AutoTuneVNextServiceGraph === 'function') {
      return goApp.AutoTuneVNextServiceGraph(presetID);
    }
    return Promise.resolve({
      status: 'UNAVAILABLE',
      preset_id: presetID,
      hosts: [],
      verified_hosts: [],
      applied: false,
      summary: 'Пакетный подбор ServiceGraph недоступен в текущем окружении.',
      errors: [],
    });
  },
  revertAutoTuneVNextServiceGraph: (): Promise<void> => {
    const goApp = getGoApp();
    if (typeof goApp?.RevertAutoTuneVNextServiceGraph === 'function') {
      return goApp.RevertAutoTuneVNextServiceGraph();
    }
    return Promise.resolve();
  },

  // Settings & System
  getSettings: () => WailsApp.GetSettings(),
  saveSettings: (settings: engine.Settings): Promise<void> => {
    const goApp = getGoApp();
    if (typeof goApp?.SaveSettings === 'function') {
      return WailsApp.SaveSettings(settings);
    }
    return Promise.resolve();
  },
  getAppVersion: () => WailsApp.GetAppVersion(),
  getOSPlatform: () => WailsApp.GetOSPlatform(),
  toggleFavoriteProfile: (profile: string) => WailsApp.ToggleFavoriteProfile(profile),
  getFavoriteProfiles: () => WailsApp.GetFavoriteProfiles(),
  updateHostlistsNow: () => WailsApp.UpdateHostlistsNow(),

  // Bypass Lists
  getBypassLists: () => WailsApp.GetBypassLists(),
  readBypassList: (name: string) => WailsApp.ReadBypassList(name),
  saveBypassList: (name: string, content: string) => WailsApp.SaveBypassList(name, content),

  // Diagnostics & Security
  checkConflicts: () => WailsApp.CheckConflicts(),
  killConflicts: () => WailsApp.KillConflicts(),
  checkPrivileges: () => WailsApp.CheckPrivileges(),
  runDiagnostics: () => WailsApp.RunDiagnostics(),
  runDoctor: (mode: string) => WailsApp.RunDoctor(mode),
  startDoctor: (mode: string): Promise<engine.DoctorRunStart> => {
    const goApp = getGoApp();
    if (typeof goApp?.StartDoctor === 'function') {
      return WailsApp.StartDoctor(mode);
    }
    return Promise.resolve(
      new engine.DoctorRunStart({
        runId: `doctor-${mode}`,
        mode,
        total: mode === 'extended' ? 24 : 16,
        startedAt: new Date().toISOString(),
      })
    );
  },
  getDoctorRunState: (runId: string) => WailsApp.GetDoctorRunState(runId),
  cancelDoctor: (runId: string) => WailsApp.CancelDoctor(runId),
  runBypassComparison: () => WailsApp.RunBypassComparison(),

  // Strategy Lab APIs
  runStrategyLab: (targetHost: string, servicePreset: string, protocol: string, customArgs: string[]) =>
    WailsApp.RunStrategyLab(targetHost, servicePreset, protocol, customArgs),
  saveDiscoveredProfile: (name: string, candidate: engine.StrategyCandidate, targetHost: string) =>
    WailsApp.SaveDiscoveredProfile(name, candidate, targetHost),

  // AutoHostlist APIs
  getAutoHostlistEntries: () => WailsApp.GetAutoHostlistEntries(),
  addAutoHostlistDomain: (domain: string, reason: string) => WailsApp.AddAutoHostlistDomain(domain, reason),
  removeAutoHostlistDomain: (domain: string) => WailsApp.RemoveAutoHostlistDomain(domain),
  clearAutoHostlist: () => WailsApp.ClearAutoHostlist(),
  promoteAutoHostlistDomain: (domain: string, targetList: string) =>
    WailsApp.PromoteAutoHostlistDomain(domain, targetList),

  // Adaptive State APIs
  getAdaptiveHostStates: () => WailsApp.GetAdaptiveHostStates(),
  resetAdaptiveHostState: () => WailsApp.ResetAdaptiveHostState(),
  verifyEngineAssets: () => WailsApp.VerifyEngineAssets(),
  generateDiagnosticReport: (res?: engine.DoctorResult) =>
    res ? WailsApp.GenerateDoctorReport(res) : WailsApp.GenerateDiagnosticReport(),
  openLogsFolder: () => WailsApp.OpenLogsFolder(),
  openCurrentLogFile: () => WailsApp.OpenCurrentLogFile(),
  checkAllUpdates: () => WailsApp.CheckAllUpdates(),
  getSystemComponentState: () => WailsApp.GetSystemComponentState(),
  rollbackEngineUpdate: () => WailsApp.RollbackEngineUpdate(),
  getAutoStartTaskInfo: () => WailsApp.GetAutoStartTaskInfo(),

  // Logs & Tools
  getLogs: () => WailsApp.GetLogs(),
  exportLogs: (content: string) => WailsApp.ExportLogs(content),
  checkDiscordRunning: () => WailsApp.CheckDiscordRunning(),
  clearDiscordCache: (closeIfRunning: boolean = false) => WailsApp.ClearDiscordCache(closeIfRunning),

  // Ping History
  getLivePing: () => WailsApp.GetLivePing(),
  savePingHistory: (latency: number, status: string) => WailsApp.SavePingHistory(latency, status),
  loadPingHistory: () => WailsApp.LoadPingHistory(),

  // LUA Scripts
  loadCustomScript: () => WailsApp.LoadCustomScript(),
  saveCustomScript: (code: string) => WailsApp.SaveCustomScript(code),
};
