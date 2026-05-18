package demo

// Canned demo data. Everything here is hand-tuned to look like a
// real, lived-in homelab NAS:
//   - 5+ month uptime, mixed-state packages, a couple of issues to
//     show off the alert colouring
//   - File trees deep enough to demo drill-down + dir-sizing
//   - Realistic share names, container stack, surveillance setup
//
// Shape: every var here maps 1:1 to a Synology DSM JSON envelope's
// `data` payload. We deliberately use map[string]any / []map[string]any
// instead of the typed dsm structs to avoid an import cycle and so the
// demo can advertise fields the typed structs don't model yet.

import (
	"strings"
	"time"
)

// — seeds for mutable state (server.newState() copies these) —

var demoServicesSeed = map[string]string{
	"nfs-server": "enabled",
	"smb":        "enabled",
	"ssh-shell":  "enabled",
	"snmpd":      "always-on",
	"ntpd":       "enabled",
	"telnetd":    "disabled",
	"ftp-pure":   "disabled",
	"sftp":       "disabled",
	"bonjour":    "disabled",
	"webdav":     "enabled",
	"upnp":       "enabled",
	"rsyncd":     "disabled",
	"tftp":       "disabled",
	"cupsd":      "always-on",
	"synoscgi":   "always-on",
	"pkg-iscsi":  "always-on",
	"ups-net":    "disabled",
	"ups-usb":    "always-on",
}

var demoPackagesStateSeed = map[string]string{
	// Force-stop a couple of installed packages so screenshots show
	// the stop chip alongside running.
	"NoteStation":     "stop",
	"DownloadStation": "running",
}

// — top processes —

var demoProcesses = []map[string]any{
	{"pid": 11242, "command": "plex_media_server", "cpu": 18, "mem": 412384, "status": "S"},
	{"pid": 1182, "command": "synoindexd", "cpu": 9, "mem": 88412, "status": "S"},
	{"pid": 2310, "command": "syno-photos-indexer", "cpu": 7, "mem": 156388, "status": "S"},
	{"pid": 5421, "command": "containerd-shim", "cpu": 5, "mem": 64210, "status": "S"},
	{"pid": 982, "command": "smbd", "cpu": 4, "mem": 41280, "status": "S"},
	{"pid": 8721, "command": "synology-drive-server", "cpu": 3, "mem": 198432, "status": "S"},
	{"pid": 14411, "command": "tailscaled", "cpu": 2, "mem": 35200, "status": "S"},
	{"pid": 7711, "command": "synoavd", "cpu": 2, "mem": 102400, "status": "S"},
	{"pid": 11234, "command": "active-backup-bus", "cpu": 1, "mem": 67400, "status": "S"},
	{"pid": 622, "command": "synologin", "cpu": 1, "mem": 22150, "status": "S"},
}

// — storage —

var demoStorage = map[string]any{
	"volumes": []map[string]any{
		{
			"id": "volume_1", "vol_path": "/volume1", "container": "internal",
			"device_type": "shr_without_disk_protect", "desc": "SHR-1", "fs_type": "btrfs",
			"num_id": 1, "pool_path": "/volume1", "raidType": "shr1",
			"space_path": "/volume1", "is_writable": true,
			"size": map[string]any{
				"free_inode": "892834", "total_inode": "67108864",
				"total": "17592186044416", "used": "10031886745600",
			},
			"status": "normal", "summary_status": "normal",
		},
		{
			"id": "volume_2", "vol_path": "/volume2", "container": "internal",
			"device_type": "basic_with_data_protect", "desc": "Basic", "fs_type": "ext4",
			"num_id": 2, "pool_path": "/volume2", "raidType": "basic",
			"is_writable": true,
			"size": map[string]any{
				"free_inode": "3284532", "total_inode": "33554432",
				"total": "4398046511104", "used": "327680000000",
			},
			"status": "normal", "summary_status": "normal",
		},
	},
	"storagePools": []map[string]any{
		{"id": "pool_1", "device_type": "shr_without_disk_protect", "raidType": "shr1",
			"num_id": 1, "disks": []string{"/dev/sda", "/dev/sdb", "/dev/sdc"},
			"pool":   map[string]any{"status": "normal"},
			"size":   map[string]any{"total": "17592186044416", "used": "10031886745600"},
			"status": "normal", "summary_status": "normal"},
		{"id": "pool_2", "device_type": "basic_with_data_protect", "raidType": "basic",
			"num_id": 2, "disks": []string{"/dev/sdd"},
			"pool":   map[string]any{"status": "normal"},
			"size":   map[string]any{"total": "4398046511104", "used": "327680000000"},
			"status": "normal", "summary_status": "normal"},
	},
	"disks": []map[string]any{
		{"id": "/dev/sda", "path": "/dev/sda", "device": "sda", "diskType": "SATA",
			"model": "WD80EFAX-68LHPN0", "vendor": "WDC", "firm": "83.H0A83",
			"status": "normal", "temp": 39, "capacity": "8001563222016",
			"container": map[string]any{"order": 0, "type": "internal", "str": "Pool 1"},
			"smart":     map[string]any{"status": "ok"}, "num_id": 1, "serial": "VLG7DEMO1"},
		{"id": "/dev/sdb", "path": "/dev/sdb", "device": "sdb", "diskType": "SATA",
			"model": "WD80EFAX-68LHPN0", "vendor": "WDC", "firm": "83.H0A83",
			"status": "normal", "temp": 41, "capacity": "8001563222016",
			"container": map[string]any{"order": 1, "type": "internal", "str": "Pool 1"},
			"smart":     map[string]any{"status": "ok"}, "num_id": 2, "serial": "VLG7DEMO2"},
		{"id": "/dev/sdc", "path": "/dev/sdc", "device": "sdc", "diskType": "SATA",
			"model": "ST4000VN006-3CW104", "vendor": "Seagate", "firm": "SC60",
			"status": "normal", "temp": 36, "capacity": "4000787030016",
			"container": map[string]any{"order": 2, "type": "internal", "str": "Pool 1"},
			"smart":     map[string]any{"status": "warn"}, "num_id": 3, "serial": "ZGY8DEMO3"},
		{"id": "/dev/sdd", "path": "/dev/sdd", "device": "sdd", "diskType": "SSD",
			"model": "Samsung SSD 870 EVO 1TB", "vendor": "Samsung", "firm": "SVT02B6Q",
			"status": "normal", "temp": 31, "capacity": "1024209543168",
			"container": map[string]any{"order": 3, "type": "internal", "str": "Pool 2"},
			"smart":     map[string]any{"status": "ok"}, "num_id": 4, "serial": "S6DEMODISK4"},
	},
}

