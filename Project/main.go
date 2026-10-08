package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// --- СТРУКТУРЫ ДАННЫХ ---

type StatusMap map[string]string
type Plan struct {
	Model     string `json:"model"`
	Produced  int    `json:"produced"`
	Target    int    `json:"target"`
	Remaining int    `json:"remaining"`
}
type Downtime struct {
	Line      string `json:"line"`
	Reason    string `json:"reason"`
	Duration  int    `json:"duration"`
	Equipment string `json:"equipment"`
}
type Supply struct {
	PartName string `json:"part_name"`
	Stock    int    `json:"stock"`
	Status   string `json:"status"`
}
type ChartData struct {
	Time string  `json:"time"`
	Svar float64 `json:"svar"`
	Okr  float64 `json:"okr"`
	Sbor float64 `json:"sbor"`
}
type Insight struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

type Response struct {
	DefectRate  float64     `json:"defect_rate"`
	TrendDefect float64     `json:"trend_defect"`
	Statuses    StatusMap   `json:"statuses"`
	Plans       []Plan      `json:"plans"`
	Progress    float64     `json:"progress"`
	Downtimes   []Downtime  `json:"downtimes"`
	Supplies    []Supply    `json:"supplies"`
	Chart       []ChartData `json:"chart"`
	Schema      map[string]string `json:"schema"`
	HealthScore int         `json:"health_score"`
	Insights    []Insight   `json:"insights"`
}

const dbPath = "factory.db"

// --- ИНИЦИАЛИЗАЦИЯ БД С ТОЧНЫМИ ДАННЫМИ ИЗ ДОКУМЕНТА ---

