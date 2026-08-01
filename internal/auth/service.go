package auth

import (
	"fmt"
	"os"
	"strings"
)

const serviceEnvPrefix = "SERVICE_"

// ServiceCred is one SERVICE_* entry: host glob, username, and bcrypt password hash.
type ServiceCred struct {
	HostGlob     string
	Username     string
	PasswordHash string
}

// LoadServicesFromEnv scans SERVICE_* environment variables, parses each value as
// hostGlob/username/passwordHash (SplitN; hash may contain '/'), and rejects
// duplicate usernames across services.
func LoadServicesFromEnv() (map[string]ServiceCred, error) {
	services := make(map[string]ServiceCred)
	usernames := make(map[string]string) // username -> service name

	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(key, serviceEnvPrefix) {
			continue
		}
		name := strings.TrimPrefix(key, serviceEnvPrefix)
		if name == "" {
			return nil, fmt.Errorf("invalid service env %q: empty service name", key)
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		cred, err := parseServiceValue(key, value)
		if err != nil {
			return nil, err
		}
		if other, exists := usernames[cred.Username]; exists {
			return nil, fmt.Errorf(
				"duplicate username %q in %s and SERVICE_%s",
				cred.Username,
				key,
				other,
			)
		}
		usernames[cred.Username] = name
		services[name] = cred
	}
	return services, nil
}

func parseServiceValue(envKey, value string) (ServiceCred, error) {
	parts := strings.SplitN(value, "/", 3)
	if len(parts) != 3 {
		return ServiceCred{}, fmt.Errorf(
			"invalid %s value %q: want hostGlob/username/passwordHash (exactly 2 separating slashes)",
			envKey,
			value,
		)
	}
	hostGlob := strings.TrimSpace(parts[0])
	username := strings.TrimSpace(parts[1])
	passwordHash := strings.TrimSpace(parts[2])
	if hostGlob == "" || username == "" || passwordHash == "" {
		return ServiceCred{}, fmt.Errorf(
			"invalid %s value %q: hostGlob, username, and passwordHash must be non-empty",
			envKey,
			value,
		)
	}
	return ServiceCred{
		HostGlob:     hostGlob,
		Username:     username,
		PasswordHash: passwordHash,
	}, nil
}
