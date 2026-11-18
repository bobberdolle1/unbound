# Spec file for .rpm packaging (Fedora/RHEL)

Name:           unbound-cli
Version:        %{version}
Release:        1%{?dist}
Summary:        Local L3/L4 DPI desynchronization CLI wrapping Zapret 2 nfqws2 with nftables

License:        GPL-3.0-only
URL:            https://github.com/bobberdolle1/unbound
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang
BuildRequires:  nodejs
BuildRequires:  npm
Requires:       nftables
Requires:       libnetfilter_queue

%description
UNBOUND headless CLI and systemd service for Linux. Extracts SHA-256-verified
Zapret 2 nfqws2 assets at runtime and manages isolated nftables/iptables NFQUEUE
rules for local TCP/UDP DPI packet desynchronization (`unbound --cli`,
`unbound --control`, `unbound --autotune`).

%prep
%setup -q -n %{name}-%{version}

%build
if [ ! -f frontend/dist/index.html ] && command -v npm >/dev/null 2>&1; then
    (cd frontend && npm ci --no-audit --no-fund && npm run build)
fi
CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X unbound/engine.Version=%{version} -X unbound/engine.BuildChannel=release" \
    -o unbound .

%install
install -Dm755 unbound %{buildroot}/usr/bin/unbound
ln -s unbound %{buildroot}/usr/bin/unbound-cli
install -Dm644 packaging/unbound.service %{buildroot}/usr/lib/systemd/system/unbound.service

%post
systemctl daemon-reload 2>/dev/null || true

%preun
if [ "$1" -eq 0 ]; then
    systemctl stop unbound.service 2>/dev/null || true
    systemctl disable unbound.service 2>/dev/null || true
fi

%files
/usr/bin/unbound
/usr/bin/unbound-cli
/usr/lib/systemd/system/unbound.service
%license LICENSE engine/third_party/ZAPRET2_LICENSE.txt
%doc README.md CHANGELOG.md

%changelog
* Wed Nov 18 2026 UNBOUND Contributors <https://github.com/bobberdolle1/unbound> - 0.9.0-1
- Bump UNBOUND CLI package metadata and runtime dependencies for v0.9.0

* Thu Oct 01 2026 UNBOUND Contributors <https://github.com/bobberdolle1/unbound> - 0.8.0-1
- Modernize RPM packaging for Go + Wails v0.8.0+ runtime and Zapret 2 nfqws2
