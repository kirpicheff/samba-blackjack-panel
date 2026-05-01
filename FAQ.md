# Frequently Asked Questions (FAQ) / Часто задаваемые вопросы

🌐 [English Version](#english-version) | 🇷🇺 [Русская версия](#русская-версия)

---

<a name="english-version"></a>
## 🇬🇧 English Version

### 1. General Questions
**Q: I lost my admin password. How can I reset it?**
A: Delete the `admins.json` file in the project root and restart the panel. It will revert to the default credentials: `admin` / `admin`.

**Q: Can I use this panel on Windows to manage a remote Linux server?**
A: No, the panel must be installed directly on the Linux server where Samba is running. However, you can run it on Windows in **Mock-mode** for development and testing.

**Q: Does it support Docker?**
A: Currently, the panel is designed to run on bare-metal or VMs because it needs direct access to system services (`systemd`), filesystem ACLs, and kernel-level quotas.

### 2. Technical Issues
**Q: The panel says "Samba service not found".**
A: Ensure that `samba` (specifically `smbd`) is installed on your system and visible to `systemctl`. Run `sudo apt install samba`.

**Q: Disk Quotas are not showing any data.**
A: Your filesystem must support quotas. Add `usrquota,grpquota` to the mount options in `/etc/fstab`, remount the partition, and run `quotacheck -cum /mount/point`.

**Q: I cannot join Active Directory.**
A: Check if your server's time is synchronized with the Domain Controller. Use `ntpdate` or `chrony`. Also, ensure `krb5-user` and `winbind` are installed.

---

<a name="русская-версия"></a>
## 🇷🇺 Русская версия

### 1. Общие вопросы
**В: Я потерял пароль администратора. Как его сбросить?**
О: Удалите файл `admins.json` в корневой директории проекта и перезапустите панель. Она вернется к стандартным данным: `admin` / `admin`.

**В: Могу ли я запустить панель на Windows для управления удаленным Linux-сервером?**
О: Нет, панель должна быть установлена непосредственно на том Linux-сервере, где работает Samba. Однако вы можете запустить её на Windows в **Mock-режиме** для ознакомления или разработки.

**В: Поддерживается ли Docker?**
О: На данный момент панель рассчитана на работу в основной системе или виртуальной машине, так как ей требуется прямой доступ к системным службам (`systemd`), ACL файловой системы и ядерным механизмам квот.

### 2. Технические проблемы
**В: Панель пишет "Samba service not found".**
О: Убедитесь, что Samba установлена в системе. Выполните `sudo apt install samba`. Также проверьте, что служба `smbd` видна в `systemctl`.

**В: В разделе "Дисковые квоты" нет данных.**
О: Ваша файловая система должна поддерживать квоты. Добавьте `usrquota,grpquota` в опции монтирования в `/etc/fstab`, пересмонтируйте раздел и выполните `quotacheck -cum /путь/к/разделу`.

**В: Не получается войти в Active Directory (Join AD).**
О: Проверьте синхронизацию времени с контроллером домена (разница должна быть менее 5 минут). Также убедитесь, что установлены пакеты `krb5-user` и `winbind`.
