package auth

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/NobleMajo/intern-auth-gateway/internal/config"
)

// CheckBasicAuth verifies username/password against a single service credential.
// Returns true only when the username matches and the password verifies against
// the stored bcrypt hash.
func CheckBasicAuth(cred config.ServiceCred, username, password string) bool {
	if username == "" || password == "" {
		return false
	}
	if username != cred.Username {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(password))
	return err == nil
}

// CheckBasicAuthAgainstServices returns the first matching service credential
// for which username/password are valid. ok is false when none match.
func CheckBasicAuthAgainstServices(creds []config.ServiceCred, username, password string) (config.ServiceCred, bool) {
	for _, cred := range creds {
		if CheckBasicAuth(cred, username, password) {
			return cred, true
		}
	}
	return config.ServiceCred{}, false
}