func initDB() {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tables := []string{
		`CREATE TABLE IF NOT EXISTS sensor_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp REAL, machine_id INTEGER, temperature REAL, vibration REAL, status TEXT)`,
		`CREATE TABLE IF NOT EXISTS quality_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp REAL, inspected_count INTEGER, defects_count INTEGER)`,
		`CREATE TABLE IF NOT EXISTS monthly_plans (id INTEGER PRIMARY KEY AUTOINCREMENT, model TEXT, target INTEGER, produced INTEGER DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS downtime_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, date TEXT, line TEXT, equipment TEXT, reason TEXT, duration INTEGER)`,
		`CREATE TABLE IF NOT EXISTS supplies (id INTEGER PRIMARY KEY AUTOINCREMENT, part_name TEXT, stock INTEGER, incoming INTEGER, min_stock INTEGER, eta_date TEXT, status TEXT)`,
	}
	for _, t := range tables {
		db.Exec(t)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM quality_logs").Scan(&count)

	if count == 0 {
		fmt.Println("🔄 Загрузка точных данных из документа...")

		historicalData := []struct {
			date        string
			line        string
			produced    int
			defects     int
			loadPercent int
		}{
			{"01.10.2026", "Сварка-1", 118, 2, 98},
			{"01.10.2026", "Окраска-1", 115, 4, 94},
			{"01.10.2026", "Сборка-1", 121, 1, 100},
			{"02.10.2026", "Сварка-1", 111, 3, 91},
			{"02.10.2026", "Окраска-1", 116, 6, 96},
			{"02.10.2026", "Сборка-1", 119, 2, 99},
		}

		for i, row := range historicalData {
			ts := time.Date(2026, 10, 1, 8+i, 0, 0, 0, time.UTC).Unix()
			mid := 1
			if row.line == "Окраска-1" {
				mid = 2
			}
			if row.line == "Сборка-1" {
				mid = 3
			}

			temp := 50 + float64(row.loadPercent)*0.4
			defectRate := float64(row.defects) / float64(row.produced) * 100
			status := "working"
			if defectRate > 5 {
				status = "critical"
			} else if defectRate > 2 {
				status = "warning"
			}

			db.Exec("INSERT INTO sensor_logs (timestamp, machine_id, temperature, vibration, status) VALUES (?, ?, ?, ?, ?)",
				float64(ts), mid, temp, 1.0, status)
			db.Exec("INSERT INTO quality_logs (timestamp, inspected_count, defects_count) VALUES (?, ?, ?)",
				float64(ts), row.produced, row.defects)
		}

		plans := []struct {
			model  string
			target int
		}{
			{"Chevrolet Onix", 2500},
			{"Chevrolet Cobalt", 1800},
			{"JAC J7", 500},
		}
		for _, p := range plans {
			db.Exec("INSERT INTO monthly_plans (model, target, produced) VALUES (?, ?, ?)",
				p.model, p.target, 0)
		}

		downtimes := []struct {
			date      string
			line      string
			equipment string
			reason    string
			duration  int
		}{
			{"01.10.2026", "Сварка", "ABB-01", "Ошибка датчика", 25},
			{"01.10.2026", "Окраска", "Камера-02", "Замена фильтра", 40},
			{"02.10.2026", "Сборка", "Конвейер-03", "Обрыв цепи", 55},
			{"02.10.2026", "Сварка", "ABB-04", "Плановое ТО", 30},
		}
		for _, d := range downtimes {
			db.Exec("INSERT INTO downtime_logs (date, line, equipment, reason, duration) VALUES (?, ?, ?, ?, ?)",
				d.date, d.line, d.equipment, d.reason, d.duration)
		}

		supplies := []struct {
			name   string
			stock  int
			min    int
			status string
		}{
			{"Тормозные колодки", 150, 200, "normal"},
			{"ЭБУ (Чипы)", 10, 50, "critical"},
			{"Шины R17", 800, 300, "normal"},
			{"Аккумуляторы", 45, 60, "warning"},
		}
		for _, s := range supplies {
			db.Exec("INSERT INTO supplies (part_name, stock, min_stock, status) VALUES (?, ?, ?, ?)",
				s.name, s.stock, s.min, s.status)
		}

		fmt.Println("✅ Точные данные загружены.")
	} else {
		fmt.Println("ℹ️ Данные уже есть.")
	}
}

// --- СИМУЛЯЦИЯ ЖИВЫХ ДАННЫХ ---

func simulateLiveData(db *sql.DB) {
	now := time.Now().Unix()
	rand.Seed(time.Now().UnixNano())
	for i := 1; i <= 3; i++ {
		base := 60 + i*5
		temp := float64(base + rand.Intn(10) - 5)
		status := "working"
		db.Exec("INSERT INTO sensor_logs (timestamp, machine_id, temperature, vibration, status) VALUES (?, ?, ?, ?, ?)",
			float64(now), i, temp, rand.Float64()*2+0.5, status)
	}
}

// --- МОДУЛЬ ПРОСТОГО ИИ (АНАЛИТИКА) ---

func analyzeData(db *sql.DB) []Insight {
	var insights []Insight

	// 1. Анализ температуры (Предсказание перегрева)
	c := db.QueryRow("SELECT AVG(temperature), MAX(temperature) FROM sensor_logs WHERE timestamp > ?", time.Now().Add(-10*time.Minute).Unix())
	var avgTemp, maxTemp float64
	err := c.Scan(&avgTemp, &maxTemp)
	if err == nil && avgTemp > 0 {
		if maxTemp > avgTemp*1.2 && maxTemp > 85 {
			insights = append(insights, Insight{
				Type:        "prediction",
				Severity:    "high",
				Title:       "Риск перегрева оборудования",
				Description: fmt.Sprintf("Температура достигла %.1f°C (средняя: %.1f°C). Темп роста критический.", maxTemp, avgTemp),
				Action:      "Снизить нагрузку или включить охлаждение немедленно.",
			})
		}
	}

	// 2. Анализ брака (Корреляция с нагрузкой)
	var totalDef, totalIns int
	db.QueryRow("SELECT SUM(defects_count), SUM(inspected_count) FROM quality_logs WHERE timestamp > ?", time.Now().Add(-1*time.Hour).Unix()).Scan(&totalDef, &totalIns)
	if totalIns > 0 {
		currentRate := float64(totalDef) / float64(totalIns) * 100
		var prevDef, prevIns int
		db.QueryRow("SELECT SUM(defects_count), SUM(inspected_count) FROM quality_logs WHERE timestamp > ? AND timestamp < ?",
			time.Now().Add(-2*time.Hour).Unix(), time.Now().Add(-1*time.Hour).Unix()).Scan(&prevDef, &prevIns)

		if prevIns > 0 {
			prevRate := float64(prevDef) / float64(prevIns) * 100
			if currentRate > prevRate*1.5 && currentRate > 2.0 {
				insights = append(insights, Insight{
					Type:        "alert",
					Severity:    "critical",
					Title:       "Резкий рост брака",
					Description: fmt.Sprintf("Брак вырос с %.2f%% до %.2f%% за последний час.", prevRate, currentRate),
					Action:      "Остановить линию для проверки качества сырья или настройки оборудования.",
				})
			}
		}
	}

	// 3. Прогноз износа
	var criticalCount int
	db.QueryRow("SELECT COUNT(*) FROM downtime_logs WHERE timestamp > ?", time.Now().Add(-24*time.Hour).Unix()).Scan(&criticalCount)
	if criticalCount > 3 {
		insights = append(insights, Insight{
			Type:        "recommendation",
			Severity:    "medium",
			Title:       "Плановое ТО необходимо",
			Description: "Зафиксировано более 3 сбоев за сутки. Оборудование требует обслуживания.",
			Action:      "Запланировать техническое обслуживание на ближайшую смену.",
		})
	}

	return insights
}

// --- ОБРАБОТЧИК ДАННЫХ ---

func getDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	resp := Response{
		Statuses: map[string]string{"Сварка": "normal", "Окраска": "normal", "Сборка": "normal"},
		Schema:   map[string]string{},
	}

	var totalDef, totalIns int
	db.QueryRow("SELECT COALESCE(SUM(defects_count), 0), COALESCE(SUM(inspected_count), 1) FROM quality_logs").Scan(&totalDef, &totalIns)
	currentRate := float64(totalDef) / float64(totalIns) * 100
	resp.DefectRate = currentRate
	resp.TrendDefect = 0.5

	rows, _ := db.Query("SELECT machine_id, status FROM sensor_logs ORDER BY timestamp DESC LIMIT 10")
	defer rows.Close()
	for rows.Next() {
		var mid int
		var status string
		rows.Scan(&mid, &status)

		lineName := ""
		if mid == 1 {
			lineName = "Сварка"
		} else if mid == 2 {
			lineName = "Окраска"
		} else if mid == 3 {
			lineName = "Сборка"
		}

		if status == "critical" {
			resp.Statuses[lineName] = "critical"
			resp.Schema[lineName] = "critical"
			break
		} else if status == "warning" {
			if resp.Statuses[lineName] != "critical" {
				resp.Statuses[lineName] = "warning"
				resp.Schema[lineName] = "warning"
			}
		}
	}

	var plans []Plan
	rows2, _ := db.Query("SELECT model, target, produced FROM monthly_plans")
	defer rows2.Close()
	var totalT, totalP int
	for rows2.Next() {
		var p Plan
		rows2.Scan(&p.Model, &p.Target, &p.Produced)
		p.Produced += totalIns / 3
		p.Remaining = p.Target - p.Produced
		plans = append(plans, p)
		totalT += p.Target
		totalP += p.Produced
	}
	resp.Plans = plans
	if totalT > 0 {
		resp.Progress = float64(totalP) / float64(totalT) * 100
	} else {
		resp.Progress = 0
	}

	var downtimes []Downtime
	rows3, _ := db.Query("SELECT line, reason, duration, equipment FROM downtime_logs ORDER BY id DESC LIMIT 5")
	defer rows3.Close()
	for rows3.Next() {
		var d Downtime
		rows3.Scan(&d.Line, &d.Reason, &d.Duration, &d.Equipment)
		downtimes = append(downtimes, d)
	}
	resp.Downtimes = downtimes

	var supplies []Supply
	rows4, _ := db.Query("SELECT part_name, stock, status FROM supplies WHERE status IN ('critical', 'warning')")
	defer rows4.Close()
	for rows4.Next() {
		var s Supply
		rows4.Scan(&s.PartName, &s.Stock, &s.Status)
		supplies = append(supplies, s)
	}
	resp.Supplies = supplies

	var chart []ChartData
	rows5, _ := db.Query("SELECT timestamp, machine_id, temperature FROM sensor_logs ORDER BY timestamp DESC LIMIT 20")
	defer rows5.Close()
	temps := make(map[int][]float64)
	times := make([]string, 0)

	for rows5.Next() {
		var ts float64
		var mid int
		var temp float64
		rows5.Scan(&ts, &mid, &temp)
		t := time.Unix(int64(ts), 0).Format("15:04:05")
		temps[mid] = append(temps[mid], temp)
		if len(times) == 0 || times[len(times)-1] != t {
			times = append(times, t)
		}
	}

	for i := len(times) - 1; i >= 0; i-- {
		t := times[i]
		c := ChartData{Time: t}
		if len(temps[1]) > 0 {
			idx := (len(temps[1]) - 1 - i) % len(temps[1])
			if idx < 0 {
				idx += len(temps[1])
			}
			c.Svar = temps[1][idx]
		}
		if len(temps[2]) > 0 {
			idx := (len(temps[2]) - 1 - i) % len(temps[2])
			if idx < 0 {
				idx += len(temps[2])
			}
			c.Okr = temps[2][idx]
		}
		if len(temps[3]) > 0 {
			idx := (len(temps[3]) - 1 - i) % len(temps[3])
			if idx < 0 {
				idx += len(temps[3])
			}
			c.Sbor = temps[3][idx]
		}
		chart = append(chart, c)
	}
	resp.Chart = chart

	score := 100
	if resp.DefectRate > 2 {
		score -= 20
	}
	if resp.DefectRate > 5 {
		score -= 30
	}
	for _, s := range resp.Statuses {
		if s == "critical" {
			score -= 25
		}
		if s == "warning" {
			score -= 10
		}
	}
	if len(resp.Supplies) > 0 {
		score -= 15
	}
	if score < 0 {
		score = 0
	}
	resp.HealthScore = score

	// Добавляем инсайты от ИИ
	resp.Insights = analyzeData(db)

	json.NewEncoder(w).Encode(resp)
}

func main() {
	initDB()

	go func() {
		db, _ := sql.Open("sqlite3", dbPath)
		defer db.Close()
		for {
			simulateLiveData(db)
			time.Sleep(3 * time.Second)
		}
	}()

	http.HandleFunc("/api/data", getDataHandler)

	if _, err := os.Stat("static"); os.IsNotExist(err) {
		os.MkdirAll("static", 0755)
		fmt.Println("⚠️ Папка static создана. Положите туда index.html!")
	}

	http.Handle("/", http.FileServer(http.Dir("static")))

	fmt.Println("🚀 Сервер запущен: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}