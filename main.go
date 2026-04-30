package main

import (
	"fmt"
	"log"
	"net/http"
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
	http.HandleFunc("/api/audit", authMiddleware(getAuditLogsHandler))
	http.HandleFunc("/api/disk/usage", authMiddleware(getDiskUsageHandler))
	http.HandleFunc("/api/status", authMiddleware(getSambaStatus))

	http.HandleFunc("/api/automation", authMiddleware(getAutomationHandler))
	http.HandleFunc("/api/automation/save", authMiddleware(saveAutomationHandler))
	http.HandleFunc("/api/maintenance/clear-recycle", authMiddleware(clearRecycleBinsHandler))

	http.HandleFunc("/api/ad/status", authMiddleware(getADStatusHandler))
	http.HandleFunc("/api/ad/join", authMiddleware(joinADHandler))
	http.HandleFunc("/api/ad/health", authMiddleware(getADHealthHandler))

	http.HandleFunc("/api/discovery/status", authMiddleware(getDiscoveryStatus))
	http.HandleFunc("/api/discovery/control", authMiddleware(controlDiscoveryHandler))

	http.HandleFunc("/api/panel/admins", authMiddleware(getAdminsHandler))
	http.HandleFunc("/api/panel/admins/create", authMiddleware(createAdminHandler))
	http.HandleFunc("/api/panel/admins/delete", authMiddleware(deleteAdminHandler))
	http.HandleFunc("/api/panel/admins/password", authMiddleware(changePasswordHandler))

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	port := ":8888"
	fmt.Printf("🚀 Samba Blackjack Panel запущен на http://localhost%s\n", port)

	go backgroundWorker()

	log.Fatal(http.ListenAndServe(port, nil))
}
