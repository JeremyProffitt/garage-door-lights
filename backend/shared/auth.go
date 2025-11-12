package shared

import (
    "log"

    "golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
    log.Println("[AUTH] Hashing password with bcrypt (cost=14)")
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    if err != nil {
        log.Printf("[AUTH] ERROR: Failed to hash password: %v", err)
        return "", err
    }
    log.Println("[AUTH] Password hashed successfully")
    return string(bytes), nil
}

// CheckPasswordHash validates a password against a hash
func CheckPasswordHash(password, hash string) bool {
    log.Println("[AUTH] Validating password against hash")
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    if err != nil {
        log.Println("[AUTH] Password validation failed")
        return false
    }
    log.Println("[AUTH] Password validated successfully")
    return true
}
