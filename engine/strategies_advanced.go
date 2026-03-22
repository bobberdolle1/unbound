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
			Description: "Universal multi-strategy with TLS/QUIC/UDP bypass",
			Category:    "universal",
			Techniques:  []string{"multisplit", "tls_payload", "quic_payload", "udp_bypass"},
			Args:        []string{"--filter-tcp=80,443", "--filter-udp=443,3478,50000-65535", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=3478,50000-65535", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name:        "Aggressive Fake + BadSeq",
			Description: "Fake packets with TCP MD5 signature and bad sequence",
			Category:    "aggressive",
			Techniques:  []string{"fake", "tcp_md5", "badseq", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:tcp_md5", "--lua-desync=multisplit:pos=1,badseq", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name:        "AutoTTL + Fake",
			Description: "Auto-detect DPI distance with TTL-based fake packets",
			Category:    "smart",
			Techniques:  []string{"autottl", "fake", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:autottl", "--lua-desync=multisplit:pos=midsld", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:autottl", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name:        "BadSum + Disorder",
			Description: "Corrupt checksum with out-of-order delivery",
			Category:    "experimental",
			Techniques:  []string{"badsum", "multidisorder"},
			Args:        []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,badsum", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multidisorder:pos=2,badsum"},
		},
		{
			Name:        "SNI Randomization",
			Description: "Fake TLS with randomized SNI to confuse DPI",
			Category:    "stealth",
			Techniques:  []string{"fake", "sni_random", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_random_tls", "--lua-desync=multisplit:pos=1", "--new", "--filter-udp=443", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_random_quic", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name:        "IP Fragmentation + Split",
			Description: "IP-level fragmentation combined with TCP split",
			Category:    "deep",
			Techniques:  []string{"ipfrag", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=ipfrag1", "--lua-desync=multisplit:pos=midsld", "--new", "--filter-udp=443", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name:        "Multi-Fake Chaos",
			Description: "Multiple fake packets with different fooling methods",
			Category:    "chaos",
			Techniques:  []string{"fake", "tcp_md5", "badsum", "ttl", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:tcp_md5", "--lua-desync=fake:badsum", "--lua-desync=fake:ttl=4", "--lua-desync=multisplit:pos=1,midsld"},
		},
		{
			Name:        "SYN-ACK Split",
			Description: "Split TCP handshake between SYN and ACK",
			Category:    "handshake",
			Techniques:  []string{"synack_split", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=synack_split", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name:        "DataNoACK + Split",
			Description: "Remove ACK flag with TCP segmentation",
			Category:    "experimental",
			Techniques:  []string{"datanoack", "multisplit"},
			Args:        []string{"--filter-tcp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,datanoack"},
		},
		{
			Name:        "QUIC Aggressive",
			Description: "Aggressive QUIC bypass with fake Initial packets",
			Category:    "quic",
			Techniques:  []string{"fake", "multisplit", "udp_length"},
			Args:        []string{"--filter-udp=443", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_quic_initial", "--lua-desync=multisplit:pos=1,2,3"},
		},
		{
			Name:        "HTTP Host Manipulation",
			Description: "HTTP Host header case change and space injection",
			Category:    "http",
			Techniques:  []string{"host_case", "space_inject", "multisplit"},
			Args:        []string{"--filter-tcp=80,443", "--wf-tcp-out=80,443", "--lua-desync=multisplit:pos=1,host_case,space_inject"},
		},
		{
			Name:        "Conntrack Stateful",
			Description: "Stateful connection tracking for persistent desync",
			Category:    "stateful",
			Techniques:  []string{"conntrack", "multisplit", "fake"},
			Args:        []string{"--filter-tcp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:conntrack", "--lua-desync=multisplit:pos=1,conntrack"},
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
