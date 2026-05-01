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

	"gopkg.in/ini.v1"
)

var automationFile = "automation.json"

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
	// Опрашиваем раз в 5 минут, чтобы точнее соблюдать интервалы и быстрее реагировать на изменения настроек
	ticker := time.NewTicker(5 * time.Minute)
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

func shouldCreateSnapshot(snapDir string, interval string) bool {
	if interval == "hourly" {
		// Для часовых снимков проверяем, прошел ли час
		files, _ := os.ReadDir(snapDir)
		if len(files) == 0 {
			return true
		}
		var latest time.Time
		for _, f := range files {
			t, err := time.Parse("@GMT-2006.01.02-15.04.05", f.Name())
			if err == nil && t.After(latest) {
				latest = t
			}
		}
		return time.Now().UTC().Sub(latest) >= 50*time.Minute
	}

	if interval == "daily" || interval == "weekly" {
		files, _ := os.ReadDir(snapDir)
		if len(files) == 0 {
			return true
		}

		var latest time.Time
		for _, f := range files {
			t, err := time.Parse("@GMT-2006.01.02-15.04.05", f.Name())
			if err == nil && t.After(latest) {
				latest = t
			}
		}

		now := time.Now().UTC()
		if interval == "daily" {
			return now.Sub(latest) >= 23*time.Hour
		}
		if interval == "weekly" {
			return now.Sub(latest) >= 6*24*time.Hour+23*time.Hour
		}
	}

	return false
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

		// Проверяем, пора ли делать новый снимок согласно интервалу
		if !shouldCreateSnapshot(snapDir, settings.SnapshotInterval) {
			continue
		}

		// Формат GMT для Windows
		timestamp := time.Now().UTC().Format("@GMT-2006.01.02-15.04.05")
		dest := filepath.Join(snapDir, timestamp)

		// Создаем папку снимка
		if err := os.MkdirAll(dest, 0755); err != nil {
			continue
		}

		// Команда для создания снимка через хардлинки
		// Используем find, чтобы исключить папку .snapshots и избежать бесконечной рекурсии
		// Это критически важно для предотвращения переполнения диска и высокой нагрузки на I/O
		cmd := exec.Command("sh", "-c", fmt.Sprintf("find \"%s\" -mindepth 1 -maxdepth 1 ! -name '.snapshots' -exec cp -al -t \"%s\" {} +", sharePath, dest))
		cmd.Run()

		// Ротация снимков: удаляем все лишние старые копии, пока их количество не придет в норму
		for {
			files, _ := os.ReadDir(snapDir)
			if len(files) <= settings.SnapshotKeep {
				break
			}
			var oldest os.DirEntry
			for _, f := range files {
				// Учитываем только папки снимков в формате @GMT-
				if !strings.HasPrefix(f.Name(), "@GMT-") {
					continue
				}
				if oldest == nil || f.Name() < oldest.Name() {
					oldest = f
				}
			}
			if oldest != nil {
				os.RemoveAll(filepath.Join(snapDir, oldest.Name()))
			} else {
				break
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
