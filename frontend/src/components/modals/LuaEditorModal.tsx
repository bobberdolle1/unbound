import React from 'react';
import { cn } from '../../lib/cn';
import { UISelect } from '../UISelect';
import { generateLuaCode } from '../../lib/lua';

interface LuaEditorModalProps {
  isOpen: boolean;
  onClose: () => void;
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
  onSave: () => void;
}

export const LuaEditorModal: React.FC<LuaEditorModalProps> = ({
  isOpen,
  onClose,
  luaTab,
  setLuaTab,
  luaIsAuto,
  setLuaIsAuto,
  luaFakeBlob,
  setLuaFakeBlob,
  luaPos,
  setLuaPos,
  luaFool,
  setLuaFool,
  luaTtl,
  setLuaTtl,
  luaCode,
  setLuaCode,
  onSave,
}) => {
  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-[9990] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 app-no-drag"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl flex flex-col max-h-[85vh] p-5 shadow-2xl relative"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-center border-b border-[var(--ui-border)] pb-3 mb-3">
          <div>
            <h3 className="text-sm font-semibold text-[var(--ui-text)]">Конструктор Lua-Стратегий</h3>
            <span className="text-xs text-[var(--ui-text-muted)]">Настройка параметров LUA-обхода DPI</span>
          </div>
          <button onClick={onClose} className="text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] text-sm">
            ✕
          </button>
        </div>

        <div className="flex gap-2 border-b border-[var(--ui-border)] mb-3">
          <button
            onClick={() => setLuaTab('builder')}
            className={cn(
              'px-3 py-1 text-xs font-semibold border-b-2 transition-all',
              luaTab === 'builder'
                ? 'border-[var(--ui-text)] text-[var(--ui-text)]'
                : 'border-transparent text-[var(--ui-text-muted)]'
            )}
          >
            Конструктор
          </button>
          <button
            onClick={() => {
              if (luaIsAuto) {
                setLuaCode(generateLuaCode({ fakeBlob: luaFakeBlob, pos: luaPos, fool: luaFool, ttl: luaTtl }));
              }
              setLuaTab('code');
            }}
            className={cn(
              'px-3 py-1 text-xs font-semibold border-b-2 transition-all',
              luaTab === 'code'
                ? 'border-[var(--ui-text)] text-[var(--ui-text)]'
                : 'border-transparent text-[var(--ui-text-muted)]'
            )}
          >
            Код LUA
          </button>
        </div>

        <div className="flex-1 overflow-y-auto mb-3 min-h-[240px] space-y-3">
          {luaTab === 'builder' ? (
            <div className="space-y-3 text-xs">
              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-semibold text-[var(--ui-text-muted)]">
                  Тип фейкового пакета:
                </span>
                <UISelect
                  value={
                    luaFakeBlob === 'fake_default_tls'
                      ? 'TLS ClientHello'
                      : luaFakeBlob === 'fake_default_quic'
                      ? 'QUIC Initial'
                      : luaFakeBlob === 'fake_http_request'
                      ? 'HTTP GET'
                      : luaFakeBlob
                  }
                  options={['TLS ClientHello', 'QUIC Initial', 'HTTP GET']}
                  onChange={(val: string) => {
                    setLuaIsAuto(true);
                    if (val.includes('TLS')) setLuaFakeBlob('fake_default_tls');
                    else if (val.includes('QUIC')) setLuaFakeBlob('fake_default_quic');
                    else setLuaFakeBlob('fake_http_request');
                  }}
                  up={false}
                />
              </div>

              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-semibold text-[var(--ui-text-muted)]">
                  Разделение пакета:
                </span>
                <UISelect
                  value={
                    luaPos === '1'
                      ? '1-й байт'
                      : luaPos === '2'
                      ? '2-й байт'
                      : luaPos === 'host'
                      ? 'Позиция Host/SNI'
                      : luaPos
                  }
                  options={['1-й байт', '2-й байт', 'Позиция Host/SNI']}
                  onChange={(val: string) => {
                    setLuaIsAuto(true);
                    if (val.includes('1-й')) setLuaPos('1');
                    else if (val.includes('2-й')) setLuaPos('2');
                    else setLuaPos('host');
                  }}
                  up={false}
                />
              </div>

              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-semibold text-[var(--ui-text-muted)]">
                  Метод обмана DPI (Fooling):
                </span>
                <UISelect
                  value={
                    luaFool === 'none'
                      ? 'Пассивный (None)'
                      : luaFool === 'badsum'
                      ? 'Неверная чексумма (Badsum)'
                      : luaFool === 'md5sig'
                      ? 'Подмена MD5 Signature'
                      : luaFool
                  }
                  options={['Пассивный (None)', 'Неверная чексумма (Badsum)', 'Подмена MD5 Signature']}
                  onChange={(val: string) => {
                    setLuaIsAuto(true);
                    if (val.includes('None') || val.includes('Пассивный')) setLuaFool('none');
                    else if (val.includes('Badsum') || val.includes('чексумма')) setLuaFool('badsum');
                    else setLuaFool('md5sig');
                  }}
                  up={false}
                />
              </div>

              <div className="flex flex-col gap-1">
                <div className="flex justify-between font-semibold text-[var(--ui-text)]">
                  <span>TTL ограничения:</span>
                  <span>{luaTtl === 0 ? 'Отключено' : `${luaTtl} хопов`}</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="12"
                  value={luaTtl}
                  onChange={(e) => {
                    setLuaIsAuto(true);
                    setLuaTtl(parseInt(e.target.value, 10));
                  }}
                  className="w-full accent-white"
                />
              </div>
            </div>
          ) : (
            <textarea
              value={luaCode}
              onChange={(e) => {
                setLuaIsAuto(false);
                setLuaCode(e.target.value);
              }}
              className="w-full h-full min-h-[220px] p-3 font-mono text-xs border rounded-lg bg-[var(--ui-bg)] text-[var(--ui-text)] border-[var(--ui-border)] focus:outline-none"
              placeholder="-- Введите ваш Lua код"
            />
          )}
        </div>

        <div className="flex gap-2 pt-2 border-t border-[var(--ui-border)]">
          <button onClick={onClose} className="btn-ui-secondary flex-1">
            Отмена
          </button>
          <button onClick={onSave} className="btn-ui-primary flex-1">
            Сохранить
          </button>
        </div>
      </div>
    </div>
  );
};
