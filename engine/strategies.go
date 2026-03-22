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
				// TCP 443 (HTTPS/TLS) - Double Fake + Split
				"--filter-tcp=443",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq",
				"--lua-desync=split2:pos=midsld:badseq",
				"--new",
				// UDP 443 (QUIC) - Aggressive fake flood
				"--filter-udp=443",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4:repeats=11",
				"--lua-desync=multisplit:pos=1",
				"--new",
				// UDP 50000-65535 (Discord Voice/RTC)
				"--filter-udp=50000-65535",
				"--payload=discord_ip_discovery,stun,unknown",
				"--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2",
				"--lua-desync=udplen:increment=2",
				"--new",
				// TCP 80,5222,5223,5228,8888 (HTTP/MTProto/Telegram)
				"--filter-tcp=80,5222,5223,5228,8888",
				"--out-range=-d10",
				"--payload=http_req,mtproto_initial",
				"--lua-desync=fake:ttl=4:tcp_md5",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "Zapret 2: Telegram MTProto Fix",
			Args: []string{
				// TCP (TLS + HTTP + MTProto)
				"--filter-tcp=80,443,5222,5223,5228,8888",
				"--out-range=-d10",
				"--payload=tls_client_hello,http_req,mtproto_initial",
				"--lua-desync=fake:ttl=4:tcp_md5",
				"--lua-desync=split2:pos=1:badseq",
				"--new",
				// UDP (QUIC + Telegram voice)
				"--filter-udp=443,8888",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4:repeats=6",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "YouTube + Discord (Universal)",
			Args: []string{
				// TCP 443 (YouTube HTTPS)
				"--filter-tcp=443",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=4:tcp_md5",
				"--lua-desync=split2:pos=midsld:badseq",
				"--new",
				// UDP (QUIC + Discord)
				"--filter-udp=443,50000-65535",
				"--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:ttl=4:repeats=6",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "Lightweight (Low CPU)",
			Args: []string{
				// Minimal overhead - single fake + simple split
				"--filter-tcp=443",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=4",
				"--lua-desync=split2:pos=2",
				"--new",
				"--filter-udp=443",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4",
			},
		},
		{
			Name: "Aggressive (Deep Inspection Bypass)",
			Args: []string{
				// Triple fake + disorder for aggressive DPI
				"--filter-tcp=443",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq",
				"--lua-desync=multidisorder:pos=1,midsld",
				"--new",
				"--filter-udp=443",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4:repeats=20",
				"--lua-desync=multisplit:pos=1",
			},
		},
	}
}
