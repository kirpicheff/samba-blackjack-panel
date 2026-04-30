# Samba Blackjack Panel
🌐 [English](README.en.md) | [中文](README.zh.md) | [Español](README.es.md) | [Русский](README.md)

基于 Go 语言的轻量级 Samba 服务器管理面板。

## 🚀 快速安装 (Ubuntu/Debian)

将面板作为系统服务安装的最简单方法：

```bash
wget https://raw.githubusercontent.com/kirpicheff/samba-blackjack-panel/main/install.sh
chmod +x install.sh
sudo ./install.sh
```

安装后，面板可通过以下地址访问：`http://您的IP:8888`。
- **用户名：** `admin`
- **密码：** `admin`（请在“服务器设置”->“面板管理员”中立即更改）

---

## ✨ 功能特性

### 📊 仪表板与监控
- **活动会话**：查看连接的用户、其 IP 和 SMB 协议版本。
- **打开的文件**：客户端当前正在使用的所有文件列表。
- **磁盘状态**：监控共享文件夹所在分区的剩余空间。
- **服务控制**：WSDD (Windows) 和 Avahi (macOS/Linux) 的状态控制。

### 📂 资源管理 (Shares)
- **创建与编辑**：通过 UI 全面管理 `smb.conf` 配置节。
- **回收站 (Recycle Bin)**：自动清理、文件排除、路径配置。
- **审计**：将操作行为（删除、重命名）记录到集成日志中。
- **文件系统权限 (ACL)**：所有者、组和访问权限 (`chmod`) 的可视化编辑器，支持八进制掩码和复选框矩阵。
- **IP 限制**：全局或按资源配置 `hosts allow` 和 `hosts deny`。
- **影子副本**：支持 VFS Shadow Copy 2 以进行文件恢复。

### 👥 用户与组
- **Samba 用户**：通过 `pdbedit` 管理账户（创建、密码、删除）。
- **系统组**：管理系统组及用户成员身份。

### 🌐 活动目录 (AD)
- **自动加入域**：配置 Kerberos、Winbind 并执行 Join 操作。
- **健康检查**：深度诊断与域控制器的连接（信任、时间、RPC、Keytab）。

### ⚙️ 设置与安全
- **全局参数**：配置工作组、Netbios 名称、网络接口。
- **服务控制**：smbd, nmbd, winbind 的启动/停止/重启。
- **自动备份**：存储最近 10 个版本的 `smb.conf`。
- **面板访问**：多用户登录、Bcrypt 密码哈希、管理员管理。

### 📜 日志与自动化
- **实时日志**：通过 WebSockets 实时查看 `log.smbd`。
- **审计日志**：在集成表格中查看用户操作历史。
- **后台任务**：自动清理回收站和创建快照 (snapshots)。

---

## 💻 开发模式 (Windows/macOS)

如果您在非 Linux 系统上运行面板，它将自动切换到 **Mock 模式**：
- 使用测试数据代替真实的系统调用。
- 创建本地 `smb.conf.dev` 文件以模拟配置。

**开发运行步骤：**
1. 确保已安装 Go。
2. 克隆仓库。
3. 执行：`go run .`
4. 访问 `http://localhost:8888`。

---

## 🛠 手动安装与恢复

### 系统要求 (Linux)
为使所有功能正常运行：
```bash
sudo apt update
sudo apt install samba samba-common-bin krb5-user winbind avahi-daemon acl
```

### 访问恢复
面板管理员密码存储在 `admins.json` 中。如果您失去访问权限：
1. 删除 `admins.json` 文件。
2. 重启面板。
3. 使用默认凭据：`admin / admin`。
