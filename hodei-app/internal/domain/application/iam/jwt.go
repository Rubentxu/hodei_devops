package iam

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

var jwtSecretKey = "mi_clave_super_secreta"

type CustomClaims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

func GenerateJWT(subject model.Subject) (string, error) {
	claims := CustomClaims{
		UserID: subject.GetID().String(),
		Roles:  subject.GetRoles(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "mi_aplicacion",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecretKey))
}

func ValidateJWT(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{},
		func(token *jwt.Token) (interface{}, error) { return []byte(jwtSecretKey), nil })
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("token inválido: %v", err)
	}
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, errors.New("no se pudieron obtener claims")
	}
	return claims, nil
}
