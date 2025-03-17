package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
	"time"
)

// TokenDocument representa el esquema del documento en MongoDB
type TokenDocument struct {
	UserID     string    `bson:"user_id"`
	TokenUUID  string    `bson:"token_uuid"`
	TokenType  string    `bson:"token_type"` // "access" o "refresh"
	ExpiresAt  time.Time `bson:"expires_at"`
	CreateDate time.Time `bson:"create_date"`
}

// tokenCacheItem representa un token en el caché en memoria
type tokenCacheItem struct {
	userID    string
	expiresAt time.Time
}

// MemoryCachedTokenRepository implementa TokenRepository utilizando un caché en memoria y MongoDB
type MemoryCachedTokenRepository struct {
	mongoDB       *mongo.Collection
	idGenerator   ports.IDGenerator
	accessCache   map[string]tokenCacheItem // Mapeo de accessUUID -> userID
	refreshCache  map[string]tokenCacheItem // Mapeo de refreshUUID -> userID
	mutex         sync.RWMutex              // Para proteger el acceso concurrente al caché
	cleanupTicker *time.Ticker              // Para limpiar tokens expirados del caché
}

// NewMemoryCachedTokenRepository crea una nueva instancia de MemoryCachedTokenRepository
func NewMemoryCachedTokenRepository(db *mongo.Database, idGenerator ports.IDGenerator) ports.TokenRepository {
	collection := db.Collection("tokens")

	// Crear índices para mejorar el rendimiento
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Índice para búsqueda por token_uuid
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "token_uuid", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// Índice para búsqueda por user_id
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
	})

	// Índice TTL para expiración automática
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})

	repo := &MemoryCachedTokenRepository{
		mongoDB:       collection,
		idGenerator:   idGenerator,
		accessCache:   make(map[string]tokenCacheItem),
		refreshCache:  make(map[string]tokenCacheItem),
		cleanupTicker: time.NewTicker(10 * time.Minute),
	}

	// Iniciar goroutine para limpiar tokens expirados del caché
	go func() {
		for range repo.cleanupTicker.C {
			repo.cleanExpiredTokens()
		}
	}()

	return repo
}

// cleanExpiredTokens elimina los tokens expirados del caché en memoria
func (r *MemoryCachedTokenRepository) cleanExpiredTokens() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()

	// Limpiar access tokens expirados
	for tokenUUID, item := range r.accessCache {
		if now.After(item.expiresAt) {
			delete(r.accessCache, tokenUUID)
		}
	}

	// Limpiar refresh tokens expirados
	for tokenUUID, item := range r.refreshCache {
		if now.After(item.expiresAt) {
			delete(r.refreshCache, tokenUUID)
		}
	}
}

// Cerrar recursos cuando el repositorio ya no se necesita
func (r *MemoryCachedTokenRepository) Close() {
	r.cleanupTicker.Stop()
}

// StoreAccessToken guarda un token de acceso en caché y en la base de datos
func (r *MemoryCachedTokenRepository) StoreAccessToken(ctx context.Context, userID, accessUUID string, expiry time.Duration) error {
	expiresAt := time.Now().Add(expiry)

	// Guardar en caché en memoria
	r.mutex.Lock()
	r.accessCache[accessUUID] = tokenCacheItem{
		userID:    userID,
		expiresAt: expiresAt,
	}
	r.mutex.Unlock()

	// Guardar en MongoDB para persistencia
	_, err := r.mongoDB.InsertOne(ctx, TokenDocument{
		UserID:     userID,
		TokenUUID:  accessUUID,
		TokenType:  "access",
		ExpiresAt:  expiresAt,
		CreateDate: time.Now(),
	})

	if err != nil {
		// Si falla la persistencia, eliminar de caché para mantener consistencia
		r.mutex.Lock()
		delete(r.accessCache, accessUUID)
		r.mutex.Unlock()
		return fmt.Errorf("failed to store access token in database: %w", err)
	}

	return nil
}

// StoreRefreshToken guarda un token de actualización en caché y en la base de datos
func (r *MemoryCachedTokenRepository) StoreRefreshToken(ctx context.Context, userID, refreshUUID string, expiry time.Duration) error {
	expiresAt := time.Now().Add(expiry)

	// Guardar en caché en memoria
	r.mutex.Lock()
	r.refreshCache[refreshUUID] = tokenCacheItem{
		userID:    userID,
		expiresAt: expiresAt,
	}
	r.mutex.Unlock()

	// Guardar en MongoDB para persistencia
	_, err := r.mongoDB.InsertOne(ctx, TokenDocument{
		UserID:     userID,
		TokenUUID:  refreshUUID,
		TokenType:  "refresh",
		ExpiresAt:  expiresAt,
		CreateDate: time.Now(),
	})

	if err != nil {
		// Si falla la persistencia, eliminar de caché para mantener consistencia
		r.mutex.Lock()
		delete(r.refreshCache, refreshUUID)
		r.mutex.Unlock()
		return fmt.Errorf("failed to store refresh token in database: %w", err)
	}

	return nil
}