// — shares —

var demoShares = []map[string]any{
	{"name": "photo", "vol_path": "/volume1", "desc": "Family photos + auto-backup target", "encryption": 1, "enc_status": 1, "hidden": false, "enable_recycle_bin": true, "is_readonly": false, "is_usb_share": false, "is_sync_share": false, "is_cloudsync_share": false, "share_quota": 2_000_000, "share_quota_used": 1_265_404},
	{"name": "video", "vol_path": "/volume1", "desc": "Movies + TV", "hidden": false, "enable_recycle_bin": true, "share_quota": 0, "share_quota_used": 0},
	{"name": "music", "vol_path": "/volume1", "desc": "FLAC + MP3 library", "hidden": false, "enable_recycle_bin": false},
	{"name": "books", "vol_path": "/volume1", "desc": "ePub / PDF / audiobooks", "hidden": false, "enable_recycle_bin": false},
	{"name": "homes", "vol_path": "/volume1", "desc": "Per-user home folders", "hidden": true, "enable_recycle_bin": true},
	{"name": "backups", "vol_path": "/volume2", "desc": "Hyper Backup destination", "hidden": false, "enable_recycle_bin": false, "is_readonly": false},
	{"name": "plex-cache", "vol_path": "/volume2", "desc": "Plex transcode + metadata", "hidden": false, "is_usb_share": false},
	{"name": "code", "vol_path": "/volume1", "desc": "Synology Drive sync target", "hidden": false, "is_sync_share": true},
	{"name": "surveillance", "vol_path": "/volume2", "desc": "Surveillance Station recordings", "hidden": false, "is_readonly": false},
	{"name": "cloud-sync", "vol_path": "/volume1", "desc": "Cloud Sync (Dropbox + Drive)", "hidden": false, "is_cloudsync_share": true},
}

// — file station roots (per-share view) —

var demoFileShares = []map[string]any{
	{"isdir": true, "name": "photo", "path": "/photo",
		"additional": map[string]any{"owner": map[string]any{"user": "baraniewski", "group": "users"},
			"volume_status": map[string]any{"freespace": 7_560_299_298_816, "totalspace": 17_592_186_044_416, "readonly": false}}},
	{"isdir": true, "name": "video", "path": "/video",
		"additional": map[string]any{"owner": map[string]any{"user": "baraniewski", "group": "users"},
			"volume_status": map[string]any{"freespace": 7_560_299_298_816, "totalspace": 17_592_186_044_416, "readonly": false}}},
	{"isdir": true, "name": "music", "path": "/music",
		"additional": map[string]any{"owner": map[string]any{"user": "baraniewski", "group": "users"},
			"volume_status": map[string]any{"freespace": 7_560_299_298_816, "totalspace": 17_592_186_044_416, "readonly": false}}},
	{"isdir": true, "name": "books", "path": "/books",
		"additional": map[string]any{"owner": map[string]any{"user": "baraniewski", "group": "users"},
			"volume_status": map[string]any{"freespace": 7_560_299_298_816, "totalspace": 17_592_186_044_416, "readonly": false}}},
	{"isdir": true, "name": "homes", "path": "/homes",
		"additional": map[string]any{"owner": map[string]any{"user": "root", "group": "root"},
			"volume_status": map[string]any{"freespace": 7_560_299_298_816, "totalspace": 17_592_186_044_416, "readonly": false}}},
	{"isdir": true, "name": "backups", "path": "/backups",
		"additional": map[string]any{"owner": map[string]any{"user": "baraniewski", "group": "users"},
			"volume_status": map[string]any{"freespace": 4_070_366_511_104, "totalspace": 4_398_046_511_104, "readonly": false}}},
	{"isdir": true, "name": "code", "path": "/code",
		"additional": map[string]any{"owner": map[string]any{"user": "baraniewski", "group": "users"},
			"volume_status": map[string]any{"freespace": 7_560_299_298_816, "totalspace": 17_592_186_044_416, "readonly": false}}},
}

// — folder contents (keyed by path) —

func fsdir(name, path string) map[string]any {
	return map[string]any{"isdir": true, "name": name, "path": path, "additional": map[string]any{
		"owner": map[string]any{"user": "baraniewski", "group": "users"},
		"time":  map[string]any{"mtime": time.Now().Add(-72 * time.Hour).Unix()},
		"perm":  map[string]any{"posix": 755},
	}}
}
func fsfile(name, path string, size int64, modAgo time.Duration) map[string]any {
	return map[string]any{"isdir": false, "name": name, "path": path, "additional": map[string]any{
		"owner": map[string]any{"user": "baraniewski", "group": "users"},
		"size":  size,
		"time":  map[string]any{"mtime": time.Now().Add(-modAgo).Unix()},
		"perm":  map[string]any{"posix": 644},
	}}
}

