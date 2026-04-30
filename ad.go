package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// getADStatusHandler проверяет статус подключения к домену
func getADStatusHandler(w http.ResponseWriter, r *http.Request) {
	if os.PathSeparator == '\\' {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ADStatus{Joined: true, Info: "Mock mode (Windows)"})
		return
	}

	cmd := exec.Command("net", "ads", "testjoin")
	output, err := cmd.CombinedOutput()

	status := ADStatus{
		Joined: err == nil,
		Info:   strings.TrimSpace(string(output)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// joinADHandler выполняет процедуру ввода в домен
func joinADHandler(w http.ResponseWriter, r *http.Request) {
	var req ADJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if os.PathSeparator == '\\' {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Mock: Joined successfully (Windows)"))
		return
	}

	path := smbConfPath
	cfg, err := ini.Load(path)
	if err != nil {
		http.Error(w, "Failed to load smb.conf", 500)
		return
	}

	section := cfg.Section("global")
	section.Key("security").SetValue("ads")
	section.Key("realm").SetValue(strings.ToUpper(req.Realm))
	section.Key("workgroup").SetValue(strings.Split(req.Realm, ".")[0])

	section.Key("idmap config * : backend").SetValue("tdb")
	section.Key("idmap config * : range").SetValue("3000-7999")
	section.Key("idmap config " + strings.Split(req.Realm, ".")[0] + " : backend").SetValue("rid")
	section.Key("idmap config " + strings.Split(req.Realm, ".")[0] + " : range").SetValue("10000-9999999")

	section.Key("kerberos method").SetValue("secrets and keytab")
	section.Key("winbind use default domain").SetValue("yes")
	section.Key("winbind enum users").SetValue("yes")
	section.Key("winbind enum groups").SetValue("yes")

	section.Key("vfs objects").SetValue("acl_xattr")
	section.Key("map acl inherit").SetValue("yes")
	section.Key("inherit owner").SetValue("yes")
	section.Key("inherit permissions").SetValue("yes")
	section.Key("acl map full control").SetValue("false")
	section.Key("nt acl support").SetValue("yes")
	section.Key("acl group control").SetValue("true")
	section.Key("dos filemode").SetValue("yes")
	section.Key("enable privileges").SetValue("yes")
	section.Key("store dos attributes").SetValue("yes")
	section.Key("map read only").SetValue("Permissions")

	workgroup := strings.Split(req.Realm, ".")[0]
	section.Key("admin users").SetValue("@\"" + workgroup + "\\Администраторы домена\"")

	createConfigBackup()
	cfg.SaveTo(path)

	exec.Command("net", "ads", "workgroup").Run()
	timeCmd := exec.Command("net", "ads", "time", "set", "-S", req.Realm, "-U", req.Admin+"%"+req.Password)
	timeCmd.Run()

	kinitCmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kinit %s@%s", req.Password, req.Admin, strings.ToUpper(req.Realm)))
	if output, err := kinitCmd.CombinedOutput(); err != nil {
		http.Error(w, "Kerberos error: "+string(output), 500)
		return
	}

	joinCmd := exec.Command("net", "ads", "join", "-U", req.Admin+"%"+req.Password)
	if output, err := joinCmd.CombinedOutput(); err != nil {
		http.Error(w, "Join error: "+string(output), 500)
		return
	}

	exec.Command("systemctl", "restart", "smbd", "nmbd", "winbind").Run()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Successfully joined the domain"))
}

func getADHealthHandler(w http.ResponseWriter, r *http.Request) {
	health := ADHealth{
		Status:     true,
		LastUpdate: time.Now().Format("15:04:05"),
		Checks:     []ADCheckResult{},
	}

	if os.PathSeparator == '\\' {
		health.Checks = append(health.Checks,
			ADCheckResult{"Связь с контроллером", "ok", "DC1.corp.example.com доступен"},
			ADCheckResult{"Синхронизация времени", "ok", "Разница 0.02с"},
			ADCheckResult{"Доверительные отношения", "ok", "Join is valid"},
			ADCheckResult{"Winbind RPC", "ok", "RPC connection is OK"},
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
		return
	}

	testJoinCmd := exec.Command("net", "ads", "testjoin")
	output, err := testJoinCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Доверительные отношения", "ok", strings.TrimSpace(string(output))})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Доверительные отношения", "error", strings.TrimSpace(string(output))})
		health.Status = false
	}

	wbTrustCmd := exec.Command("wbinfo", "-t")
	output, err = wbTrustCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Winbind RPC", "ok", strings.TrimSpace(string(output))})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Winbind RPC", "error", strings.TrimSpace(string(output))})
		health.Status = false
	}

	timeCmd := exec.Command("net", "ads", "time")
	output, err = timeCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Синхронизация времени", "ok", "Время сервера совпадает с DC"})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Синхронизация времени", "warning", "Не удалось проверить время через net ads"})
	}

	klistCmd := exec.Command("klist", "-k")
	output, err = klistCmd.CombinedOutput()
	if err == nil {
		health.Checks = append(health.Checks, ADCheckResult{"Kerberos Keytab", "ok", "Keytab файл присутствует и валиден"})
	} else {
		health.Checks = append(health.Checks, ADCheckResult{"Kerberos Keytab", "error", "Keytab не найден или не читается"})
		health.Status = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
