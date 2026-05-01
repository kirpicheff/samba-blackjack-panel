package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type SessionData struct {
	Expiry   time.Time
	Username string
}

var sessions = make(map[string]SessionData)
var sessionsMu sync.RWMutex

const sessionCookieName = "samba_session"

var adminsPath = "admins.json"
var admins []AdminUser
var adminsMu sync.RWMutex

// Загрузка администраторов из файла
func loadAdmins() {
	adminsMu.Lock()
	defer adminsMu.Unlock()

	if _, err := os.Stat(adminsPath); os.IsNotExist(err) {
		// Создаем дефолтного админа, если файла нет
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		admins = []AdminUser{
			{Username: "admin", Hash: string(hash), Role: "superadmin"},
		}
		saveAdminsNoLock()
		return
	}

	data, err := os.ReadFile(adminsPath)
	if err != nil {
		log.Println("Ошибка чтения admins.json:", err)
		return
	}
	json.Unmarshal(data, &admins)
}

// saveAdminsNoLock сохраняет админов без блокировки (вызывать внутри блокировки)
func saveAdminsNoLock() {
	data, _ := json.MarshalIndent(admins, "", "  ")
	os.WriteFile(adminsPath, data, 0600)
}

func saveAdmins() {
	adminsMu.Lock()
	defer adminsMu.Unlock()
	saveAdminsNoLock()
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

	adminsMu.RLock()
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
	adminsMu.RUnlock()

	if !found {
		http.Error(w, "Invalid credentials", 401)
		return
	}

	// 1. Генерируем токен сессии
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("Error generating session token: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(b)

	// 2. Генерируем независимый CSRF токен
	cb := make([]byte, 32)
	if _, err := rand.Read(cb); err != nil {
		log.Printf("Error generating CSRF token: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	csrfToken := hex.EncodeToString(cb)

	sessionsMu.Lock()
	sessions[token] = SessionData{
		Expiry:   time.Now().Add(24 * time.Hour),
		Username: user.Username,
	}
	sessionsMu.Unlock()

	// Устанавливаем куку сессии (HttpOnly)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	// Устанавливаем куку CSRF (не HttpOnly)
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"role":  user.Role,
		"user":  user.Username,
		"csrf":  csrfToken,
	})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		sessionsMu.Lock()
		delete(sessions, cookie.Value)
		sessionsMu.Unlock()
	}

	// Очищаем куку сессии
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	// Очищаем куку CSRF
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
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

		sessionsMu.RLock()
		sess, v := sessions[cookie.Value]
		sessionsMu.RUnlock()

		if !v || time.Now().After(sess.Expiry) {
			http.Error(w, "Unauthorized", 401)
			return
		}

		next(w, r)
	}
}

func getAdminsHandler(w http.ResponseWriter, r *http.Request) {
	adminsMu.RLock()
	defer adminsMu.RUnlock()

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
	sessionsMu.RLock()
	sess, ok := sessions[cookie.Value]
	sessionsMu.RUnlock()

	if !ok {
		http.Error(w, "Unauthorized", 401)
		return
	}
	currentUser := sess.Username

	adminsMu.Lock()
	defer adminsMu.Unlock()
	for i, a := range admins {
		if a.Username == currentUser {
			err := bcrypt.CompareHashAndPassword([]byte(a.Hash), []byte(req.OldPassword))
			if err != nil {
				http.Error(w, "Старый пароль неверен", 403)
				return
			}
			newHash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
			admins[i].Hash = string(newHash)
			saveAdminsNoLock()
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

	hash, err := bcrypt.GenerateFromPassword([]byte(newUser.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	newUser.Hash = string(hash)
	newUser.Password = ""

	adminsMu.Lock()
	defer adminsMu.Unlock()
	admins = append(admins, newUser)
	saveAdminsNoLock()
	w.WriteHeader(http.StatusCreated)
}

func deleteAdminHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "admin" {
		http.Error(w, "Нельзя удалить основного администратора", 400)
		return
	}

	adminsMu.Lock()
	defer adminsMu.Unlock()
	newAdmins := []AdminUser{}
	for _, a := range admins {
		if a.Username != username {
			newAdmins = append(newAdmins, a)
		}
	}
	admins = newAdmins
	saveAdminsNoLock()
	w.WriteHeader(http.StatusOK)
}

// StartSessionCleanup запускает фоновую очистку истекших сессий
func StartSessionCleanup() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			sessionsMu.Lock()
			now := time.Now()
			deleted := 0
			for token, session := range sessions {
				if now.After(session.Expiry) {
					delete(sessions, token)
					deleted++
				}
			}
			if deleted > 0 {
				log.Printf("Session cleanup: removed %d expired sessions", deleted)
			}
			sessionsMu.Unlock()
		}
	}()
}
