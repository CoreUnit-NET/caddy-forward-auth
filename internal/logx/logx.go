package logx

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Atom formats a log field value: empty -> "-", otherwise the raw value, or
// a quoted form when the value contains whitespace or quotes.
func Atom(value string) string {
	if value == "" {
		return "-"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '"'
	}) >= 0 {
		return strconv.Quote(value)
	}
	return value
}

// AuthEvent writes one structured auth probe line.
// Rejections (status >= 400) always include detail and hint when provided.
// Successes include detail only when verbose is true.
func AuthEvent(
	logger *log.Logger,
	verbose bool,
	status int,
	method, path, host, origin, ip, service, user, reason, detail, hint string,
) {
	if status < 400 && !verbose {
		return
	}
	fields := []string{
		"section=auth",
		fmt.Sprintf("status=%d", status),
		"method=" + Atom(method),
		"path=" + Atom(path),
		"host=" + Atom(host),
		"origin=" + Atom(origin),
		"ip=" + Atom(ip),
		"service=" + Atom(service),
		"user=" + Atom(user),
		"reason=" + Atom(reason),
	}
	if detail != "" && (status >= 400 || verbose) {
		fields = append(fields, "detail="+Atom(detail))
	}
	if hint != "" && status >= 400 {
		fields = append(fields, "hint="+Atom(hint))
	}
	logger.Printf("%s", strings.Join(fields, " "))
}

// Info writes a structured info line with section and msg fields.
func Info(logger *log.Logger, section, msg string, extra ...string) {
	fields := []string{
		"section=" + Atom(section),
		"level=info",
		"msg=" + Atom(msg),
	}
	fields = append(fields, extra...)
	logger.Printf("%s", strings.Join(fields, " "))
}

// Warn writes a structured warning line with optional detail and hint.
func Warn(logger *log.Logger, section, msg, detail, hint string, extra ...string) {
	fields := []string{
		"section=" + Atom(section),
		"level=warn",
		"msg=" + Atom(msg),
	}
	if detail != "" {
		fields = append(fields, "detail="+Atom(detail))
	}
	if hint != "" {
		fields = append(fields, "hint="+Atom(hint))
	}
	fields = append(fields, extra...)
	logger.Printf("%s", strings.Join(fields, " "))
}

// Error writes a structured error line with optional detail and hint.
func Error(logger *log.Logger, section, msg string, err error, hint string, extra ...string) {
	fields := []string{
		"section=" + Atom(section),
		"level=error",
		"msg=" + Atom(msg),
	}
	if err != nil {
		fields = append(fields, "error="+Atom(err.Error()))
	}
	if hint != "" {
		fields = append(fields, "hint="+Atom(hint))
	}
	fields = append(fields, extra...)
	logger.Printf("%s", strings.Join(fields, " "))
}

// PersistError logs a periodic persistence failure with path context.
func PersistError(logger *log.Logger, component, path, op string, err error) {
	hint := dataDirHint(err)
	Error(logger, "persist", "periodic save failed", err, hint,
		"component="+Atom(component),
		"path="+Atom(path),
		"op="+Atom(op),
	)
}

// Fatal prints a user-facing fatal error to stderr and exits.
func Fatal(err error) {
	if err == nil {
		os.Exit(1)
	}
	msg := err.Error()
	hint := startupHint(err)
	if hint != "" {
		fmt.Fprintf(os.Stderr, "error: %s\nhint: %s\n", msg, hint)
	} else {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	os.Exit(1)
}

func dataDirHint(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "permission denied") {
		return "ensure ./data is writable by the process user; in Docker check compose user UID:GID and run chown -R $UID:$GID ./data"
	}
	return ""
}

func startupHint(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no SERVICE_* entries"):
		return "set at least one SERVICE_<name>=hostGlob/username/bcryptHash in .env or the environment"
	case strings.Contains(msg, "duplicate username"):
		return "each SERVICE_* entry must use a unique username"
	case strings.Contains(msg, "passwordHash"):
		return "use a valid bcrypt hash; in Docker Compose escape each $ as $$ in env files"
	case strings.Contains(msg, "invalid service env"):
		return "SERVICE_ keys must be SERVICE_<name> with a non-empty name"
	case strings.Contains(msg, "want hostGlob/username/passwordHash"):
		return "format each SERVICE_* value as hostGlob/username/passwordHash with exactly two slashes"
	case strings.Contains(msg, "permission denied"):
		return "ensure ./data exists and is writable by the process user (see compose.yml UID:GID)"
	case strings.Contains(msg, "parse"):
		return "repair or remove the corrupt JSON file under ./data and restart"
	default:
		return dataDirHint(err)
	}
}
