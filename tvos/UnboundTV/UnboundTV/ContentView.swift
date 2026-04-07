//
//  ContentView.swift
//  UnboundTV
//
//  Main UI view — elegant toggle interface with visual feedback
//  Designed for tvOS focus engine (Siri remote D-pad navigation)
//

import SwiftUI

struct ContentView: View {
    @EnvironmentObject var viewModel: UnboundViewModel
    @FocusState private var focusedElement: FocusElement?
    
    enum FocusElement: Hashable {
        case connectButton
        case profilePicker
        case settingsButton
    }
    
    var body: some View {
        ZStack {
            // Background gradient
            LinearGradient(
                colors: [Color(hex: "0d0d1a"), Color(hex: "1a1a2e")],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()
            
            VStack(spacing: 40) {
                // Title
                Text("Unbound")
                    .font(.system(size: 72, weight: .bold, design: .rounded))
                    .foregroundColor(.white)
                    .padding(.top, 60)
                
                // Status indicator
                HStack(spacing: 20) {
                    Circle()
                        .fill(viewModel.isConnected ? Color.green : Color.gray)
                        .frame(width: 24, height: 24)
                        .shadow(color: viewModel.isConnected ? .green : .clear, radius: 10)
                    
                    Text(viewModel.statusText)
                        .font(.system(size: 32, weight: .medium))
                        .foregroundColor(.secondary)
                }
                
                // Main CONNECT/DISCONNECT button
                Button {
                    Task {
                        await viewModel.toggleConnection()
                    }
                } label: {
                    HStack(spacing: 16) {
                        Image(systemName: viewModel.isConnected ? "wifi.slash" : "wifi")
                            .font(.system(size: 40))
                        Text(viewModel.isConnected ? "DISCONNECT" : "CONNECT")
                            .font(.system(size: 48, weight: .bold, design: .rounded))
                    }
                    .frame(minWidth: 600, minHeight: 120)
                    .background(
                        LinearGradient(
                            colors: viewModel.isConnected
                                ? [Color(hex: "f43b47"), Color(hex: "453a94")]
                                : [Color(hex: "667eea"), Color(hex: "764ba2")],
                            startPoint: .leading,
                            endPoint: .trailing
                        )
                    )
                    .foregroundColor(.white)
                    .cornerRadius(20)
                }
                .buttonStyle(TVButtonStyle())
                .focused($focusedElement, equals: .connectButton)
                
                // Profile selector
                VStack(spacing: 16) {
                    Text("Profile")
                        .font(.system(size: 28, weight: .medium))
                        .foregroundColor(.secondary)
                    
                    HStack(spacing: 20) {
                        ForEach(UnboundProfile.allCases, id: \.self) { profile in
                            ProfileButton(
                                profile: profile,
                                isSelected: viewModel.selectedProfile == profile
                            ) {
                                viewModel.selectedProfile = profile
                            }
                        }
                    }
                }
                .padding(.top, 20)
                
                // Settings button
                Button {
                    viewModel.showSettings.toggle()
                } label: {
                    Label("Settings", systemImage: "gear")
                        .font(.system(size: 28, weight: .medium))
                        .frame(minWidth: 300, minHeight: 80)
                        .foregroundColor(.secondary)
                }
                .buttonStyle(TVButtonStyle())
                .focused($focusedElement, equals: .settingsButton)
                .padding(.top, 40)
                
                Spacer()
            }
            .padding(.horizontal, 100)
            .padding(.bottom, 60)
            
            // Settings sheet
            if viewModel.showSettings {
                SettingsView()
                    .transition(.move(edge: .bottom).combined(with: .opacity))
            }
        }
        .onAppear {
            focusedElement = .connectButton
            Task {
                await viewModel.checkStatus()
            }
        }
    }
}

// MARK: - Profile Button Component
struct ProfileButton: View {
    let profile: UnboundProfile
    let isSelected: Bool
    let action: () -> Void
    
    var body: some View {
        Button(action: action) {
            VStack(spacing: 8) {
                Text(profile.displayName)
                    .font(.system(size: 24, weight: .semibold))
                if isSelected {
                    Circle()
                        .fill(Color(hex: "667eea"))
                        .frame(width: 12, height: 12)
                }
            }
            .frame(minWidth: 200, minHeight: 80)
            .background(
                isSelected ? Color(hex: "2a2a4a") : Color(hex: "1a1a2e")
            )
            .cornerRadius(16)
            .overlay(
                RoundedRectangle(cornerRadius: 16)
                    .stroke(isSelected ? Color(hex: "667eea") : Color.clear, lineWidth: 2)
            )
        }
        .buttonStyle(TVButtonStyle())
    }
}

// MARK: - Custom Button Style for tvOS focus
struct TVButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isFocused ? 1.05 : 1.0)
            .shadow(color: configuration.isFocused ? .white.opacity(0.3) : .clear, radius: 10)
            .animation(.easeInOut(duration: 0.2), value: configuration.isFocused)
    }
}

// MARK: - Color Helper
extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 6: // RGB (24-bit)
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xff, int & 0xff)
        case 8: // ARGB (32-bit)
            (a, r, g, b) = (int >> 24, int >> 16 & 0xff, int >> 8 & 0xff, int & 0xff)
        default:
            (a, r, g, b) = (1, 1, 1, 0)
        }
        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue:  Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}

#Preview {
    ContentView()
        .environmentObject(UnboundViewModel())
}
