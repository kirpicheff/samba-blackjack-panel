package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/ini.v1"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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

func getUsers(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("pdbedit", "-L")
	output, err := cmd.Output()
	if err != nil {
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
		if strings.TrimSpace(line) == "" { continue }
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

func saveUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	checkCmd := exec.Command("pdbedit", "-L", "-u", req.Username)
	err := checkCmd.Run()

	var cmd *exec.Cmd
	if err != nil {
		cmd = exec.Command("smbpasswd", "-a", "-s", req.Username)
	} else {
		cmd = exec.Command("smbpasswd", "-s", req.Username)
	}

	cmd.Stdin = strings.NewReader(req.Password + "\n" + req.Password + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		if os.PathSeparator == '\\' {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Ошибка Samba: "+string(output), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username string `json:"username"` }
	json.NewDecoder(r.Body).Decode(&req)
	cmd := exec.Command("pdbedit", "-x", "-u", req.Username)
	if output, err := cmd.CombinedOutput(); err != nil {
		if os.PathSeparator == '\\' {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Ошибка: "+string(output), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func getGroups(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		groups := []SambaGroup{
			{Name: "smb_admins", GID: "2000", Members: []string{"admin"}},
			{Name: "users", GID: "2001", Members: []string{"user1", "admin"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
		return
	}

	cmd := exec.Command("getent", "group")
	output, _ := cmd.Output()
	var groups []SambaGroup
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" { continue }
		parts := strings.Split(line, ":")
		if len(parts) >= 4 {
			members := []string{}
			if parts[3] != "" { members = strings.Split(parts[3], ",") }
			groups = append(groups, SambaGroup{Name: parts[0], GID: parts[2], Members: members})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

func saveGroupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct { Name string `json:"name"` }
	json.NewDecoder(r.Body).Decode(&req)
	if os.PathSeparator == '\\' { w.WriteHeader(http.StatusOK); return }
	cmd := exec.Command("groupadd", req.Name)
	cmd.Run()
	w.WriteHeader(http.StatusOK)
}

func deleteGroupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct { Name string `json:"name"` }
	json.NewDecoder(r.Body).Decode(&req)
	if os.PathSeparator == '\\' { w.WriteHeader(http.StatusOK); return }
	cmd := exec.Command("groupdel", req.Name)
	cmd.Run()
	w.WriteHeader(http.StatusOK)
}

func toggleGroupMemberHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Group    string `json:"group"`
		Username string `json:"username"`
		Action   string `json:"action"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if os.PathSeparator == '\\' { w.WriteHeader(http.StatusOK); return }
	var cmd *exec.Cmd
	if req.Action == "add" {
		cmd = exec.Command("gpasswd", "-a", req.Username, req.Group)
	} else {
		cmd = exec.Command("gpasswd", "-d", req.Username, req.Group)
	}
	cmd.Run()
	w.WriteHeader(http.StatusOK)
}

func applyServiceConfig(w http.ResponseWriter, r *http.Request) {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) { path = devSmbConfPath }
	testCmd := exec.Command("testparm", "-s", path)
	if output, err := testCmd.CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(output)
		return
	}
	if runtime.GOOS == "linux" {
		exec.Command("systemctl", "reload", "smbd").Run()
	}
	w.WriteHeader(http.StatusOK)
}

func getServiceStatus(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' { w.Write([]byte("active")); return }
	cmd := exec.Command("systemctl", "is-active", "smbd")
	output, _ := cmd.Output()
	w.Write([]byte(strings.TrimSpace(string(output))))
}

func controlServiceHandler(w http.ResponseWriter, r *http.Request) {
	var req struct { Action string `json:"action"` }
	json.NewDecoder(r.Body).Decode(&req)
	if os.PathSeparator == '\\' { w.WriteHeader(http.StatusOK); return }
	exec.Command("systemctl", req.Action, "smbd").Run()
	w.WriteHeader(http.StatusOK)
}

func getDiscoveryStatus(w http.ResponseWriter, r *http.Request) {
	type ServiceStatus struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
		Installed bool `json:"installed"`
	}
	services := []string{"wsdd2", "avahi-daemon"}
	var results []ServiceStatus
	for _, s := range services {
		active := false
		installed := true
		if os.PathSeparator == '/' {
			cmd := exec.Command("systemctl", "is-active", s)
			out, _ := cmd.Output()
			active = strings.TrimSpace(string(out)) == "active"
			checkCmd := exec.Command("systemctl", "list-unit-files", s+".service")
			checkOut, _ := checkCmd.Output()
			installed = strings.Contains(string(checkOut), s+".service")
		} else { active = true }
		results = append(results, ServiceStatus{Name: s, Active: active, Installed: installed})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func controlDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
		Action  string `json:"action"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if os.PathSeparator == '\\' { w.WriteHeader(http.StatusOK); return }
	exec.Command("systemctl", req.Action, req.Service).Run()
	w.WriteHeader(http.StatusOK)
}

func getLogsHandler(w http.ResponseWriter, r *http.Request) {
	logFile := "/var/log/samba/log.smbd"
	if os.PathSeparator == '\\' {
		w.Write([]byte("MOCK LOGS: [2026/04/30 15:30:00] smbd version 4.15.13 started.\n[2026/04/30 15:31:05] Request from 192.168.1.50 accepted.\n"))
		return
	}
	cmd := exec.Command("tail", "-n", "200", logFile)
	output, _ := cmd.CombinedOutput()
	w.Write(output)
}

func wsLogsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	logFile := "/var/log/samba/log.smbd"
	if os.PathSeparator == '\\' {
		// Mock streaming for Windows
		for i := 0; i < 10; i++ {
			msg := fmt.Sprintf("[LIVE MOCK] New log entry %d\n", i)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
			time.Sleep(2 * time.Second)
		}
		return
	}

	cmd := exec.Command("tail", "-f", "-n", "100", logFile)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Process.Kill()

	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if err != nil {
			break
		}
		if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
			break
		}
	}
}

func getDiskUsageHandler(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		usage := []DiskUsage{{Path: "/data", MountPoint: "/dev/sda1", Total: "1.8T", Used: "450G", Free: "1.3T", Percent: 25, Shares: []string{"Обмен"}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(usage)
		return
	}
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) { path = devSmbConfPath }
	cfg, _ := ini.Load(path)
	uniquePaths := make(map[string]bool)
	for _, section := range cfg.Sections() {
		if p := section.Key("path").String(); p != "" { uniquePaths[p] = true }
	}
	var results []DiskUsage
	seenMounts := make(map[string]bool)
	for p := range uniquePaths {
		cmd := exec.Command("df", "-h", "--output=source,size,used,avail,pcent,target", p)
		out, err := cmd.Output()
		if err != nil { continue }
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) < 2 { continue }
		fields := strings.Fields(lines[1])
		if len(fields) < 6 { continue }
		mount := fields[5]
		if seenMounts[mount] { continue }
		seenMounts[mount] = true
		var percent float64
		fmt.Sscanf(strings.TrimSuffix(fields[4], "%"), "%f", &percent)
		usage := DiskUsage{Path: p, MountPoint: fields[0], Total: fields[1], Used: fields[2], Free: fields[3], Percent: percent, Shares: []string{}}
		for _, section := range cfg.Sections() {
			if strings.HasPrefix(section.Key("path").String(), mount) { usage.Shares = append(usage.Shares, section.Name()) }
		}
		results = append(results, usage)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func getAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		logs := []AuditEntry{{Timestamp: "2026/04/30 13:45:10", User: "admin", Action: "unlink", File: "test.docx"}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
		return
	}
	cmd := exec.Command("grep", "smbd_audit", "/var/log/syslog")
	output, _ := cmd.CombinedOutput()
	var entries []AuditEntry
	lines := strings.Split(string(output), "\n")
	start := len(lines) - 100
	if start < 0 { start = 0 }
	for i := len(lines) - 1; i >= start; i-- {
		line := lines[i]
		if !strings.Contains(line, "smbd_audit:") { continue }
		parts := strings.Split(line, "smbd_audit:")
		data := strings.Split(strings.TrimSpace(parts[1]), "|")
		if len(data) < 7 { continue }
		entries = append(entries, AuditEntry{
			Timestamp: strings.Fields(line)[0] + " " + strings.Fields(line)[1] + " " + strings.Fields(line)[2],
			User:      data[0], IP: data[1], Action: data[4], File: data[6],
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func clearRecycleBinsHandler(w http.ResponseWriter, r *http.Request) {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) { path = devSmbConfPath }
	cfg, _ := ini.Load(path)
	for _, section := range cfg.Sections() {
		if !strings.Contains(section.Key("vfs objects").String(), "recycle") { continue }
		sharePath := section.Key("path").String()
		repo := section.Key("recycle:repository").String()
		if repo == "" { repo = ".recycle" }
		fullPath := filepath.Join(sharePath, strings.Split(repo, "/")[0])
		if os.PathSeparator == '\\' { continue }
		files, _ := os.ReadDir(fullPath)
		for _, f := range files {
			os.RemoveAll(filepath.Join(fullPath, f.Name()))
		}
	}
	w.WriteHeader(http.StatusOK)
}

func getPathPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", 400)
		return
	}

	if os.PathSeparator == '\\' {
		perms := PathPermissions{
			Path:  path,
			Owner: "admin",
			Group: "admins",
			Mode:  "0755",
		}
		json.NewEncoder(w).Encode(perms)
		return
	}

	cmd := exec.Command("stat", "-c", "%U:%G:%a", path)
	out, err := cmd.Output()
	if err != nil {
		http.Error(w, "Failed to get stats: "+err.Error(), 500)
		return
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(parts) < 3 {
		http.Error(w, "Invalid stat output", 500)
		return
	}

	perms := PathPermissions{
		Path:  path,
		Owner: parts[0],
		Group: parts[1],
		Mode:  parts[2],
	}

	// Try to get ACLs if available
	aclCmd := exec.Command("getfacl", "-cp", path)
	aclOut, _ := aclCmd.Output()
	perms.ACLs = string(aclOut)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perms)
}

func updatePathPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		Owner     string `json:"owner"`
		Group     string `json:"group"`
		Mode      string `json:"mode"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Chown
	if req.Owner != "" && req.Group != "" {
		args := []string{req.Owner + ":" + req.Group, req.Path}
		if req.Recursive {
			args = append([]string{"-R"}, args...)
		}
		if out, err := exec.Command("chown", args...).CombinedOutput(); err != nil {
			http.Error(w, "Chown error: "+string(out), 500)
			return
		}
	}

	// Chmod
	if req.Mode != "" {
		args := []string{req.Mode, req.Path}
		if req.Recursive {
			args = append([]string{"-R"}, args...)
		}
		if out, err := exec.Command("chmod", args...).CombinedOutput(); err != nil {
			http.Error(w, "Chmod error: "+string(out), 500)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func getOpenFilesHandler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("smbstatus", "-L", "--json")
	output, err := cmd.Output()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"locks": {"sharemode": []}}`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func closeOpenFileHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PID string `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	if req.PID == "" {
		http.Error(w, "PID is required", 400)
		return
	}

	// Принудительно завершаем процесс, который держит файл
	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		cmd = exec.Command("taskkill", "/F", "/PID", req.PID)
	} else {
		cmd = exec.Command("kill", "-9", req.PID)
	}

	if err := cmd.Run(); err != nil {
		http.Error(w, "Не удалось закрыть файл: "+err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}
