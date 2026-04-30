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
