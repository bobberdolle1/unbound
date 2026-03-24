#!/usr/bin/env python3
import struct

# Build TLS ClientHello from scratch with correct lengths
hostname = b'www.cloudflare-dns.com'

# Extensions
sni_ext = struct.pack('!HH', 0x0000, len(hostname) + 5)  # Type=0 (SNI), Length
sni_ext += struct.pack('!H', len(hostname) + 3)  # Server Name List Length
sni_ext += struct.pack('!B', 0x00)  # Name Type = hostname
sni_ext += struct.pack('!H', len(hostname))  # Name Length
sni_ext += hostname

ec_point = b'\x00\x0b\x00\x04\x03\x00\x01\x02'
supported_groups = b'\x00\x0a\x00\x0c\x00\x0a\x00\x1d\x00\x17\x00\x1e\x00\x19\x00\x18'
session_ticket = b'\x00\x23\x00\x00'
encrypt_then_mac = b'\x00\x16\x00\x00'
extended_master = b'\x00\x17\x00\x00'
sig_algs = b'\x00\x0d\x00\x1e\x00\x1c\x04\x03\x05\x03\x06\x03\x08\x07\x08\x08\x08\x09\x08\x0a\x08\x0b\x08\x04\x08\x05\x08\x06\x04\x01\x05\x01\x06\x01'

extensions = sni_ext + ec_point + supported_groups + session_ticket + encrypt_then_mac + extended_master + sig_algs

# ClientHello body
random_bytes = bytes(range(32))
session_id_len = b'\x20'
session_id = bytes(range(32))
cipher_suites_len = b'\x00\x20'
cipher_suites = b'\x13\x01\x13\x02\x13\x03\xc0\x2c\xc0\x30\x00\x9f\xcc\xa9\xcc\xa8\xcc\xaa\xc0\x2b\xc0\x2f\x00\x9e\xc0\x24\xc0\x28\x00\x6b\xc0\x23'
compression = b'\x01\x00'

handshake_body = b'\x03\x03' + random_bytes + session_id_len + session_id + cipher_suites_len + cipher_suites + compression
handshake_body += struct.pack('!H', len(extensions)) + extensions

# Handshake header
handshake = b'\x01' + struct.pack('!I', len(handshake_body))[1:] + handshake_body

# TLS Record
record = b'\x16\x03\x01' + struct.pack('!H', len(handshake)) + handshake

print(f'Total: {len(record)} bytes')
print(f'Record payload: {len(handshake)} bytes')
print(f'Handshake payload: {len(handshake_body)} bytes')
print(f'Extensions: {len(extensions)} bytes')

# Generate Lua string
lua_str = 'fake_default_tls = '
for i in range(0, len(record), 16):
    chunk = record[i:i+16]
    hex_str = ''.join(f'\\x{b:02x}' for b in chunk)
    if i == 0:
        lua_str += f'"{hex_str}" ..\n'
    elif i + 16 >= len(record):
        lua_str += f'    "{hex_str}"'
    else:
        lua_str += f'    "{hex_str}" ..\n'

print('\n' + lua_str)
