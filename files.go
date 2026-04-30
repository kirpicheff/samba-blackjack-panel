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

// listContentHandler возвращает список файлов и папок по указанному пути
func listContentHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Путь не указан", 400)
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
	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, "Ошибка удаления: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}
