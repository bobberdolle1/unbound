//go:build linux

package engine

import (
	"os"
	"os/exec"
	"strings"

	"unbound/engine/providers"
)

// EnableTCPTimestamps turns on RFC 1323 TCP timestamps.
//
// Several desync strategies rely on the md5sig/badseq fooling modes, which need
// timestamps present in the outgoing SYN. This used to `return nil` without
// doing anything, so the corresponding checkbox in the UI did nothing.
func EnableTCPTimestamps() error {
	return exec.Command("sysctl", "-w", "net.ipv4.tcp_timestamps=1").Run()
}


// RunDiagnostics inspects the parts of the system the Linux engine depends on.
// It previously returned a single hardcoded "Linux diagnostics active - OK",
// which told the user nothing and hid every real misconfiguration.
func RunDiagnostics() []DiagnosticResult {
	return []DiagnosticResult{
		checkRootPrivileges(),
		checkEngineBinary(),
		checkNfqueueSupport(),
		checkFirewallTooling(),
		checkTCPTimestampsLinux(),
		checkConflictingProcessesLinux(),
	}
}

func checkRootPrivileges() DiagnosticResult {
	if os.Geteuid() == 0 {
		return DiagnosticResult{"Privileges", "OK", "Запущено от root.", false}
	}
	return DiagnosticResult{"Privileges", "Error", "Нужны права root — запустите через sudo.", true}
}

func checkEngineBinary() DiagnosticResult {
	binDir := ""
	if assets, err := ExtractAssets(); err == nil {
		binDir = assets.BinDir
	}

	path, err := providers.ResolveEngineBinary(providers.LinuxEngineBinary, binDir)
	if err != nil {
		return DiagnosticResult{
			"Engine", "Error",
			"nfqws не найден. Установите пакет zapret или положите nfqws рядом с приложением.",
			true,
		}
	}
	return DiagnosticResult{"Engine", "OK", "nfqws: " + path, false}
}

// checkNfqueueSupport verifies the kernel can hand packets to userspace.
// Without nfnetlink_queue every profile fails the moment nfqws opens the queue.
func checkNfqueueSupport() DiagnosticResult {
	if _, err := os.Stat("/proc/net/netfilter/nfnetlink_queue"); err == nil {
		return DiagnosticResult{"NFQUEUE", "OK", "Модуль nfnetlink_queue загружен.", false}
	}

	// Not loaded yet is fine as long as it is available: the kernel autoloads
	// it when the first NFQUEUE rule is installed.
	if out, err := exec.Command("modinfo", "nfnetlink_queue").Output(); err == nil && len(out) > 0 {
		return DiagnosticResult{"NFQUEUE", "OK", "Модуль nfnetlink_queue доступен (загрузится при старте).", false}
	}

	return DiagnosticResult{
		"NFQUEUE", "Error",
		"Ядро без поддержки NFQUEUE (nfnetlink_queue). Обход работать не будет.",
		true,
	}
}

func checkFirewallTooling() DiagnosticResult {
	var found []string
	for _, tool := range []string{"iptables", "ip6tables", "nft"} {
		if _, err := exec.LookPath(tool); err == nil {
			found = append(found, tool)
		}
	}
	if len(found) == 0 {
		return DiagnosticResult{
			"Firewall", "Error",
			"Не найдены ни iptables, ни nft — правила установить нечем.",
			true,
		}
	}
	return DiagnosticResult{"Firewall", "OK", "Доступно: " + strings.Join(found, ", "), false}
}

func checkTCPTimestampsLinux() DiagnosticResult {
	data, err := os.ReadFile("/proc/sys/net/ipv4/tcp_timestamps")
	if err != nil {
		return DiagnosticResult{"TCP Stack", "Warning", "Не удалось прочитать tcp_timestamps.", true}
	}
	if strings.TrimSpace(string(data)) == "0" {
		return DiagnosticResult{"TCP Stack", "Warning", "TCP timestamps отключены — часть стратегий не сработает.", true}
	}
	return DiagnosticResult{"TCP Stack", "OK", "TCP timestamps включены.", false}
}

func checkConflictingProcessesLinux() DiagnosticResult {
	var found []string
	for _, proc := range []string{"nfqws", "tpws", "goodbyedpi", "ciadpi", "byedpi"} {
		out, err := exec.Command("pgrep", "-x", proc).Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			found = append(found, proc)
		}
	}
	if len(found) > 0 {
		return DiagnosticResult{"Conflicts", "Warning", "Запущены: " + strings.Join(found, ", "), true}
	}
	return DiagnosticResult{"Conflicts", "OK", "Конфликтов нет.", false}
}
