# Samba Blackjack Panel
🌐 [English](README.en.md) | [中文](README.zh.md) | [Español](README.es.md) | [Русский](README.md)

Lightweight Samba server management panel written in Go.

## 🚀 Quick Install (Ubuntu/Debian)

The easiest way to install the panel as a system service:

```bash
wget https://raw.githubusercontent.com/kirpicheff/samba-blackjack-panel/main/install.sh
chmod +x install.sh
sudo ./install.sh
```

After installation, the panel will be available at: `http://your-ip:8888`.
- **Login:** `admin`
- **Password:** `admin` (change immediately in "Server Settings" -> "Panel Administrators")

---

## ✨ Features

### 📊 Dashboard and Monitoring
- **Active Sessions**: View connected users, their IPs, and SMB protocol versions.
- **Open Files**: List of all files currently in use by clients.
- **Disk Status**: Monitoring free space on partitions with shared folders.
- **Services**: Status control for WSDD (Windows) and Avahi (macOS/Linux).

### 📂 Resource Management (Shares)
- **Create and Edit**: Full management of `smb.conf` sections via UI.
- **Recycle Bin**: Automatic cleanup, file exclusion, path configuration.
- **Audit**: Action logging (delete, rename) into an integrated log.
- **FS Permissions (ACL)**: Visual editor for owner, group, and access rights (`chmod`) with octal mask and checkbox matrix.
- **IP Restriction**: `hosts allow` and `hosts deny` configuration globally and per-resource.
- **Shadow Copies**: Support for VFS Shadow Copy 2 for file recovery.

### 👥 Users and Groups
- **Samba Users**: Account management via `pdbedit` (create, password, delete).
- **OS Groups**: System group management and user membership control.

### 🌐 Active Directory (AD)
- **Auto-Join**: Kerberos, Winbind configuration and Join execution.
- **Health Check**: Deep diagnostics of connection with DC (Trust, Time, RPC, Keytab).

### ⚙️ Settings and Security
- **Global Parameters**: Workgroup, Netbios Name, network interface settings.
- **Service Control**: Start/Stop/Restart for smbd, nmbd, winbind.
- **Auto-Backups**: Storage of the last 10 `smb.conf` versions.
- **Panel Access**: Multi-user login, Bcrypt password hashing, admin management.

### 📜 Logs and Automation
- **Live Logs**: Real-time view of `log.smbd` via WebSockets.
- **Audit Log**: History of user actions in an integrated table.
- **Background Tasks**: Automatic recycle bin cleanup and snapshot creation.

---

## 💻 Development Mode (Windows/macOS)

If you run the panel on non-Linux systems, it automatically switches to **Mock mode**:
- Uses test data instead of real system calls.
- Creates a local `smb.conf.dev` file to simulate configuration.

**Run for development:**
1. Ensure Go is installed.
2. Clone the repository.
3. Run: `go run .`
4. Open `http://localhost:8888`.

---

## 🛠 Manual Installation and Recovery

### System Requirements (Linux)
For all functions to work:
```bash
sudo apt update
sudo apt install samba samba-common-bin krb5-user winbind avahi-daemon acl
```

### Access Recovery
Panel administrator passwords are stored in `admins.json`. If you lose access:
1. Delete the `admins.json` file.
2. Restart the panel.
3. Use default credentials: `admin / admin`.
