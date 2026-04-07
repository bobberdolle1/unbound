//
//  SettingsView.swift
//  UnboundTV
//
//  Settings panel shown as an overlay when user taps Settings button
//

import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var viewModel: UnboundViewModel
    @Environment(\.dismiss) var dismiss
    
    var body: some View {
        ZStack {
            // Backdrop
            Color.black.opacity(0.7)
                .ignoresSafeArea()
                .onTapGesture {
                    withAnimation {
                        viewModel.showSettings = false
                    }
                }
            
            // Settings panel
            VStack(spacing: 30) {
                Text("Settings")
                    .font(.system(size: 48, weight: .bold, design: .rounded))
                    .foregroundColor(.white)
                
                VStack(spacing: 20) {
                    SettingsRow(label: "Engine", value: "tpws (SOCKS proxy)")
                    SettingsRow(label: "Mode", value: "Packet Tunnel (tvOS 17+)")
                    SettingsRow(label: "Root Required", value: "No")
                    SettingsRow(label: "Engine Version", value: viewModel.engineVersion)
                }
                .padding()
                .background(Color(hex: "1a1a2e"))
                .cornerRadius(16)
                
                Button {
                    Task {
                        viewModel.engineVersion = "Checking..."
                        // In production, query the tunnel for version info
                        viewModel.engineVersion = "1.0.0"
                    }
                } label: {
                    Text("Check Engine Status")
                        .font(.system(size: 28, weight: .medium))
                        .frame(minWidth: 400, minHeight: 70)
                        .foregroundColor(.white)
                        .background(Color(hex: "667eea"))
                        .cornerRadius(12)
                }
                .buttonStyle(TVButtonStyle())
                
                Button {
                    withAnimation {
                        viewModel.showSettings = false
                    }
                } label: {
                    Text("Close")
                        .font(.system(size: 28, weight: .medium))
                        .frame(minWidth: 400, minHeight: 70)
                        .foregroundColor(.secondary)
                }
                .buttonStyle(TVButtonStyle())
            }
            .padding(50)
            .background(Color(hex: "0d0d1a"))
            .cornerRadius(24)
            .frame(maxWidth: 800)
        }
        .transition(.opacity)
    }
}

// MARK: - Settings Row Component
struct SettingsRow: View {
    let label: String
    let value: String
    
    var body: some View {
        HStack {
            Text(label)
                .font(.system(size: 28, weight: .medium))
                .foregroundColor(.secondary)
            Spacer()
            Text(value)
                .font(.system(size: 28, weight: .regular))
                .foregroundColor(.white)
        }
        .padding(.vertical, 8)
        .overlay(
            Divider()
                .frame(height: 1)
                .background(Color.gray.opacity(0.3)),
            alignment: .bottom
        )
    }
}

#Preview {
    SettingsView()
        .environmentObject(UnboundViewModel())
}
