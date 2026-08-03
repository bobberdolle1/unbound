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
  verifyEngineAssets: () => WailsApp.VerifyEngineAssets(),
  generateDiagnosticReport: () => WailsApp.GenerateDiagnosticReport(),

  // Logs & Tools
  getLogs: () => WailsApp.GetLogs(),
  exportLogs: (content: string) => WailsApp.ExportLogs(content),
  clearDiscordCache: () => WailsApp.ClearDiscordCache(),

  // Ping History
  getLivePing: () => WailsApp.GetLivePing(),
  savePingHistory: (latency: number, status: string) => WailsApp.SavePingHistory(latency, status),
  loadPingHistory: () => WailsApp.LoadPingHistory(),

  // LUA Scripts
  loadCustomScript: () => WailsApp.LoadCustomScript(),
  saveCustomScript: (code: string) => WailsApp.SaveCustomScript(code),
};
