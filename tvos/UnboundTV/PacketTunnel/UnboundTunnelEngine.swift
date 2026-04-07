//
//  UnboundTunnelEngine.swift
//  PacketTunnel
//
//  Swift wrapper around the C-based tunnel engine
//  Manages the lifecycle of the tpws engine within the tvOS extension
//

import Foundation

enum TunnelError: Error, LocalizedError {
    case initializationFailed
    case startFailed
    case alreadyRunning
    case notRunning
    
    var errorDescription: String? {
        switch self {
        case .initializationFailed: return "Failed to initialize tunnel engine"
        case .startFailed: return "Failed to start tunnel engine"
        case .alreadyRunning: return "Tunnel engine is already running"
        case .notRunning: return "Tunnel engine is not running"
        }
    }
}

class UnboundTunnelEngine {
    private var config: tunnel_config_t
    private var isRunning = false
    
    init(zapretArguments: [String: String]) {
        // Initialize C struct with defaults
        config = tunnel_config_t()
        config.port = 1993
        config.bind_addr = strdup("127.0.0.1")
        config.log_level = 1
        
        // Apply zapret arguments
        if let desync = zapretArguments["dpi-desync"] {
            config.desync_mode = strdup(desync)
        } else {
            config.desync_mode = strdup("split")
        }
        
        if let pos = zapretArguments["dpi-desync-pos"] {
            config.desync_pos = Int32(pos) ?? 2
        } else {
            config.desync_pos = 2
        }
        
        if let repeats = zapretArguments["dpi-desync-repeats"] {
            config.desync_repeats = Int32(repeats) ?? 6
        } else {
            config.desync_repeats = 6
        }
        
        if let autottl = zapretArguments["dpi-desync-autottl"] {
            config.autottl = autottl == "1"
        } else {
            config.autottl = false
        }
        
        if let ttl = zapretArguments["fake-ttl"] {
            config.fake_ttl = Int32(ttl) ?? 1
        } else {
            config.fake_ttl = 1
        }
        
        // Set hostlist file path from bundle
        if let hostlistPath = Bundle.main.path(forResource: "youtube", ofType: "txt") {
            config.hostlist_file = strdup(hostlistPath)
        }
    }
    
    deinit {
        // Free allocated strings
        if config.bind_addr != nil { free(UnsafeMutablePointer(mutating: config.bind_addr)) }
        if config.desync_mode != nil { free(UnsafeMutablePointer(mutating: config.desync_mode)) }
        if config.hostlist_file != nil { free(UnsafeMutablePointer(mutating: config.hostlist_file)) }
    }
    
    /// Start the tunnel engine
    /// - Parameter completion: Called with error if initialization/start fails
    func start(completion: @escaping (Error?) -> Void) {
        // Initialize the C engine
        let initResult = tunnel_init(&config)
        if initResult < 0 {
            completion(TunnelError.initializationFailed)
            return
        }
        
        // Start the engine in a background thread
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            guard let self = self else { return }
            
            let startResult = tunnel_start()
            if startResult < 0 {
                DispatchQueue.main.async {
                    completion(TunnelError.startFailed)
                }
                return
            }
            
            DispatchQueue.main.async {
                self.isRunning = true
                completion(nil)
            }
        }
    }
    
    /// Stop the tunnel engine
    func stop() {
        guard isRunning else { return }
        
        tunnel_stop()
        isRunning = false
    }
    
    /// Check if the engine is running
    var running: Bool {
        return tunnel_is_running()
    }
    
    /// Get active connection count
    var activeConnections: Int {
        return Int(tunnel_get_active_connections())
    }
    
    /// Get engine version
    var version: String {
        if let versionPtr = tunnel_get_version() {
            return String(cString: versionPtr)
        }
        return "unknown"
    }
}
