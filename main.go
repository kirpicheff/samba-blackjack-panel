package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// Константы путей (на Linux это /etc/samba/smb.conf)
const smbConfPath = "/etc/samba/smb.conf"
const devSmbConfPath = "smb.conf.dev" // для тестов на Windows

type SambaStatus struct {
	Timestamp string              `json:"timestamp"`
	Version   string              `json:"version"`
	Sessions  map[string]Session  `json:"sessions"`
	Tcons     map[string]Tcon     `json:"tcons"`
	OpenFiles map[string]OpenFile `json:"open_files"`
}

var sessions = make(map[string]time.Time)
const adminPass = "admin" // Упрощенно для начала
const sessionCookieName = "samba_session"

type Session struct {
	RemoteMachine string `json:"remote_machine"`
	User          string `json:"user"`
	Protocol      string `json:"protocol_version"`
}

type Tcon struct {
	Service string `json:"service"`
	User    string `json:"user"`
}

type OpenFile struct {
	Path string `json:"path"`
	User string `json:"user"`
}

type SambaUser struct {
	Username string `json:"username"`
	UID      string `json:"uid"`
	FullName string `json:"full_name"`
}

type ShareInfo struct {
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Comment   string            `json:"comment"`
	IsRecycle bool              `json:"is_recycle"`
	IsAudit   bool              `json:"is_audit"`
	AuditOpen bool              `json:"audit_open"`
	Params    map[string]string `json:"params"`
}

type GlobalConfig struct {
	Params map[string]string `json:"params"`
}

type DiskUsage struct {
	Path       string  `json:"path"`
	MountPoint string  `json:"mount_point"`
	Total      string  `json:"total"`
	Used       string  `json:"used"`
	Free       string  `json:"free"`
	Percent    float64 `json:"percent"`
}

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Action    string `json:"action"`
	File      string `json:"file"`
}

// getSambaStatus вызывает smbstatus --json
func getSambaStatus(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("smbstatus", "--json")
	output, err := cmd.Output()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": "MOCK-MODE", "sessions": {}, "open_files": {}}`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

// getShares парсит smb.conf и возвращает список ресурсов
func getShares(w http.ResponseWriter, r *http.Request) {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Не удалось прочитать smb.conf", 500)
		return
	}

	var shares []ShareInfo
	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" || name == "global" {
			continue
		}

		share := ShareInfo{
			Name:   name,
			Path:   section.Key("path").String(),
			Params: section.KeysHash(),
		}
		
		// Проверяем наличие корзины и аудита в vfs objects
		vfs := section.Key("vfs objects").String()
		share.IsRecycle = strings.Contains(vfs, "recycle")
		share.IsAudit = strings.Contains(vfs, "full_audit")
		share.AuditOpen = strings.Contains(section.Key("full_audit:success").String(), "open")
		
		shares = append(shares, share)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shares)
}