var demoFolderContents = map[string][]map[string]any{
	"/photo": {
		fsdir("2026", "/photo/2026"),
		fsdir("2025", "/photo/2025"),
		fsdir("2024", "/photo/2024"),
		fsdir("Family", "/photo/Family"),
		fsdir("Travel", "/photo/Travel"),
		fsdir("Wedding", "/photo/Wedding"),
		fsfile("front.jpg", "/photo/front.jpg", 1_034_240, 8*time.Hour),
		fsfile("DSC02184.RAF", "/photo/DSC02184.RAF", 38_421_400, 28*time.Hour),
		fsfile("DSC02185.RAF", "/photo/DSC02185.RAF", 41_388_812, 28*time.Hour),
	},
	"/photo/2026": {
		fsdir("01-jan", "/photo/2026/01-jan"),
		fsdir("02-feb", "/photo/2026/02-feb"),
		fsdir("03-mar", "/photo/2026/03-mar"),
		fsdir("04-apr", "/photo/2026/04-apr"),
		fsdir("05-may", "/photo/2026/05-may"),
	},
	"/video": {
		fsdir("Movies", "/video/Movies"),
		fsdir("TV", "/video/TV"),
		fsdir("Home", "/video/Home"),
		fsfile("rip-2026-03-bday.mp4", "/video/rip-2026-03-bday.mp4", 4_280_412_192, 30*24*time.Hour),
	},
	"/music": {
		fsdir("FLAC", "/music/FLAC"),
		fsdir("Lossy", "/music/Lossy"),
		fsdir("Playlists", "/music/Playlists"),
	},
	"/books": {
		fsdir("fiction", "/books/fiction"),
		fsdir("non-fiction", "/books/non-fiction"),
		fsdir("audiobooks", "/books/audiobooks"),
		fsfile("how-linux-works-3rd.pdf", "/books/how-linux-works-3rd.pdf", 14_280_412, 10*24*time.Hour),
		fsfile("the-pragmatic-programmer.epub", "/books/the-pragmatic-programmer.epub", 2_140_220, 80*24*time.Hour),
	},
	"/homes": {
		fsdir("baraniewski", "/homes/baraniewski"),
		fsdir("demo", "/homes/demo"),
	},
	"/backups": {
		fsdir("hyperbackup", "/backups/hyperbackup"),
		fsdir("active-backup", "/backups/active-backup"),
		fsdir("offsite-cold", "/backups/offsite-cold"),
	},
	"/code": {
		fsdir("synology-ctl", "/code/synology-ctl"),
		fsdir("openusage", "/code/openusage"),
		fsdir("homelab-iac", "/code/homelab-iac"),
		fsfile(".gitconfig", "/code/.gitconfig", 1248, 200*24*time.Hour),
	},
	"__default__": {
		fsdir(".cache", "/__default__/.cache"),
		fsfile("README.md", "/__default__/README.md", 4280, 24*time.Hour),
	},
}

// demoDirSize returns plausible total/dirs/files for a given path.
func demoDirSize(path string) (int64, int64, int64) {
	// Quote-aware: dsm.Client sends path as `["..."]`.
	path = strings.TrimPrefix(strings.TrimSuffix(path, `"]`), `["`)
	switch path {
	case "/photo":
		return 1_265_404_887_040, 12, 18432
	case "/video":
		return 6_482_440_142_848, 8, 1842
	case "/music":
		return 298_412_991_456, 6, 4288
	case "/books":
		return 18_240_412_416, 4, 1244
	case "/homes":
		return 232_140_412_416, 12, 24288
	case "/backups":
		return 980_280_412_416, 5, 188
	case "/code":
		return 12_840_412_416, 124, 18244
	case "/cloud-sync":
		return 480_412_416, 8, 488
	case "/surveillance":
		return 1_240_412_412_416, 8, 12440
	case "/plex-cache":
		return 76_240_412_416, 2, 4288
	}
	// Subfolder default — scale down a bit.
	return 1_280_412_416, 4, 88
}

// — users / groups —

var demoUsers = []map[string]any{
	{"name": "baraniewski", "uid": 1026, "description": "Jan (admin)", "email": "jan@example.com", "expired": "normal", "groups": []string{"administrators", "users"}, "password_never_expire": true},
	{"name": "demo", "uid": 1027, "description": "Demo viewer", "email": "demo@example.com", "expired": "normal", "groups": []string{"users"}},
	{"name": "backup-svc", "uid": 1028, "description": "Service account for Hyper Backup", "email": "", "expired": "normal", "groups": []string{"backup-operators"}, "password_never_expire": true},
	{"name": "guest", "uid": 1029, "description": "Disabled guest account", "email": "", "expired": "now", "groups": []string{"users"}},
	{"name": "kid", "uid": 1030, "description": "Family — restricted access", "email": "", "expired": "normal", "groups": []string{"users", "kids"}},
}

var demoGroups = []map[string]any{
	{"name": "administrators", "gid": 101, "description": "DSM admins"},
	{"name": "users", "gid": 100, "description": "Standard users"},
	{"name": "kids", "gid": 1050, "description": "Restricted profile"},
	{"name": "backup-operators", "gid": 1100, "description": "Allowed to write to /backups"},
	{"name": "developers", "gid": 1200, "description": "Synology Drive shared workspace"},
}

// — network —

var demoNetworkInterfaces = []map[string]any{
	{"id": "eth0", "ifname": "LAN 1", "type": "lan", "ip": "192.168.1.36", "mask": "255.255.255.0", "gateway": "192.168.1.1", "mac": "00:11:32:DE:M0:01", "mtu": 1500, "speed": 2500, "status": "connected", "use_dhcp": true},
	{"id": "eth1", "ifname": "LAN 2", "type": "lan", "ip": "10.0.0.36", "mask": "255.255.255.0", "gateway": "10.0.0.1", "mac": "00:11:32:DE:M0:02", "mtu": 9000, "speed": 1000, "status": "connected", "use_dhcp": false},
	{"id": "ovs_bond0", "ifname": "Bond 1", "type": "bond", "ip": "192.168.1.37", "mask": "255.255.255.0", "mac": "00:11:32:DE:M0:03", "mtu": 1500, "speed": 4000, "status": "connected", "use_dhcp": false},
	{"id": "tailscale0", "ifname": "Tailscale", "type": "vpn", "ip": "100.123.191.95", "mask": "255.255.255.255", "gateway": "100.100.100.100", "mac": "", "mtu": 1280, "speed": 0, "status": "connected", "use_dhcp": false},
}

// — logs —

func logEntry(t time.Time, level, user, ip, event, descr string) map[string]any {
	return map[string]any{
		"time":  t.Format("2006-01-02 15:04:05"),
		"level": level, "user": user, "ip": ip, "event": event, "descr": descr,
	}
}

