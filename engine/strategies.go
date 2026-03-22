package engine

type Profile struct {
	Name string
	Args []string
}

func GetProfiles(luaDir string) []Profile {
	return []Profile{
		{
			Name: "Unbound Ultimate (God Mode)",
			Args: []string{
				// TCP 443 (HTTPS/TLS)
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--hostlist-auto-fail-threshold=1",
				"--hostlist-auto-fail-time=10",
				"--hostlist-auto-retrans-threshold=3",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=1:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=2:badseq",
				"--new",
				// UDP (QUIC)
				"--filter-udp=443,50000-65535",
				"--hostlist-auto=autodetect.txt",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=1:repeats=6:blob=fake_default_quic",
			},
		},
		{
			Name: "Zapret 2: Telegram MTProto Fix",
			Args: []string{
				// TCP (TLS + HTTP + MTProto) - EXPLICIT PORTS
				"--filter-tcp=80,443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello,http_req,mtproto_initial",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=midsld:badseq",
				"--new",
				// UDP (QUIC + Telegram voice)
				"--filter-udp=443,50000-65535",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4:repeats=6:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "YouTube + Discord (Universal)",
			Args: []string{
				// TCP 443 (YouTube HTTPS) - EXPLICIT PORT
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=4:tcp_md5:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=midsld:badseq",
				"--new",
				// UDP (QUIC + Discord)
				"--filter-udp=443,50000-65535",
				"--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:ttl=4:repeats=6:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "Lightweight (Low CPU)",
			Args: []string{
				// Minimal overhead - single fake + simple split
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=5:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=2",
				"--new",
				"--filter-udp=443,50000-65535",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=5:blob=fake_default_quic",
			},
		},
		{
			Name: "Aggressive (Deep Inspection Bypass)",
			Args: []string{
				// Triple fake + disorder for aggressive DPI
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=3:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=fake:ttl=5:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=multidisorder:pos=1,midsld",
				"--new",
				"--filter-udp=443,50000-65535",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4:repeats=20:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1",
			},
		},
	}
}
