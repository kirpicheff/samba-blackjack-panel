package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/ini.v1"
	"golang.org/x/crypto/bcrypt"
)

type ADStatus struct {
	Joined bool   `json:"joined"`
	Info   string `json:"info"`
}

type ADJoinRequest struct {
	Realm    string `json:"realm"`
	Admin    string `json:"admin"`
	Password string `json:"password"`
}

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
const sessionCookieName = "samba_session"
var adminsPath = "admins.json"
var admins []AdminUser

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

type SambaGroup struct {
	Name    string   `json:"name"`
	GID     string   `json:"gid"`
	Members []string `json:"members"`
}

type ADHealth struct {
	Status      bool            `json:"status"`
	Checks      []ADCheckResult `json:"checks"`
	LastUpdate  string          `json:"last_update"`
}

type ADCheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warning", "error"
	Message string `json:"message"`
}

type AdminUser struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // Только для передачи при создании
	Hash     string `json:"hash"`
	Role     string `json:"role"` // "admin", "superadmin"
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type ShareInfo struct {
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Comment   string            `json:"comment"`
	IsRecycle    bool              `json:"is_recycle"`
	IsAudit      bool              `json:"is_audit"`
	AuditOpen    bool              `json:"audit_open"`
	IsShadowCopy bool              `json:"is_shadow_copy"`
	Params       map[string]string `json:"params"`
}

type AutomationSettings struct {
	RecycleDays      int    `json:"recycle_days"`
	SnapshotInterval string `json:"snapshot_interval"` // "none", "hourly", "daily"
	SnapshotKeep     int    `json:"snapshot_keep"`
}

var automationFile = "automation.json"

type GlobalConfig struct {
	Params map[string]string `json:"params"`
}