// DeleteAccessToken elimina un token de acceso de la caché y la base de datos
func (r *MemoryCachedTokenRepository) DeleteAccessToken(ctx context.Context, accessUUID string) error {
	// Eliminar de caché en memoria
	r.mutex.Lock()
	delete(r.accessCache, accessUUID)
	r.mutex.Unlock()

	// Eliminar de MongoDB
	filter := bson.M{"token_uuid": accessUUID, "token_type": "access"}
	_, err := r.mongoDB.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete access token from database: %w", err)
	}

	return nil
}

// DeleteRefreshToken elimina un token de actualización de la caché y la base de datos
func (r *MemoryCachedTokenRepository) DeleteRefreshToken(ctx context.Context, refreshUUID string) error {
	// Eliminar de caché en memoria
	r.mutex.Lock()
	delete(r.refreshCache, refreshUUID)
	r.mutex.Unlock()

	// Eliminar de MongoDB
	filter := bson.M{"token_uuid": refreshUUID, "token_type": "refresh"}
	_, err := r.mongoDB.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token from database: %w", err)
	}

	return nil
}

// DeleteUserTokens elimina todos los tokens asociados a un usuario
func (r *MemoryCachedTokenRepository) DeleteUserTokens(ctx context.Context, userID string) error {
	// Primero, obtener todos los tokens del usuario desde MongoDB para eliminarlos de caché
	filter := bson.M{"user_id": userID}
	cursor, err := r.mongoDB.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to find user tokens: %w", err)
	}
	defer cursor.Close(ctx)

	// Eliminar cada token de la caché
	var tokens []TokenDocument
	if err := cursor.All(ctx, &tokens); err == nil {
		r.mutex.Lock()
		for _, token := range tokens {
			if token.TokenType == "access" {
				delete(r.accessCache, token.TokenUUID)
			} else if token.TokenType == "refresh" {
				delete(r.refreshCache, token.TokenUUID)
			}
		}
		r.mutex.Unlock()
	}

	// Eliminar todos los tokens del usuario de MongoDB
	_, err = r.mongoDB.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete user tokens from database: %w", err)
	}

	return nil
}

// ValidateAccessToken valida un token de acceso y devuelve el ID del usuario
func (r *MemoryCachedTokenRepository) ValidateAccessToken(ctx context.Context, accessUUID string) (string, error) {
	// Primero intentar obtener desde el caché en memoria
	r.mutex.RLock()
	item, found := r.accessCache[accessUUID]
	r.mutex.RUnlock()

	if found {
		// Comprobar si ha expirado
		if time.Now().After(item.expiresAt) {
			r.mutex.Lock()
			delete(r.accessCache, accessUUID)
			r.mutex.Unlock()
			return "", model.ErrTokenExpired
		}
		return item.userID, nil
	}

	// Si no está en caché, buscar en MongoDB
	filter := bson.M{"token_uuid": accessUUID, "token_type": "access"}
	var token TokenDocument
	err := r.mongoDB.FindOne(ctx, filter).Decode(&token)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", model.ErrInvalidToken
		}
		return "", fmt.Errorf("failed to validate token from database: %w", err)
	}

	// Si el token ha expirado
	if time.Now().After(token.ExpiresAt) {
		return "", model.ErrTokenExpired
	}

	// Actualizar el caché con el valor obtenido
	r.mutex.Lock()
	r.accessCache[accessUUID] = tokenCacheItem{
		userID:    token.UserID,
		expiresAt: token.ExpiresAt,
	}
	r.mutex.Unlock()

	return token.UserID, nil
}

// ValidateRefreshToken valida un token de actualización y devuelve el ID del usuario
func (r *MemoryCachedTokenRepository) ValidateRefreshToken(ctx context.Context, refreshUUID string) (string, error) {
	// Primero intentar obtener desde el caché en memoria
	r.mutex.RLock()
	item, found := r.refreshCache[refreshUUID]
	r.mutex.RUnlock()

	if found {
		// Comprobar si ha expirado
		if time.Now().After(item.expiresAt) {
			r.mutex.Lock()
			delete(r.refreshCache, refreshUUID)
			r.mutex.Unlock()
			return "", model.ErrRefreshTokenExpired
		}
		return item.userID, nil
	}

	// Si no está en caché, buscar en MongoDB
	filter := bson.M{"token_uuid": refreshUUID, "token_type": "refresh"}
	var token TokenDocument
	err := r.mongoDB.FindOne(ctx, filter).Decode(&token)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", model.ErrInvalidRefreshToken
		}
		return "", fmt.Errorf("failed to validate refresh token from database: %w", err)
	}

	// Si el token ha expirado
	if time.Now().After(token.ExpiresAt) {
		return "", model.ErrRefreshTokenExpired
	}

	// Actualizar el caché con el valor obtenido
	r.mutex.Lock()
	r.refreshCache[refreshUUID] = tokenCacheItem{
		userID:    token.UserID,
		expiresAt: token.ExpiresAt,
	}
	r.mutex.Unlock()

	return token.UserID, nil
}
