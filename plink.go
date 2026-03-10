package plink

import (
	"embed"
	"log"
	"net/http"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
	"github.com/srmdn/plink/internal/server"
)

//go:embed web/admin.html
var webFS embed.FS

func Run() {
	cfg := config.Load()

	database, err := db.Init(cfg.DBPath)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer database.Close()

	srv := server.New(cfg, database, webFS)

	log.Printf("plink listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, srv); err != nil {
		log.Fatal(err)
	}
}
