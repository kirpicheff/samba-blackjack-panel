package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileItem struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	Mode    string `json:"mode"`
}

// isPathSafe проверяет, находится ли путь внутри одной из разрешенных директорий (шар)
func isPathSafe(path string) bool {
	if path == "" {
		return false
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}

	allowedPaths := getAllSharePaths()
	for _, allowed := range allowedPaths {
		absAllowed, err := filepath.Abs(filepath.Clean(allowed))
		if err != nil {
			continue
		}

		// Путь должен быть либо самой шарой, либо находиться внутри неё.
		// Добавляем разделитель пути к префиксу, чтобы избежать частичного совпадения имен папок
		if cleanPath == absAllowed || strings.HasPrefix(cleanPath, absAllowed+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// listContentHandler возвращает список файлов и папок по указанному пути
func listContentHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" || path == "/" || path == "\\" {
		allowedPaths := getAllSharePaths()
		items := []FileItem{}
		for _, p := range allowedPaths {
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			// Возвращаем путь как имя папки для виртуального корня
			// Очищаем путь, чтобы он выглядел аккуратно
			name := strings.TrimPrefix(filepath.Clean(p), string(os.PathSeparator))
			
			items = append(items, FileItem{
				Name:    name,
				IsDir:   true,
				Size:    0,
				ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
				Mode:    fmt.Sprintf("%#o", info.Mode().Perm()),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
		return
	}

	if !isPathSafe(path) {
		http.Error(w, "Доступ запрещен: путь вне разрешенных директорий", 403)
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, "Не удалось прочитать директорию: "+err.Error(), 500)
		return
	}

	items := []FileItem{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, FileItem{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			Mode:    fmt.Sprintf("%#o", info.Mode().Perm()),
		})
	}

	// Сортировка: сначала папки, потом файлы
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// createFolderHandler создает новую директорию
func createFolderHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	fullPath := filepath.Join(req.Path, req.Name)
	if !isPathSafe(fullPath) {
		http.Error(w, "Доступ запрещен", 403)
		return
	}
	if err := os.Mkdir(fullPath, 0755); err != nil {
		http.Error(w, "Ошибка создания папки: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// renameFileHandler переименовывает файл или папку
func renameFileHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	oldPath := filepath.Join(req.Path, req.OldName)
	newPath := filepath.Join(req.Path, req.NewName)
	if !isPathSafe(oldPath) || !isPathSafe(newPath) {
		http.Error(w, "Доступ запрещен", 403)
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, "Ошибка переименования: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deleteFileHandler удаляет файл или папку (рекурсивно)
func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	fullPath := filepath.Join(req.Path, req.Name)
	if !isPathSafe(fullPath) {
		http.Error(w, "Доступ запрещен", 403)
		return
	}
	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, "Ошибка удаления: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}
