package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPasswordHash(password, hash string) error {
	h, p := []byte(hash), []byte(password)
	return bcrypt.CompareHashAndPassword(h, p)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	issueTime := time.Now().Local().UTC()
	expireTime := issueTime.Add(expiresIn)
	register := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  &jwt.NumericDate{Time: issueTime},
		ExpiresAt: &jwt.NumericDate{Time: expireTime},
		Subject:   userID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, register)
	stingToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return stingToken, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected Signing Method: %v", t.Header["alg"])
		}
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if token.Valid && ok {
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	}
	return uuid.Nil, fmt.Errorf("Invalid Token")
}

func GetBearerToken(headers http.Header) (string, error) {
	auth := headers.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("Authorization header not present")
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return "", fmt.Errorf("Authorization header format must be 'Bearer {token}'")
	}

	parts := strings.Fields(auth)

	if len(parts) != 2 {
		return "", fmt.Errorf("Authorization header must have exactly two parts")
	}

	bearerToken := parts[1]
	return bearerToken, nil
}
