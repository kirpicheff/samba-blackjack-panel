package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
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
	Params    map[string]string `json:"params"`
}

type GlobalConfig struct {
	Params map[string]string `json:"params"`
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
		if k == "path" || k == "comment" || k == "vfs objects" {
			continue
		}
		section.Key(k).SetValue(v)
	}

	if share.IsRecycle {
		section.Key("vfs objects").SetValue("acl_xattr recycle")
		
		repo := ".recycle/%U"
		if share.Params["guest ok"] == "yes" {
			repo = ".recycle/guest"
		}
		
		section.Key("recycle:repository").SetValue(repo)
		section.Key("recycle:keeptree").SetValue("yes")
		section.Key("recycle:versions").SetValue("yes")
		section.Key("recycle:touch").SetValue("yes")
		section.Key("recycle:directory_mode").SetValue("0770")
		section.Key("recycle:exclude").SetValue("*.tmp *.temp ~$* *.bak Thumbs.db")
		section.Key("recycle:exclude_dir").SetValue("/tmp /cache .recycle")
	} else {
		section.Key("vfs objects").SetValue("acl_xattr")
		// Удаляем все параметры recycle:*
		for _, k := range section.KeyStrings() {
			if strings.HasPrefix(k, "recycle:") {
				section.DeleteKey(k)
			}
		}
	}

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
		section.Key(k).SetValue(v)
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
	http.HandleFunc("/api/status", authMiddleware(getSambaStatus))
	http.HandleFunc("/api/shares", authMiddleware(getShares))
	http.HandleFunc("/api/users", authMiddleware(getUsers))

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	port := ":8888"
	fmt.Printf("🚀 Samba Blackjack Panel запущен на http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
