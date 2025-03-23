package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	tokenExp   = time.Hour * 3000
	secretKey  = "supersecretkey"
	cookieName = "gophermart_auth"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID string
}

type loginContextKey struct{}

func SetAuthCookie(login string, res http.ResponseWriter) error {
	JWT, err := buildJWTString(login)
	if err != nil {
		return err
	}
	var authCookieOut http.Cookie
	authCookieOut.Name = cookieName
	authCookieOut.Value = JWT
	http.SetCookie(res, &authCookieOut)
	return nil
}

// BuildJWTString создаёт токен и возвращает его в виде строки.
func buildJWTString(login string) (string, error) {
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			// когда создан токен
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExp)),
		},
		UserID: login,
	})

	// создаём строку токена
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	// возвращаем строку токена
	return tokenString, nil
}

func Auth(h http.HandlerFunc) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var login = ""
		authCookieIn, err := req.Cookie(cookieName)
		if err == nil {
			login, err = getLogin(authCookieIn.Value)
		}
		if err != nil {
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		if login == "" {
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(req.Context(), loginContextKey{}, login)
		req = req.WithContext(ctx)
		h.ServeHTTP(res, req)
	}
}

func getLogin(tokenString string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secretKey), nil
		})
	if err != nil {
		return "", err
	}

	if !token.Valid {
		return "", fmt.Errorf("token is not valid")
	}

	return claims.UserID, nil
}

func LoginFromContext(ctx context.Context) string {
	return ctx.Value(loginContextKey{}).(string)
}
