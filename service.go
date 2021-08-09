package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"


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
