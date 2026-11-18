//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"unbound/engine"
)

const windowsSecureDNSStateFile = "securedns_windows_state.json"

type windowsNetAdapterDNS struct {
	InterfaceIndex       int      `json:"InterfaceIndex"`
	InterfaceAlias       string   `json:"InterfaceAlias"`
	InterfaceDescription string   `json:"InterfaceDescription"`
	ServerAddresses      []string `json:"ServerAddresses"`
}

type windowsSavedAdapterDNS struct {
	InterfaceIndex  int      `json:"interface_index"`
	InterfaceAlias  string   `json:"interface_alias"`
	ServerAddresses []string `json:"server_addresses,omitempty"`
}

type persistedWindowsSecureDNSState struct {
	Applied        bool                     `json:"applied"`
	AppliedServers []string                 `json:"applied_servers,omitempty"`
	Adapters       []windowsSavedAdapterDNS `json:"adapters,omitempty"`
}

const windowsQueryAdaptersScript = `$adapters = @(Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'Up' } | ForEach-Object {
	$idx = [int]$_.ifIndex
	$dns = @(Get-DnsClientServerAddress -InterfaceIndex $idx -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty ServerAddresses)
	[PSCustomObject]@{
		InterfaceIndex = $idx
		InterfaceAlias = [string]$_.Name
		InterfaceDescription = [string]$_.InterfaceDescription
		ServerAddresses = @($dns | Where-Object { $_ })
	}
})
ConvertTo-Json -InputObject $adapters -Compress -Depth 4`

