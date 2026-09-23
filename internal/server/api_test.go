package server

import (
	"encoding/csv"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

func TestDailyExportUsesSelectedDateAndClickOrder(t *testing.T) {
	database, err := db.Init(filepath.Join(t.TempDir(), "export.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	quiet, err := database.CreateLink("quiet", "https://example.com/quiet", "", "Tools")
	if err != nil {
		t.Fatal(err)
	}
	top, err := database.CreateLink("top", "https://example.com/top", "", "Courses")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{PublicURL: "https://go.example.com", Timezone: "Asia/Jakarta"}
	day := time.Now().In(cfg.ReportLocation())
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, cfg.ReportLocation())
	for i := 0; i < 2; i++ {
		if _, err := database.Exec(`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, '', '')`, quiet.ID, day.Add(time.Hour+time.Duration(i)*time.Minute).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if _, err := database.Exec(`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, '', '')`, top.ID, day.Add(2*time.Hour+time.Duration(i)*time.Minute).Unix()); err != nil {
			t.Fatal(err)
		}
	}

	s := &Server{cfg: cfg, db: database}
	date := day.Format(dashboardDateLayout)
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/api/export?format=daily-csv&date="+date, nil)
	recorder := httptest.NewRecorder()
	s.handleDailyExport(recorder, r)

	if recorder.Code != 200 {
		t.Fatalf("daily export status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "plink-daily-clicks-"+date+".csv") {
		t.Fatalf("content disposition = %q", got)
	}
	reader := csv.NewReader(strings.NewReader(recorder.Body.String()))
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(header, ",") != "date,slug,short_url,destination,category,provider,channel,campaign,clicks" {
		t.Fatalf("csv header = %q", header)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != date || row[1] != "top" || row[8] != "5" || row[2] != "https://go.example.com/top" {
		t.Fatalf("top csv row = %#v", row)
	}
	row, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[1] != "quiet" || row[8] != "2" {
		t.Fatalf("quiet csv row = %#v", row)
	}
	if _, err := reader.Read(); err != io.EOF {
		t.Fatalf("extra csv rows, read error = %v", err)
	}
}
