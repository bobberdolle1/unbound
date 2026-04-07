//
//  UnboundTVApp.swift
//  UnboundTV
//
//  Main entry point for the tvOS application
//  Uses SwiftUI with tvOS 17+ focus engine for remote navigation
//

import SwiftUI

@main
struct UnboundTVApp: App {
    @StateObject private var viewModel = UnboundViewModel()
    
    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(viewModel)
        }
    }
}
