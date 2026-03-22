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
				"--blob=tls_google:tls_clienthello_www_google_com.bin",
				"--blob=quic_google:quic_initial_www_google_com.bin",
				// TCP 443 - YouTube & General TLS
				"--filter-tcp=443", "--payload=tls_client_hello", 
				"--lua-desync=fake:blob=tls_google:ttl=4:repeats=11:tls_mod=rnd,dupsid,sni=www.google.com", 
				"--lua-desync=multisplit:pos=1:seqovl=681:seqovl_pattern=tls_google",
				"--new",
				// UDP 443 & Discord Discovery
				"--filter-udp=443,50000-65535", "--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:blob=quic_google:ttl=4:repeats=11",
				"--lua-desync=multisplit:pos=1",
				"--new",
				// Discord Media & Voice (General UDP)
				"--filter-udp=19294-19344,50000-65535", "--payload=unknown",
				"--lua-desync=udplen:increment=2",
				"--new",
				// Telegram MTProto & Other TCP
				"--filter-tcp=80,443,5222,5223,5228,8888", "--payload=mtproto_initial,unknown",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "Zapret 2: Telegram MTProto Fix",
			Args: []string{
				"--filter-tcp=80,443,5222,5223,5228,8888", "--payload=mtproto_initial,tls_client_hello,unknown",
				"--lua-desync=fake:blob=fake_default_tls:ttl=4",
				"--lua-desync=multisplit:pos=1",
				"--new",
				"--filter-udp=443,8888", "--payload=all",
				"--lua-desync=fake:blob=fake_default_quic:ttl=4",
				"--lua-desync=multisplit:pos=1",
			},
		},
		{
			Name: "YouTube + Discord (Universal)",
			Args: []string{
				"--filter-tcp=443", "--payload=tls_client_hello",
				"--lua-desync=fake:blob=fake_default_tls:ttl=4",
				"--lua-desync=multisplit:pos=1",
				"--new",
				"--filter-udp=443,19294-19344,50000-65535", "--payload=all",
				"--lua-desync=fake:blob=fake_default_quic:ttl=4",
			},
		},
	}
}
