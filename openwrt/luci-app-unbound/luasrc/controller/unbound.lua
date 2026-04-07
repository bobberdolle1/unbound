module("luci.controller.unbound", package.seeall)

function index()
    entry(
        {"admin", "services", "unbound"},
        cbi("unbound/unbound"),
        _("Unbound-WRT"),
        60
    ).dependent = true

    entry(
        {"admin", "services", "unbound", "status"},
        call("act_status")
    ).leaf = true
end

function act_status()
    local e = {}
    e.running = luci.sys.call("/usr/bin/nfqws --version >/dev/null 2>&1") == 0
    e.enabled = luci.sys.init.enabled("unbound")
    luci.http.prepare_content("application/json")
    luci.http.write_json(e)
end
