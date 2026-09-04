import React from 'react';
import { ConflictModal } from './ConflictModal';
import { PrivilegesModal } from './PrivilegesModal';
import { DiagnosticsModal } from './DiagnosticsModal';
import { LuaEditorModal } from './LuaEditorModal';

interface ModalHostProps {
  // Conflict Overlay
  conflictWarning: string[];
  onIgnoreConflicts: () => void;
  onKillConflicts: () => void;

  // Privilege Overlay
  privilegeError: string;
  platform: string;
  onClosePrivilegeModal: () => void;

  // Diagnostics Modal
  isDiagOpen: boolean;
  isDiagRunning: boolean;
  diagResults: any[];
  doctorResult?: any;
  onRunMode?: (mode: string) => void;
  onCloseDiagnosticsModal: () => void;
  // LUA Editor Modal
  isLuaOpen: boolean;
  onCloseLuaModal: () => void;
  luaTab: 'builder' | 'code';
  setLuaTab: (tab: 'builder' | 'code') => void;
  luaIsAuto: boolean;
  setLuaIsAuto: (auto: boolean) => void;
  luaFakeBlob: string;
  setLuaFakeBlob: (blob: string) => void;
  luaPos: string;
  setLuaPos: (pos: string) => void;
  luaFool: string;
  setLuaFool: (fool: string) => void;
  luaTtl: number;
  setLuaTtl: (ttl: number) => void;
  luaCode: string;
  setLuaCode: (code: string) => void;
  onSaveLua: () => void;
}

export const ModalHost: React.FC<ModalHostProps> = (props) => {
  return (
    <>
      <ConflictModal
        conflicts={props.conflictWarning}
        onIgnore={props.onIgnoreConflicts}
        onKillConflicts={props.onKillConflicts}
      />

      <PrivilegesModal
        privilegeError={props.privilegeError}
        platform={props.platform}
        onClose={props.onClosePrivilegeModal}
      />

      <DiagnosticsModal
        isOpen={props.isDiagOpen}
        isRunning={props.isDiagRunning}
        results={props.diagResults}
        doctorResult={props.doctorResult}
        onRunMode={props.onRunMode}
        onClose={props.onCloseDiagnosticsModal}
      />

      <LuaEditorModal
        isOpen={props.isLuaOpen}
        onClose={props.onCloseLuaModal}
        luaTab={props.luaTab}
        setLuaTab={props.setLuaTab}
        luaIsAuto={props.luaIsAuto}
        setLuaIsAuto={props.setLuaIsAuto}
        luaFakeBlob={props.luaFakeBlob}
        setLuaFakeBlob={props.setLuaFakeBlob}
        luaPos={props.luaPos}
        setLuaPos={props.setLuaPos}
        luaFool={props.luaFool}
        setLuaFool={props.setLuaFool}
        luaTtl={props.luaTtl}
        setLuaTtl={props.setLuaTtl}
        luaCode={props.luaCode}
        setLuaCode={props.setLuaCode}
        onSave={props.onSaveLua}
      />
    </>
  );
};
