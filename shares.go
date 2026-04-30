package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

const smbConfPath = "/etc/samba/smb.conf"
const devSmbConfPath = "smb.conf.dev"

func getAllSharePaths() []string {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}
	cfg, err := ini.Load(path)
	if err != nil {
		return nil
	}
	var paths []string
	for _, section := range cfg.Sections() {
		if p := section.Key("path").String(); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

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
		
		vfs := section.Key("vfs objects").String()
		share.IsRecycle = strings.Contains(vfs, "recycle")
		share.IsAudit = strings.Contains(vfs, "full_audit")
		share.AuditOpen = strings.Contains(section.Key("full_audit:success").String(), "open")
		share.IsShadowCopy = strings.Contains(vfs, "shadow_copy2")
		
		shares = append(shares, share)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shares)
}

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

	cfg.DeleteSection(share.Name)
	section, _ := cfg.NewSection(share.Name)
	section.Key("path").SetValue(share.Path)
	if share.Comment != "" {
		section.Key("comment").SetValue(share.Comment)
	}

	for k, v := range share.Params {
		if k == "path" || k == "comment" || k == "vfs objects" || strings.Contains(k, "recycle:") || strings.Contains(k, "full_audit:") {
			continue
		}
		if v != "" {
			section.Key(k).SetValue(v)
		} else {
			section.DeleteKey(k)
		}
	}

	vfs := []string{"acl_xattr"}
	if share.IsRecycle {
		vfs = append(vfs, "recycle")
		repo := share.Params["recycle:repository"]
		if repo == "" {
			repo = ".recycle/%U"
			if share.Params["guest ok"] == "yes" { repo = ".recycle/guest" }
		}
		exclude := share.Params["recycle:exclude"]
		if exclude == "" { exclude = "*.tmp *.temp ~$* *.bak Thumbs.db" }
		excludeDir := share.Params["recycle:exclude_dir"]
		if excludeDir == "" { excludeDir = "/tmp /cache .recycle" }

		section.Key("recycle:repository").SetValue(repo)
		section.Key("recycle:keeptree").SetValue("yes")
		section.Key("recycle:versions").SetValue("yes")
		section.Key("recycle:touch").SetValue("yes")
		section.Key("recycle:directory_mode").SetValue("0770")
		section.Key("recycle:exclude").SetValue(exclude)
		section.Key("recycle:exclude_dir").SetValue(excludeDir)
	} else {
		for _, k := range section.KeyStrings() {
			if strings.HasPrefix(k, "recycle:") { section.DeleteKey(k) }
		}
	}

	if share.IsAudit {
		vfs = append(vfs, "full_audit")
		section.Key("full_audit:prefix").SetValue("%u|%I|%m|%S")
		success := "mkdir rename unlink"
		if share.AuditOpen { success += " open" }
		section.Key("full_audit:success").SetValue(success)
		section.Key("full_audit:failure").SetValue("none")
		section.Key("full_audit:facility").SetValue("local7")
		section.Key("full_audit:priority").SetValue("NOTICE")
	} else {
		for _, k := range section.KeyStrings() {
			if strings.HasPrefix(k, "full_audit:") { section.DeleteKey(k) }
		}
	}
	if share.IsShadowCopy {
		vfs = append(vfs, "shadow_copy2")
		section.Key("shadow:snapdir").SetValue(".snapshots")
		section.Key("shadow:sort").SetValue("desc")
		section.Key("shadow:format").SetValue("@GMT-%Y.%m.%d-%H.%M.%S")
	} else {
		section.DeleteKey("shadow:snapdir")
		section.DeleteKey("shadow:sort")
		section.DeleteKey("shadow:format")
	}

	section.Key("vfs objects").SetValue(strings.Join(vfs, " "))

	createConfigBackup()
	if err := cfg.SaveTo(path); err != nil {
		http.Error(w, "Failed to save smb.conf", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

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
	createConfigBackup()
	cfg.SaveTo(path)
	w.WriteHeader(http.StatusOK)
}

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
		if v != "" {
			section.Key(k).SetValue(v)
		} else {
			section.DeleteKey(k)
		}
	}

	createConfigBackup()
	if err := cfg.SaveTo(path); err != nil {
		http.Error(w, "Failed to save smb.conf", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func createConfigBackup() {
	path := smbConfPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = devSmbConfPath
	}

	backupDir := "backups"
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s/smb.conf.%s", backupDir, timestamp)

	input, _ := os.ReadFile(path)
	os.WriteFile(backupPath, input, 0644)

	files, _ := os.ReadDir(backupDir)
	if len(files) > 10 {
		var oldest os.DirEntry
		for _, f := range files {
			if oldest == nil || f.Name() < oldest.Name() {
				oldest = f
			}
		}
		if oldest != nil {
			os.Remove(backupDir + "/" + oldest.Name())
		}
	}
}
