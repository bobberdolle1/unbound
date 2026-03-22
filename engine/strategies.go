package engine

type Profile struct {
	Name string
	Args []string
}

func GetProfiles(luaDir string) []Profile {
	return []Profile{
		{
			Name: "Unbound Ultimate (God Mode)",
			Args: []string{"--filter-tcp=80,443", "--filter-udp=443,3478,50000-65535", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=3478,50000-65535", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "The Ultimate Combo",
			Args: []string{"--filter-tcp=80,443", "--filter-udp=443,3478,50000-65535", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=3478,50000-65535", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "YouTube QUIC Aggressive",
			Args: []string{"--filter-tcp=80,443", "--filter-udp=443", "--wf-tcp-out=80,443", "--lua-desync=multisplit:pos=1,midsld", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "Fake TLS & QUIC",
			Args: []string{"--filter-tcp=80,443", "--filter-udp=443,50000-65535", "--wf-tcp-out=80,443", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=443,50000-65535", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "Multi-Strategy Chaos",
			Args: []string{"--filter-tcp=80,443", "--filter-udp=443,3478,50000-65535", "--wf-tcp-out=80,443", "--lua-desync=multidisorder:pos=1,midsld", "--new", "--wf-udp-out=443", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=3478,50000-65535", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "Standard Split",
			Args: []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "Fake Packets + BadSeq",
			Args: []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multidisorder:pos=1,midsld"},
		},
		{
			Name: "Disorder",
			Args: []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--lua-desync=multidisorder:pos=2", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multidisorder:pos=2"},
		},
		{
			Name: "Split Handshake",
			Args: []string{"--filter-tcp=443", "--filter-udp=443", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=midsld", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=midsld"},
		},
		{
			Name: "Flowseal Legacy",
			Args: []string{"--filter-tcp=443", "--filter-udp=443,50000-65535", "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1", "--new", "--wf-udp-out=50000-65535", "--lua-desync=multisplit:pos=1"},
		},
	}
}
