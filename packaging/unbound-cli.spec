# Spec file for .rpm packaging (Fedora/RHEL)

Name:           unbound-cli
Version:        %{version}
Release:        1%{?dist}
Summary:        DPI/censorship bypass daemon wrapping nfqws with nftables

License:        MIT
URL:            https://github.com/unbound/unbound
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang
Requires:       nftables
Requires:       libnetfilter_queue

%description
Linux DPI/censorship bypass daemon. Wraps the C-based nfqws binary
from the zapret project, managing nftables rules to route HTTP/HTTPS/QUIC
traffic through NFQUEUE for transparent DPI bypass.

%prep
%setup -q

%build
go build -trimpath -ldflags="-s -w -X unbound/engine.Version=%{version}" -o unbound-cli .

%install
install -Dm755 unbound-cli %{buildroot}/usr/bin/unbound-cli
install -Dm644 packaging/unbound.service %{buildroot}/usr/lib/systemd/system/unbound.service

%post
systemctl daemon-reload
systemctl enable unbound.service 2>/dev/null || true

%preun
if [ $1 -eq 0 ]; then
    systemctl stop unbound.service 2>/dev/null || true
    systemctl disable unbound.service 2>/dev/null || true
fi

%files
/usr/bin/unbound-cli
/usr/lib/systemd/system/unbound.service
%license LICENSE
%doc README.md

%changelog
* Tue Apr 07 2026 Unbound Contributors - 0.1.0-1
- Initial package release