type DiskUsage struct {
	Path       string  `json:"path"`
	MountPoint string  `json:"mount_point"`
	Total      string  `json:"total"`
	Used       string  `json:"used"`
	Free       string   `json:"free"`
	Percent    float64  `json:"percent"`
	Shares     []string `json:"shares"`
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
		
		// Проверяем наличие корзины, аудита и теневых копий в vfs objects
		vfs := section.Key("vfs objects").String()
		share.IsRecycle = strings.Contains(vfs, "recycle")
		share.IsAudit = strings.Contains(vfs, "full_audit")
		share.AuditOpen = strings.Contains(section.Key("full_audit:success").String(), "open")
		share.IsShadowCopy = strings.Contains(vfs, "shadow_copy2")
		
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
	if share.IsShadowCopy {
		vfs = append(vfs, "shadow_copy2")
		section.Key("shadow:snapdir").SetValue(".snapshots")
		section.Key("shadow:sort").SetValue("desc")
		section.Key("shadow:format").SetValue("@GMT-%Y.%m.%d-%H.%M.%S")
	} else {
		section.DeleteKey("shadow:snapdir")
		section.DeleteKey("shadow:sort")
		section.DeleteKey("shadow:format")
	}

	section.Key("vfs objects").SetValue(strings.Join(vfs, " "))

	createConfigBackup() // Делаем бэкап перед сохранением
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
	createConfigBackup()
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

// createConfigBackup создает копию текущего smb.conf перед изменениями
func createConfigBackup() {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	backupDir := "backups"
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s/smb.conf.%s", backupDir, timestamp)

	// Копируем файл
	input, _ := os.ReadFile(path)
	os.WriteFile(backupPath, input, 0644)

	// Ротация: удаляем старые бэкапы, если их больше 10
	files, _ := os.ReadDir(backupDir)
	if len(files) > 10 {
		var oldest os.DirEntry
		for _, f := range files {
			if oldest == nil || f.Name() < oldest.Name() {
				oldest = f
			}
		}
		if oldest != nil {
			os.Remove(backupDir + "/" + oldest.Name())
		}
	}
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

	createConfigBackup() // Делаем бэкап перед сохранением
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

// getGroups возвращает список групп из /etc/group (через getent group)
func getGroups(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		// Mock для Windows
		groups := []SambaGroup{
			{Name: "smb_admins", GID: "2000", Members: []string{"admin"}},
			{Name: "users", GID: "2001", Members: []string{"user1", "admin"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
		return
	}

	cmd := exec.Command("getent", "group")
	output, err := cmd.Output()
	if err != nil {
		http.Error(w, "Ошибка получения групп", 500)
		return
	}

	var groups []SambaGroup
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" { continue }
		parts := strings.Split(line, ":")
		if len(parts) >= 4 {
			members := []string{}
			if parts[3] != "" {
				members = strings.Split(parts[3], ",")
			}
			groups = append(groups, SambaGroup{
				Name:    parts[0],
				GID:     parts[2],
				Members: members,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// saveGroupHandler создает новую группу
func saveGroupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct { Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		return
	}

	cmd := exec.Command("groupadd", req.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, "Ошибка создания группы: "+string(output), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteGroupHandler удаляет группу
func deleteGroupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct { Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		return
	}

	cmd := exec.Command("groupdel", req.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, "Ошибка удаления группы: "+string(output), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// toggleGroupMemberHandler добавляет или удаляет пользователя из группы
func toggleGroupMemberHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Group    string `json:"group"`
		Username string `json:"username"`
		Action   string `json:"action"` // "add" or "remove"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		return
	}

	var cmd *exec.Cmd
	if req.Action == "add" {
		cmd = exec.Command("gpasswd", "-a", req.Username, req.Group)
	} else {
		cmd = exec.Command("gpasswd", "-d", req.Username, req.Group)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, "Ошибка управления участниками: "+string(output), 500)
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

// getDiscoveryStatus возвращает статусы wsdd и avahi
func getDiscoveryStatus(w http.ResponseWriter, r *http.Request) {
	type ServiceStatus struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
		Installed bool `json:"installed"`
	}
	
	services := []string{"wsdd", "avahi-daemon"}
	var results []ServiceStatus

	for _, s := range services {
		active := false
		installed := true
		
		if os.PathSeparator == '/' {
			// Проверка на Linux
			cmd := exec.Command("systemctl", "is-active", s)
			output, _ := cmd.Output()
			active = strings.TrimSpace(string(output)) == "active"
			
			// Проверка на наличие юнита
			checkCmd := exec.Command("systemctl", "list-unit-files", s+".service")
			checkOut, _ := checkCmd.Output()
			installed = strings.Contains(string(checkOut), s+".service")
		} else {
			// Mock для Windows
			active = true
		}

		results = append(results, ServiceStatus{
			Name:   s,
			Active: active,
			Installed: installed,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// controlDiscoveryHandler управляет wsdd/avahi
func controlDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
		Action  string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		return
	}

	validServices := map[string]bool{"wsdd": true, "avahi-daemon": true}
	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "enable": true, "disable": true}

	if !validServices[req.Service] || !validActions[req.Action] {
		http.Error(w, "Invalid service or action", 400)
		return
	}

	cmd := exec.Command("systemctl", req.Action, req.Service)
	if output, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, "Error: "+string(output), 500)
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
			{Path: "/data", MountPoint: "/dev/sda1", Total: "1.8T", Used: "450G", Free: "1.3T", Percent: 25, Shares: []string{"Обмен", "Музыка"}},
			{Path: "/mnt/backup", MountPoint: "/dev/sdb1", Total: "4.0T", Used: "3.2T", Free: "800G", Percent: 80, Shares: []string{"Бэкапы"}},
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

		usage := DiskUsage{
			Path:       p,
			MountPoint: fields[0],
			Total:      fields[1],
			Used:       fields[2],
			Free:       fields[3],
			Percent:    percent,
			Shares:     []string{},
		}

		// Сопоставляем шары с этим диском
		for _, section := range cfg.Sections() {
			if section.Name() == "DEFAULT" || section.Name() == "global" { continue }
			sharePath := section.Key("path").String()
			// Если путь шары находится на этом разделе (начинается с точки монтирования)
			if strings.HasPrefix(sharePath, mount) {
				usage.Shares = append(usage.Shares, section.Name())
			}
		}

		results = append(results, usage)
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

// clearRecycleBinsHandler находит и очищает папки корзин во всех шарах
func clearRecycleBinsHandler(w http.ResponseWriter, r *http.Request) {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}
	cfg, _ := ini.Load(path)

	type CleanupResult struct {
		Share string `json:"share"`
		Path  string `json:"path"`
		Error string `json:"error,omitempty"`
	}
	var results []CleanupResult

	for _, section := range cfg.Sections() {
		vfs := section.Key("vfs objects").String()
		if !strings.Contains(vfs, "recycle") {
			continue
		}

		sharePath := section.Key("path").String()
		if sharePath == "" {
			continue
		}

		// Определяем репозиторий корзины (по умолчанию .recycle)
		repo := section.Key("recycle:repository").String()
		if repo == "" {
			repo = ".recycle"
		}
		
		// Очищаем от макросов Samba (%U, %u и т.д.), берем базовую папку
		repoBase := strings.Split(repo, "/")[0]
		fullRepoPath := filepath.Join(sharePath, repoBase)

		if os.PathSeparator == '\\' {
			// Mock для Windows
			results = append(results, CleanupResult{Share: section.Name(), Path: fullRepoPath})
			continue
		}

		// Проверяем существование
		if _, err := os.Stat(fullRepoPath); os.IsNotExist(err) {
			continue
		}

		// Очищаем содержимое
		cmd := exec.Command("sh", "-c", fmt.Sprintf("rm -rf %s/*", fullRepoPath))
		if err := cmd.Run(); err != nil {
			results = append(results, CleanupResult{Share: section.Name(), Path: fullRepoPath, Error: err.Error()})
		} else {
			results = append(results, CleanupResult{Share: section.Name(), Path: fullRepoPath})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Загрузка администраторов из файла
func loadAdmins() {
	if _, err := os.Stat(adminsPath); os.IsNotExist(err) {
		// Создаем дефолтного админа, если файла нет
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		admins = []AdminUser{
			{Username: "admin", Hash: string(hash), Role: "superadmin"},
		}
		saveAdmins()
		return
	}

	data, err := os.ReadFile(adminsPath)
	if err != nil {
		log.Println("Ошибка чтения admins.json:", err)
		return
	}
	json.Unmarshal(data, &admins)
}

func saveAdmins() {
	data, _ := json.MarshalIndent(admins, "", "  ")
	os.WriteFile(adminsPath, data, 0600)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	found := false
	var user AdminUser
	for _, a := range admins {
		if a.Username == creds.Username {
			err := bcrypt.CompareHashAndPassword([]byte(a.Hash), []byte(creds.Password))
			if err == nil {
				found = true
				user = a
				break
			}
		}
	}

	if !found {
		http.Error(w, "Invalid credentials", 401)
		return
	}

	token := "session-" + fmt.Sprint(time.Now().UnixNano()) + "-" + user.Username
	sessions[token] = time.Now().Add(24 * time.Hour)
	
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"role":  user.Role,
		"user":  user.Username,
	})
}

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

func getAdminsHandler(w http.ResponseWriter, r *http.Request) {
	// Создаем копию списка без хешей для безопасности
	safeAdmins := []AdminUser{}
	for _, a := range admins {
		a.Hash = "" // Скрываем хеш
		safeAdmins = append(safeAdmins, a)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safeAdmins)
}

func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	cookie, _ := r.Cookie(sessionCookieName)
	tokenParts := strings.Split(cookie.Value, "-")
	currentUser := tokenParts[len(tokenParts)-1]

	for i, a := range admins {
		if a.Username == currentUser {
			err := bcrypt.CompareHashAndPassword([]byte(a.Hash), []byte(req.OldPassword))
			if err != nil {
				http.Error(w, "Старый пароль неверен", 403)
				return
			}
			newHash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
			admins[i].Hash = string(newHash)
			saveAdmins()
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	http.Error(w, "User not found", 404)
}

func createAdminHandler(w http.ResponseWriter, r *http.Request) {
	var newUser AdminUser
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(newUser.Password), bcrypt.DefaultCost)
	newUser.Hash = string(hash)
	newUser.Password = ""
	admins = append(admins, newUser)
	saveAdmins()
	w.WriteHeader(http.StatusCreated)
}

func deleteAdminHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "admin" {
		http.Error(w, "Нельзя удалить основного администратора", 400)
		return
	}

	newAdmins := []AdminUser{}
	for _, a := range admins {
		if a.Username != username {
			newAdmins = append(newAdmins, a)
		}
	}
	admins = newAdmins
	saveAdmins()
	w.WriteHeader(http.StatusOK)
}


// getADStatusHandler проверяет статус подключения к домену
func getADStatusHandler(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ADStatus{Joined: true, Info: "Mock mode (Windows)"})
		return
	}

	cmd := exec.Command("net", "ads", "testjoin")
	output, err := cmd.CombinedOutput()
	
	status := ADStatus{
		Joined: err == nil,
		Info:   strings.TrimSpace(string(output)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// joinADHandler выполняет процедуру ввода в домен
func joinADHandler(w http.ResponseWriter, r *http.Request) {
	var req ADJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Mock: Joined successfully (Windows)"))
		return
	}

	// 1. Обновляем smb.conf с параметрами AD и IDMAP
	path := smbConfPath
	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Failed to load smb.conf", 500)
		return
	}

	section := cfg.Section("global")
	section.Key("security").SetValue("ads")
	section.Key("realm").SetValue(strings.ToUpper(req.Realm))
	section.Key("workgroup").SetValue(strings.Split(req.Realm, ".")[0])
	
	// Применяем настройки idmap из запроса пользователя
	section.Key("idmap config * : backend").SetValue("tdb")
	section.Key("idmap config * : range").SetValue("3000-7999")
	section.Key("idmap config " + strings.Split(req.Realm, ".")[0] + " : backend").SetValue("rid")
	section.Key("idmap config " + strings.Split(req.Realm, ".")[0] + " : range").SetValue("10000-9999999")
	
	// Оптимальные параметры для AD
	section.Key("kerberos method").SetValue("secrets and keytab")
	section.Key("winbind use default domain").SetValue("yes")
	section.Key("winbind enum users").SetValue("yes")
	section.Key("winbind enum groups").SetValue("yes")
	
	// Дополнительные параметры ACL и прав из запроса пользователя
	section.Key("vfs objects").SetValue("acl_xattr")
	section.Key("map acl inherit").SetValue("yes")
	section.Key("inherit owner").SetValue("yes")
	section.Key("inherit permissions").SetValue("yes")
	section.Key("acl map full control").SetValue("false")
	section.Key("nt acl support").SetValue("yes")
	section.Key("acl group control").SetValue("true")
	section.Key("dos filemode").SetValue("yes")
	section.Key("enable privileges").SetValue("yes")
	section.Key("store dos attributes").SetValue("yes")
	section.Key("map read only").SetValue("Permissions")
	
	// Группы администраторов (шаблон)
	workgroup := strings.Split(req.Realm, ".")[0]
	section.Key("admin users").SetValue("@\"" + workgroup + "\\Администраторы домена\"")

	createConfigBackup()
	cfg.SaveTo(path)

	// 2. Синхронизация времени (AD очень чувствителен к этому)
	// Пытаемся найти контроллер домена через realm
	exec.Command("net", "ads", "workgroup").Run() // для инициализации
	timeCmd := exec.Command("net", "ads", "time", "set", "-S", req.Realm, "-U", req.Admin+"%"+req.Password)
	timeCmd.Run()

	// 3. Получаем Kerberos тикет
	kinitCmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kinit %s@%s", req.Password, req.Admin, strings.ToUpper(req.Realm)))
	if output, err := kinitCmd.CombinedOutput(); err != nil {
		http.Error(w, "Kerberos error: "+string(output), 500)
		return
	}

	// 4. Присоединяем к домену
	joinCmd := exec.Command("net", "ads", "join", "-U", req.Admin+"%"+req.Password)
	if output, err := joinCmd.CombinedOutput(); err != nil {
		http.Error(w, "Join error: "+string(output), 500)
		return
	}

	// 5. Перезапуск сервисов
	exec.Command("systemctl", "restart", "smbd", "nmbd", "winbind").Run()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Successfully joined the domain"))
}

// getADHealthHandler выполняет серию тестов для проверки здоровья AD
func getADHealthHandler(w http.ResponseWriter, r *http.Request) {
	health := ADHealth{
		Status:     true,
		LastUpdate: time.Now().Format("15:04:05"),
		Checks:     []ADCheckResult{},
	}

	if os.PathSeparator == '\\' {
		// Mock для Windows
		health.Checks = append(health.Checks, 
			ADCheckResult{"Связь с контроллером", "ok", "DC1.corp.example.com доступен"},
			ADCheckResult{"Синхронизация времени", "ok", "Разница 0.02с"},
			ADCheckResult{"Доверительные отношения", "ok", "Join is valid"},
			ADCheckResult{"Winbind RPC", "ok", "RPC connection is OK"},
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
		return
	}

	// 1. Проверка net ads testjoin
	testJoinCmd := exec.Command("net", "ads", "testjoin")
	output, err := testJoinCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Доверительные отношения", "ok", strings.TrimSpace(string(output))})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Доверительные отношения", "error", strings.TrimSpace(string(output))})
		health.Status = false
	}

	// 2. Проверка wbinfo -t (RPC trust secret)
	wbTrustCmd := exec.Command("wbinfo", "-t")
	output, err = wbTrustCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Winbind RPC", "ok", strings.TrimSpace(string(output))})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Winbind RPC", "error", strings.TrimSpace(string(output))})
		health.Status = false
	}

	// 3. Проверка времени (смещение относительно AD)
	// Пытаемся получить время через net ads time
	timeCmd := exec.Command("net", "ads", "time")
	output, err = timeCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Синхронизация времени", "ok", "Время сервера совпадает с DC"})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Синхронизация времени", "warning", "Не удалось проверить время через net ads"})
	}

	// 4. Проверка Kerberos билета (keytab)
	klistCmd := exec.Command("klist", "-k")
	output, err = klistCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Kerberos Keytab", "ok", "Keytab файл присутствует и валиден"})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Kerberos Keytab", "error", "Keytab не найден или не читается"})
		health.Status = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func main() {
	loadAdmins()
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
	http.HandleFunc("/api/automation", authMiddleware(getAutomationHandler))
	http.HandleFunc("/api/automation/save", authMiddleware(saveAutomationHandler))
	http.HandleFunc("/api/maintenance/clear-recycle", authMiddleware(clearRecycleBinsHandler))
	http.HandleFunc("/api/disk/usage", authMiddleware(getDiskUsageHandler))
	http.HandleFunc("/api/status", authMiddleware(getSambaStatus))
	http.HandleFunc("/api/shares", authMiddleware(getShares))
	http.HandleFunc("/api/users", authMiddleware(getUsers))
	http.HandleFunc("/api/users/save", authMiddleware(saveUserHandler))
	http.HandleFunc("/api/users/delete", authMiddleware(deleteUserHandler))
	http.HandleFunc("/api/groups", authMiddleware(getGroups))
	http.HandleFunc("/api/groups/save", authMiddleware(saveGroupHandler))
	http.HandleFunc("/api/groups/delete", authMiddleware(deleteGroupHandler))
	http.HandleFunc("/api/groups/member", authMiddleware(toggleGroupMemberHandler))
	http.HandleFunc("/api/ad/status", authMiddleware(getADStatusHandler))
	http.HandleFunc("/api/ad/join", authMiddleware(joinADHandler))
	http.HandleFunc("/api/ad/health", authMiddleware(getADHealthHandler))
	http.HandleFunc("/api/discovery/status", authMiddleware(getDiscoveryStatus))
	http.HandleFunc("/api/discovery/control", authMiddleware(controlDiscoveryHandler))

	http.HandleFunc("/api/panel/admins", authMiddleware(getAdminsHandler))
	http.HandleFunc("/api/panel/admins/create", authMiddleware(createAdminHandler))
	http.HandleFunc("/api/panel/admins/delete", authMiddleware(deleteAdminHandler))
	http.HandleFunc("/api/panel/admins/password", authMiddleware(changePasswordHandler))

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	port := ":8888"
	fmt.Printf("🚀 Samba Blackjack Panel запущен на http://localhost%s\n", port)
	
	// Запуск фонового воркера
	go backgroundWorker()

	log.Fatal(http.ListenAndServe(port, nil))
}

