#!/bin/bash
# install.sh for Samba Blackjack Panel

set -e

echo "🚀 Installing Samba Blackjack Panel..."

# 1. Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "❌ Please run with sudo: sudo ./install.sh"
    exit 1
fi

# 2. Install system dependencies
echo "📦 Installing system dependencies (Samba, Git, Quota, Discovery)..."
apt update
apt install -y samba samba-common-bin git quota avahi-daemon wsdd2

# 3. Check for Go
if ! command -v go &> /dev/null; then
    echo "🐹 Go not found. Installing golang-go..."
    apt install -y golang-go
fi

# 4. Clone or update repository
INSTALL_DIR="/opt/samba-blackjack-panel"

if [ -d "$INSTALL_DIR" ]; then
    echo "🔄 Existing version detected. Updating..."
    # Stop service before building
    systemctl stop samba-blackjack.service || true
    cd "$INSTALL_DIR"
    
    # Reset local changes and pull latest code
    git fetch --all
    git reset --hard origin/main
else
    echo "📥 Cloning repository..."
    git clone https://github.com/kirpicheff/samba-blackjack-panel.git "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# 5. Build binary
echo "🛠 Building application..."
go mod tidy
go build -o samba-blackjack-panel .

# 6. Create systemd service
echo "⚙️ Creating system service..."
cat > /etc/systemd/system/samba-blackjack.service <<EOF
[Unit]
Description=Samba Blackjack Panel
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/samba-blackjack-panel
ExecStart=/opt/samba-blackjack-panel/samba-blackjack-panel
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# 7. Start services
echo "🔄 Starting services..."
systemctl daemon-reload
systemctl enable samba-blackjack.service avahi-daemon wsdd2
systemctl start samba-blackjack.service avahi-daemon wsdd2

# 8. Final message
echo ""
echo "===================================================="
echo "✅ Installation successfully completed!"
echo "🌐 Panel available at: http://$(hostname -I | awk '{print $1}'):8888"
echo "🔐 Default Login: admin"
echo "🔑 Default Password: admin"
echo ""
echo "💡 It is recommended to change the password after login."
echo "📜 Service logs: journalctl -u samba-blackjack -f"
echo "===================================================="
