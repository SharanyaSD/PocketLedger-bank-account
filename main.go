// Command bank-account starts the HTTP server. It wires Store ->
// Service -> Handler -> Router and listens on :8080. PORT and DATA_FILE
// env vars override the defaults.
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"bank-account/handler"
	"bank-account/router"
	"bank-account/service"
	"bank-account/store"
)

const defaultDataFile = "bank-data.json"

func main() {
	gin.SetMode(gin.ReleaseMode)

	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = defaultDataFile
	}
	persister := store.NewPersister(dataFile)
	st := store.NewStore().WithPersister(persister)
	log.Printf("persistence: snapshot file %s", persister.Path())

	svc := service.NewService(st)
	h := handler.NewHandler(svc)
	r := router.New(h)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("bank-account API listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