// saveShare сохраняет или обновляет ресурс в smb.conf
func saveShare(w http.ResponseWriter, r *http.Request) {
	var share ShareInfo
	if err := json.NewDecoder(r.Body).Decode(&share); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Failed to load smb.conf", 500)
		return
	}

	cfg.DeleteSection(share.Name) // Пересоздаем для чистоты
	section, _ := cfg.NewSection(share.Name)
	section.Key("path").SetValue(share.Path)
	if share.Comment != "" {
		section.Key("comment").SetValue(share.Comment)
	}

	for k, v := range share.Params {
		if k == "path" || k == "comment" || k == "vfs objects" || strings.Contains(k, "recycle:") || strings.Contains(k, "full_audit:") {
			continue
		}
		if v != "" {
			section.Key(k).SetValue(v)
		} else {
			section.DeleteKey(k)
		}
	}

	vfs := []string{"acl_xattr"}
	if share.IsRecycle {
		vfs = append(vfs, "recycle")
		repo := share.Params["recycle:repository"]
		if repo == "" {
			repo = ".recycle/%U"
			if share.Params["guest ok"] == "yes" { repo = ".recycle/guest" }
		}
		exclude := share.Params["recycle:exclude"]
		if exclude == "" { exclude = "*.tmp *.temp ~$* *.bak Thumbs.db" }
		excludeDir := share.Params["recycle:exclude_dir"]
		if excludeDir == "" { excludeDir = "/tmp /cache .recycle" }

		section.Key("recycle:repository").SetValue(repo)
		section.Key("recycle:keeptree").SetValue("yes")
		section.Key("recycle:versions").SetValue("yes")
		section.Key("recycle:touch").SetValue("yes")
		section.Key("recycle:directory_mode").SetValue("0770")
		section.Key("recycle:exclude").SetValue(exclude)
		section.Key("recycle:exclude_dir").SetValue(excludeDir)
	} else {
		// Чистим старые параметры recycle
		for _, k := range section.KeyStrings() {
			if strings.HasPrefix(k, "recycle:") { section.DeleteKey(k) }
		}
	}

	if share.IsAudit {
		vfs = append(vfs, "full_audit")
		section.Key("full_audit:prefix").SetValue("%u|%I|%m|%S")
		success := "mkdir rename unlink"
		if share.AuditOpen { success += " open" }
		section.Key("full_audit:success").SetValue(success)
		section.Key("full_audit:failure").SetValue("none")
		section.Key("full_audit:facility").SetValue("local7")
		section.Key("full_audit:priority").SetValue("NOTICE")
	} else {
		// Чистим старые параметры audit
		for _, k := range section.KeyStrings() {
			if strings.HasPrefix(k, "full_audit:") { section.DeleteKey(k) }
		}
	}
	section.Key("vfs objects").SetValue(strings.Join(vfs, " "))

	if err := cfg.SaveTo(path); err != nil {
		http.Error(w, "Failed to save smb.conf", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deleteShare удаляет секцию из smb.conf
func deleteShare(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Failed to load smb.conf", 500)
		return
	}

	cfg.DeleteSection(req.Name)
	cfg.SaveTo(path)
	w.WriteHeader(http.StatusOK)
}

// getGlobalConfig возвращает параметры секции [global]
func getGlobalConfig(w http.ResponseWriter, r *http.Request) {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Failed to load smb.conf", 500)
		return
	}

	global := GlobalConfig{
		Params: cfg.Section("global").KeysHash(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(global)
}

// saveGlobalConfig сохраняет параметры в секцию [global]
func saveGlobalConfig(w http.ResponseWriter, r *http.Request) {
	var config GlobalConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Failed to load smb.conf", 500)
		return
	}

	section := cfg.Section("global")
	for k, v := range config.Params {
		if v != "" {
			section.Key(k).SetValue(v)
		} else {
			section.DeleteKey(k)
		}
	}

	if err := cfg.SaveTo(path); err != nil {
		http.Error(w, "Failed to save smb.conf", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getUsers возвращает список пользователей Samba через pdbedit -L
func getUsers(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("pdbedit", "-L")
	output, err := cmd.Output()
	if err != nil {
		// Режим разработки или отсутствие pdbedit
		users := []SambaUser{
			{Username: "admin", UID: "1000", FullName: "System Administrator"},
			{Username: "user1", UID: "1001", FullName: "Иван Иванов"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
		return
	}

	var users []SambaUser
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 3 {
			users = append(users, SambaUser{
				Username: parts[0],
				UID:      parts[1],
				FullName: strings.TrimSpace(parts[2]),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// saveUserHandler создает пользователя или меняет пароль
func saveUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	// 1. Проверяем, существует ли пользователь в Samba
	checkCmd := exec.Command("pdbedit", "-L", "-u", req.Username)
	err := checkCmd.Run()

	var cmd *exec.Cmd
	if err != nil {
		// Пользователя нет, создаем (на Linux требуется существующий системный пользователь)
		// Для простоты используем smbpasswd -a
		cmd = exec.Command("smbpasswd", "-a", "-s", req.Username)
	} else {
		// Пользователь есть, меняем пароль
		cmd = exec.Command("smbpasswd", "-s", req.Username)
	}

	cmd.Stdin = strings.NewReader(req.Password + "\n" + req.Password + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		// В Mock-режиме (Windows) просто возвращаем успех для UI
		if os.PathSeparator == '\\' {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Ошибка Samba: "+string(output), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteUserHandler удаляет пользователя Samba
func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username string `json:"username"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	cmd := exec.Command("pdbedit", "-x", "-u", req.Username)
	if output, err := cmd.CombinedOutput(); err != nil {
		if os.PathSeparator == '\\' {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Ошибка при удалении: "+string(output), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// applyServiceConfig проверяет конфиг и перезапускает Samba
func applyServiceConfig(w http.ResponseWriter, r *http.Request) {
	// 1. Проверка testparm
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	testCmd := exec.Command("testparm", "-s", path)
	if output, err := testCmd.CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(output)
		return
	}

	// 2. Перезапуск (только на Linux)
	if runtime.GOOS == "linux" {
		reloadCmd := exec.Command("systemctl", "reload", "smbd")
		if err := reloadCmd.Run(); err != nil {
			http.Error(w, "Failed to reload smbd: "+err.Error(), 500)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Settings applied successfully"))
}

// getServiceStatus возвращает текущий статус smbd
func getServiceStatus(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		// Mock для Windows
		w.Write([]byte("active"))
		return
	}

	cmd := exec.Command("systemctl", "is-active", "smbd")
	output, _ := cmd.Output()
	w.Write([]byte(strings.TrimSpace(string(output))))
}

// controlServiceHandler управляет состоянием сервиса (start, stop, restart)
func controlServiceHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		return
	}

	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[req.Action] {
		http.Error(w, "Invalid action", 400)
		return
	}

	cmd := exec.Command("systemctl", req.Action, "smbd")
	if output, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, "Service error: "+string(output), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// getLogsHandler читает последние строки из файла логов
func getLogsHandler(w http.ResponseWriter, r *http.Request) {
	logFile := "/var/log/samba/log.smbd"
	if os.PathSeparator == '\\' {
		// Mock для разработки
		w.Write([]byte("[2026/04/30 13:40:00, 0] ../../source3/smbd/server.c:1738(main)\n  smbd version 4.15.13-Ubuntu started.\n  Copyright Andrew Tridgell and the Samba Team 1992-2021\n[2026/04/30 13:42:15, 1] ../../source3/smbd/service.c:1123(make_connection_snum)\n  connect to service data by user admin\n"))
		return
	}

	// Читаем последние 200 строк через tail для скорости
	cmd := exec.Command("tail", "-n", "200", logFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		w.Write([]byte("Ошибка чтения логов: " + err.Error() + "\n" + string(output)))
		return
	}
	w.Write(output)
}

// getDiskUsageHandler возвращает информацию о дисках, на которых лежат шары
func getDiskUsageHandler(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		// Mock для Windows
		usage := []DiskUsage{
			{Path: "/data", MountPoint: "/dev/sda1", Total: "1.8T", Used: "450G", Free: "1.3T", Percent: 25},
			{Path: "/mnt/backup", MountPoint: "/dev/sdb1", Total: "4.0T", Used: "3.2T", Free: "800G", Percent: 80},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(usage)
		return
	}

	// 1. Получаем список уникальных путей из конфига
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}
	cfg, _ := ini.Load(path)
	
	uniquePaths := make(map[string]bool)
	for _, section := range cfg.Sections() {
		p := section.Key("path").String()
		if p != "" {
			uniquePaths[p] = true
		}
	}

	var results []DiskUsage
	seenMounts := make(map[string]bool)

	for p := range uniquePaths {
		// Выполняем df -h для конкретного пути
		cmd := exec.Command("df", "-h", "--output=source,size,used,avail,pcent,target", p)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) < 2 {
			continue
		}

		// Парсим вторую строку (первая - заголовок)
		fields := strings.Fields(lines[1])
		if len(fields) < 6 {
			continue
		}

		mount := fields[5]
		if seenMounts[mount] {
			continue
		}
		seenMounts[mount] = true

		percentStr := strings.TrimSuffix(fields[4], "%")
		var percent float64
		fmt.Sscanf(percentStr, "%f", &percent)

		results = append(results, DiskUsage{
			Path:       p,
			MountPoint: fields[0],
			Total:      fields[1],
			Used:       fields[2],
			Free:       fields[3],
			Percent:    percent,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// getAuditLogsHandler парсит логи аудита из системного лога
func getAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		// Mock для разработки
		logs := []AuditEntry{
			{Timestamp: "2026/04/30 13:45:10", User: "admin", IP: "192.168.1.10", Action: "unlink", File: "secret_report.docx"},
			{Timestamp: "2026/04/30 13:48:22", User: "ivan", IP: "192.168.1.15", Action: "rename", File: "photo.jpg -> profile.jpg"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
		return
	}

	// Читаем из /var/log/syslog (или /var/log/samba/log.audit если настроено)
	// Для универсальности ищем в syslog
	cmd := exec.Command("grep", "smbd_audit", "/var/log/syslog")
	output, _ := cmd.CombinedOutput()
	
	var entries []AuditEntry
	lines := strings.Split(string(output), "\n")
	
	// Берем последние 100 записей
	start := len(lines) - 100
	if start < 0 { start = 0 }

	for i := len(lines) - 1; i >= start; i-- {
		line := lines[i]
		if !strings.Contains(line, "smbd_audit:") { continue }
		
		parts := strings.Split(line, "smbd_audit:")
		if len(parts) < 2 { continue }
		
		msg := strings.TrimSpace(parts[1])
		data := strings.Split(msg, "|")
		if len(data) < 7 { continue }
		
		entries = append(entries, AuditEntry{
			Timestamp: strings.Fields(line)[0] + " " + strings.Fields(line)[1] + " " + strings.Fields(line)[2],
			User:      data[0],
			IP:        data[1],
			Action:    data[4],
			File:      data[6],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// loginHandler проверяет пароль и выдает сессию
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var creds struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if creds.Password == adminPass {
		token := "session-" + fmt.Sprint(time.Now().UnixNano())
		sessions[token] = time.Now().Add(24 * time.Hour)
		
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(24 * time.Hour),
		})
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Unauthorized", 401)
}

// logoutHandler удаляет сессию
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		delete(sessions, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.WriteHeader(http.StatusOK)
}

// authMiddleware проверяет наличие сессии
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Error(w, "Unauthorized", 401)
			return
		}

		expiry, v := sessions[cookie.Value]
		if !v || time.Now().After(expiry) {
			http.Error(w, "Unauthorized", 401)
			return
		}

		next(w, r)
	}
}

func main() {
	// Если мы на Windows, создадим тестовый конфиг для разработки
	if _, err := os.Stat(smbConfPath); os.IsNotExist(err) {
		if _, err := os.Stat(devSmbConfPath); os.IsNotExist(err) {
			f, _ := os.Create(devSmbConfPath)
			f.WriteString("[global]\n  workgroup = WORKGROUP\n\n[Обмен]\n  path = /tmp\n  vfs objects = acl_xattr recycle\n")
			f.Close()
		}
	}

	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/logout", logoutHandler)
	http.HandleFunc("/api/shares/save", authMiddleware(saveShare))
	http.HandleFunc("/api/shares/delete", authMiddleware(deleteShare))
	http.HandleFunc("/api/global", authMiddleware(getGlobalConfig))
	http.HandleFunc("/api/global/save", authMiddleware(saveGlobalConfig))
	http.HandleFunc("/api/service/apply", authMiddleware(applyServiceConfig))
	http.HandleFunc("/api/service/status", authMiddleware(getServiceStatus))
	http.HandleFunc("/api/service/control", authMiddleware(controlServiceHandler))
	http.HandleFunc("/api/logs", authMiddleware(getLogsHandler))
	http.HandleFunc("/api/audit", authMiddleware(getAuditLogsHandler))
	http.HandleFunc("/api/disk/usage", authMiddleware(getDiskUsageHandler))
	http.HandleFunc("/api/status", authMiddleware(getSambaStatus))
	http.HandleFunc("/api/shares", authMiddleware(getShares))
	http.HandleFunc("/api/users", authMiddleware(getUsers))
	http.HandleFunc("/api/users/save", authMiddleware(saveUserHandler))
	http.HandleFunc("/api/users/delete", authMiddleware(deleteUserHandler))

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	port := ":8888"
	fmt.Printf("🚀 Samba Blackjack Panel запущен на http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
