//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"

	"unbound/engine"
)

// activeNetworkServices returns the enabled network services, in the order
// macOS prefers them. Entries prefixed with "*" are disabled and are skipped;
// the first line of the output is a human-readable header, not a service.
func activeNetworkServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, fmt.Errorf("не удалось получить список сетевых служб: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var services []string
	for i, line := range lines {
		if i == 0 || strings.HasPrefix(line, "*") || strings.TrimSpace(line) == "" {
			continue
		}
		services = append(services, strings.TrimSpace(line))
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("активные сетевые службы не найдены")
	}
	return services, nil
}

func setSecureDNSImpl(enabled bool) error {
	services, err := activeNetworkServices()
	if err != nil {
		return err
	}

	// "Empty" is networksetup's sentinel for "revert to the DHCP-supplied
	// resolvers"; passing no argument at all is a syntax error.
	servers := []string{"Empty"}
	if enabled {
		servers = engine.SecureDNSServers
	}

	var failures []string
	applied := 0
	for _, service := range services {
		args := append([]string{"-setdnsservers", service}, servers...)
		if out, err := exec.Command("networksetup", args...).CombinedOutput(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", service, strings.TrimSpace(string(out))))
			continue
		}
		applied++
	}

	if applied == 0 {
		return fmt.Errorf("не удалось настроить DNS ни для одной службы: %s", strings.Join(failures, "; "))
	}
	return nil
}

func isSecureDNSEnabledImpl() bool {
	services, err := activeNetworkServices()
	if err != nil {
		return false
	}

	primary := engine.SecureDNSServers[0]
	for _, service := range services {
		out, err := exec.Command("networksetup", "-getdnsservers", service).Output()
		if err == nil && strings.Contains(string(out), primary) {
			return true
		}
	}
	return false
}