var demoLogs = map[string][]map[string]any{
	"system": {
		logEntry(time.Now().Add(-15*time.Minute), "info", "System", "", "Volume 1 scrub completed", "0 errors detected; checksum matches across 6.4 TB of data."),
		logEntry(time.Now().Add(-32*time.Minute), "warn", "System", "", "Disk SMART warning", "sdc reported 4 reallocated sectors (was 3). Consider replacement."),
		logEntry(time.Now().Add(-1*time.Hour), "info", "Package", "", "Synology Drive started", "Service started after auto-update to 3.1.0-22920."),
		logEntry(time.Now().Add(-90*time.Minute), "info", "Hyper Backup", "", "Backup task completed", `"Daily homes → /volume2/backups" finished in 22m 14s.`),
		logEntry(time.Now().Add(-2*time.Hour), "info", "baraniewski", "192.168.1.5", "User logged in", "Login via DSM web UI from 192.168.1.5."),
		logEntry(time.Now().Add(-3*time.Hour), "warn", "Network", "", "Tailscale reconnected", "Tailscale lost connection at 17:02; auto-reconnected at 17:03."),
		logEntry(time.Now().Add(-4*time.Hour), "info", "System", "", "Scheduled task triggered", `"Antivirus weekly scan" started.`),
		logEntry(time.Now().Add(-5*time.Hour), "err", "Surveillance", "", "Camera disconnected", "Front Door camera unreachable for 3 minutes — recordings paused."),
		logEntry(time.Now().Add(-6*time.Hour), "info", "demo", "10.0.0.42", "User logged in", "Login via SSH."),
		logEntry(time.Now().Add(-7*time.Hour), "info", "Package", "", "Container Manager updated", "Updated to 24.0.2-1535."),
		logEntry(time.Now().Add(-9*time.Hour), "warn", "Security", "", "SecurityAdvisor flagged item", "SSH allowed from any IP — consider restricting to LAN."),
		logEntry(time.Now().Add(-10*time.Hour), "info", "Storage", "", "Snapshot created", `Snapshot of "photo" share created (daily auto).`),
		logEntry(time.Now().Add(-11*time.Hour), "info", "System", "", "DSM auto-update check", "No updates available."),
		logEntry(time.Now().Add(-13*time.Hour), "info", "Hyper Backup", "", "Backup task completed", `"Monthly cold storage → external" finished in 4h 38m.`),
		logEntry(time.Now().Add(-14*time.Hour), "err", "guest", "203.0.113.42", "Login blocked", "5 failed login attempts → IP auto-banned for 24h."),
		logEntry(time.Now().Add(-18*time.Hour), "info", "System", "", "NTP sync", "Time synchronised with pool.ntp.org (offset -0.142s)."),
		logEntry(time.Now().Add(-22*time.Hour), "info", "Package", "", "Active Backup ran", "laptop-jan ran scheduled backup (incremental, 412 MB)."),
		logEntry(time.Now().Add(-26*time.Hour), "warn", "Storage", "", "Volume nearly full", "Volume 1 used 91% — consider clean-up or expansion."),
		logEntry(time.Now().Add(-30*time.Hour), "info", "System", "", "Daily housekeeping", "Recycle bin auto-purge removed 38 items (1.4 GB)."),
	},
	"connection": {
		logEntry(time.Now().Add(-1*time.Hour), "info", "baraniewski", "192.168.1.5", "SMB connected", `Mounted "homes" via SMB.`),
		logEntry(time.Now().Add(-2*time.Hour), "info", "baraniewski", "100.123.191.95", "WebDAV connected", `Accessed "/code" via WebDAV.`),
		logEntry(time.Now().Add(-3*time.Hour), "info", "demo", "10.0.0.42", "SFTP connected", "SFTP session opened."),
		logEntry(time.Now().Add(-4*time.Hour), "info", "baraniewski", "192.168.1.5", "AFP connected", `Mounted "video" via AFP.`),
		logEntry(time.Now().Add(-6*time.Hour), "info", "backup-svc", "192.168.1.10", "rsync connected", "rsyncd session opened from 192.168.1.10."),
	},
}

// — packages —

