// validator.go — ValidationError and ValidationErrors types shared across all request validation.
package validator

import (
	"strings"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []ValidationError

// fieldLabels names request fields the way staff and customers know them.
// Error() is shown in the panel and the store, so it reads in Spanish.
var fieldLabels = map[string]string{ //nolint:gosec // G101: field labels ("Contraseña"), not credentials
	"customer_name":   "Nombre",
	"customer_phone":  "Teléfono",
	"payment_method":  "Método de pago",
	"order_type":      "Tipo de orden",
	"table_number":    "Mesa",
	"amount_received": "Monto recibido",
	"items":           "Productos",
	"status":          "Estado",
	"reason":          "Motivo",
	"name":            "Nombre",
	"email":           "Correo",
	"phone":           "Teléfono",
	"password":        "Contraseña",
	"role":            "Rol",
	"state":           "Estado",
	"slug":            "Identificador",
	"price":           "Precio",
	"display_order":   "Orden",
	"description":     "Descripción",
	"group_name":      "Grupo",
	"stock_status":    "Disponibilidad",
	"user_type":       "Tipo de cliente",
}

// messageText translates the rule messages used in rules.go. Unknown
// messages pass through unchanged.
var messageText = map[string]string{
	"required":                                        "es obligatorio",
	"cannot be empty":                                 "no puede estar vacío",
	"invalid format":                                  "no tiene un formato válido",
	"max 50 characters":                               "admite máximo 50 caracteres",
	"max 100 characters":                              "admite máximo 100 caracteres",
	"max 200 characters":                              "admite máximo 200 caracteres",
	"must be greater than 0":                          "debe ser mayor que 0",
	"must be zero or greater":                         "debe ser 0 o más",
	"must be zero or positive":                        "debe ser 0 o más",
	"at least one item required":                      "agrega al menos un producto",
	"must be active or inactive":                      "debe ser activo o inactivo",
	"must be user or company":                         "debe ser persona o empresa",
	"must be in_stock, out_of_stock, or discontinued": "no es una disponibilidad válida",
	"must be 'employee', 'manager' or 'admin'":        "debe ser empleado, encargado o administrador",
	"must be at least 12 chars and include uppercase, lowercase, number, and symbol": "necesita 12 caracteres o más, con mayúscula, minúscula, número y símbolo",
	"required for en_local and must be greater than 0":                               "es obligatoria para comer aquí",
	"must be recibida, en_proceso, lista, or entregada":                              "no es un estado válido",
	"must be efectivo, tarjeta, transferencia, or otro":                              "debe ser efectivo, tarjeta, transferencia u otro",
	"must be en_local or para_llevar":                                                "debe ser comer aquí o para llevar",
	"only allowed for efectivo payment method":                                       "solo aplica a pagos en efectivo",
	"must be greater than or equal to the order total":                               "no cubre el total de la orden",
}

// Text returns the error as a Spanish sentence for one field.
func (v ValidationError) Text() string {
	label, ok := fieldLabels[v.Field]
	if !ok {
		label = v.Field
	}
	msg, ok := messageText[v.Message]
	if !ok {
		msg = v.Message
	}
	return label + " " + msg
}

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}

	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Text()
	}

	return strings.Join(msgs, ". ")
}

// Unwrap lets apperror.StatusCode map validation failures to 400 instead
// of an internal error.
func (e ValidationErrors) Unwrap() error {
	return apperror.ErrValidation
}

func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}
