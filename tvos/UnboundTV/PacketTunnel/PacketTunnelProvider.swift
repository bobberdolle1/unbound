//
//  PacketTunnelProvider.swift
//  PacketTunnel
//
//  NEPacketTunnelProvider implementation for tvOS 17+
//  Intercepts all network traffic and routes it through the local
//  Unbound DPI bypass engine (adapted from tpws/nfqws)
//

import NetworkExtension
import os

class PacketTunnelProvider: NEPacketTunnelProvider {
    
    private let logger = Logger(subsystem: "com.unbound.PacketTunnel", category: "PacketTunnelProvider")
    private var tunnelEngine: UnboundTunnelEngine?
    private var currentProfile: String = "default"
    
    // MARK: - NEPacketTunnelProvider Lifecycle
    
    override func startTunnel(options: [String : NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        logger.info("Starting packet tunnel...")
        
        // Extract profile configuration from provider settings
        if let providerConfig = protocolConfiguration as? NETunnelProviderProtocol,
           let providerConfiguration = providerConfig.providerConfiguration,
           let profile = providerConfiguration["profile"] as? String {
            currentProfile = profile
            logger.info("Profile: \(profile, privacy: .public)")
        }
        
        // Initialize the tunnel engine with profile settings
        let zapretArgs = getZapretArguments(for: currentProfile)
        tunnelEngine = UnboundTunnelEngine(zapretArguments: zapretArgs)
        
        // Set up network settings (virtual interface)
        let networkSettings = createNetworkSettings()
        
        setTunnelNetworkSettings(networkSettings) { [weak self] error in
            guard let self = self else { return }
            
            if let error = error {
                self.logger.error("Failed to set network settings: \(error.localizedDescription)")
                completionHandler(error)
                return
            }
            
            // Start reading packets from the system
            self.startReadingPackets()
            
            // Start the local DPI bypass engine
            self.tunnelEngine?.start { engineError in
                if let engineError = engineError {
                    self.logger.error("Engine failed to start: \(engineError.localizedDescription)")
                    completionHandler(engineError)
                } else {
                    self.logger.info("Tunnel started successfully")
                    completionHandler(nil)
                }
            }
        }
    }
    
    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        logger.info("Stopping tunnel (reason: \(reason.rawValue))")
        
        tunnelEngine?.stop()
        tunnelEngine = nil
        
        completionHandler()
    }
    
    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        logger.info("Received app message")
        
        // Handle messages from the main app (status queries, config changes, etc.)
        if let message = String(data: messageData, encoding: .utf8) {
            logger.info("Message: \(message, privacy: .public)")
        }
        
        // Echo back for now; in production, handle specific commands
        if let completionHandler = completionHandler {
            completionHandler(messageData)
        }
    }
    
    // MARK: - Packet Processing
    
    private func startReadingPackets() {
        // Continuously read packets from the system and process them
        Task {
            do {
                while true {
                    // Get packet from system
                    let packetBuffers = try await packetFlow.readPackets()
                    
                    // Process each packet through the DPI bypass engine
                    for packetBuffer in packetBuffers {
                        processPacket(packetBuffer)
                    }
                }
            } catch {
                logger.error("Packet read error: \(error.localizedDescription)")
            }
        }
    }
    
    private func processPacket(_ packetBuffer: Data) {
        // In a full implementation, this would:
        // 1. Parse the IP packet
        // 2. Check if it's a TCP connection to a blocked domain (YouTube, etc.)
        // 3. Apply DPI bypass techniques (desync, fake packets, etc.)
        // 4. Forward the modified packet back to the system
        //
        // For now, we simply forward packets unchanged (passthrough mode).
        // The actual DPI bypass logic lives in the UnboundTunnelEngine C library.
        
        // Forward packet back to the system (passthrough)
        packetFlow.writePackets([packetBuffer], withProtocols: [AF_INET as NSNumber])
    }
    
    // MARK: - Network Settings Configuration
    
    private func createNetworkSettings() -> NEPacketTunnelNetworkSettings {
        // Create virtual interface settings
        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "127.0.0.1")
        
        // Configure the virtual interface
        settings.mtu = 1500
        
        // DNS settings — use Google DNS to avoid ISP DNS poisoning
        let dnsSettings = NEDNSSettings(servers: ["8.8.8.8", "8.8.4.4", "1.1.1.1"])
        dnsSettings.matchDomains = [""]
        settings.dnsSettings = dnsSettings
        
        // IPv4 settings — assign a local address to the tunnel interface
        let ipv4Settings = NEIPv4Settings(
            addresses: ["192.168.200.1"],
            subnetMasks: ["255.255.255.0"]
        )
        
        // Route all traffic through the tunnel
        ipv4Settings.includedRoutes = [NEIPv4Route.default()]
        settings.ipv4Settings = ipv4Settings
        
        // IPv6 settings (optional, for IPv6 networks)
        let ipv6Settings = NEIPv6Settings(
            addresses: ["fd00::1"],
            networkPrefixLengths: [64]
        )
        ipv6Settings.includedRoutes = [NEIPv6Route.default()]
        settings.ipv6Settings = ipv6Settings
        
        // Proxy settings — point to local DPI bypass engine
        // The tpws engine will run as a local SOCKS proxy on 127.0.0.1:1993
        let proxySettings = NEProxySettings()
        let proxyServer = NEProxyServer(address: "127.0.0.1", port: 1993)
        proxySettings.httpEnabled = true
        proxySettings.httpServer = proxyServer
        proxySettings.httpsEnabled = true
        proxySettings.httpsServer = proxyServer
        
        // Bypass proxy for local addresses
        proxySettings.exceptionList = ["localhost", "127.0.0.1", "10.*", "192.168.*", "172.16.*"]
        
        settings.proxySettings = proxySettings
        
        return settings
    }
    
    // MARK: - Profile Configuration
    
    private func getZapretArguments(for profile: String) -> [String: String] {
        switch profile {
        case "default":
            return [
                "dpi-desync": "split",
                "dpi-desync-pos": "2",
                "dpi-desync-repeats": "6"
            ]
        case "aggressive":
            return [
                "dpi-desync": "fake,split",
                "dpi-desync-pos": "1,midsld",
                "dpi-desync-repeats": "11",
                "dpi-desync-autottl": "1",
                "fake-ttl": "1"
            ]
        case "lite":
            return [
                "dpi-desync": "split",
                "dpi-desync-pos": "2",
                "dpi-desync-repeats": "3"
            ]
        default:
            return [
                "dpi-desync": "split",
                "dpi-desync-pos": "2",
                "dpi-desync-repeats": "6"
            ]
        }
    }
}
