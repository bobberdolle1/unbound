import { useState, useCallback } from 'react';
import { generateLuaCode, parseLuaCode } from '../lib/lua';
import { backendService } from '../services/backend';

export function useLuaEditor(
  selectedProfile: string,
  onSelectProfile: (profile: string) => void,
  onAddToast: (toast: { id: number; type: string; title: string; message: string }) => void
) {
  const [isLuaOpen, setIsLuaOpen] = useState<boolean>(false);
  const [luaTab, setLuaTab] = useState<'builder' | 'code'>('builder');
  const [luaIsAuto, setLuaIsAuto] = useState<boolean>(true);
  const [luaFakeBlob, setLuaFakeBlob] = useState<string>('fake_default_tls');
  const [luaPos, setLuaPos] = useState<string>('1');
  const [luaFool, setLuaFool] = useState<string>('none');
  const [luaTtl, setLuaTtl] = useState<number>(0);
  const [luaCode, setLuaCode] = useState<string>('');

  const openLuaEditor = useCallback(async () => {
    try {
      const code = await backendService.loadCustomScript();
      setLuaCode(code);
      const parsed = parseLuaCode(code);
      setLuaIsAuto(parsed.isAuto);
      if (parsed.isAuto) {
        setLuaFakeBlob(parsed.fakeBlob);
        setLuaPos(parsed.pos);
        setLuaFool(parsed.fool);
        setLuaTtl(parsed.ttl);
        setLuaTab('builder');
      } else {
        setLuaTab('code');
      }
      setIsLuaOpen(true);
    } catch (err) {
      onAddToast({ id: Date.now(), type: 'error', title: 'Ошибка', message: 'Не удалось загрузить LUA-стратегию.' });
    }
  }, [onAddToast]);

  const saveLuaStrategy = useCallback(async () => {
    try {
      const finalCode = luaIsAuto
        ? generateLuaCode({ fakeBlob: luaFakeBlob, pos: luaPos, fool: luaFool, ttl: luaTtl })
        : luaCode;

      await backendService.saveCustomScript(finalCode);
      setIsLuaOpen(false);
      onAddToast({ id: Date.now(), type: 'success', title: 'Успех', message: 'Стратегия LUA сохранена.' });

      if (selectedProfile !== 'Custom Profile') {
        onSelectProfile('Custom Profile');
      }
    } catch (err) {
      onAddToast({ id: Date.now(), type: 'error', title: 'Ошибка', message: 'Не удалось сохранить стратегию.' });
    }
  }, [luaIsAuto, luaFakeBlob, luaPos, luaFool, luaTtl, luaCode, selectedProfile, onSelectProfile, onAddToast]);

  return {
    state: {
      isLuaOpen,
      luaTab,
      luaIsAuto,
      luaFakeBlob,
      luaPos,
      luaFool,
      luaTtl,
      luaCode,
    },
    actions: {
      setIsLuaOpen,
      setLuaTab,
      setLuaIsAuto,
      setLuaFakeBlob,
      setLuaPos,
      setLuaFool,
      setLuaTtl,
      setLuaCode,
      openLuaEditor,
      saveLuaStrategy,
    },
  };
}
