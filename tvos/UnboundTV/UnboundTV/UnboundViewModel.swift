//
//  UnboundViewModel.swift
//  UnboundTV
//
//  ViewModel that manages the connection state and communicates with
//  the Packet Tunnel Extension
//

import Foundation
import NetworkExtension

@MainActor
class UnboundViewModel: ObservableObject {
    @Published var isConnected = false
    @Published var statusText = "Ready to connect"
    @Published var selectedProfile: UnboundProfile = .default
    @Published var showSettings = false
    @Published var engineVersion = "unknown"
    
    private var manager: NETunnelProviderManager?
    
    // MARK: - Profile Configuration
    
    enum UnboundProfile: String, CaseIterable {
        case `default` = "default"
        case aggressive = "aggressive"
        case lite = "lite"
        
        var displayName: String {
            switch self {
            case .default: return "Default"
            case .aggressive: return "Aggressive"
            case .lite: return "Lite"
            }
        }
        
        var zapretArguments: [String: String] {
            switch self {
            case .default:
                return [
                    "dpi-desync": "split",
                    "dpi-desync-pos": "2",
                    "dpi-desync-repeats": "6"
                ]
            case .aggressive:
                return [
                    "dpi-desync": "fake,split",
                    "dpi-desync-pos": "1,midsld",
                    "dpi-desync-repeats": "11",
                    "dpi-desync-autottl": "1",
                    "fake-ttl": "1"
                ]
            case .lite:
                return [
                    "dpi-desync": "split",
                    "dpi-desync-pos": "2",
                    "dpi-desync-repeats": "3"
                ]
            }
        }
    }
    
    // MARK: - Connection Management
    
    func toggleConnection() async {
        if isConnected {
            await disconnect()
        } else {
            await connect()
        }
    }
    
    func connect() async {
        statusText = "Starting Unbound..."
        
        do {
            // Load or create tunnel manager
            try await loadManager()
            
            guard let manager = manager else {
                statusText = "Failed to initialize tunnel"
                return
            }
            
            // Configure tunnel provider
            try await configureTunnel(manager: manager)
            
            // Enable and start the tunnel
            manager.isEnabled = true
            
            do {
                try await manager.connection.startVPNTunnel()
                isConnected = true
                statusText = "Unbound is active — YouTube unblocked"
            } catch {
                statusText = "Connection failed"
                print("Failed to start VPN tunnel: \(error)")
            }
        } catch {
            statusText = "Initialization error"
            print("Failed to load manager: \(error)")
        }
    }
    
    func disconnect() async {
        statusText = "Disconnecting..."
        
        do {
            try await manager?.connection.stopVPNTunnel()
            isConnected = false
            statusText = "Ready to connect"
        } catch {
            statusText = "Error disconnecting"
            print("Failed to stop VPN tunnel: \(error)")
        }
    }
    
    func checkStatus() async {
        do {
            try await loadManager()
            if let manager = manager {
                let status = manager.connection.status
                isConnected = status == .connected || status == .connecting
                statusText = isConnected ? "Unbound is active" : "Ready to connect"
            }
        } catch {
            print("Failed to check status: \(error)")
        }
    }
    
    // MARK: - Private Methods
    
    private func loadManager() async throws {
        let managers = try await NETunnelProviderManager.loadAllFromPreferences()
        
        if let existingManager = managers.first {
            manager = existingManager
        } else {
            let newManager = NETunnelProviderManager()
            newManager.protocolConfiguration = NETunnelProviderProtocol()
            manager = newManager
        }
    }
    
    private func configureTunnel(manager: NETunnelProviderManager) async throws {
        guard let protocolConfig = manager.protocolConfiguration as? NETunnelProviderProtocol else {
            return
        }
        
        // Configure the packet tunnel provider
        protocolConfig.providerBundleIdentifier = "com.unbound.PacketTunnel"
        protocolConfig.providerConfiguration = [
            "profile": selectedProfile.rawValue,
            "zapret_args": selectedProfile.zapretArguments
        ]
        protocolConfig.serverAddress = "127.0.0.1"
        
        manager.protocolConfiguration = protocolConfig
        manager.localizedDescription = "Unbound DPI Bypass"
        
        try await manager.saveToPreferences()
        try await manager.loadFromPreferences()
    }
}
