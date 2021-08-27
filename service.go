package grails

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"gorm.io/gorm"
)

type Role string

type Service interface {
	Setup(db *gorm.DB) error
	GetHandlers() map[string]Handler
	GetPrefix() string
}

type Handler interface {
	Handle(w http.ResponseWriter, r *http.Request)
	GetRoles() (isProtected bool, roles []Role)
	GetHTTPMethods() []string
}

func HandleWithErrorToHandle(withError func(w http.ResponseWriter, r *http.Request) (interface{}, *ResponseError)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rsp, rspError := withError(w, r)
		if rspError != nil {
			fmt.Println(rspError.Error.Error())
			http.Error(w, rspError.Error.Error(), rspError.StatusCode)
			return
		}
		if rsp == nil {
			return
		}
		data, err := json.Marshal(rsp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = w.Write(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type ResponseError struct {
	Error      error
	StatusCode int
}

func AugmentWithStatusCode(message string, err error, statusCode int) *ResponseError {
	if err != nil {
		return &ResponseError{
			Error:      Augment(message, err),
			StatusCode: statusCode,
		}
	}
	return &ResponseError{
		Error:      errors.New(message),
		StatusCode: statusCode,
	}
}

func Augment(message string, err error) error {
	return errors.New(fmt.Sprintf(message+": %v", err))
}

func originValid(origin string) bool {
	corsOrigins := strings.Split(os.Getenv("CORS_ORIGINS"), ",")
	for _, o := range corsOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

func CORS(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !originValid(r.Header.Get("Origin")) {
			http.Error(w, "Invalid Origin", http.StatusUnauthorized)
			return
		}

		(w).Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		(w).Header().Set("Access-Control-Allow-Credentials", "true")
		(w).Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE") // TODO: have this read from the handler some how?
		(w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// Based off: https://www.html5rocks.com/static/images/cors_server_flowchart.png
		accessControlRequestMethod := r.Header.Values("Access-Control-Request-Method")
		if r.Method == "OPTIONS" && len(accessControlRequestMethod) != 0 {
			// This means it is a preflight request sent from a browser so we can just return
			return
		}

		handler(w, r)
	}
}