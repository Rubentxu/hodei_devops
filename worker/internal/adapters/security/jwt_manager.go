package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

// JWTManager maneja la generación y validación de tokens JWT
type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

// Claims representa los claims del token JWT
type Claims struct {
	jwt.StandardClaims
	Role string `json:"role"`
}

// NewJWTManager crea una nueva instancia de JWTManager
func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey:     secretKey,
		tokenDuration: 24 * time.Hour,
	}
}

// GenerateToken genera un nuevo token JWT para un rol específico
func (m *JWTManager) GenerateToken(role string) (string, error) {
	claims := Claims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(m.tokenDuration).Unix(),
			IssuedAt:  time.Now().Unix(),
			Subject:   "worker-process",
		},
		Role: role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		return "", fmt.Errorf("error generando token JWT: %v", err)
	}

	return signedToken, nil
}

// ValidateToken valida un token JWT y retorna sus claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
			}
			return []byte(m.secretKey), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("error validando token: %v", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("claims inválidos")
	}

	return claims, nil
}
