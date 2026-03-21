package engine

import (
	"path/filepath"
)

type Profile struct {
	Name string
	Args []string
}

func GetProfiles(luaDir string) []Profile {
	absLuaLib, _ := filepath.Abs(filepath.Join(luaDir, "zapret-lib.lua"))
	absLuaAntiDpi, _ := filepath.Abs(filepath.Join(luaDir, "zapret-antidpi.lua"))

	luaLib := filepath.ToSlash(absLuaLib)
	luaAntiDpi := filepath.ToSlash(absLuaAntiDpi)

	// Updated to correct Zapret 2 syntax
	base := []string{
		"--filter-tcp=80,443", 
		"--filter-udp=50000-65535",
		"--lua=" + luaLib,
		"--lua=" + luaAntiDpi,
	}

	makeArgs := func(flags ...string) []string {
		res := make([]string, len(base))
		copy(res, base)
		return append(res, flags...)
	}

	return []Profile{
		{
			Name: "Standard Split",
			Args: makeArgs("--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new"),
		},
		{
			Name: "Fake Packets + BadSeq",
			Args: makeArgs("--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=multidisorder:pos=1,midsld", "--lua-desync=badseq", "--new"),
		},
		{
			Name: "Disorder",
			Args: makeArgs("--filter-tcp=443", "--lua-desync=split:pos=2", "--lua-desync=disorder", "--new"),
		},
		{
			Name: "Split Handshake",
			Args: makeArgs("--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=midsld", "--new"),
		},
		{
			Name: "Flowseal Legacy",
			Args: makeArgs("--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new",
				"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=6", "--new",
				"--filter-udp=50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=6"),
		},
	}
}
