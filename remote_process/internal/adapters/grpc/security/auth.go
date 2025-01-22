package security

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authHeader = "authorization"
	bearer     = "bearer"
)

type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

type UserClaims struct {
	jwt.StandardClaims
	Username string `json:"username"`
	Role     string `json:"role"`
}

func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:     secretKey,
		tokenDuration: tokenDuration,
	}
}

// GenerateToken genera un nuevo JWT token
func (m *JWTManager) GenerateToken(username, role string) (string, error) {
	claims := UserClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(m.tokenDuration).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
		Username: username,
		Role:     role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		log.Printf("Error generando token para usuario %s con rol %s: %v", username, role, err)
		return "", err
	}
	log.Printf("Token generado para usuario %s con rol %s", username, role)
	return signedToken, nil
}

// ValidateToken valida y extrae la información del token
func (m *JWTManager) ValidateToken(accessToken string) (*UserClaims, error) {
	log.Printf("Validando token de acceso: %s******", accessToken[:6])
	token, err := jwt.ParseWithClaims(
		accessToken,
		&UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			_, ok := token.Method.(*jwt.SigningMethodHMAC)
			if !ok {
				return nil, fmt.Errorf("unexpected token signing method")
			}
			return []byte(m.secretKey), nil
		},
	)

	if err != nil {
		log.Printf("Token inválido: %v", err)
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		log.Println("Token claims inválidos")
		return nil, fmt.Errorf("invalid token claims")
	}

	log.Printf("Token validado para usuario %s con rol %s", claims.Username, claims.Role)
	return claims, nil
}

// AuthInterceptor implementa la autenticación para gRPC
type AuthInterceptor struct {
	jwtManager      *JWTManager
	accessibleRoles map[string][]string
}

func NewAuthInterceptor(jwtManager *JWTManager, accessibleRoles map[string][]string) *AuthInterceptor {
	return &AuthInterceptor{
		jwtManager:      jwtManager,
		accessibleRoles: accessibleRoles,
	}
}

// Unary interceptor para métodos unarios
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		log.Printf("Interceptando método unario: %s", info.FullMethod)
		if err := i.authorize(ctx, info.FullMethod); err != nil {
			log.Printf("Autorización fallida para método unario: %s, error: %v", info.FullMethod, err)
			return nil, err
		}
		log.Printf("Autorización exitosa para método unario: %s", info.FullMethod)
		return handler(ctx, req)
	}
}

// Stream interceptor para métodos streaming
func (i *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		log.Printf("Interceptando método streaming: %s", info.FullMethod)
		if err := i.authorize(stream.Context(), info.FullMethod); err != nil {
			log.Printf("Autorización fallida para método streaming: %s, error: %v", info.FullMethod, err)
			return err
		}
		log.Printf("Autorización exitosa para método streaming: %s", info.FullMethod)
		return handler(srv, stream)
	}
}

func (i *AuthInterceptor) authorize(ctx context.Context, method string) error {
	accessibleRoles, ok := i.accessibleRoles[method]
	if !ok {
		// Si el método no está en el mapa, permitir acceso por defecto
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("Metadata no proporcionada")
		return status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md.Get(authHeader)
	if len(values) == 0 {
		log.Println("Token de autorización no proporcionado")
		return status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}
	log.Printf("Token de autorización: %s******", values[0][:6])

	accessToken := values[0]
	if !strings.HasPrefix(strings.ToLower(accessToken), bearer) {
		log.Println("Tipo de autenticación inválido")
		return status.Errorf(codes.Unauthenticated, "invalid auth type")
	}

	accessToken = strings.TrimPrefix(accessToken, bearer)
	accessToken = strings.TrimSpace(accessToken)

	claims, err := i.jwtManager.ValidateToken(accessToken)
	if err != nil {
		log.Printf("Token de acceso inválido: %v", err)
		return status.Errorf(codes.Unauthenticated, "access token is invalid: %v", err)
	}
	log.Printf("Token de acceso válido para usuario %s con rol %s", claims.Username, claims.Role)

	for _, role := range accessibleRoles {
		log.Printf("Revisando si el rol %s es permitido para el usuario %s", role, claims.Username)
		if role == claims.Role {
			log.Printf("Acceso autorizado para usuario %s con rol %s al método %s", claims.Username, claims.Role, method)
			return nil
		}

	}

	log.Printf("Permiso denegado para usuario %s con rol %s al método %s", claims.Username, claims.Role, method)
	return status.Errorf(codes.PermissionDenied, "no permission to access this RPC")
}
