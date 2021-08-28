package grails

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HykoAPI/grails/errhelpers"
	"net/http"
	"os"
	"strings"
	"time"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
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


type Claims struct {
	UserID uint `json:"user_id"`
	jwt.Claims
}

type User interface {
	GetRole() string
}

var jwtKey = getJWTSigningKey()

func getJWTSigningKey() []byte {
	if os.Getenv("ENVIRONMENT") == "" {
		return []byte("LOCAL_MOCK_JWT_SIGNING_KEY")
	}

	key := os.Getenv("JWT_SIGNING_KEY")
	if key == "" {
		// Without this we can issue tokens or verify them so we shouldn't be running
		panic("JWT Signing key not found")
	}
	return []byte(key)
}

func ProtectedRoute(db *gorm.DB, fetchUser func(*gorm.DB, uint) (User, error), roles []Role, handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		claims := &Claims{}
		parsedJWT, err := jwt.ParseSigned(tokenString)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		err = parsedJWT.Claims(jwtKey, claims)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Get user
		user, err := fetchUser(db, claims.UserID)
		if err != nil {
			http.Error(w, errhelpers.Augment("error getting user by id", err).Error(), http.StatusInternalServerError)
			return
		}

		// Check roles
		hasRole := false
		for _, r := range roles {
			if string(r) == user.GetRole() {
				hasRole = true
				break
			}
		}

		if !hasRole {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if claims.Expiry.Time().Before(time.Now().UTC()) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

type Middleware func(func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request)

const TOKEN_DURATION_IN_MINUTES = 5 * time.Minute

func IssueAuthToken(userID uint) (string, error) {
	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		UserID: userID,
		Claims: jwt.Claims{
			Expiry: jwt.NewNumericDate(time.Now().UTC().Add(TOKEN_DURATION_IN_MINUTES)),
		},
	}

	var signerOpts = jose.SignerOptions{}
	signerOpts.WithType("JWT")
	signingKey := jose.SigningKey{
		Algorithm: jose.HS256,
		Key:       jwtKey,
	}
	jwtSigner, err := jose.NewSigner(signingKey, &signerOpts)
	if err != nil {
		return "", err
	}
	builder := jwt.Signed(jwtSigner).Claims(claims)
	token, err := builder.CompactSerialize()
	if err != nil {
		return "", err
	}

	return token, nil
}

func GetUserFromRequest(fetchUser func(*gorm.DB, uint) (User, error)) func(db *gorm.DB, r *http.Request)(User, error) {
	return func(db *gorm.DB, r *http.Request) (User, error) {
		tokenString := r.Header.Get("Authorization")
		claims := &Claims{}
		parsedJWT, err := jwt.ParseSigned(tokenString)
		if err != nil {
			return nil, err
		}
		err = parsedJWT.Claims(jwtKey, claims)
		if err != nil {
			return nil, err
		}

		user, err := fetchUser(db, claims.UserID)
		if err != nil {
			return nil, err
		}
		return user, nil
	}

}
