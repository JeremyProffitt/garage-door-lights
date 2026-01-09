package shared

import (
    "golang.org/x/crypto/bcrypt"
)

const PasswordCost = 10

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
    return string(bytes), err
}

// CheckPasswordHash validates a password against a hash
func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// NeedsRehash checks if the password hash needs to be updated (e.g. if cost is too high)
func NeedsRehash(hash string) bool {
    cost, err := bcrypt.Cost([]byte(hash))
    if err != nil {
        return false // If we can't determine cost, assume it's fine to avoid loop
    }
    return cost > PasswordCost
}
