package apperror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPublicMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"strips sentinel prefix", fmt.Errorf("%w: elige un topping", ErrValidation), "Elige un topping"},
		{"auth reasons stay generic", &UnauthorizedError{Reason: "token revoked"}, DefaultMessage(401)},
		{"inactivity notice is kept", &UnauthorizedError{Reason: "Tu sesión se cerró por inactividad."}, "Tu sesión se cerró por inactividad."},
		{"bare sentinel gets Spanish default", ErrConflict, DefaultMessage(409)},
		{"typed not found", &NotFoundError{Resource: "order"}, DefaultMessage(404)},
		{"internal stays generic", errors.New("pq: boom"), DefaultMessage(500)},
		{"validation fields", &ValidationError{Fields: map[string]string{"location_id": "Elige una sucursal"}}, "Elige una sucursal"},
	}
	for _, tc := range cases {
		if got := PublicMessage(tc.err); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestInsufficientStockError(t *testing.T) {
	err := &InsufficientStockError{Name: "Mango", Unit: "bola", Needed: 1, Available: 0}
	if !errors.Is(err, ErrConflict) {
		t.Fatal("must map to 409")
	}
	if got := PublicMessage(err); !strings.HasPrefix(got, "No queda Mango en esta sucursal (la orden necesita 1 bola)") {
		t.Errorf("unexpected message %q", got)
	}
	err = &InsufficientStockError{Name: "Paleta", Unit: "unidad", Needed: 3, Available: 2}
	if got := err.Error(); !strings.Contains(got, "necesita 3 unidades y quedan 2 unidades") {
		t.Errorf("unexpected message %q", got)
	}
}
