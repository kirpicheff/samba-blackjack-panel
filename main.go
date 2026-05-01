package main

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

func main() {
	loadAdmins()

	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/logout", logoutHandler)

	http.HandleFunc("/api/shares", authMiddleware(getShares))
	http.HandleFunc("/api/shares/save", authMiddleware(saveShare))
	http.HandleFunc("/api/shares/delete", authMiddleware(deleteShare))

	http.HandleFunc("/api/global", authMiddleware(getGlobalConfig))
	http.HandleFunc("/api/global/save", authMiddleware(saveGlobalConfig))

	http.HandleFunc("/api/service/status", authMiddleware(getServiceStatus))
	http.HandleFunc("/api/service/control", authMiddleware(controlServiceHandler))
	http.HandleFunc("/api/service/apply", authMiddleware(applyServiceConfig))

	http.HandleFunc("/api/users", authMiddleware(getUsers))
	http.HandleFunc("/api/users/save", authMiddleware(saveUserHandler))
	http.HandleFunc("/api/users/delete", authMiddleware(deleteUserHandler))

	http.HandleFunc("/api/groups", authMiddleware(getGroups))
	http.HandleFunc("/api/groups/save", authMiddleware(saveGroupHandler))
	http.HandleFunc("/api/groups/delete", authMiddleware(deleteGroupHandler))
	http.HandleFunc("/api/groups/member", authMiddleware(toggleGroupMemberHandler))

	http.HandleFunc("/api/logs", authMiddleware(getLogsHandler))
	http.HandleFunc("/api/ws/logs", authMiddleware(wsLogsHandler))
	http.HandleFunc("/api/audit", authMiddleware(getAuditLogsHandler))
	http.HandleFunc("/api/disk/usage", authMiddleware(getDiskUsageHandler))
	http.HandleFunc("/api/status", authMiddleware(getSambaStatus))

	http.HandleFunc("/api/automation", authMiddleware(getAutomationHandler))
	http.HandleFunc("/api/automation/save", authMiddleware(saveAutomationHandler))
	http.HandleFunc("/api/maintenance/clear-recycle", authMiddleware(clearRecycleBinsHandler))
	http.HandleFunc("/api/fs/permissions", authMiddleware(getPathPermissionsHandler))
	http.HandleFunc("/api/fs/permissions/save", authMiddleware(updatePathPermissionsHandler))

	http.HandleFunc("/api/ad/status", authMiddleware(getADStatusHandler))
	http.HandleFunc("/api/ad/join", authMiddleware(joinADHandler))
	http.HandleFunc("/api/ad/health", authMiddleware(getADHealthHandler))

	http.HandleFunc("/api/discovery/status", authMiddleware(getDiscoveryStatus))
	http.HandleFunc("/api/discovery/control", authMiddleware(controlDiscoveryHandler))

	http.HandleFunc("/api/panel/admins", authMiddleware(getAdminsHandler))
	http.HandleFunc("/api/panel/admins/create", authMiddleware(createAdminHandler))
	http.HandleFunc("/api/panel/admins/delete", authMiddleware(deleteAdminHandler))
	http.HandleFunc("/api/panel/admins/password", authMiddleware(changePasswordHandler))

	http.HandleFunc("/api/files/list", authMiddleware(listContentHandler))
	http.HandleFunc("/api/files/mkdir", authMiddleware(createFolderHandler))
	http.HandleFunc("/api/files/rename", authMiddleware(renameFileHandler))
	http.HandleFunc("/api/files/delete", authMiddleware(deleteFileHandler))

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// Квоты
	http.HandleFunc("/api/quotas/list", authMiddleware(listQuotasHandler))
	http.HandleFunc("/api/quotas/update", authMiddleware(updateQuotaHandler))

	// Открытые файлы
	http.HandleFunc("/api/openfiles", authMiddleware(getOpenFilesHandler))
	http.HandleFunc("/api/openfiles/close", authMiddleware(closeOpenFileHandler))

	port := ":8888"
	// Запуск фоновых задач
	StartSessionCleanup()

	log.Printf("Starting Samba Admin Panel on http://localhost%s", port)

	go backgroundWorker()

	// Оборачиваем все обработчики в middleware безопасности
	handler := securityMiddleware(http.DefaultServeMux)
	log.Fatal(http.ListenAndServe(port, handler))
}

// securityMiddleware объединяет заголовки безопасности, CSRF и rate limiting
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Заголовки безопасности
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")

		// 2. Ограничение частоты запросов (Rate Limiting)
		if !isRateAllowed(r) {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		// 3. CSRF защита для API (Double Submit Cookie)
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/login" {
				cookie, err := r.Cookie("csrf_token")
				header := r.Header.Get("X-CSRF-Token")
				if err != nil || header == "" || cookie.Value != header {
					http.Error(w, "CSRF Check Failed", http.StatusForbidden)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

var (
	ipsMu sync.Mutex
	ips   = make(map[string][]time.Time)
)

func isRateAllowed(r *http.Request) bool {
	// 1. Получаем реальный IP (учитываем прокси)
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	// Берём только первый IP из списка и убираем порт
	ip = strings.Split(ip, ",")[0]
	ip = strings.Split(ip, ":")[0]

	ipsMu.Lock()
	defer ipsMu.Unlock()

	now := time.Now()
	window := 10 * time.Second
	limit := 30

	// 2. Очистка старых данных (предотвращение memory leak)
	// Очищаем каждые 1000 новых записей или по размеру мапы
	if len(ips) > 500 {
		for k, timestamps := range ips {
			var active []time.Time
			for _, t := range timestamps {
				if now.Sub(t) < window {
					active = append(active, t)
				}
			}
			if len(active) == 0 {
				delete(ips, k)
			} else {
				ips[k] = active
			}
		}
	}

	// 3. Проверка текущего IP
	var active []time.Time
	for _, t := range ips[ip] {
		if now.Sub(t) < window {
			active = append(active, t)
		}
	}

	if len(active) >= limit {
		ips[ip] = active
		return false
	}

	ips[ip] = append(active, now)
	return true
}
