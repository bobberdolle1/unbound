"""
Unbound DPI Bypass -- Decky Loader Python Backend

Runs as root (Decky executes backend plugins as root).
Calls the compiled unbound-cli Rust binary to manage the DPI bypass.
All binaries and config reside within the plugin directory to survive SteamOS updates.
"""

import os
import subprocess
import logging
import asyncio

logger = logging.getLogger("unbound")

PLUGIN_DIR = os.path.dirname(os.path.abspath(__file__))
BIN_DIR = os.path.join(PLUGIN_DIR, "bin")
UNBOUND_CLI = os.path.join(BIN_DIR, "unbound-cli")


class UnboundPlugin:
    """Main plugin class exposed to Decky frontend via serverAPI."""

    async def _run_cli(self, *args):
        """Execute unbound-cli (we are already root via Decky)."""
        if not os.path.isfile(UNBOUND_CLI):
            return {"success": False, "error": "unbound-cli binary not found at {}".format(UNBOUND_CLI)}

        cmd = [UNBOUND_CLI] + list(args)
        logger.info("Executing: %s", " ".join(cmd))

        try:
            proc = await asyncio.create_subprocess_exec(
                *cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await proc.communicate()

            result = {
                "success": proc.returncode == 0,
                "stdout": stdout.decode("utf-8", errors="replace").strip(),
                "stderr": stderr.decode("utf-8", errors="replace").strip(),
                "returncode": proc.returncode,
            }

            if not result["success"]:
                logger.error("unbound-cli failed: %s", result["stderr"])

            return result
        except Exception as e:
            logger.error("Exception running unbound-cli: %s", e)
            return {"success": False, "error": str(e)}

    async def start(self, queue=200, iface=""):
        """Start the DPI bypass daemon."""
        args = ["start", "--queue", str(queue)]
        if iface:
            args += ["--iface", iface]
        return await self._run_cli(*args)

    async def stop(self):
        """Stop the DPI bypass daemon."""
        return await self._run_cli("stop")

    async def status(self):
        """Check current status of the DPI bypass."""
        result = await self._run_cli("status")
        if result["success"]:
            output = result.get("stdout", "")
            is_running = "RUNNING" in output and "ACTIVE" in output
            return {
                "success": True,
                "running": is_running,
                "status": "running" if is_running else "stopped",
                "output": output,
            }
        return result

    async def toggle(self, enable, queue=200, iface=""):
        """Toggle the bypass on or off."""
        if enable:
            return await self.start(queue=queue, iface=iface)
        else:
            return await self.stop()

    async def get_version(self):
        """Return plugin and binary versions."""
        return {
            "plugin": "0.1.0",
            "unbound_cli": "0.1.0",
        }

    async def get_config(self):
        """Return current configuration."""
        return {
            "queue": 200,
            "iface": "",
            "bin_path": UNBOUND_CLI,
            "plugin_dir": PLUGIN_DIR,
        }

    async def _main(self):
        """Called when the plugin is loaded."""
        logger.info("Unbound plugin initialized")

    async def _unload(self):
        """Called when the plugin is unloaded."""
        logger.info("Unbound plugin unloaded")
