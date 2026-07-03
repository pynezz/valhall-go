#!/bin/bash
# Install RHEL 8.10-flavored command shims into /usr/bin for screenshot
# and smoke-test purposes (build container only, not part of stoker).
set -e
cat > /usr/bin/systemctl << 'EOF'
#!/bin/bash
case "$1" in
  is-system-running) echo "degraded"; exit 1;;
  --failed)
    echo "chronyd.service loaded failed failed NTP client/server"
    ;;
  list-units)
    cat << 'UNITS'
auditd.service loaded active running Security Auditing Service
chronyd.service loaded failed failed NTP client/server
crond.service loaded active running Command Scheduler
dbus.service loaded active running D-Bus System Message Bus
dnf-makecache.timer loaded active waiting dnf makecache --timer
firewalld.service loaded active running firewalld - dynamic firewall daemon
irqbalance.service loaded active running irqbalance daemon
kdump.service loaded active exited Crash recovery kernel arming
NetworkManager.service loaded active running Network Manager
polkit.service loaded active running Authorization Manager
rngd.service loaded active running Hardware RNG Entropy Gatherer Daemon
rsyslog.service loaded active running System Logging Service
sshd.service loaded active running OpenSSH server daemon
sssd.service loaded active running System Security Services Daemon
stoker-agent.service loaded active running Hearth stoker telemetry agent
systemd-journald.service loaded active running Journal Service
systemd-logind.service loaded active running Login Service
systemd-udevd.service loaded active running udev Kernel Device Manager
tmux-keepalive.service loaded inactive dead Persistent ops tmux session
tuned.service loaded active running Dynamic System Tuning Daemon
logrotate.timer loaded active waiting Daily rotation of log files
sssd-kcm.socket loaded active listening SSSD Kerberos Cache Manager responder
dbus.socket loaded active running D-Bus System Message Bus Socket
UNITS
    ;;
  status)
    unit="${@: -1}"
    cat << EOF2
● $unit - NTP client/server
   Loaded: loaded (/usr/lib/systemd/system/$unit; enabled; vendor preset: enabled)
   Active: failed (Result: exit-code) since Thu 2026-07-02 08:14:02 CEST; 2h 31min ago
     Docs: man:chronyd(8)
  Process: 1287 ExecStart=/usr/sbin/chronyd \$OPTIONS (code=exited, status=1/FAILURE)
 Main PID: 1287 (code=exited, status=1/FAILURE)

Jul 02 08:14:02 hearth-node01 chronyd[1287]: Fatal error : Could not open configuration file /etc/chrony.conf
Jul 02 08:14:02 hearth-node01 systemd[1]: chronyd.service: Main process exited, code=exited, status=1/FAILURE
Jul 02 08:14:02 hearth-node01 systemd[1]: chronyd.service: Failed with result 'exit-code'.
Jul 02 08:14:02 hearth-node01 systemd[1]: Failed to start NTP client/server.
EOF2
    ;;
  *) exit 0;;
esac
EOF
cat > /usr/bin/journalctl << 'EOF'
#!/bin/bash
emit() {
cat << 'LOG'
2026-07-02T08:13:58+0200 hearth-node01 systemd[1]: Starting NTP client/server...
2026-07-02T08:14:02+0200 hearth-node01 chronyd[1287]: Fatal error : Could not open configuration file /etc/chrony.conf
2026-07-02T08:14:02+0200 hearth-node01 systemd[1]: chronyd.service: Failed with result 'exit-code'.
2026-07-02T08:14:11+0200 hearth-node01 sshd[1502]: Server listening on 0.0.0.0 port 22.
2026-07-02T08:14:30+0200 hearth-node01 NetworkManager[1101]: <info>  device (ens192): state change: ip-config -> activated
2026-07-02T08:15:02+0200 hearth-node01 sshd[2214]: Accepted publickey for kevin from 10.0.20.14 port 51244 ssh2: ED25519
2026-07-02T08:15:02+0200 hearth-node01 systemd-logind[1130]: New session 3 of user kevin.
2026-07-02T09:02:17+0200 hearth-node01 setroubleshoot[3384]: SELinux is preventing /usr/sbin/nginx from name_connect access on the tcp_socket port 8443
2026-07-02T09:02:17+0200 hearth-node01 audit[3391]: AVC avc:  denied  { name_connect } for  pid=3391 comm="nginx" dest=8443
2026-07-02T10:12:44+0200 hearth-node01 dnf[4102]: Metadata cache created.
2026-07-02T10:41:09+0200 hearth-node01 kernel: XFS (dm-0): Ending clean mount
2026-07-02T10:45:33+0200 hearth-node01 stoker-agent[988]: heartbeat ok, queue=0, lag=12ms
LOG
}
if [[ "$*" == *"-f"* ]]; then
  emit
  i=0
  while true; do
    sleep 1; i=$((i+1))
    echo "2026-07-02T10:4$((5+i%5)):$((10+i))+0200 hearth-node01 stoker-agent[988]: heartbeat ok, queue=0, lag=$((10+i))ms"
  done
else
  emit
fi
EOF
cat > /usr/bin/nmcli << 'EOF'
#!/bin/bash
cat << 'NM'
DEVICE  TYPE      STATE      CONNECTION
ens192  ethernet  connected  lan0
ens224  ethernet  connected  storage0
lo      loopback  unmanaged  --
NM
EOF
cat > /usr/bin/sestatus << 'EOF'
#!/bin/bash
cat << 'SE'
SELinux status:                 enabled
SELinuxfs mount:                /sys/fs/selinux
SELinux root directory:         /etc/selinux
Loaded policy name:             targeted
Current mode:                   enforcing
Mode from config file:          enforcing
Policy MLS status:              enabled
Policy deny_unknown status:     allowed
Memory protection checking:     actual (secure)
Max kernel policy version:      33
SE
EOF
cat > /usr/bin/dnf << 'EOF'
#!/bin/bash
case "$*" in
  *check-update*)
cat << 'UP'
kernel.x86_64                    4.18.0-553.58.1.el8_10          baseos
openssl-libs.x86_64              1:1.1.1k-14.el8_10              baseos
systemd.x86_64                   239-82.el8_10.4                 baseos
tmux.x86_64                      2.7-3.el8                       baseos
git-core.x86_64                  2.43.7-1.el8_10                 appstream
UP
    ;;
  *history*)
cat << 'H'
ID     | Command line             | Date and time    | Action(s)      | Altered
-------------------------------------------------------------------------------
    41 | -y update                | 2026-06-28 03:12 | I, U           |   14
    40 | install go-toolset       | 2026-06-25 11:40 | Install        |   12
    39 | install tmux             | 2026-06-25 11:32 | Install        |    2
H
    ;;
esac
EOF
cat > /usr/bin/systemd-cgls << 'EOF'
#!/bin/bash
echo "Control group /: (mock)"
EOF
for f in systemctl journalctl nmcli sestatus dnf systemd-cgls; do chmod 755 /usr/bin/$f; done
cp /etc/os-release /tmp/os-release.bak 2>/dev/null || true
cat > /etc/os-release << 'EOF'
PRETTY_NAME="Hearth OS 8.10 (RHEL 8.10 base)"
EOF
hostname hearth-node01 2>/dev/null || echo hearth-node01 > /proc/sys/kernel/hostname 2>/dev/null || true
echo "mock env ready"
