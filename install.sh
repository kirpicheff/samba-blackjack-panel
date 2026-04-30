#!/bin/bash
# install.sh для Samba Blackjack Panel

set -e

echo "🚀 Устанавливаю Samba Blackjack Panel..."

# 1. Проверка, что запущено от root
if [ "$EUID" -ne 0 ]; then 
    echo "❌ Пожалуйста, запустите с sudo: sudo ./install.sh"
    exit 1
fi

# 2. Установка зависимостей системы
echo "📦 Установка системных зависимостей (Samba)..."
apt update
apt install -y samba samba-common-bin git

# 3. Проверка Go
if ! command -v go &> /dev/null; then
    echo "🐹 Go не установлен. Устанавливаю golang-go..."
    apt install -y golang-go
fi

# 4. Клонирование репозитория в /opt
echo "📥 Клонирование репозитория..."
rm -rf /opt/samba-blackjack-panel
git clone https://github.com/kirpicheff/samba-blackjack-panel.git /opt/samba-blackjack-panel

# 5. Сборка бинарника
echo "🛠 Сборка приложения..."
cd /opt/samba-blackjack-panel
go mod tidy
go build -o samba-blackjack-panel .

# 6. Создание systemd сервиса
echo "⚙️ Создание системной службы..."
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

# 7. Запуск сервиса
echo "🔄 Запуск службы..."
systemctl daemon-reload
systemctl enable samba-blackjack.service
systemctl start samba-blackjack.service

# 8. Финальное сообщение
echo ""
echo "===================================================="
echo "✅ Установка успешно завершена!"
echo "🌐 Панель доступна по адресу: http://$(hostname -I | awk '{print $1}'):8888"
echo "🔐 Логин: admin"
echo "🔑 Пароль: admin"
echo ""
echo "💡 Рекомендуется сменить пароль сразу после входа."
echo "📜 Логи сервиса: journalctl -u samba-blackjack -f"
echo "===================================================="
