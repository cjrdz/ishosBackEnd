package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"
)

// InsufficientStockError is returned when an order needs more of a tracked
// flavour or product than its location has. The message is shown to staff
// as is, so it says what is missing and what to do about it.
type InsufficientStockError struct {
	Name      string
	Unit      string // "bola" or "unidad"
	Needed    int
	Available int
}

func (e *InsufficientStockError) Error() string {
	unit := e.Unit
	if unit == "" {
		unit = "unidad"
	}
	plural := func(n int) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, unit)
		}
		if strings.HasSuffix(unit, "d") {
			return fmt.Sprintf("%d %ses", n, unit)
		}
		return fmt.Sprintf("%d %ss", n, unit)
	}
	if e.Available <= 0 {
		return fmt.Sprintf("No queda %s en esta sucursal (la orden necesita %s). Registra una entrada en Inventario o cambia la selección.",
			e.Name, plural(e.Needed))
	}
	return fmt.Sprintf("No alcanza %s: la orden necesita %s y quedan %s. Registra una entrada en Inventario o ajusta la orden.",
		e.Name, plural(e.Needed), plural(e.Available))
}

func (e *InsufficientStockError) Unwrap() error {
	return ErrConflict
}

// sentinelPrefixes are the "<sentinel>: " prefixes fmt.Errorf("%w: ...")
// adds; the text after them is the part meant for people.
var sentinelPrefixes = []string{
	ErrConflict.Error() + ": ",
	ErrValidation.Error() + ": ",
	ErrNotFound.Error() + ": ",
	ErrForbidden.Error() + ": ",
	ErrUnauthorized.Error() + ": ",
	ErrRateLimited.Error() + ": ",
	ErrUnavailable.Error() + ": ",
}

// defaultMessages are the Spanish messages used when an error carries no
// text of its own.
var defaultMessages = map[int]string{
	http.StatusBadRequest:          "Revisa los datos e intenta de nuevo.",
	http.StatusUnauthorized:        "Tu sesión expiró. Inicia sesión de nuevo.",
	http.StatusForbidden:           "No tienes permiso para hacer esto.",
	http.StatusNotFound:            "No se encontró lo que buscabas.",
	http.StatusConflict:            "No se pudo completar porque los datos cambiaron. Actualiza e intenta de nuevo.",
	http.StatusTooManyRequests:     "Demasiados intentos. Espera un momento e intenta de nuevo.",
	http.StatusServiceUnavailable:  "El servicio no está disponible por ahora. Intenta de nuevo en unos minutos.",
	http.StatusInternalServerError: "Ocurrió un error inesperado. Intenta de nuevo.",
}

// DefaultMessage returns the Spanish fallback message for an HTTP status.
func DefaultMessage(status int) string {
	if msg, ok := defaultMessages[status]; ok {
		return msg
	}
	return defaultMessages[http.StatusInternalServerError]
}

// PublicMessage returns the message to show people for a recognised app
// error: the specific text when there is one (without the sentinel prefix),
// otherwise a Spanish default for its status.
func PublicMessage(err error) string {
	status := StatusCode(err)
	if status == http.StatusInternalServerError {
		return DefaultMessage(status)
	}

	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return DefaultMessage(http.StatusNotFound)
	}
	var validation *ValidationError
	if errors.As(err, &validation) && len(validation.Fields) > 0 {
		keys := make([]string, 0, len(validation.Fields))
		for k := range validation.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		msgs := make([]string, 0, len(keys))
		for _, k := range keys {
			msgs = append(msgs, validation.Fields[k])
		}
		return strings.Join(msgs, ". ")
	}
	var conflict *ConflictError
	if errors.As(err, &conflict) {
		return DefaultMessage(http.StatusConflict)
	}
	// Auth failures carry technical reasons ("token revoked"); only the
	// Spanish inactivity notice is worth showing as is.
	var unauthorized *UnauthorizedError
	if errors.As(err, &unauthorized) {
		if strings.Contains(unauthorized.Reason, "sesión") {
			return unauthorized.Reason
		}
		return DefaultMessage(http.StatusUnauthorized)
	}

	msg := err.Error()
	for _, prefix := range sentinelPrefixes {
		msg = strings.TrimPrefix(msg, prefix)
	}
	for _, sentinel := range []error{ErrConflict, ErrValidation, ErrNotFound, ErrForbidden, ErrUnauthorized, ErrRateLimited, ErrUnavailable} {
		if msg == sentinel.Error() {
			return DefaultMessage(status)
		}
	}
	if strings.TrimSpace(msg) == "" {
		return DefaultMessage(status)
	}
	return capitalize(msg)
}

// capitalize upper-cases the first letter of a message shown on its own.
func capitalize(msg string) string {
	r := []rune(msg)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
