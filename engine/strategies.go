package engine

type Profile struct {
	Name string
	Args []string
}

func GetProfiles(luaDir string) []Profile {
	return []Profile{
		{
			Name: "Standard Split",
			Args: []string{"--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "Fake Packets + BadSeq",
			Args: []string{"--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld"},
		},
		{
			Name: "Disorder",
			Args: []string{"--wf-tcp-out=443", "--lua-desync=multidisorder:pos=2"},
		},
		{
			Name: "Split Handshake",
			Args: []string{"--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=midsld"},
		},
		{
			Name: "Flowseal Legacy",
			Args: []string{"--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1", "--new",
				"--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=multisplit:pos=1", "--new",
				"--wf-udp-out=50000-65535", "--lua-desync=multisplit:pos=1"},
		},
		{
			Name: "AmneziaWG (VPN Mode)",
			Args: []string{},
		},
		{
			Name: "Xray VLESS/Reality",
			Args: []string{},
		},
	}
}