var demoPackages = []map[string]any{
	{"id": "ActiveInsight", "name": "Active Insight", "version": "1.2.0-1214", "maintainer": "Synology Inc.", "description": "Cloud monitoring service for Synology NAS.", "beta": false, "ctl_uninstall": true, "timestamp": time.Now().Add(-45 * 24 * time.Hour).UnixMilli()},
	{"id": "AudioStation", "name": "Audio Station", "version": "7.0.3-5401", "maintainer": "Synology Inc.", "description": "Stream your audio collection from your NAS.", "beta": false, "ctl_uninstall": true},
	{"id": "CodecPack", "name": "Advanced Media Extensions", "version": "1.1.2-0301", "maintainer": "Synology Inc.", "description": "Codecs for advanced media playback.", "beta": false, "ctl_uninstall": true},
	{"id": "Container-Manager", "name": "Container Manager", "version": "24.0.2-1535", "maintainer": "Synology Inc.", "description": "Manage Docker containers and Compose stacks.", "beta": false, "ctl_uninstall": true},
	{"id": "DhcpServer", "name": "DHCP Server", "version": "1.0.1-0036", "maintainer": "Synology Inc.", "description": "Run a DHCP server on your NAS.", "beta": false, "ctl_uninstall": true},
	{"id": "DownloadStation", "name": "Download Station", "version": "3.9.5-4627", "maintainer": "Synology Inc.", "description": "BitTorrent + HTTP/FTP download manager.", "beta": false, "ctl_uninstall": true},
	{"id": "FileStation", "name": "File Station", "version": "1.3.1-1382", "maintainer": "Synology Inc.", "description": "Web-based file manager (always installed).", "beta": false, "ctl_uninstall": false},
	{"id": "Git", "name": "Git Server", "version": "2.39.1-1017", "maintainer": "Git", "description": "Self-hosted Git repositories.", "beta": false, "ctl_uninstall": true},
	{"id": "HyperBackup", "name": "Hyper Backup", "version": "3.0.2-2446", "maintainer": "Synology Inc.", "description": "Multi-version backup to local + cloud destinations.", "beta": false, "ctl_uninstall": true},
	{"id": "ActiveBackup-Business", "name": "Active Backup for Business", "version": "2.6.2-12517", "maintainer": "Synology Inc.", "description": "Centralized backup for PCs, servers, and VMs.", "beta": false, "ctl_uninstall": true},
	{"id": "MediaServer", "name": "Media Server", "version": "2.0.5-3152", "maintainer": "Synology Inc.", "description": "DLNA media server.", "beta": false, "ctl_uninstall": true},
	{"id": "NoteStation", "name": "Note Station", "version": "2.8.2-3508", "maintainer": "Synology Inc.", "description": "Take + sync notes between devices.", "beta": false, "ctl_uninstall": true},
	{"id": "Node.js_v18", "name": "Node.js v18", "version": "18.18.2-0011", "maintainer": "nodejs.org", "description": "Node.js runtime v18.", "beta": false, "ctl_uninstall": true},
	{"id": "OAuthService", "name": "OAuth Service", "version": "1.1.2-0071", "maintainer": "Synology Inc.", "description": "OAuth 2.0 provider for DSM accounts.", "beta": false, "ctl_uninstall": true},
	{"id": "Plex", "name": "Plex Media Server", "version": "1.41.5.9626-72180", "maintainer": "Plex Inc", "description": "Stream your media to any device.", "beta": false, "ctl_uninstall": true},
	{"id": "Python3", "name": "Python3", "version": "3.9.14-0003", "maintainer": "Python Software Foundation", "description": "Python 3 interpreter.", "beta": false, "ctl_uninstall": true},
	{"id": "SMBService", "name": "SMB Service", "version": "4.15.13-0330", "maintainer": "Synology Inc.", "description": "SMB / CIFS file sharing.", "beta": false, "ctl_uninstall": false},
	{"id": "StorageAnalyzer", "name": "Storage Analyzer", "version": "2.1.0-0421", "maintainer": "Synology Inc.", "description": "Detailed storage usage breakdowns.", "beta": false, "ctl_uninstall": true},
	{"id": "SurveillanceStation", "name": "Surveillance Station", "version": "9.2.5-11979", "maintainer": "Synology Inc.", "description": "IP camera management + recording.", "beta": false, "ctl_uninstall": true},
	{"id": "SynologyApplicationService", "name": "Synology Application Service", "version": "1.7.2-10549", "maintainer": "Synology Inc.", "description": "Shared service backend for Synology apps.", "beta": false, "ctl_uninstall": true},
	{"id": "SynologyDrive", "name": "Synology Drive Server", "version": "3.1.0-22920", "maintainer": "Synology Inc.", "description": "File sync + collaboration.", "beta": false, "ctl_uninstall": true},
	{"id": "SynologyPhotos", "name": "Synology Photos", "version": "1.3.4-0340", "maintainer": "Synology Inc.", "description": "Photo management with face + subject AI.", "beta": false, "ctl_uninstall": true},
	{"id": "Tailscale", "name": "Tailscale", "version": "1.58.2-700058002", "maintainer": "Tailscale, Inc.", "description": "Zero-config VPN built on WireGuard.", "beta": false, "ctl_uninstall": true},
	{"id": "VideoStation", "name": "Video Station", "version": "3.0.7-2512", "maintainer": "Synology Inc.", "description": "Organise + stream video collections.", "beta": false, "ctl_uninstall": true},
	{"id": "iTunesServer", "name": "iTunes Server", "version": "2.0.0-2723", "maintainer": "Synology Inc.", "description": "Share music via the iTunes protocol.", "beta": false, "ctl_uninstall": true},
}

// — package catalog (Available) —

func avail(id, name, ver, maintainer, desc string, size int64) map[string]any {
	return map[string]any{"id": id, "package": id, "name": name, "version": ver, "maintainer": maintainer, "desc": desc, "size": size}
}

var demoCatalog = []map[string]any{
	avail("AntiVirus", "AntiVirus Essential", "1.5.4-3099", "Synology Inc.", "Signature-based malware scanning for DSM shared folders.", 234_881_024),
	avail("Apache2.4", "Apache HTTP Server 2.4", "2.4.54-0125", "apache.org", "Run Apache 2.4 alongside the built-in nginx.", 1_785_446),
	avail("CalendarServer", "Calendar", "2.4.6-10942", "Synology Inc.", "CalDAV server + DSM calendar UI.", 7_858_345),
	avail("Chat", "Chat", "2.4.2-12125", "Synology Inc.", "Team chat hosted on your NAS.", 70_254_530),
	avail("CloudStation", "Cloud Station Backup", "2.7.2-2464", "Synology Inc.", "Versioned backup target for client devices.", 5_871_546),
	avail("ContactsServer", "Contacts", "1.0.5-10492", "Synology Inc.", "CardDAV address-book server.", 8_283_135),
	avail("DDB", "Data Deposit Box", "0.1-700350", "Data Deposit Box Inc.", "Cloud backup target (DDB).", 5_558_310),
	avail("DirectoryServer", "Directory Server", "2.4.57-2653", "Synology Inc.", "LDAP-compatible directory service.", 1_572_864),
	avail("Emby", "Emby Server", "4.7.14.0-70405", "Emby LLC", "Media server (Plex alternative).", 74_516_004),
	avail("GlacierBackup", "Glacier Backup", "1.5.2-1143", "Synology Inc.", "Backup to AWS Glacier cold storage.", 1_785_446),
	avail("HyperBackupVault", "Hyper Backup Vault", "3.0.2-2446", "Synology Inc.", "Receive Hyper Backups from other Synology NAS units.", 4_809_768),
	avail("MailPlus-Server", "MailPlus Server", "3.2.0-21029", "Synology Inc.", "Full-stack mail server (SMTP + IMAP + AV).", 198_412_416),
	avail("MailStation", "Mail Station", "20231031-10324", "Synology Inc.", "Webmail front-end.", 4_298_752),
	avail("MariaDB10", "MariaDB 10", "10.3.32-1040", "MariaDB Foundation", "Drop-in MySQL replacement.", 59_768_832),
	avail("MediaWiki", "MediaWiki", "1.39.2-1078", "MediaWiki", "Self-hosted wiki software.", 40_894_464),
	avail("PDFViewer", "PDF Viewer", "1.2.3-1124", "Synology Inc.", "In-browser PDF reader.", 798_720),
	avail("PHP8.0", "PHP 8.0", "8.0.23-0103", "php.net", "PHP runtime, v8.0 line.", 12_582_912),
	avail("RadiusServer", "RADIUS Server", "3.0.27-0453", "Synology Inc.", "RADIUS authentication backend.", 1_677_722),
	avail("ScsiTarget", "SAN Manager", "1.0.2-0207", "Synology Inc.", "iSCSI / Fibre Channel target manager.", 4_194_304),
	avail("Spreadsheet", "Synology Office", "3.4.2-18342", "Synology Inc.", "Browser-based docs + spreadsheets + slides.", 124_780_544),
	avail("TeamViewer", "TeamViewer", "3.5.10-7005", "TeamViewer Germany GmbH", "Remote-control your NAS shell.", 16_777_216),
	avail("TextEditor", "Text Editor", "1.2.4-0245", "Synology Inc.", "Lightweight in-browser text editor.", 2_097_152),
	avail("USBCopy", "USB Copy", "2.2.1-1103", "Synology Inc.", "One-touch external-drive sync.", 1_363_148),
	avail("VPNCenter", "VPN Server", "1.4.4-2855", "Synology Inc.", "Run OpenVPN / L2TP / PPTP server.", 2_516_582),
	avail("WebDAVServer", "WebDAV Server", "2.4.8-10135", "Synology Inc.", "Expose shares via WebDAV.", 1_677_722),
	avail("WebStation", "Web Station", "3.0.0-0309", "Synology Inc.", "Host websites with nginx + Apache + PHP.", 14_680_064),
	avail("WordPress", "WordPress", "6.1.1-1062", "WordPress", "Blogging + CMS platform.", 16_777_216),
}

