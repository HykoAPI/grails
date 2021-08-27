package grails

const serverTemplate = `
package webserver

import (
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"net/http"
)

func CreateRouter(db *gorm.DB) *mux.Router {
	r := mux.NewRouter()
	services := []service.Service{
	}

	for _, service := range services {
		err := service.Setup(db)
		if err != nil {
			panic(err)
		}
		registerHandlers(db, r, service.GetPrefix(), service.GetHandlers())
	}
	return r
}

type Middleware func(func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request)

func registerHandlers(db *gorm.DB, router *mux.Router, prefix string, handlers map[string]grails.Handler) {
	// Note: CORS must be the last middleware such that is is applied first
	var middlewares []Middleware = []Middleware{}

	for path, handler := range handlers {
		h := handler.Handle
		isProtected, roles := handler.GetRoles()
		if isProtected {
			h = auth.ProtectedRoute(db, roles, h)
		}
		for _, middleware := range middlewares {
			h = middleware(h)
		}
		// We append the Options method in order to allow CORS preflight requests
		router.HandleFunc(prefix+"/"+path, h).Methods(append(handler.GetHTTPMethods(), http.MethodOptions)...)
	}
}
`

const mainFileTemplate = `
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/HykoAPI/gocodebase/webserver"
	"github.com/getsentry/sentry-go"

	"github.com/HykoAPI/gocodebase/db"
	"github.com/gorilla/mux"
)

var r *mux.Router

func main() {
	db, err := db.GetDatabase()
	if err != nil {
		panic(err)
	}

	r := webserver.CreateRouter(db)

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(getPort(), nil))
}

func getPort() string {
	port := os.Getenv("PORT") // Heroku automatically assigns us a port using this variable
	if port == "" {
		port = "8000"
	}

	fmt.Println("🗼 Listening on http://localhost:" + port)

	return ":" + port
}

`
