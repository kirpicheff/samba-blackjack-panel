package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

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
	Params    map[string]string `json:"params"`
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
		
		// Проверяем наличие корзины в vfs objects
		vfs := section.Key("vfs objects").String()
		share.IsRecycle = strings.Contains(vfs, "recycle")
		
		shares = append(shares, share)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shares)
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

func main() {
	// Если мы на Windows, создадим тестовый конфиг для разработки
	if _, err := os.Stat(smbConfPath); os.IsNotExist(err) {
		if _, err := os.Stat(devSmbConfPath); os.IsNotExist(err) {
			f, _ := os.Create(devSmbConfPath)
			f.WriteString("[global]\n  workgroup = WORKGROUP\n\n[Обмен]\n  path = /tmp\n  vfs objects = acl_xattr recycle\n")
			f.Close()
		}
	}

	http.HandleFunc("/api/status", getSambaStatus)
	http.HandleFunc("/api/shares", getShares)
	http.HandleFunc("/api/users", getUsers)

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	port := ":8888"
	fmt.Printf("🚀 Samba Blackjack Panel запущен на http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
