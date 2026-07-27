/**
 * Unbound Core — Magisk / KernelSU WebUI Frontend Controller
 */
document.addEventListener('DOMContentLoaded', () => {
    // UI Elements
    const statusBadge = document.getElementById('statusBadge');
    const statusText = document.getElementById('statusText');
    const statusDot = statusBadge.querySelector('.status-dot');
    const engineStatus = document.getElementById('engineStatus');
    const modeStatus = document.getElementById('modeStatus');
    const ttlStatus = document.getElementById('ttlStatus');
    const portsStatus = document.getElementById('portsStatus');
    const logOutput = document.getElementById('logOutput');

    // Controls
    const btnStart = document.getElementById('btnStart');
    const btnRestart = document.getElementById('btnRestart');
    const btnStop = document.getElementById('btnStop');
    const btnSaveConfig = document.getElementById('btnSaveConfig');
    const btnRefreshLogs = document.getElementById('btnRefreshLogs');

    // Form Fields
    const fixTtl = document.getElementById('fixTtl');
    const ttlValue = document.getElementById('ttlValue');
    const enableHotspot = document.getElementById('enableHotspot');
    const iptablesMode = document.getElementById('iptablesMode');
    const filterPorts = document.getElementById('filterPorts');
    const nfqueueNum = document.getElementById('nfqueueNum');
    const enableIpv6 = document.getElementById('enableIpv6');
    const excludedUids = document.getElementById('excludedUids');

    // System Executive Command API (KernelSU / Magisk Exec Bridge)
    async function execCommand(cmd) {
        if (window.ksu && window.ksu.exec) {
            return new Promise((resolve) => {
                window.ksu.exec(cmd, '{}', (result) => {
                    resolve(result);
                });
            });
        } else if (window.exec) {
            return window.exec(cmd);
        } else {
            console.log('[WebUI Simulation] Command executed:', cmd);
            return { errno: 0, stdout: 'ACTIVE (PID: 1234)', stderr: '' };
        }
    }

    // Check Module Status
    async function updateStatus() {
        try {
            const res = await execCommand('/data/adb/modules/unbound-core/service.sh status');
            const output = (res.stdout || '').trim();

            if (output.includes('ACTIVE')) {
                statusDot.className = 'status-dot active pulse';
                statusText.textContent = 'АКТИВЕН';
                engineStatus.textContent = 'nfqws Daemon (Running)';
                engineStatus.className = 'value active-text';
            } else {
                statusDot.className = 'status-dot';
                statusText.textContent = 'ОСТАНОВЛЕН';
                engineStatus.textContent = 'Остановлен';
                engineStatus.className = 'value';
            }
        } catch (e) {
            statusDot.className = 'status-dot';
            statusText.textContent = 'ОШИБКА';
        }
    }

    // Read Configuration File
    async function loadConfig() {
        try {
            const res = await execCommand('cat /data/adb/modules/unbound-core/etc/unbound.conf');
            const content = res.stdout || '';

            const config = {};
            content.split('\n').forEach(line => {
                line = line.trim();
                if (line && !line.startsWith('#')) {
                    const parts = line.split('=');
                    if (parts.length >= 2) {
                        config[parts[0].trim()] = parts[1].trim();
                    }
                }
            });

            if (config.fix_ttl !== undefined) fixTtl.checked = config.fix_ttl === 'true';
            if (config.ttl_value !== undefined) ttlValue.value = config.ttl_value;
            if (config.enable_hotspot !== undefined) enableHotspot.checked = config.enable_hotspot === 'true';
            if (config.iptables_mode !== undefined) iptablesMode.value = config.iptables_mode;
            if (config.filter_ports !== undefined) filterPorts.value = config.filter_ports;
            if (config.nfqueue_num !== undefined) nfqueueNum.value = config.nfqueue_num;
            if (config.enable_ipv6 !== undefined) enableIpv6.checked = config.enable_ipv6 === 'true';
            if (config.excluded_uids !== undefined) excludedUids.value = config.excluded_uids;

            // Summary Update
            modeStatus.textContent = `${iptablesMode.value} (mangle)`;
            ttlStatus.textContent = fixTtl.checked ? `Активен (TTL=${ttlValue.value})` : 'Отключен';
            portsStatus.textContent = filterPorts.value;

        } catch (e) {
            console.error('Failed to load configuration:', e);
        }
    }

    // Save Configuration File
    async function saveConfig() {
        const confText = `# Unbound Core Configuration (WebUI Generated)
nfqueue_num=${nfqueueNum.value}
nfqws_path=/data/adb/modules/unbound-core/bin/nfqws
nfqws_args=
iptables_mode=${iptablesMode.value}
filter_ports=${filterPorts.value}
filter_connbytes_out=1:6
filter_connbytes_in=1:3
fwmark=0x40000000
fwmark_mask=0x40000000
enable_ipv6=${enableIpv6.checked}
enable_hotspot=${enableHotspot.checked}
fix_ttl=${fixTtl.checked}
ttl_value=${ttlValue.value}
excluded_uids=${excludedUids.value}
debug_mode=false
`;

        try {
            btnSaveConfig.textContent = '⏳ Сохранение...';
            const cmd = `cat << 'EOF' > /data/adb/modules/unbound-core/etc/unbound.conf\n${confText}\nEOF`;
            await execCommand(cmd);
            
            // Restart service to apply changes
            await execCommand('/data/adb/modules/unbound-core/service.sh restart');
            
            btnSaveConfig.textContent = '✓ Сохранено и Перезапущено!';
            setTimeout(() => {
                btnSaveConfig.textContent = '💾 Сохранить и Применить';
            }, 2500);

            updateStatus();
            refreshLogs();
        } catch (e) {
            alert('Ошибка при сохранении конфигурации: ' + e.message);
            btnSaveConfig.textContent = '💾 Сохранить и Применить';
        }
    }

    // Fetch Logs
    async function refreshLogs() {
        try {
            const res = await execCommand('tail -n 30 /data/adb/modules/unbound-core/log/nfqws.log');
            logOutput.textContent = res.stdout || '[info] Лог-файл пуст или еще не создан.';
            logOutput.scrollTop = logOutput.scrollHeight;
        } catch (e) {
            logOutput.textContent = '[error] Не удалось загрузить журнал логов.';
        }
    }

    // Event Handlers
    btnStart.addEventListener('click', async () => {
        await execCommand('/data/adb/modules/unbound-core/service.sh start');
        updateStatus();
        refreshLogs();
    });

    btnRestart.addEventListener('click', async () => {
        await execCommand('/data/adb/modules/unbound-core/service.sh restart');
        updateStatus();
        refreshLogs();
    });

    btnStop.addEventListener('click', async () => {
        await execCommand('/data/adb/modules/unbound-core/service.sh stop');
        updateStatus();
        refreshLogs();
    });

    btnSaveConfig.addEventListener('click', saveConfig);
    btnRefreshLogs.addEventListener('click', refreshLogs);

    // Initial Load
    updateStatus();
    loadConfig();
    refreshLogs();
    setInterval(updateStatus, 5000);
});
