package shared

import (
    "log"

    "golang.org/x/crypto/bcrypt"
)

// PasswordCost is the bcrypt cost parameter for new passwords
const PasswordCost = 10

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
    log.Printf("[AUTH] Hashing password with bcrypt (cost=%d)", PasswordCost)
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
    if err != nil {
        log.Printf("[AUTH] ERROR: Failed to hash password: %v", err)
        return "", err
    }
    log.Println("[AUTH] Password hashed successfully")
    return string(bytes), nil
}

// NeedsRehash checks if the password hash needs to be updated (e.g. if cost is different from current)
func NeedsRehash(hash string) bool {
    cost, err := bcrypt.Cost([]byte(hash))
    if err != nil {
        return false // If we can't determine cost, assume it's fine
    }
    return cost != PasswordCost
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
