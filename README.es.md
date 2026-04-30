# Samba Blackjack Panel
🌐 [English](README.en.md) | [中文](README.zh.md) | [Español](README.es.md) | [Русский](README.md)

Panel de administración ligero para servidores Samba escrito en Go.

## 🚀 Instalación Rápida (Ubuntu/Debian)

La forma más sencilla de instalar el panel como un servicio del sistema:

```bash
wget https://raw.githubusercontent.com/kirpicheff/samba-blackjack-panel/main/install.sh
chmod +x install.sh
sudo ./install.sh
```

Después de la instalación, el panel estará disponible en: `http://su-ip:8888`.
- **Usuario:** `admin`
- **Contraseña:** `admin` (cámbiela inmediatamente en "Configuración del servidor" -> "Administradores del panel")

---

## ✨ Características

### 📊 Panel y Monitoreo
- **Sesiones Activas**: Ver usuarios conectados, sus IPs y versiones del protocolo SMB.
- **Archivos Abiertos**: Lista de todos los archivos en uso por los clientes en este momento.
- **Estado de Discos**: Monitoreo de espacio libre en particiones con carpetas compartidas.
- **Servicios**: Control de estado para WSDD (Windows) y Avahi (macOS/Linux).

### 📂 Gestión de Recursos (Shares)
- **Creación y Edición**: Gestión completa de secciones `smb.conf` a través de la UI.
- **Papelera de Red**: Limpieza automática, exclusión de archivos, configuración de rutas.
- **Auditoría**: Registro de acciones (eliminar, renombrar) en un log integrado.
- **Permisos de FS (ACL)**: Editor visual de propietario, grupo y derechos de acceso (`chmod`) con máscara octal y matriz de casillas.
- **Restricción por IP**: Configuración de `hosts allow` y `hosts deny` globalmente y por recurso.
- **Copias de Sombra**: Soporte para VFS Shadow Copy 2 para recuperación de archivos.

### 👥 Usuarios y Grupos
- **Usuarios Samba**: Gestión de cuentas a través de `pdbedit` (creación, contraseñas, eliminación).
- **Grupos de SO**: Gestión de grupos del sistema y membresía de usuarios.

### 🌐 Active Directory (AD)
- **Unión Automática**: Configuración de Kerberos, Winbind y ejecución de Join.
- **Health Check**: Diagnóstico profundo de la conexión con el DC (Confianza, Hora, RPC, Keytab).

### ⚙️ Configuración y Seguridad
- **Parámetros Globales**: Configuración de Workgroup, Netbios Name, interfaces de red.
- **Control de Servicios**: Start/Stop/Restart para smbd, nmbd, winbind.
- **Respaldos Automáticos**: Almacenamiento de las últimas 10 versiones de `smb.conf`.
- **Acceso al Panel**: Inicio de sesión multiusuario, hashing de contraseñas con Bcrypt, gestión de administradores.

### 📜 Logs y Automatización
- **Live Logs**: Visualización en tiempo real de `log.smbd` a través de WebSockets.
- **Registro de Auditoría**: Historial de acciones de usuario en una tabla integrada.
- **Tareas en Segundo Plano**: Limpieza automática de papeleras y creación de instantáneas (snapshots).

---

## 💻 Modo de Desarrollo (Windows/macOS)

Si ejecuta el panel en sistemas que no son Linux, cambia automáticamente al **Modo Mock**:
- Utiliza datos de prueba en lugar de llamadas reales al sistema.
- Crea un archivo `smb.conf.dev` local para simular la configuración.

**Ejecución para desarrollo:**
1. Asegúrese de tener Go instalado.
2. Clone el repositorio.
3. Ejecute: `go run .`
4. Abra `http://localhost:8888`.

---

## 🛠 Instalación Manual y Recuperación

### Requisitos del Sistema (Linux)
Para que todas las funciones funcionen:
```bash
sudo apt update
sudo apt install samba samba-common-bin krb5-user winbind avahi-daemon acl
```

### Recuperación de Acceso
Las contraseñas de los administradores del panel se almacenan en `admins.json`. Si pierde el acceso:
1. Elimine el archivo `admins.json`.
2. Reinicie el panel.
3. Use las credenciales predeterminadas: `admin / admin`.
