-- init_vars.lua
-- Инициализация переменных для стратегий Zapret 2
-- Этот файл загружается через --lua-init ПОСЛЕ zapret-lib.lua и zapret-antidpi.lua

-- ============== BASE BLOBS ==============
-- Valid TLS 1.2 ClientHello (programmatically generated)
fake_default_tls = "\x16\x03\x01\x00\xd2\x01\x00\x00\xce\x03\x03\x00\x01\x02\x03\x04" ..
    "\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14" ..
    "\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x20\x00\x01\x02\x03" ..
    "\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13" ..
    "\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x00\x20\x13\x01" ..
    "\x13\x02\x13\x03\xc0\x2c\xc0\x30\x00\x9f\xcc\xa9\xcc\xa8\xcc\xaa" ..
    "\xc0\x2b\xc0\x2f\x00\x9e\xc0\x24\xc0\x28\x00\x6b\xc0\x23\x01\x00" ..
    "\x00\x65\x00\x00\x00\x1b\x00\x19\x00\x00\x16\x77\x77\x77\x2e\x63" ..
    "\x6c\x6f\x75\x64\x66\x6c\x61\x72\x65\x2d\x64\x6e\x73\x2e\x63\x6f" ..
    "\x6d\x00\x0b\x00\x04\x03\x00\x01\x02\x00\x0a\x00\x0c\x00\x0a\x00" ..
    "\x1d\x00\x17\x00\x1e\x00\x19\x00\x18\x00\x23\x00\x00\x00\x16\x00" ..
    "\x00\x00\x17\x00\x00\x00\x0d\x00\x1e\x00\x1c\x04\x03\x05\x03\x06" ..
    "\x03\x08\x07\x08\x08\x08\x09\x08\x0a\x08\x0b\x08\x04\x08\x05\x08" ..
    "\x06\x04\x01\x05\x01\x06\x01"

-- QUIC Initial packet
quic_google = "\xc0\x00\x00\x00\x01" ..
    "\x08\x00\x00\x00\x00\x00\x00\x00\x00" ..
    "\x00" ..
    "\x41\x00" ..
    "\x06\x00\x40\x5a\x02\x00\x00\x56\x03\x03" ..
    "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f" ..
    "\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f"

-- UDP fake for STUN/Discord
fake_default_udp = "\x00\x01\x00\x00\x21\x12\xa4\x42" ..
    "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

-- STUN pattern
stun_pat = "\x00\x01\x00\x00\x21\x12\xa4\x42"

-- ============== TLS с модификацией SNI ==============
-- Используются в стратегиях как seqovl_pattern=tls_google и т.д.

-- Google SNI
tls_google = tls_mod(fake_default_tls, 'sni=www.google.com')

-- Max.ru SNI  
bin_max = tls_mod(fake_default_tls, 'sni=web.max.ru')
fake_max = tls_mod(fake_default_tls, 'rnd,sni=web.max.ru')

-- ============== Рандомизированные TLS ==============
-- Для обхода сигнатурного анализа

tls_rnd = tls_mod(fake_default_tls, 'rnd')
tls_rndsni = tls_mod(fake_default_tls, 'rnd,rndsni')
tls_rnd_google = tls_mod(fake_default_tls, 'rnd,sni=www.google.com')
tls_rnd_dupsid = tls_mod(fake_default_tls, 'rnd,dupsid')
tls_rnd_dupsid_google = tls_mod(fake_default_tls, 'rnd,dupsid,sni=www.google.com')
tls_padencap = tls_mod(fake_default_tls, 'rnd,padencap')
tls_padencap_google = tls_mod(fake_default_tls, 'rnd,padencap,sni=www.google.com')

-- ============== Специальные SNI для российских сервисов ==============
tls_vk = tls_mod(fake_default_tls, 'sni=vk.com')
tls_sber = tls_mod(fake_default_tls, 'sni=sberbank.ru')
tls_yandex = tls_mod(fake_default_tls, 'sni=yandex.ru')
tls_mail = tls_mod(fake_default_tls, 'sni=mail.ru')

-- ============== Cloudflare/CDN ==============
tls_cloudflare = tls_mod(fake_default_tls, 'sni=cloudflare.com')
tls_discord = tls_mod(fake_default_tls, 'sni=discord.com')
tls_youtube = tls_mod(fake_default_tls, 'sni=youtube.com')

-- init_vars.lua
function invert_bytes(s)
    local result = ""
    for i = 1, #s do
        result = result .. string.char(bit.bxor(string.byte(s, i), 0xFF))
    end
    return result
end

fake_inverted_tls = invert_bytes(fake_default_tls)
