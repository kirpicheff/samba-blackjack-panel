# Samba Blackjack Panel: Full Product Guide / Полное руководство

🌐 [English Version](#english-version) | 🇷🇺 [Русская версия](#русская-версия)

---

<a name="english-version"></a>
## 🇬🇧 English Version

### 1. Dashboard (Monitoring)
The main screen provides a bird's-eye view of your server's health and activity.

*   **Samba Status**: Shows if the `smbd` service is running or stopped.
*   **Active Sessions**: Real-time list of users connected to the server, including their IP addresses and the SMB protocol version they are using (e.g., SMB3_11).
*   **Open Files**: Displays which files are currently being read or written to by clients.
*   **Disk Usage**: Visual progress bars showing the free space on partitions where your shares are located.
*   **Network Discovery**: Status of WSDD and Avahi services that help your server appear automatically in Windows "Network" and macOS "Finder".

### 2. Share Management (Resources)
This is the core of the panel where you manage your folders.

#### General Settings
*   **Name**: The name of the folder as it appears in the network.
*   **Path**: The actual location on the server (e.g., `/mnt/data/files`).
*   **Comment**: A description for the network resource.
*   **Read Only**: If checked, users can only download files, not upload or delete.
*   **Guest OK**: Allows access without a password (anonymous access).
*   **Browseable**: If unchecked, the folder is hidden from the network list (you must enter the path manually to access it).

#### Advanced Access & VFS
*   **Recycle Bin**: 
    *   **Repository**: Where deleted files are moved (e.g., `.recycle/%U`).
    *   **Exclude**: File types that should NOT be moved to the recycle bin (e.g., `*.tmp`).
*   **Full Audit**: Logs user actions. You can choose to log `open`, `mkdir`, `rename`, and `unlink` (delete) operations.
*   **Shadow Copy**: Enables Windows "Previous Versions" support (requires filesystem snapshots).
*   **IP Restrictions**:
    *   **Hosts Allow**: List of subnets or IPs allowed to access this share (e.g., `192.168.1.0/24`).
    *   **Hosts Deny**: List of subnets or IPs blocked from this share.

### 3. Users & Groups
Management of access credentials.

*   **Samba Users**: Separate from system users, these are accounts used specifically for SMB access. You can create, delete, and change passwords.
*   **OS Groups**: System-level groups. You can add Samba users to these groups to grant them filesystem permissions.
*   **Admin Management**: In the "Server Settings", you can manage the administrators who can log into this panel.

### 4. Global Server Settings
Settings that affect the entire Samba server.

*   **Workgroup**: The name of your network group (default: `WORKGROUP`).
*   **Netbios Name**: The name of the server in the network (e.g., `NAS-SERVER`).
*   **Interfaces**: Specific network interfaces Samba should listen on (e.g., `eth0 lo`).
*   **Security Mode**: Usually `user` for standard password authentication.
*   **Service Control**: Buttons to Start, Stop, or Restart `smbd`, `nmbd`, and `winbind`.
*   **Apply Changes**: A critical button that validates your `smb.conf` with `testparm` and reloads the configuration if everything is correct.

### 5. Active Directory (AD) Integration
For connecting your server to a corporate domain.

*   **Join Domain**: Enter the Realm, Admin name, and Password to join the AD.
*   **AD Health**: A diagnostic tool that checks Kerberos tickets, time synchronization with the DC, and RPC connectivity.

### 6. File Manager & ACLs
Direct control over the server's filesystem.

*   **File Browser**: Basic operations like creating folders, renaming, and deleting files.
*   **ACL Editor**: 
    *   **Owner/Group**: Change who owns the folder.
    *   **Permissions Matrix**: Visual checkboxes for Read, Write, and Execute for User, Group, and Others.
    *   **Octal Mode**: Direct input of permissions like `0775`.
    *   **Recursive**: Apply these permissions to all subfolders and files inside.

### 7. Disk Quotas
Enforcing limits on how much space users can take.

*   **Soft Limit**: A warning limit. The user can still write but will receive a warning.
*   **Hard Limit**: A strict limit. The user cannot write more than this amount.
*   **Usage**: Real-time tracking of used space vs. limits.

### 8. Automation & Logs
*   **Live Logs**: Stream of `log.smbd` output. Useful for troubleshooting connection issues.
*   **Audit Journal**: A table of actions recorded by the Full Audit module.
*   **Recycle Bin Auto-Clear**: A background task that deletes old files from the `.recycle` folders after a specified number of days.

---

<a name="русская-версия"></a>
## 🇷🇺 Русская версия

### 1. Дашборд (Мониторинг)
Главный экран для быстрого контроля состояния сервера.

*   **Статус Samba**: Показывает, запущена ли служба `smbd`.
*   **Активные сессии**: Список пользователей, подключенных в данный момент, их IP-адреса и версия используемого протокола (например, SMB3_11).
*   **Открытые файлы**: Список файлов, которые сейчас читаются или записываются клиентами.
*   **Использование дисков**: Наглядные индикаторы свободного места на разделах, где расположены ваши сетевые папки.
*   **Сетевое обнаружение**: Статус служб WSDD и Avahi, благодаря которым сервер автоматически виден в "Сетевом окружении" Windows и Finder на macOS.

### 2. Управление ресурсами (Shares)
Основной раздел для настройки сетевых папок.

#### Общие настройки
*   **Имя**: Название папки, как его увидят пользователи в сети.
*   **Путь**: Реальный путь к папке на сервере (например, `/mnt/data/files`).
*   **Комментарий**: Описание ресурса.
*   **Только чтение (Read Only)**: Если включено, пользователи смогут только скачивать файлы, но не изменять или удалять их.
*   **Гостевой вход (Guest OK)**: Разрешает доступ без пароля (анонимно).
*   **Видимость (Browseable)**: Если выключено, папка будет скрыта из списка сетевых ресурсов (в неё можно будет зайти только введя путь вручную).

#### Расширенный доступ и VFS
*   **Корзина (Recycle Bin)**: 
    *   **Хранилище**: Куда перемещаются удаленные файлы (например, `.recycle/%U`).
    *   **Исключения**: Типы файлов, которые НЕ нужно помещать в корзину (например, `*.tmp`).
*   **Аудит (Full Audit)**: Запись действий пользователей. Можно выбрать логирование операций открытия (`open`), создания папок (`mkdir`), переименования (`rename`) и удаления (`unlink`).
*   **Теневые копии (Shadow Copy)**: Включение поддержки "Предыдущих версий" Windows (требует настройки снимков ФС).
*   **Ограничение по IP**:
    *   **Hosts Allow**: Список подсетей или IP, которым разрешен доступ (например, `192.168.1.0/24`).
    *   **Hosts Deny**: Список запрещенных подсетей или IP.

### 3. Пользователи и Группы
Управление правами доступа.

*   **Samba пользователи**: Отдельные от системных аккаунты, используемые именно для доступа по сети. Можно создавать, удалять и менять пароли.
*   **Группы ОС**: Системные группы. Вы можете добавлять пользователей Samba в эти группы для управления правами на уровне файлов.
*   **Администраторы панели**: В настройках сервера можно управлять списком пользователей, имеющих доступ к самой панели управления.

### 4. Настройки сервера (Global)
Параметры, влияющие на весь сервер Samba.

*   **Workgroup**: Имя рабочей группы (по умолчанию `WORKGROUP`).
*   **Netbios Name**: Имя сервера в сети (например, `NAS-SERVER`).
*   **Интерфейсы**: Список сетевых интерфейсов, на которых будет работать Samba (например, `eth0 lo`).
*   **Режим безопасности**: Обычно `user` — доступ по логину и паролю.
*   **Управление службой**: Кнопки Старт, Стоп и Рестарт для `smbd`, `nmbd` и `winbind`.
*   **Применить изменения**: Важнейшая кнопка, которая проверяет ваш `smb.conf` через `testparm` и, если ошибок нет, перезагружает конфигурацию.

### 5. Active Directory (AD)
Интеграция сервера в корпоративный домен.

*   **Ввод в домен**: Поля для ввода Realm (домена), имени администратора и пароля для выполнения Join.
*   **Здоровье AD**: Диагностика связи с контроллером домена, проверки билетов Kerberos и синхронизации времени.

### 6. Файлы и Права (ACL)
Прямое управление файловой системой сервера.

*   **Файловый менеджер**: Базовые операции: создание папок, переименование и удаление файлов.
*   **Редактор ACL**: 
    *   **Владелец/Группа**: Смена владельца папки.
    *   **Матрица прав**: Галочки Чтение, Запись, Выполнение для Владельца, Группы и Остальных.
    *   **Числовой код**: Прямой ввод маски прав (например, `0775`).
    *   **Рекурсивно**: Применить эти права ко всем вложенным файлам и папкам.

### 7. Дисковые квоты
Ограничение использования места пользователями.

*   **Soft Limit**: Предупредительный лимит. Пользователь сможет писать дальше, но получит предупреждение.
*   **Hard Limit**: Строгий лимит. Пользователь не сможет записать больше этого объема.
*   **Использование**: Отслеживание в реальном времени объема занятого места относительно лимитов.

### 8. Автоматизация и Логи
*   **Живые логи**: Поток данных из `log.smbd`. Полезно для отладки проблем с подключением.
*   **Журнал аудита**: Таблица действий, записанных модулем Full Audit.
*   **Очистка корзины**: Фоновая задача, удаляющая старые файлы из папок `.recycle` через заданное количество дней.
