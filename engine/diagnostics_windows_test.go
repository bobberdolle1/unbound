//go:build windows

package engine

import (
	"testing"
)

func TestParseNetshTimestamps(t *testing.T) {
	// 1. English netsh output with RFC 1323 allowed, while other lines have enabled
	englishAllowed := `
TCP Global Parameters
----------------------------------------------
Receive-Side Scaling State          : enabled
Receive Window Auto-Tuning Level    : normal
Add-On Congestion Control Provider  : default
ECN Capability                      : disabled
RFC 1323 Timestamps                 : allowed
Initial RTO                         : 1000
Receive Segment Coalescing State    : enabled
Fast Open                           : enabled
`
	enabled, matched := parseNetshTimestamps(englishAllowed)
	if !matched {
		t.Fatal("Expected matched=true for English allowed output")
	}
	if !enabled {
		t.Fatal("Expected enabled=true for RFC 1323 allowed")
	}

	// 2. English netsh output with RFC 1323 disabled
	englishDisabled := `
TCP Global Parameters
----------------------------------------------
Receive-Side Scaling State          : enabled
Receive Window Auto-Tuning Level    : normal
Add-On Congestion Control Provider  : default
ECN Capability                      : disabled
RFC 1323 Timestamps                 : disabled
Initial RTO                         : 1000
`
	enabled, matched = parseNetshTimestamps(englishDisabled)
	if !matched {
		t.Fatal("Expected matched=true for English disabled output")
	}
	if enabled {
		t.Fatal("Expected enabled=false for RFC 1323 disabled")
	}

	// 3. Russian netsh output with RFC 1323 allowed
	russianAllowed := `
Глобальные параметры TCP
----------------------------------------------
Состояние масштабирования на стороне приема : enabled
Уровень автонастройки окна приема           : normal
Поставщик надстройки управления перегрузкой : default
Возможность ECN                             : disabled
Временные метки RFC 1323                    : разрешены
Начальный RTO                               : 1000
`
	enabled, matched = parseNetshTimestamps(russianAllowed)
	if !matched {
		t.Fatal("Expected matched=true for Russian allowed output")
	}
	if !enabled {
		t.Fatal("Expected enabled=true for RFC 1323 разрешены")
	}

	// 4. Russian netsh output with RFC 1323 disabled
	russianDisabled := `
Глобальные параметры TCP
----------------------------------------------
Состояние масштабирования на стороне приема : enabled
Уровень автонастройки окна приема           : normal
Поставщик надстройки управления перегрузкой : default
Возможность ECN                             : disabled
Временные метки RFC 1323                    : отключено
Начальный RTO                               : 1000
`
	enabled, matched = parseNetshTimestamps(russianDisabled)
	if !matched {
		t.Fatal("Expected matched=true for Russian disabled output")
	}
	if enabled {
		t.Fatal("Expected enabled=false for RFC 1323 отключено")
	}

	// 5. Unrelated output without 1323 line
	unrelated := `
Some other output
Without any standard mentions
`
	enabled, matched = parseNetshTimestamps(unrelated)
	if matched {
		t.Fatal("Expected matched=false when 1323 line is absent")
	}
	if enabled {
		t.Fatal("Expected enabled=false for unmatched output")
	}
}