// — services —

var demoServices = []map[string]any{
	{"service": "nfs-server", "display_name": "NFS", "display_name_section_key": "nfs", "enable_status": "enabled"},
	{"service": "smb", "display_name": "SMB / CIFS", "display_name_section_key": "smb", "enable_status": "enabled"},
	{"service": "sftp", "display_name": "sftp", "display_name_section_key": "sftp", "enable_status": "disabled"},
	{"service": "ssh-shell", "display_name": "ssh-shell", "display_name_section_key": "ssh", "enable_status": "enabled"},
	{"service": "telnetd", "display_name": "Telnet", "display_name_section_key": "telnet", "enable_status": "disabled"},
	{"service": "ftp-pure", "display_name": "FTP", "display_name_section_key": "ftp", "enable_status": "disabled"},
	{"service": "tftp", "display_name": "TFTP", "display_name_section_key": "tftp", "enable_status": "disabled"},
	{"service": "snmpd", "display_name": "SNMP", "display_name_section_key": "snmp", "enable_status": "always-on"},
	{"service": "ntpd", "display_name": "NTP", "display_name_section_key": "ntp", "enable_status": "enabled"},
	{"service": "bonjour", "display_name": "Bonjour mDNS", "display_name_section_key": "bonjour", "enable_status": "disabled"},
	{"service": "webdav", "display_name": "WebDAV", "display_name_section_key": "webdav", "enable_status": "enabled"},
	{"service": "upnp", "display_name": "UPnP", "display_name_section_key": "upnp", "enable_status": "enabled"},
	{"service": "rsyncd", "display_name": "rsyncd", "display_name_section_key": "rsync", "enable_status": "disabled"},
	{"service": "cupsd", "display_name": "CUPS print daemon", "display_name_section_key": "cups", "enable_status": "always-on"},
	{"service": "synoscgi", "display_name": "synoscgi", "display_name_section_key": "synoscgi", "enable_status": "always-on"},
	{"service": "pkg-iscsi", "display_name": "iSCSI", "display_name_section_key": "iscsi", "enable_status": "always-on"},
	{"service": "ups-net", "display_name": "ups-net", "display_name_section_key": "ups", "enable_status": "disabled"},
	{"service": "ups-usb", "display_name": "ups-usb", "display_name_section_key": "ups", "enable_status": "always-on"},
	{"service": "pkg-synosamba-wstransfer-genc", "display_name": "WS-Discovery", "display_name_section_key": "ws", "enable_status": "enabled"},
}

// — containers (Docker) —

var demoContainers = []map[string]any{
	{"id": "a1b2c3d4e5f6", "name": "plex", "image": "linuxserver/plex:latest", "status": "running", "cpu": 18, "mem": 412_384_000, "created": time.Now().Add(-12 * 24 * time.Hour).Unix(), "ports": "32400:32400/tcp"},
	{"id": "f6e5d4c3b2a1", "name": "jellyfin", "image": "jellyfin/jellyfin:10.9", "status": "running", "cpu": 6, "mem": 312_400_000, "created": time.Now().Add(-9 * 24 * time.Hour).Unix(), "ports": "8096:8096/tcp"},
	{"id": "1234567890ab", "name": "homepage", "image": "ghcr.io/gethomepage/homepage:v0.9.2", "status": "running", "cpu": 1, "mem": 28_412_000, "created": time.Now().Add(-22 * 24 * time.Hour).Unix(), "ports": "3000:3000/tcp"},
	{"id": "ba0987654321", "name": "pihole", "image": "pihole/pihole:latest", "status": "stopped", "cpu": 0, "mem": 0, "created": time.Now().Add(-100 * 24 * time.Hour).Unix(), "ports": "53:53/udp,80:8081/tcp"},
	{"id": "cafebabe1234", "name": "traefik", "image": "traefik:v3.0", "status": "running", "cpu": 2, "mem": 64_412_000, "created": time.Now().Add(-30 * 24 * time.Hour).Unix(), "ports": "80:80/tcp,443:443/tcp"},
	{"id": "deadbeef5678", "name": "uptime-kuma", "image": "louislam/uptime-kuma:1", "status": "running", "cpu": 1, "mem": 132_140_000, "created": time.Now().Add(-40 * 24 * time.Hour).Unix(), "ports": "3001:3001/tcp"},
}

