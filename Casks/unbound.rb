cask "unbound" do
  version "0.6.8"
  sha256 "d431bc1738a356691c7ec14fcbd3baea05d7497262177aee5bd2cbfe19b0ded1"

  url "https://github.com/bobberdolle1/unbound/releases/download/v#{version}/unbound-v#{version}-macOS-Installer.pkg"
  name "UNBOUND"
  desc "Ultimate DPI bypass engine for macOS"
  homepage "https://github.com/bobberdolle1/unbound"

  livecheck do
    url :url
    strategy :github_latest
  end

  auto_updates false
  depends_on macos: ">= :catalina"

  pkg "unbound-v#{version}-macOS-Installer.pkg"

  uninstall pkgutil: "com.unbound.app",
            delete:  [
              "/Applications/Unbound.app",
              "/etc/sudoers.d/unbound_zapret",
            ]

  zap trash: [
    "~/Library/Application Support/Unbound",
    "~/Library/Caches/com.wails.unbound",
    "~/Library/Preferences/com.wails.unbound.plist",
    "~/Library/Saved Application State/com.wails.unbound.savedState",
  ]
end
