/*
 * UnboundService — Luna service client for WebOS
 * Communicates with the native nfqws engine via webosbrew root execution service
 */

// Luna service bridge helper
class LunaService {
  constructor(serviceName) {
    this.serviceName = serviceName;
    this.serial = 0;
  }

  async call(method, params = {}) {
    const serial = ++this.serial;
    return new Promise((resolve, reject) => {
      const callbackName = `lunaCallback_${serial}`;
      
      window[callbackName] = (response) => {
        delete window[callbackName];
        if (response && response.returnValue) {
          resolve(response);
        } else {
          reject(new Error(response?.errorText || 'Luna call failed'));
        }
      };

      // Use webOS bridge if available (in Enyo/Enact apps)
      if (window.PalmServiceBridge) {
        const bridge = new PalmServiceBridge();
        bridge.call(`${this.serviceName}/${method}`, JSON.stringify(params), (msg) => {
          try {
            const response = JSON.parse(msg);
            window[callbackName](response);
          } catch (e) {
            reject(e);
          }
        });
      } else {
        reject(new Error('PalmServiceBridge not available'));
      }
    });
  }
}

// WebOS Homebrew Channel service (root execution)
const HBChannelService = 'org.webosbrew.hbchannel.service';

class UnboundService {
  constructor() {
    this.luna = new LunaService(HBChannelService);
    this._running = false;
  }

  /**
   * Start the Unbound engine with the given profile
   * This executes nfqws via the root execution service and sets up iptables rules
   */
  async start(profile = 'default') {
    try {
      // Get profile-specific zapret arguments
      const args = this._getProfileArgs(profile);
      
      // Start nfqws daemon via root execution service
      const startCommand = `/media/developer/apps/usr/palm/applications/com.unbound.app/bin/nfqws ${args} &`;
      
      await this.luna.call('spawn', { command: startCommand });

      // Set up iptables rules to route YouTube traffic through NFQUEUE
      // Wait a moment for engine to initialize
      await this._sleep(1000);

      const iptablesSetup = `
        # Flush existing Unbound rules
        iptables -F UNBOUND_CHAIN 2>/dev/null
        iptables -X UNBOUND_CHAIN 2>/dev/null

        # Create new chain
        iptables -N UNBOUND_CHAIN

        # Route HTTPS traffic to YouTube domains through NFQUEUE
        iptables -A UNBOUND_CHAIN -p tcp --dport 443 -j NFQUEUE --queue-num 200

        # Add jump from OUTPUT chain
        iptables -I OUTPUT -j UNBOUND_CHAIN
      `;

      await this.luna.call('exec', { command: iptablesSetup });

      this._running = true;
      return { success: true };
    } catch (error) {
      console.error('Failed to start Unbound:', error);
      throw error;
    }
  }

  /**
   * Stop the Unbound engine and clean up iptables
   */
  async stop() {
    try {
      // Kill nfqws process
      await this.luna.call('exec', { command: 'killall nfqws 2>/dev/null' });

      // Clean up iptables rules
      const cleanup = `
        iptables -D OUTPUT -j UNBOUND_CHAIN 2>/dev/null
        iptables -F UNBOUND_CHAIN 2>/dev/null
        iptables -X UNBOUND_CHAIN 2>/dev/null
      `;

      await this.luna.call('exec', { command: cleanup });

      this._running = false;
      return { success: true };
    } catch (error) {
      console.error('Failed to stop Unbound:', error);
      throw error;
    }
  }

  /**
   * Check if the engine is currently running
   */
  async isRunning() {
    try {
      const result = await this.luna.call('exec', { 
        command: 'pgrep -f nfqws >/dev/null && echo "running" || echo "stopped"'
      });
      return result?.stdout?.includes('running') || false;
    } catch (error) {
      return false;
    }
  }

  /**
   * Get engine version
   */
  async getVersion() {
    try {
      const result = await this.luna.call('exec', {
        command: '/media/developer/apps/usr/palm/applications/com.unbound.app/bin/nfqws --version'
      });
      return result?.stdout?.trim() || 'unknown';
    } catch (error) {
      throw error;
    }
  }

  /**
   * Get profile-specific zapret command-line arguments
   * These match the profiles defined in the main Unbound engine
   */
  _getProfileArgs(profile) {
    const profiles = {
      'default': `--qnum=200 --dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=6 --hostlist=/media/developer/apps/usr/palm/applications/com.unbound.app/lists/youtube.txt`,
      'aggressive': `--qnum=200 --dpi-desync=fake,split --dpi-desync-pos=1,midsld --dpi-desync-repeats=11 --dpi-desync-autottl --fake-ttl=1 --hostlist=/media/developer/apps/usr/palm/applications/com.unbound.app/lists/youtube.txt`,
      'lite': `--qnum=200 --dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=3 --hostlist=/media/developer/apps/usr/palm/applications/com.unbound.app/lists/youtube.txt`
    };
    return profiles[profile] || profiles['default'];
  }

  _sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}

export default new UnboundService();
