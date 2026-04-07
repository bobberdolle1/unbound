local m = Map("unbound", translate("Unbound-WRT"),
    translate("Network-level DPI/censorship bypass. Protects all LAN clients transparently."))

-- General Settings
local s = m:section(TypedSection, "general", translate("General Settings"))
s.anonymous = true

local enabled = s:option(Flag, "enabled", translate("Enable"),
    translate("Master switch to activate or deactivate the DPI bypass engine."))
enabled.default = 0
enabled.rmempty = false

local strategy = s:option(ListValue, "strategy", translate("Bypass Strategy"),
    translate("Select the packet mangling strategy used by the DPI bypass engine."))
strategy:value("multidisorder", translate("Multidisorder"),
    translate("Disorders packet segments; effective against many DPI systems."))
strategy:value("split-tls", translate("Split TLS"),
    translate("Splits TLS ClientHello; bypasses TLS-based SNI inspection."))
strategy:value("fake-ping", translate("Fake Ping"),
    translate("Injects fake low-TTL packets; confuses DPI state tracking."))
strategy:value("disorder-fake", translate("Disorder + Fake"),
    translate("Combines disorder with fake packet injection for maximum evasion."))
strategy.default = "multidisorder"
strategy.rmempty = false

-- Exclusions
local ex = m:section(TypedSection, "general", translate("Exclusions"),
    translate("Specify domains or IP addresses that should bypass the DPI engine."))
ex.anonymous = true

local domains = ex:option(TextValue, "exclude_domains", translate("Exclude Domains"),
    translate("Enter domains (one per line) that should bypass DPI."))
domains.rows = 5
domains.monospace = true
domains.optional = true

local ips = ex:option(TextValue, "exclude_ips", translate("Exclude IPs"),
    translate("Enter IP addresses or CIDR ranges (one per line) to bypass the DPI engine."))
ips.rows = 5
ips.monospace = true
ips.optional = true

function m.on_commit(self, map)
    luci.sys.call("/etc/init.d/unbound restart >/dev/null 2>&1 &")
end

return m
