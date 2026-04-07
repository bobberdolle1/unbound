// swift-tools-version: 5.9
// The swift-tools-version declares the minimum version of Swift required to build this package.

import PackageDescription

let package = Package(
    name: "UnboundTV",
    platforms: [
        .tvOS(.v17)
    ],
    products: [
        // Main app product
        .executable(
            name: "UnboundTV",
            targets: ["UnboundTV"]
        ),
        // Packet Tunnel Extension
        .library(
            name: "PacketTunnel",
            targets: ["PacketTunnel"]
        ),
        // C-based engine wrapper
        .library(
            name: "UnboundEngine",
            targets: ["UnboundEngine"]
        )
    ],
    dependencies: [
        // No external dependencies - uses Apple's NetworkExtension framework
    ],
    targets: [
        // Main tvOS app target
        .executableTarget(
            name: "UnboundTV",
            dependencies: [],
            path: "UnboundTV/UnboundTV",
            resources: [
                .process("Resources")
            ]
        ),
        
        // Packet Tunnel Extension
        .target(
            name: "PacketTunnel",
            dependencies: ["UnboundEngine"],
            path: "UnboundTV/PacketTunnel",
            resources: [
                .process("Resources")
            ],
            linkerSettings: [
                .linkedFramework("NetworkExtension")
            ]
        ),
        
        // C-based tunnel engine (bridges to tpws)
        .target(
            name: "UnboundEngine",
            dependencies: [],
            path: "UnboundTV/UnboundEngine",
            publicHeadersPath: "include",
            cSettings: [
                .headerSearchPath("."),
                .define("tvOS"),
                .define("DARWIN")
            ],
            linkerSettings: [
                .linkedLibrary("z"),
                .linkedLibrary("pthread")
            ]
        )
    ]
)
