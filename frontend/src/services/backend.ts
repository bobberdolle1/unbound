import * as WailsApp from '../../wailsjs/go/main/App';
import { engine } from '../../wailsjs/go/models';

export const backendService = {
  // Engine APIs
  getEngineNames: () => WailsApp.GetEngineNames(),
  getProfiles: (engineName: string) => WailsApp.GetProfiles(engineName),
  startEngine: (engineName: string, profileName: string) => WailsApp.StartEngine(engineName, profileName),
  stopEngine: () => WailsApp.StopEngine(),
  autoTune: () => WailsApp.AutoTune(),
  cancelAutoTune: () => WailsApp.CancelAutoTune(),
  killWinws2: () => WailsApp.KillWinws2(),

  // Settings & System
  getSettings: () => WailsApp.GetSettings(),
  saveSettings: (settings: engine.Settings) => WailsApp.SaveSettings(settings),
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
  startDoctor: (mode: string) => WailsApp.StartDoctor(mode),
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
  promoteAutoHostlistDomain: (domain: string, targetList: string) => WailsApp.PromoteAutoHostlistDomain(domain, targetList),

  // Adaptive State APIs
  getAdaptiveHostStates: () => WailsApp.GetAdaptiveHostStates(),
  resetAdaptiveHostState: () => WailsApp.ResetAdaptiveHostState(),
  verifyEngineAssets: () => WailsApp.VerifyEngineAssets(),
  generateDiagnosticReport: (res?: engine.DoctorResult) => res ? WailsApp.GenerateDoctorReport(res) : WailsApp.GenerateDiagnosticReport(),
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
