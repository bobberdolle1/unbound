package engine

type AdvancedProfile struct {
	Name        string
	Description string
	Args        []string
	Category    string
	Techniques  []string
}

func GetAdvancedProfiles(luaDir string) []AdvancedProfile {
	return []AdvancedProfile{
		{
			Name:        "Unbound Ultimate (God Mode)",
			Description: "Universal multi-strategy with TLS/QUIC/UDP bypass and stateful tracking",
			Category:    "universal",
			Techniques:  []string{"multidisorder", "tls_mod", "quic_fake", "udp_bypass", "autottl", "tcp_md5", "badseq"},
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=autodetect.txt",
				"--hostlist-auto-fail-threshold=2",
				"--hostlist-auto-fail-time=30",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=multidisorder:pos=1,midsld:repeats=6",
				"--new",
				"--filter-udp=443",
				"--filter-l7=quic",
				"--hostlist-auto=autodetect.txt",
				"--payload=quic_initial",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=3:ip6_ttl=3:repeats=15",
				"--lua-desync=multisplit:pos=1,2,3",
				"--new",
				"--filter-udp=50000-65535",
				"--payload=discord_ip_discovery,stun",
				"--lua-desync=fake:blob=0x00000000000000000000000000000000:ip_ttl=4:ip6_ttl=4:repeats=3",
				"--new",
				"--filter-tcp=80,5222,5223,5228,8888",
				"--filter-l7=http",
				"--out-range=-d10",
				"--payload=http_req",
				"--lua-desync=fake:blob=fake_default_http:ip_autottl=-2,3-20:ip6_autottl=-2,3-20:tcp_md5",
				"--lua-desync=fakedsplit:pos=method+2:ip_autottl=-2,3-20:ip6_autottl=-2,3-20:tcp_md5",
			},
		},
		{
			Name:        "Stealth Mode (Minimal Footprint)",
			Description: "Low-profile bypass with minimal packet manipulation",
			Category:    "stealth",
			Techniques:  []string{"single_fake", "multisplit", "low_ttl", "tcp_md5"},
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5",
				"--lua-desync=multisplit:pos=midsld",
				"--new",
				"--filter-udp=443",
				"--filter-l7=quic",
				"--payload=quic_initial",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=6:ip6_ttl=6:repeats=8",
			},
		},
		{
			Name:        "Chaos Engineering (Max Entropy)",
			Description: "Extreme randomization and disorder for advanced DPI",
			Category:    "chaos",
			Techniques:  []string{"multidisorder", "triple_fake", "tcp_md5", "tls_mod", "badseq"},
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tcp_seq=-66000:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tcp_seq=-66000:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tcp_seq=-66000:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=multidisorder:pos=midsld:repeats=11",
				"--new",
				"--filter-udp=443",
				"--filter-l7=quic",
				"--payload=quic_initial",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=2:ip6_ttl=2:repeats=20",
				"--lua-desync=multisplit:pos=1,2,3,4",
			},
		},
		{
			Name:        "QUIC Specialist",
			Description: "Optimized for QUIC/HTTP3 traffic (YouTube, Google services)",
			Category:    "quic",
			Techniques:  []string{"quic_initial", "fake_flood", "multisplit", "high_repeats"},
			Args: []string{
				"--filter-udp=443",
				"--filter-l7=quic",
				"--hostlist-auto=autodetect.txt",
				"--payload=quic_initial",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=3:ip6_ttl=3:repeats=25",
				"--lua-desync=multisplit:pos=1,2,3",
				"--new",
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tls_mod=rnd,rndsni",
				"--lua-desync=multisplit:pos=midsld",
			},
		},
		{
			Name:        "Deep Packet Inspection Killer",
			Description: "Multi-layer attack against stateful DPI with connection tracking",
			Category:    "deep",
			Techniques:  []string{"multidisorder", "tcp_md5", "badseq", "tls_mod", "repeats"},
			Args: []string{
				"--filter-tcp=443",
				"--filter-l7=tls",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tcp_seq=-66000:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tcp_seq=-66000:tls_mod=rnd,rndsni,dupsid",
				"--lua-desync=multidisorder:pos=midsld:repeats=6",
				"--new",
				"--filter-udp=443",
				"--filter-l7=quic",
				"--hostlist-auto=autodetect.txt",
				"--payload=quic_initial",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=3:ip6_ttl=3:repeats=15",
				"--lua-desync=multisplit:pos=1,2",
				"--new",
				"--filter-udp=50000-65535",
				"--payload=discord_ip_discovery,stun",
				"--lua-desync=fake:blob=0x00000000000000000000000000000000:ip_ttl=4:ip6_ttl=4:repeats=3",
			},
		},
		{
			Name:        "HTTP/MTProto Focus",
			Description: "Specialized for Telegram and HTTP-based services",
			Category:    "http",
			Techniques:  []string{"http_req", "fakedsplit", "autottl", "tcp_md5"},
			Args: []string{
				"--filter-tcp=80,443,5222,5223,5228,8888",
				"--filter-l7=http,tls",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=http_req",
				"--lua-desync=fake:blob=fake_default_http:ip_autottl=-2,3-20:ip6_autottl=-2,3-20:tcp_md5",
				"--lua-desync=fakedsplit:pos=method+2:ip_autottl=-2,3-20:ip6_autottl=-2,3-20:tcp_md5",
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ip_ttl=8:ip6_ttl=8:tcp_md5:tls_mod=rnd,rndsni",
				"--lua-desync=multisplit:pos=midsld",
				"--new",
				"--filter-udp=443,8888",
				"--filter-l7=quic",
				"--payload=quic_initial",
				"--lua-desync=fake:blob=fake_default_quic:ip_ttl=4:ip6_ttl=4:repeats=10",
			},
		},
	}
}

func GetProfilesByCategory(category string) []AdvancedProfile {
	allProfiles := GetAdvancedProfiles("")
	filtered := make([]AdvancedProfile, 0)

	for _, p := range allProfiles {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

func GetProfileCategories() []string {
	return []string{
		"universal",
		"aggressive",
		"smart",
		"experimental",
		"stealth",
		"deep",
		"chaos",
		"handshake",
		"quic",
		"http",
		"stateful",
	}
}
