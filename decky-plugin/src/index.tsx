import { definePlugin, PanelSection, PanelSectionRow, ToggleField, Button } from "@decky/ui";
import { useState, useEffect } from "react";
import { FaShieldAlt } from "react-icons/fa";
import { BsWifi, BsWifiOff } from "react-icons/bs";

interface ServerAPI {
  callPluginMethod<TArgs, TResult>(method: string, args?: TArgs): Promise<TResult>;
}

interface StatusResponse {
  success: boolean;
  running?: boolean;
  status?: string;
  output?: string;
  error?: string;
}

function UnboundPanel({ serverAPI }: { serverAPI: ServerAPI }) {
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(false);
  const [statusText, setStatusText] = useState("Checking...");

  useEffect(() => {
    fetchStatus();
  }, []);

  const fetchStatus = async () => {
    try {
      const result = await serverAPI.callPluginMethod<{}, StatusResponse>(
        "status", {}
      );
      if (result.success && result.running !== undefined) {
        setEnabled(result.running);
        setStatusText(result.running ? "Bypass Active" : "Bypass Inactive");
      } else {
        setStatusText("Unable to reach daemon");
      }
    } catch (e) {
      setStatusText("Error checking status");
    }
  };

  const handleToggle = async (value: boolean) => {
    setLoading(true);
    try {
      const result = await serverAPI.callPluginMethod<
        { enable: boolean },
        StatusResponse
      >("toggle", { enable: value });

      if (result.success) {
        setEnabled(value);
        setStatusText(value ? "Bypass Active" : "Bypass Disabled");
      } else {
        setStatusText(result.error || "Failed to toggle");
      }
    } catch (e) {
      setStatusText("Error toggling bypass");
    } finally {
      setLoading(false);
    }
  };

  return (
    <PanelSection title="Unbound DPI Bypass">
      <div>
        <PanelSectionRow>
          <div
            style={{
              background: enabled
                ? "linear-gradient(135deg, rgba(40,160,80,0.15), rgba(40,160,80,0.05))"
                : "linear-gradient(135deg, rgba(180,60,60,0.15), rgba(180,60,60,0.05))",
              borderRadius: "8px",
              padding: "12px 16px",
              marginBottom: "12px",
              display: "flex",
              alignItems: "center",
              gap: "10px",
              border: enabled ? "1px solid rgba(40,160,80,0.3)" : "1px solid rgba(180,60,60,0.3)",
            }}
          >
            {enabled ? (
              <BsWifi size={22} color="#28a050" />
            ) : (
              <BsWifiOff size={22} color="#b43c3c" />
            )}
            <div style={{ flex: 1 }}>
              <div style={{ fontWeight: 600, fontSize: "14px" }}>
                {enabled ? "Unbound Active" : "Unbound Inactive"}
              </div>
              <div style={{ fontSize: "11px", opacity: 0.7, color: "#aaa" }}>
                {statusText}
              </div>
            </div>
          </div>
        </PanelSectionRow>

        <ToggleField
          label="Enable DPI Bypass"
          value={enabled}
          onChange={handleToggle}
          disabled={loading}
          description="Routes traffic through nfqws to bypass DPI/censorship"
        />

        <Button
          onClick={() => fetchStatus()}
          disabled={loading}
          style={{ marginTop: "8px", width: "100%" }}
        >
          {loading ? "Refreshing..." : "Refresh Status"}
        </Button>
      </div>
    </PanelSection>
  );
}

export default definePlugin((serverAPI: ServerAPI) => {
  return {
    title: <div>Unbound</div>,
    content: <UnboundPanel serverAPI={serverAPI} />,
    icon: <FaShieldAlt />,
    onDismount() {},
  };
});
