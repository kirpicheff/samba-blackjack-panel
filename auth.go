package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var sessions = make(map[string]time.Time)

const sessionCookieName = "samba_session"

var adminsPath = "admins.json"
var admins []AdminUser

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
