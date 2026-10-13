#!/bin/sh
set -eu
if [ "$(id -u)" != 0 ] || [ "$#" != 2 ]; then
    echo "usage: sudo $0 BINARY WEB_DIST" >&2
    exit 1
fi
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
test -s "$1"
test -s "$2/index.html"
install -d /opt/vim4-nanokvm/web
install -d /usr/libexec
install -m 0755 "$base/vim4-video" /usr/libexec/vim4-video
visudo -cf "$base/vim4-video.sudoers"
install -m 0440 "$base/vim4-video.sudoers" /etc/sudoers.d/vim4-video
install -m 0755 "$base/vim4-media" /usr/libexec/vim4-media
visudo -cf "$base/vim4-media.sudoers"
install -m 0440 "$base/vim4-media.sudoers" /etc/sudoers.d/vim4-media
for helper in vim4-system vim4-update vim4-edid vim4-tailscale; do
    install -m 0755 "$base/$helper" "/usr/libexec/$helper"
    visudo -cf "$base/$helper.sudoers"
    install -m 0440 "$base/$helper.sudoers" "/etc/sudoers.d/$helper"
done
install -d -o khadas -g khadas -m 0700 /var/lib/kvm/update-jobs /var/lib/vim4-kvm/update-cache
install -d -o khadas -g khadas -m 0700 /var/lib/vim4-kvm/picoclaw
install -m 0755 "$base/picoclaw" /usr/libexec/vim4-picoclaw
install -d /opt/vim4-nanokvm/picoclaw
cp -R "$base/../../kvmapp/picoclaw/." /opt/vim4-nanokvm/picoclaw/
install -d /var/lib/vim4-kvm
install -d -o khadas -g khadas -m 0755 /var/lib/vim4-kvm/images
install -d -o khadas -g khadas -m 0755 /etc/kvm
if [ ! -e /etc/kvm/server.yaml ]; then
    install -o khadas -g khadas -m 0600 "$base/server.yaml" /etc/kvm/server.yaml
fi
install -m 0755 "$1" /opt/vim4-nanokvm/NanoKVM-Server.new
cp -R "$2/." /opt/vim4-nanokvm/web/
sh "$base/remove-legacy-assistant"
mv /opt/vim4-nanokvm/NanoKVM-Server.new /opt/vim4-nanokvm/NanoKVM-Server
install -m 0644 "$base/nanokvm.service" /etc/systemd/system/nanokvm.service
install -m 0644 "$base/vim4-kvm-cpufreq.service" /etc/systemd/system/vim4-kvm-cpufreq.service
install -m 0644 "$base/99-vim4-kvm-hid.rules" /etc/udev/rules.d/99-vim4-kvm-hid.rules
udevadm control --reload-rules
udevadm trigger --subsystem-match=hidg --action=change
udevadm settle
systemctl daemon-reload
systemctl disable --now vim4-kvm.service
systemctl disable --now audioserver.service
systemctl enable --now vim4-kvm-cpufreq.service
systemctl enable nanokvm.service
sync
systemctl restart nanokvm.service
