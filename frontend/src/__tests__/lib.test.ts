import { describe, it, expect } from 'vitest';
import { cn } from '../lib/cn';
import { formatLog } from '../lib/format';
import { generateLuaCode, parseLuaCode } from '../lib/lua';

describe('Frontend Lib Helpers', () => {
  describe('cn (classnames merger)', () => {
    it('should merge classnames and resolve tailwind conflicts', () => {
      expect(cn('px-2 py-1', 'bg-red-500', 'px-4')).toBe('py-1 bg-red-500 px-4');
      expect(cn('text-sm', false && 'text-lg', null, undefined, 'font-bold')).toBe('text-sm font-bold');
    });
  });

  describe('formatLog', () => {
    it('should strip STDOUT and STDERR prefixes', () => {
      expect(formatLog('[STDOUT] Engine started successfully')).toBe('Engine started successfully');
      expect(formatLog('[STDERR] Warning: low memory')).toBe('Warning: low memory');
      expect(formatLog('Normal log message')).toBe('Normal log message');
    });
  });

  describe('lua strategy builder', () => {
    it('should generate valid Lua code with configuration comments', () => {
      const config = { fakeBlob: 'fake_default_quic', pos: '1,midslip', fool: 'md5sig', ttl: 5 };
      const lua = generateLuaCode(config);

      expect(lua).toContain('-- Params: fakeBlob=fake_default_quic, pos=1,midslip, fool=md5sig, ttl=5');
      expect(lua).toContain('desync.arg.pos = "1,midslip"');
      expect(lua).toContain('desync.arg.fool = "md5sig"');
      expect(lua).toContain('desync.arg.ip_ttl = 5');
      expect(lua).toContain('desync.arg.blob = "fake_default_quic"');
    });

    it('should roundtrip parse generated Lua code correctly', () => {
      const config = { fakeBlob: 'fake_http_request', pos: '2', fool: 'badseq', ttl: 12 };
      const generated = generateLuaCode(config);
      const parsed = parseLuaCode(generated);

      expect(parsed.isAuto).toBe(true);
      expect(parsed.fakeBlob).toBe('fake_http_request');
      expect(parsed.pos).toBe('2');
      expect(parsed.fool).toBe('badseq');
      expect(parsed.ttl).toBe(12);
    });

    it('should return default fallback settings when parsing non-builder custom code', () => {
      const customCode = 'function fake(ctx, desync) return 0 end';
      const parsed = parseLuaCode(customCode);

      expect(parsed.isAuto).toBe(false);
      expect(parsed.fakeBlob).toBe('fake_default_tls');
      expect(parsed.pos).toBe('1');
      expect(parsed.fool).toBe('none');
      expect(parsed.ttl).toBe(0);
    });
  });
});
