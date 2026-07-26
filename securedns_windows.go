//go:build windows

package main

import (
	"os/exec"
	"strings"

	"unbound/engine"
)

func setSecureDNSImpl(enabled bool) error {
	script := "Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | " +
		"Set-DnsClientServerAddress -ServerAddresses ('" +
		strings.Join(engine.SecureDNSServers, "','") + "')"
	if !enabled {
		script = "Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | " +
			"Set-DnsClientServerAddress -ResetServerAddress"
	}

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = engine.GetHiddenSysProcAttr()
	return cmd.Run()
}

func isSecureDNSEnabledImpl() bool {
	script := "Get-DnsClientServerAddress | Where-Object {$_.ServerAddresses -contains '" +
		engine.SecureDNSServers[0] + "'} | Select-Object -First 1"

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = engine.GetHiddenSysProcAttr()
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}