var demoDockerImages = []map[string]any{
	{"id": "img_plex_latest", "name": "linuxserver/plex", "tag": "latest", "size": 388_412_408, "created": time.Now().Add(-12 * 24 * time.Hour).Unix()},
	{"id": "img_jellyfin_10_9", "name": "jellyfin/jellyfin", "tag": "10.9", "size": 412_840_000, "created": time.Now().Add(-9 * 24 * time.Hour).Unix()},
	{"id": "img_homepage_092", "name": "ghcr.io/gethomepage/homepage", "tag": "v0.9.2", "size": 280_412_000, "created": time.Now().Add(-22 * 24 * time.Hour).Unix()},
	{"id": "img_pihole_latest", "name": "pihole/pihole", "tag": "latest", "size": 198_412_000, "created": time.Now().Add(-100 * 24 * time.Hour).Unix()},
	{"id": "img_traefik_v3", "name": "traefik", "tag": "v3.0", "size": 88_240_000, "created": time.Now().Add(-30 * 24 * time.Hour).Unix()},
	{"id": "img_kuma_1", "name": "louislam/uptime-kuma", "tag": "1", "size": 312_140_000, "created": time.Now().Add(-40 * 24 * time.Hour).Unix()},
}

var demoDockerNetworks = []map[string]any{
	{"id": "net_bridge", "name": "bridge", "driver": "bridge", "scope": "local", "containers": 4},
	{"id": "net_host", "name": "host", "driver": "host", "scope": "local", "containers": 1},
	{"id": "net_homelab", "name": "homelab", "driver": "bridge", "scope": "local", "containers": 3},
}

// — surveillance —

var demoCameras = []map[string]any{
	{"id": 1, "name": "Living Room", "model": "Reolink RLC-823A", "vendor": "Reolink", "status": 1, "newName": "Living Room"},
	{"id": 2, "name": "Garage", "model": "Hikvision DS-2CD2143G2", "vendor": "Hikvision", "status": 1, "newName": "Garage"},
	{"id": 3, "name": "Front Door", "model": "Reolink Argus 3 Pro", "vendor": "Reolink", "status": 7, "newName": "Front Door"},
	{"id": 4, "name": "Back Garden", "model": "Reolink RLC-810A", "vendor": "Reolink", "status": 1, "newName": "Back Garden"},
}

var demoRecordings = []map[string]any{
	{"id": 121, "camera_name": "Living Room", "start_time": time.Now().Add(-2 * time.Hour).Unix(), "length": 1842},
	{"id": 122, "camera_name": "Garage", "start_time": time.Now().Add(-3 * time.Hour).Unix(), "length": 1280},
	{"id": 123, "camera_name": "Front Door", "start_time": time.Now().Add(-5 * time.Hour).Unix(), "length": 482},
}

var demoSurveillanceInfo = map[string]any{
	"version":       map[string]any{"version": "9.2.5-11979", "build": "11979"},
	"path":          "/volume2/@surveillance",
	"hostname":      "demo-ds923",
	"cameras_total": 4, "cameras_online": 3,
}

// — backup —

var demoHyperBackupTasks = []map[string]any{
	{"task_id": 1, "name": "Daily homes → /volume2/backups", "target": "Synology NAS (local)", "schedule": "daily 03:00", "last_run": time.Now().Add(-90 * time.Minute).Unix(), "last_status": "success", "total_size": 980_412_416_416},
	{"task_id": 2, "name": "Monthly cold storage → external", "target": "External USB", "schedule": "monthly", "last_run": time.Now().Add(-13 * time.Hour).Unix(), "last_status": "success", "total_size": 4_120_412_416_416},
	{"task_id": 3, "name": "Photos → B2", "target": "Backblaze B2", "schedule": "weekly Sun 02:00", "last_run": time.Now().Add(-6 * 24 * time.Hour).Unix(), "last_status": "warning", "total_size": 1_265_404_887_040},
}

var demoActiveBackupTasks = []map[string]any{
	{"task_id": 11, "name": "laptop-jan", "source_type": "device", "target": "/volume2/active-backup", "last_run": time.Now().Add(-22 * time.Hour).Unix(), "status": "success"},
	{"task_id": 12, "name": "laptop-kasia", "source_type": "device", "target": "/volume2/active-backup", "last_run": time.Now().Add(-48 * time.Hour).Unix(), "status": "success"},
	{"task_id": 13, "name": "vm-homelab", "source_type": "vm", "target": "/volume2/active-backup", "last_run": time.Now().Add(-10 * 24 * time.Hour).Unix(), "status": "failed"},
}

var demoActiveBackupVersions = []map[string]any{
	{"version_id": 4002, "task_id": 11, "time": time.Now().Add(-22 * time.Hour).Unix(), "size": 1_240_412_416, "status": "complete"},
	{"version_id": 4001, "task_id": 11, "time": time.Now().Add(-46 * time.Hour).Unix(), "size": 980_412_416, "status": "complete"},
	{"version_id": 4000, "task_id": 11, "time": time.Now().Add(-72 * time.Hour).Unix(), "size": 1_540_412_416, "status": "complete"},
}

// — drive —

var demoDriveFiles = []map[string]any{
	{"name": "Q4-planning.gdoc", "path": "/Drive/Q4-planning.gdoc", "size": 124_280, "type": "document", "modified": time.Now().Add(-6 * time.Hour).Unix(), "owner": "baraniewski"},
	{"name": "homelab-roadmap.gsheet", "path": "/Drive/homelab-roadmap.gsheet", "size": 88_412, "type": "spreadsheet", "modified": time.Now().Add(-2 * 24 * time.Hour).Unix(), "owner": "baraniewski"},
	{"name": "talk-slides.gslides", "path": "/Drive/talk-slides.gslides", "size": 1_412_408, "type": "presentation", "modified": time.Now().Add(-4 * 24 * time.Hour).Unix(), "owner": "baraniewski"},
	{"name": "video-2026-05-01.mp4", "path": "/Drive/video-2026-05-01.mp4", "size": 482_412_408, "type": "video", "modified": time.Now().Add(-18 * 24 * time.Hour).Unix(), "owner": "demo"},
}

var demoDriveStats = map[string]any{
	"total_users": 5, "total_files": 18_412, "storage_used": 12_412_840_000, "storage_quota": 100_000_000_000,
}

// — security —

