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

func GetWinDivertFilterDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "windivert.filter"), nil
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
	listsDir, _ := GetListsDir()
	windivertDir, _ := GetWinDivertFilterDir()
	
	return []Profile{
		{
			Name: "Recommended (hostfakesplit)",
			Args: []string{
				"--wf-tcp-out=80,443,2053,2083,2087,2096,8443",
				"--wf-udp-out=443,19294-19344,50000-50100",
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.discord_media.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.stun.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.wireguard.txt")),
				"--filter-udp=443",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--payload=quic_initial",
				"--lua-desync=fake:blob=quic_google:repeats=6",
				"--new",
				"--filter-l7=discord,stun",
				"--payload=stun,discord_ip_discovery",
				"--lua-desync=fake:blob=fake_default_udp:repeats=6",
				"--new",
				"--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
				"--hostlist-domains=discord.media",
				"--lua-desync=hostfakesplit:host=ozon.ru:tcp_ts=-1000:tcp_md5:repeats=4",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist-domains=googlevideo.com",
				"--out-range=-d8",
				"--lua-desync=hostfakesplit:host=ozon.ru:tcp_ts=-1000:tcp_md5:repeats=4",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "youtube.txt")),
				"--out-range=-d8",
				"--lua-desync=hostfakesplit:host=ozon.ru:tcp_ts=-1000:tcp_md5:repeats=4",
				"--new",
				"--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "discord.txt")),
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "other.txt")),
				"--out-range=-d8",
				"--lua-desync=hostfakesplit:host=ozon.ru:tcp_ts=-1000:tcp_md5:repeats=4",
				"--new",
				"--filter-tcp=80,443",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--lua-desync=hostfakesplit:host=ozon.ru:repeats=4:tcp_ts=-600000:tcp_md5",
				"--new",
				"--filter-udp=443",
				"--ipset=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-all.txt")),
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--payload=quic_initial",
				"--lua-desync=fake:blob=quic_google:repeats=6",
				"--new",
				"--filter-tcp=80,443",
				"--ipset=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-all.txt")),
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--lua-desync=hostfakesplit:host=ozon.ru:repeats=4:tcp_ts=-600000",
			},
		},
		{
			Name: "Alternative 1 (multisplit)",
			Args: []string{
				"--wf-tcp-out=80,443,2053,2083,2087,2096,8443",
				"--wf-udp-out=443,19294-19344,50000-50100",
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.discord_media.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.stun.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.wireguard.txt")),
				"--filter-udp=443",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-dns.txt")),
				"--payload=quic_initial",
				"--lua-desync=fake:blob=quic_google:repeats=6",
				"--new",
				"--filter-l7=discord,stun",
				"--payload=stun,discord_ip_discovery",
				"--lua-desync=fake:blob=fake_default_udp:repeats=6",
				"--new",
				"--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
				"--hostlist-domains=discord.media",
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=tls_google",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist-domains=googlevideo.com",
				"--out-range=-d8",
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=tls_google",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "youtube.txt")),
				"--out-range=-d8",
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=tls_google",
				"--new",
				"--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "discord.txt")),
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "other.txt")),
				"--out-range=-d10",
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=tls_google",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "list-general.txt")),
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=tls_google",
				"--new",
				"--filter-udp=443",
				"--ipset=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-all.txt")),
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-dns.txt")),
				"--payload=quic_initial",
				"--lua-desync=fake:blob=quic_google:repeats=6",
				"--new",
				"--filter-tcp=80,443",
				"--ipset=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-all.txt")),
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-dns.txt")),
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=tls_google",
			},
		},
		{
			Name: "Alternative 2 (fake TLS)",
			Args: []string{
				"--wf-tcp-out=80,443,2053,2083,2087,2096,8443",
				"--wf-udp-out=443,19294-19344,50000-50100",
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.discord_media.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.stun.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.wireguard.txt")),
				"--filter-udp=443",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--payload=quic_initial",
				"--lua-desync=fake:blob=quic_google:repeats=11",
				"--new",
				"--filter-l7=discord,stun",
				"--payload=stun,discord_ip_discovery",
				"--lua-desync=fake:blob=fake_default_udp:repeats=6",
				"--new",
				"--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
				"--hostlist-domains=discord.media",
				"--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist-domains=googlevideo.com",
				"--out-range=-d8",
				"--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
				"--new",
				"--filter-tcp=80,443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "youtube.txt")),
				"--out-range=-d8",
				"--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
				"--new",
				"--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "discord.txt")),
				"--hostlist=" + filepath.ToSlash(filepath.Join(listsDir, "other.txt")),
				"--out-range=-d10",
				"--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
				"--new",
				"--filter-tcp=80,443",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--payload=tls_client_hello",
				"--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
				"--new",
				"--filter-udp=443",
				"--ipset=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-all.txt")),
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--lua-desync=fake:blob=quic_google:repeats=11",
				"--new",
				"--filter-tcp=80,443",
				"--ipset=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-all.txt")),
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-exclude.txt")),
				"--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
				"--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
			},
		},
		{
			Name: "Alternative 3 (multisplit SNI)",
			Args: []string{
				"--wf-tcp-out=80,443-65535",
				"--wf-udp-out=80,443-65535",
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.discord_media.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.stun.txt")),
				"--wf-raw-part=@" + filepath.ToSlash(filepath.Join(windivertDir, "windivert_part.wireguard.txt")),
				"--filter-tcp=80,443-65535",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-ru.txt")),
				"--out-range=-d7",
				"--lua-desync=send:repeats=2",
				"--lua-desync=syndata:blob=stun_pat:repeats=2",
				"--lua-desync=tls_multisplit_sni:seqovl=652:seqovl_pattern=stun_pat:ip_autottl=-3,3-20:ip6_autottl=-3,3-20",
				"--new",
				"--filter-udp=80,443-65535",
				"--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-ru.txt")),
				"--out-range=-d8",
				"--payload=all",
				"--lua-desync=fake:blob=quic_google:ip_autottl=-2,3-20:ip6_autottl=-2,3-20:repeats=10:payload=all",
			},
		},
	}
}
