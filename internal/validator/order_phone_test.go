package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
)

func TestValidateOrderCreate_PhoneOptionalForStaffOnly(t *testing.T) {
	if errs := ValidateOrderCreate("Carlos", "", "efectivo", "para_llevar", nil, 1, true, nil); errs.HasErrors() {
		t.Fatalf("counter orders must not require a phone: %v", errs)
	}
	errs := ValidateOrderCreate("Carlos", "", "efectivo", "para_llevar", nil, 1, false, nil)
	if !errs.HasErrors() || errs[0].Field != "customer_phone" {
		t.Fatalf("online orders need a phone, got %v", errs)
	}
	if errs := ValidateOrderCreate("Carlos", "12", "efectivo", "para_llevar", nil, 1, true, nil); !errs.HasErrors() {
		t.Fatal("a phone given at the counter is still validated")
	}
}

func TestValidationErrors_SpanishAndMapsTo400(t *testing.T) {
	errs := ValidateOrderCreate("", "", "efectivo", "para_llevar", nil, 0, false, nil)
	msg := errs.Error()
	for _, want := range []string{"Nombre es obligatorio", "Teléfono es obligatorio", "Productos agrega al menos un producto"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not contain %q", msg, want)
		}
	}
	if !errors.Is(errs, apperror.ErrValidation) {
		t.Fatal("ValidationErrors must unwrap to ErrValidation")
	}
	if apperror.StatusCode(errs) != 400 {
		t.Fatalf("expected 400, got %d", apperror.StatusCode(errs))
	}
}
