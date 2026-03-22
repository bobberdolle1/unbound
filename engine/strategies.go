package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Name string
	Args []string
}

func EnsureHostlistFiles() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	autodetectFile := filepath.Join(configDir, "autodetect.txt")
	if _, err := os.Stat(autodetectFile); os.IsNotExist(err) {
		if err := os.WriteFile(autodetectFile, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create autodetect.txt: %w", err)
		}
	}

	return nil
}

func GetProfiles(luaDir string) []Profile {
	configDir, _ := GetConfigDir()
	autodetectFile := filepath.ToSlash(filepath.Join(configDir, "autodetect.txt"))

	return []Profile{
		{
			Name: "Low-TTL Fake",
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=" + autodetectFile,
				"--hostlist-auto-fail-threshold=2",
				"--hostlist-auto-fail-time=30",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=3:ip6_ttl=3:repeats=2",
				"--lua-desync=multisplit:pos=midsld",
				"--new",
				"--filter-udp=443,50000-65535",
				"--filter-l7=quic",
				"--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=4:ip6_ttl=4",
				"--lua-desync=multisplit",
			},
		},
		{
			Name: "Multidisorder",
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=" + autodetectFile,
				"--hostlist-auto-fail-threshold=2",
				"--hostlist-auto-fail-time=30",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=multidisorder:pos=1,midsld:repeats=6",
				"--new",
				"--filter-udp=443,50000-65535",
				"--filter-l7=quic",
				"--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=3:ip6_ttl=3",
				"--lua-desync=multisplit",
			},
		},
		{
			Name: "Syndata + Split",
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=" + autodetectFile,
				"--hostlist-auto-fail-threshold=2",
				"--hostlist-auto-fail-time=30",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=syndata",
				"--lua-desync=multisplit:pos=midsld",
				"--new",
				"--filter-udp=443,50000-65535",
				"--filter-l7=quic",
				"--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=3:ip6_ttl=3",
				"--lua-desync=multisplit",
			},
		},
	}
}
