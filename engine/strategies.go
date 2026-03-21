package engine

type Profile struct {
	Name string
	Args []string
}

func GetProfiles(luaDir string) []Profile {
	return []Profile{
		{
			Name: "Standard Split",
			Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1"},
		},
		{
			Name: "Fake Packets + BadSeq",
			Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=multidisorder:pos=1,midsld", "--lua-desync=badseq"},
		},
		{
			Name: "Disorder",
			Args: []string{"--filter-tcp=443", "--lua-desync=split:pos=2", "--lua-desync=disorder"},
		},
		{
			Name: "Split Handshake",
			Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=midsld"},
		},
		{
			Name: "Flowseal Legacy",
			Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new",
				"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=6", "--new",
				"--filter-udp=50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=6"},
		},
	}
}
