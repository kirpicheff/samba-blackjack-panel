package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

type QuotaInfo struct {
	User      string `json:"user"`
	Used      int64  `json:"used"`      // в КБ
	SoftLimit int64  `json:"soft_limit"` // в КБ
	HardLimit int64  `json:"hard_limit"` // в КБ
	UsagePct  int    `json:"usage_pct"`
}

// listQuotasHandler возвращает список квот для пользователей
func listQuotasHandler(w http.ResponseWriter, r *http.Request) {
	// Если мы на Windows, отдаем мок-данные для тестов
	if runtime.GOOS == "windows" {
		mockData := []QuotaInfo{
			{"admin", 500000, 1000000, 1200000, 41},
			{"ivanov", 850000, 1000000, 1500000, 85},
			{"petrov", 1200000, 2000000, 2500000, 48},
			{"guest", 100, 50000, 100000, 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockData)
		return
	}

	// На Linux выполняем repquota -au
	// Формат вывода repquota сложный для парсинга через Split, 
	// в идеале использовать регулярки или специфичные флаги.
	out, err := exec.Command("repquota", "-au").Output()
	if err != nil {
		http.Error(w, "Ошибка получения квот: "+err.Error()+". Возможно, пакет 'quota' не установлен или квоты не включены в fstab.", 500)
		return
	}

	lines := strings.Split(string(out), "\n")
	var quotas []QuotaInfo

	// Простой парсер вывода repquota (пропускаем заголовки)
	for i, line := range lines {
		if i < 5 { continue } // Пропуск заголовков
		fields := strings.Fields(line)
		if len(fields) < 8 { continue }

		user := strings.TrimRight(fields[0], "-")
		used := parseSize(fields[2])
		soft := parseSize(fields[3])
		hard := parseSize(fields[4])

		pct := 0
		if hard > 0 {
			pct = int((used * 100) / hard)
		}

		quotas = append(quotas, QuotaInfo{
			User:      user,
			Used:      used,
			SoftLimit: soft,
			HardLimit: hard,
			UsagePct:  pct,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quotas)
}

// updateQuotaHandler устанавливает новые лимиты
func updateQuotaHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User      string `json:"user"`
		SoftLimit int64  `json:"soft_limit"` // в МБ
		HardLimit int64  `json:"hard_limit"` // в МБ
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	if runtime.GOOS == "windows" {
		fmt.Printf("Mock: Set quota for %s to Soft:%dMB, Hard:%dMB\n", req.User, req.SoftLimit, req.HardLimit)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Команда: setquota -u user soft hard 0 0 /
	// Мы конвертируем МБ обратно в блоки (1 block = 1 KB обычно)
	softBlocks := req.SoftLimit * 1024
	hardBlocks := req.HardLimit * 1024
	
	cmd := exec.Command("setquota", "-u", req.User, 
		fmt.Sprintf("%d", softBlocks), 
		fmt.Sprintf("%d", hardBlocks), 
		"0", "0", "/")
	
	if err := cmd.Run(); err != nil {
		http.Error(w, "Ошибка установки квоты: "+err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func parseSize(s string) int64 {
	var val int64
	fmt.Sscanf(s, "%d", &val)
	return val
}