var runSecureDNSPowerShell = func(script string) ([]byte, error) {
	cmd := exec.Command("powershell", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = engine.GetHiddenSysProcAttr()
	return cmd.CombinedOutput()
}

var excludedAdapterSubstrings = []string{
	"wsl",
	"hyper-v",
	"vethernet",
	"tailscale",
	"wireguard",
	"wintun",
	"clash",
	"sing-box",
	"singbox",
	"loopback",
	"openvpn",
	"vmware",
	"virtualbox",
	"npcap",
	"zerotier",
	"docker",
	"radmin",
	"hamachi",
}

// isExcludedSecureDNSAdapter filters out virtual, tunnel, VPN, proxy, and
// loopback interfaces so SecureDNS never mutates VPN or Hyper-V/WSL resolvers.
func isExcludedSecureDNSAdapter(alias, description string) bool {
	combined := strings.ToLower(strings.TrimSpace(alias) + " " + strings.TrimSpace(description))
	if combined == "" {
		return true
	}
	for _, marker := range excludedAdapterSubstrings {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	tokens := strings.FieldsFunc(combined, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, tok := range tokens {
		if tok == "tun" || tok == "tap" || tok == "tunnel" ||
			strings.HasPrefix(tok, "tun") && allDigits(tok[3:]) ||
			strings.HasPrefix(tok, "tap") && allDigits(tok[3:]) {
			return true
		}
	}
	return false
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validatedIPv4Literals(servers []string) ([]string, error) {
	out := make([]string, 0, len(servers))
	for _, raw := range servers {
		trimmed := strings.TrimSpace(raw)
		ip := net.ParseIP(trimmed)
		if ip == nil || ip.To4() == nil {
			return nil, fmt.Errorf("invalid IPv4 DNS server literal %q", raw)
		}
		out = append(out, ip.To4().String())
	}
	if len(out) == 0 {
		return nil, errors.New("no valid IPv4 DNS servers configured")
	}
	return out, nil
}

func knownSecureDNSSet(extra []string) map[string]struct{} {
	known := map[string]struct{}{
		"1.1.1.1": {},
		"1.0.0.1": {},
		"8.8.8.8": {},
		"8.8.4.4": {},
	}
	for _, s := range engine.SecureDNSServers {
		if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil {
			known[ip.String()] = struct{}{}
		}
	}
	for _, s := range extra {
		if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil {
			known[ip.String()] = struct{}{}
		}
	}
	return known
}

func adapterHasKnownSecureDNS(addresses []string, known map[string]struct{}) bool {
	for _, raw := range addresses {
		ip := net.ParseIP(strings.TrimSpace(raw))
		if ip == nil {
			continue
		}
		if _, ok := known[ip.String()]; ok {
			return true
		}
	}
	return false
}

func parseWindowsAdaptersJSON(raw []byte) ([]windowsNetAdapterDNS, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var list []windowsNetAdapterDNS
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	var single windowsNetAdapterDNS
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return nil, err
	}
	return []windowsNetAdapterDNS{single}, nil
}

func queryEligibleWindowsAdapters() ([]windowsNetAdapterDNS, error) {
	out, err := runSecureDNSPowerShell(windowsQueryAdaptersScript)
	if err != nil {
		return nil, fmt.Errorf("query network adapters: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	all, err := parseWindowsAdaptersJSON(out)
	if err != nil {
		return nil, fmt.Errorf("parse network adapters JSON: %w", err)
	}
	eligible := make([]windowsNetAdapterDNS, 0, len(all))
	for _, adapter := range all {
		if adapter.InterfaceIndex <= 0 {
			continue
		}
		if isExcludedSecureDNSAdapter(adapter.InterfaceAlias, adapter.InterfaceDescription) {
			continue
		}
		eligible = append(eligible, adapter)
	}
	return eligible, nil
}

func getWindowsSecureDNSStatePath() (string, error) {
	dir, err := engine.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, windowsSecureDNSStateFile), nil
}

func loadWindowsSecureDNSState() (persistedWindowsSecureDNSState, bool) {
	path, err := getWindowsSecureDNSStatePath()
	if err != nil {
		return persistedWindowsSecureDNSState{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return persistedWindowsSecureDNSState{}, false
	}
	var state persistedWindowsSecureDNSState
	if err := json.Unmarshal(data, &state); err != nil || !state.Applied {
		return persistedWindowsSecureDNSState{}, false
	}
	return state, true
}

func saveWindowsSecureDNSState(state persistedWindowsSecureDNSState) {
	path, err := getWindowsSecureDNSStatePath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".securedns-state-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpName, path)
}

func clearWindowsSecureDNSState() {
	path, err := getWindowsSecureDNSStatePath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

func formatPowerShellIPArray(ips []string) string {
	quoted := make([]string, len(ips))
	for i, ip := range ips {
		quoted[i] = "'" + strings.ReplaceAll(ip, "'", "''") + "'"
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}

func setSecureDNSImpl(enabled bool) error {
	eligible, err := queryEligibleWindowsAdapters()
	if err != nil {
		return err
	}
	prevState, hasPrevState := loadWindowsSecureDNSState()

	if enabled {
		servers, err := validatedIPv4Literals(engine.SecureDNSServers)
		if err != nil {
			return err
		}
		if len(eligible) == 0 {
			return errors.New("активные физические сетевые адаптеры не найдены")
		}
		known := knownSecureDNSSet(servers)
		prevByIndex := make(map[int]windowsSavedAdapterDNS, len(prevState.Adapters))
		if hasPrevState {
			for _, rec := range prevState.Adapters {
				prevByIndex[rec.InterfaceIndex] = rec
			}
		}

		savedAdapters := make([]windowsSavedAdapterDNS, 0, len(eligible))
		statements := make([]string, 0, len(eligible))
		serverExpr := formatPowerShellIPArray(servers)
		for _, adapter := range eligible {
			if existing, ok := prevByIndex[adapter.InterfaceIndex]; ok {
				savedAdapters = append(savedAdapters, existing)
			} else {
				var prior []string
				if !adapterHasKnownSecureDNS(adapter.ServerAddresses, known) {
					if validated, vErr := validatedIPv4Literals(adapter.ServerAddresses); vErr == nil {
						prior = validated
					}
				}
				savedAdapters = append(savedAdapters, windowsSavedAdapterDNS{
					InterfaceIndex:  adapter.InterfaceIndex,
					InterfaceAlias:  adapter.InterfaceAlias,
					ServerAddresses: prior,
				})
			}
			statements = append(statements, fmt.Sprintf("Set-DnsClientServerAddress -InterfaceIndex %d -ServerAddresses %s -ErrorAction Stop", adapter.InterfaceIndex, serverExpr))
		}

		if out, err := runSecureDNSPowerShell(strings.Join(statements, "; ")); err != nil {
			return fmt.Errorf("apply SecureDNS: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		saveWindowsSecureDNSState(persistedWindowsSecureDNSState{
			Applied:        true,
			AppliedServers: servers,
			Adapters:       savedAdapters,
		})
		return nil
	}

	// Disabling SecureDNS: never clobber custom user DNS unless SecureDNS was
	// actually applied on the eligible physical adapter (or recorded in state).
	known := knownSecureDNSSet(prevState.AppliedServers)
	prevByIndex := make(map[int]windowsSavedAdapterDNS, len(prevState.Adapters))
	if hasPrevState {
		for _, rec := range prevState.Adapters {
			prevByIndex[rec.InterfaceIndex] = rec
		}
	}

	statements := make([]string, 0, len(eligible))
	for _, adapter := range eligible {
		saved, wasTracked := prevByIndex[adapter.InterfaceIndex]
		hasAppliedDNS := adapterHasKnownSecureDNS(adapter.ServerAddresses, known)
		if !wasTracked && !hasAppliedDNS {
			continue
		}
		if wasTracked && len(saved.ServerAddresses) > 0 {
			if restoredIPs, vErr := validatedIPv4Literals(saved.ServerAddresses); vErr == nil {
				statements = append(statements, fmt.Sprintf("Set-DnsClientServerAddress -InterfaceIndex %d -ServerAddresses %s -ErrorAction Stop", adapter.InterfaceIndex, formatPowerShellIPArray(restoredIPs)))
				continue
			}
		}
		statements = append(statements, fmt.Sprintf("Set-DnsClientServerAddress -InterfaceIndex %d -ResetServerAddress -ErrorAction Stop", adapter.InterfaceIndex))
	}

	if len(statements) == 0 {
		clearWindowsSecureDNSState()
		return nil
	}

	if out, err := runSecureDNSPowerShell(strings.Join(statements, "; ")); err != nil {
		return fmt.Errorf("restore DNS: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	clearWindowsSecureDNSState()
	return nil
}

func isSecureDNSEnabledImpl() bool {
	eligible, err := queryEligibleWindowsAdapters()
	if err != nil || len(eligible) == 0 {
		return false
	}
	servers := engine.SecureDNSServers
	if len(servers) == 0 {
		return false
	}
	primary := strings.TrimSpace(servers[0])
	for _, adapter := range eligible {
		for _, addr := range adapter.ServerAddresses {
			if strings.TrimSpace(addr) == primary {
				return true
			}
		}
	}
	return false
}