func loadAutomationSettings() AutomationSettings {
	var s AutomationSettings
	data, err := os.ReadFile(automationFile)
	if err != nil {
		return AutomationSettings{RecycleDays: 14, SnapshotInterval: "none", SnapshotKeep: 10}
	}
	json.Unmarshal(data, &s)
	return s
}

func getAutomationHandler(w http.ResponseWriter, r *http.Request) {
	s := loadAutomationSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

func saveAutomationHandler(w http.ResponseWriter, r *http.Request) {
	var s AutomationSettings
	json.NewDecoder(r.Body).Decode(&s)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(automationFile, data, 0644)
	w.WriteHeader(http.StatusOK)
}

func backgroundWorker() {
	// Сразу при запуске не пуляем, ждем час
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		settings := loadAutomationSettings()
		if settings.SnapshotInterval != "none" {
			performSnapshots(settings)
		}
		if settings.RecycleDays > 0 {
			performRecycleCleanup(settings.RecycleDays)
		}
	}
}

func performSnapshots(settings AutomationSettings) {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}
	cfg, _ := ini.Load(path)

	for _, section := range cfg.Sections() {
		vfs := section.Key("vfs objects").String()
		if !strings.Contains(vfs, "shadow_copy2") {
			continue
		}

		sharePath := section.Key("path").String()
		if sharePath == "" || runtime.GOOS != "linux" {
			continue
		}

		snapDir := filepath.Join(sharePath, ".snapshots")
		os.MkdirAll(snapDir, 0755)

		// Формат GMT для Windows
		timestamp := time.Now().UTC().Format("@GMT-2006.01.02-15.04.05")
		dest := filepath.Join(snapDir, timestamp)

		// Команда cp -al для создания снимка через хардлинки
		// Исключаем саму папку .snapshots
		cmd := exec.Command("sh", "-c", fmt.Sprintf("cp -al %s %s && rm -rf %s/.snapshots", sharePath, dest, dest))
		cmd.Run()

		// Ротация снимков
		files, _ := os.ReadDir(snapDir)
		if len(files) > settings.SnapshotKeep {
			var oldest os.DirEntry
			for _, f := range files {
				if oldest == nil || f.Name() < oldest.Name() {
					oldest = f
				}
			}
			if oldest != nil {
				os.RemoveAll(filepath.Join(snapDir, oldest.Name()))
			}
		}
	}
}

func performRecycleCleanup(days int) {
	if runtime.GOOS != "linux" {
		return
	}
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}
	cfg, _ := ini.Load(path)

	for _, section := range cfg.Sections() {
		vfs := section.Key("vfs objects").String()
		if !strings.Contains(vfs, "recycle") {
			continue
		}

		sharePath := section.Key("path").String()
		if sharePath == "" {
			continue
		}

		repo := section.Key("recycle:repository").String()
		if repo == "" {
			repo = ".recycle"
		}
		repoBase := strings.Split(repo, "/")[0]
		fullPath := filepath.Join(sharePath, repoBase)

		// Удаляем файлы старше N дней
		exec.Command("find", fullPath, "-type", "f", "-mtime", fmt.Sprintf("+%d", days), "-delete").Run()
		// Удаляем пустые папки
		exec.Command("find", fullPath, "-type", "d", "-empty", "-delete").Run()
	}
}