func cert(id, cn, issuer string, expiresIn time.Duration, isDefault bool) map[string]any {
	return map[string]any{
		"id": id, "common_name": cn, "issuer": map[string]any{"common_name": issuer},
		"subject":          map[string]any{"common_name": cn},
		"subject_alt_name": []string{cn, "*." + cn},
		"valid_from":       time.Now().Add(-365 * 24 * time.Hour).Format(time.RFC3339),
		"valid_till":       time.Now().Add(expiresIn).Format(time.RFC3339),
		"is_default":       isDefault, "is_broken": false,
		"services": []string{"system", "webdav"},
	}
}

var demoCertificates = []map[string]any{
	cert("default", "demo-ds923.local", "synology", 8*365*24*time.Hour, false),
	cert("letsencrypt-1", "homelab.example.com", "Let's Encrypt R3", 60*24*time.Hour, true),
	cert("letsencrypt-2", "vpn.example.com", "Let's Encrypt R3", 8*24*time.Hour, false), // expiry warn
	cert("self-signed-old", "old-host.lan", "Synology Inc.", -7*24*time.Hour, false),    // already expired
}

var demoSecAdvisorConf = map[string]any{
	"last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix(),
	"total_items":  6, "critical": 1, "warn": 2, "info": 3,
}

var demoSecAdvisorItems = []map[string]any{
	{"id": "ssh_any", "severity": "critical", "title": "SSH allowed from any IP", "description": "Restrict SSH to LAN or specific subnets to reduce brute-force exposure.", "last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix()},
	{"id": "auto_block_off", "severity": "warn", "title": "Auto Block disabled", "description": "Enable Auto Block to ban IPs after repeated failed logins.", "last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix()},
	{"id": "password_policy", "severity": "warn", "title": "Weak password policy", "description": "Increase minimum password length to 12 and require mixed case + digits.", "last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix()},
	{"id": "ntp_sync", "severity": "info", "title": "NTP healthy", "description": "Time synchronisation is within 1 second of the configured pool.", "last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix()},
	{"id": "dsm_update", "severity": "info", "title": "DSM up to date", "description": "Running latest DSM 7.2.2-72806 Update 3.", "last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix()},
	{"id": "2fa_enrolled", "severity": "info", "title": "2-step verification enrolled for admin", "description": "Admin accounts have 2FA enabled.", "last_scanned": time.Now().Add(-3 * 24 * time.Hour).Unix()},
}

var demoSchedTasks = []map[string]any{
	{"id": 1, "name": "Antivirus weekly scan", "type": "antivirus", "enable": true, "next_trigger_time": time.Now().Add(4 * 24 * time.Hour).Unix(), "owner": "system", "repeat": "weekly"},
	{"id": 2, "name": "Synology Drive index rebuild", "type": "package", "enable": true, "next_trigger_time": time.Now().Add(20 * 24 * time.Hour).Unix(), "owner": "system", "repeat": "monthly"},
	{"id": 3, "name": "Photo upload script", "type": "user-defined", "enable": false, "next_trigger_time": 0, "owner": "baraniewski", "repeat": "daily"},
	{"id": 4, "name": "Quarterly cold-storage rotation", "type": "user-defined", "enable": true, "next_trigger_time": time.Now().Add(80 * 24 * time.Hour).Unix(), "owner": "baraniewski", "repeat": "quarterly"},
	{"id": 5, "name": "S.M.A.R.T. extended test", "type": "smart", "enable": true, "next_trigger_time": time.Now().Add(11 * 24 * time.Hour).Unix(), "owner": "system", "repeat": "monthly"},
}

var demoFirewallStatus = map[string]any{
	"enabled": true, "profile": "Default", "notify_via_dsm_notify": true,
}

var demoFirewallProfiles = []map[string]any{
	{"name": "Default", "is_default": true, "enabled": true, "rules": 8},
	{"name": "Strict (away)", "is_default": false, "enabled": false, "rules": 12},
}

var demoFirewallRules = []map[string]any{
	{"id": 1, "name": "Allow LAN", "enabled": true, "action": "allow", "protocol": "all", "source": "192.168.1.0/24", "dest_port": "all"},
	{"id": 2, "name": "Allow Tailscale", "enabled": true, "action": "allow", "protocol": "all", "source": "100.64.0.0/10", "dest_port": "all"},
	{"id": 3, "name": "Allow HTTPS", "enabled": true, "action": "allow", "protocol": "tcp", "source": "any", "dest_port": "443"},
	{"id": 4, "name": "Allow HTTP", "enabled": true, "action": "allow", "protocol": "tcp", "source": "any", "dest_port": "80"},
	{"id": 5, "name": "Allow SSH from LAN only", "enabled": true, "action": "allow", "protocol": "tcp", "source": "192.168.1.0/24", "dest_port": "22"},
	{"id": 6, "name": "Block country: RU", "enabled": true, "action": "deny", "protocol": "all", "source": "geo:RU", "dest_port": "all"},
	{"id": 7, "name": "Block country: CN", "enabled": true, "action": "deny", "protocol": "all", "source": "geo:CN", "dest_port": "all"},
	{"id": 8, "name": "Default deny", "enabled": true, "action": "deny", "protocol": "all", "source": "any", "dest_port": "all"},
}

var demoDDNSProviders = []map[string]any{
	{"name": "Synology", "url": "checkip.synology.com"},
	{"name": "DynDNS", "url": "checkip.dyndns.com"},
	{"name": "No-IP", "url": "dynupdate.no-ip.com"},
	{"name": "Cloudflare", "url": "api.cloudflare.com"},
}

var demoDDNSRecords = []map[string]any{
	{"hostname": "demo-ds923.synology.me", "provider": "Synology", "ipv4": "203.0.113.42", "ipv6": "2001:db8::42", "status": "OK", "last_updated": time.Now().Add(-15 * time.Minute).Unix(), "enabled": true},
	{"hostname": "homelab.dyndns.org", "provider": "DynDNS", "ipv4": "203.0.113.42", "ipv6": "", "status": "OK", "last_updated": time.Now().Add(-32 * time.Minute).Unix(), "enabled": true},
	{"hostname": "vpn.cloudflare.net", "provider": "Cloudflare", "ipv4": "203.0.113.42", "ipv6": "2001:db8::42", "status": "OK", "last_updated": time.Now().Add(-5 * time.Minute).Unix(), "enabled": true},
}
