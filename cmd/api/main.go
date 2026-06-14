package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/rafliesa/aravoice-backend/internal/config"
	"github.com/rafliesa/aravoice-backend/internal/database"
	"github.com/rafliesa/aravoice-backend/internal/news"
	"github.com/rafliesa/aravoice-backend/internal/upload"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	mux := http.NewServeMux()

	newsRepository := news.NewRepository(db)
	newsService := news.NewService(newsRepository)
	newsHandler := news.NewHandler(newsService)
	newsHandler.RegisterRoutes(mux)

	uploadHandler, err := upload.NewHandler(cfg.UploadDir)
	if err != nil {
		log.Fatal(err)
	}
	uploadHandler.RegisterRoutes(mux)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := ":" + cfg.Port

	fmt.Println("Server runing on", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
