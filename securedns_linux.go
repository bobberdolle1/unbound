//go:build linux

package main

import (
	"fmt"
	"os/exec"
	"strings"

	"unbound/engine"
)

// Linux has no single DNS configuration point, so try the managers in the order
// a desktop is likely to use them. Rewriting /etc/resolv.conf directly is
// deliberately *not* attempted: on a systemd-resolved or NetworkManager system
// that file is a symlink regenerated behind our back, so editing it either gets
// silently reverted or permanently breaks name resolution.

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// activeLinkNames returns the names of up, non-loopback interfaces.
func activeLinkNames() ([]string, error) {
	out, err := exec.Command("ip", "-o", "link", "show", "up").Output()
	if err != nil {
		return nil, fmt.Errorf("не удалось получить список интерфейсов: %w", err)
	}

	var links []string
	for _, line := range strings.Split(string(out), "\n") {
		// Format: "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 ..."
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		// Strip the "@if12" suffix that veth/vlan links carry.
		if at := strings.IndexByte(name, '@'); at >= 0 {
			name = name[:at]
		}
		if name == "" || name == "lo" {
			continue
		}
		links = append(links, name)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("активные сетевые интерфейсы не найдены")
	}
	return links, nil
}

func setSecureDNSResolvectl(enabled bool) error {
	links, err := activeLinkNames()
	if err != nil {
		return err
	}

	var failures []string
	applied := 0
	for _, link := range links {
		args := []string{"dns", link}
		if enabled {
			args = append(args, engine.SecureDNSServers...)
		}
		// `resolvectl dns <link>` with no servers reverts the link to the
		// values supplied by DHCP.
		if out, err := exec.Command("resolvectl", args...).CombinedOutput(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", link, strings.TrimSpace(string(out))))
			continue
		}
		applied++
	}

	if applied == 0 {
		return fmt.Errorf("resolvectl не смог настроить ни один интерфейс: %s", strings.Join(failures, "; "))
	}
	return nil
}

func setSecureDNSNmcli(enabled bool) error {
	out, err := exec.Command("nmcli", "-t", "-f", "NAME", "connection", "show", "--active").Output()
	if err != nil {
		return fmt.Errorf("не удалось получить активные подключения NetworkManager: %w", err)
	}

	servers := strings.Join(engine.SecureDNSServers, ",")
	applied := 0
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name == "" {
			continue
		}
		if err := exec.Command("nmcli", "connection", "modify", name, "ipv4.dns", servers).Run(); err != nil {
			continue
		}
		// ignore-auto-dns stops DHCP from re-adding the ISP resolver alongside
		// ours, which would let queries leak to the resolver we are avoiding.
		ignore := "yes"
		if !enabled {
			ignore = "no"
			_ = exec.Command("nmcli", "connection", "modify", name, "ipv4.dns", "").Run()
		}
		_ = exec.Command("nmcli", "connection", "modify", name, "ipv4.ignore-auto-dns", ignore).Run()
		_ = exec.Command("nmcli", "connection", "up", name).Run()
		applied++
	}

	if applied == 0 {
		return fmt.Errorf("не найдено активных подключений NetworkManager")
	}
	return nil
}

func setSecureDNSImpl(enabled bool) error {
	switch {
	case hasCommand("resolvectl"):
		return setSecureDNSResolvectl(enabled)
	case hasCommand("nmcli"):
		return setSecureDNSNmcli(enabled)
	default:
		return fmt.Errorf(
			"не найден поддерживаемый менеджер DNS (systemd-resolved или NetworkManager); "+
				"укажите DNS %s вручную", strings.Join(engine.SecureDNSServers, ", "))
	}
}

func isSecureDNSEnabledImpl() bool {
	primary := engine.SecureDNSServers[0]

	if hasCommand("resolvectl") {
		if out, err := exec.Command("resolvectl", "status").Output(); err == nil {
			return strings.Contains(string(out), primary)
		}
	}
	if hasCommand("nmcli") {
		if out, err := exec.Command("nmcli", "-t", "-f", "IP4.DNS", "device", "show").Output(); err == nil {
			return strings.Contains(string(out), primary)
		}
	}
	return false
}
