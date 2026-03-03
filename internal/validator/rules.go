package validator

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func ValidateLogin(email, password string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(email) == "" {
		errs.Add("email", "required")
	} else if !emailRe.MatchString(email) {
		errs.Add("email", "invalid format")
	}

	if strings.TrimSpace(password) == "" {
		errs.Add("password", "required")
	}

	return errs
}

func ValidateEmployeeCreate(email, password, name, phone, role, state string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(email) == "" {
		errs.Add("email", "required")
	} else if !emailRe.MatchString(email) {
		errs.Add("email", "invalid format")
	}

	if strings.TrimSpace(password) == "" {
		errs.Add("password", "required")
	} else if utf8.RuneCountInString(password) < 8 {
		errs.Add("password", "must be at least 8 characters")
	}

	if role != "employee" && role != "admin" {
		errs.Add("role", "must be 'employee' or 'admin'")
	}

	if state != "" && state != "active" && state != "inactive" {
		errs.Add("state", "must be active or inactive")
	}

	if strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) > 200 {
		errs.Add("name", "max 200 characters")
	}

	if strings.TrimSpace(phone) != "" && !isValidPhone(phone) {
		errs.Add("phone", "invalid format")
	}

	return errs
}

func ValidateEmployeeUpdate(email, password, name, phone, role, state *string) ValidationErrors {
	var errs ValidationErrors

	if email != nil {
		v := strings.TrimSpace(*email)
		if v == "" {
			errs.Add("email", "cannot be empty")
		} else if !emailRe.MatchString(v) {
			errs.Add("email", "invalid format")
		}
	}

	if password != nil {
		v := strings.TrimSpace(*password)
		if v != "" && utf8.RuneCountInString(v) < 8 {
			errs.Add("password", "must be at least 8 characters")
		}
	}

	if role != nil {
		v := strings.TrimSpace(*role)
		if v != "employee" && v != "admin" {
			errs.Add("role", "must be 'employee' or 'admin'")
		}
	}

	if state != nil {
		v := strings.TrimSpace(*state)
		if v != "active" && v != "inactive" {
			errs.Add("state", "must be active or inactive")
		}
	}

	if name != nil {
		v := strings.TrimSpace(*name)
		if v != "" && utf8.RuneCountInString(v) > 200 {
			errs.Add("name", "max 200 characters")
		}
	}

	if phone != nil {
		v := strings.TrimSpace(*phone)
		if v != "" && !isValidPhone(v) {
			errs.Add("phone", "invalid format")
		}
	}

	return errs
}

func ValidateCategoryCreate(name, slug string, displayOrder int) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(name) == "" {
		errs.Add("name", "required")
	} else if utf8.RuneCountInString(name) > 100 {
		errs.Add("name", "max 100 characters")
	}

	if strings.TrimSpace(slug) == "" {
		errs.Add("slug", "required")
	} else if utf8.RuneCountInString(slug) > 100 {
		errs.Add("slug", "max 100 characters")
	}

	if displayOrder < 0 {
		errs.Add("display_order", "must be zero or positive")
	}

	return errs
}

func ValidateCategoryUpdate(name, slug *string, displayOrder *int) ValidationErrors {
	var errs ValidationErrors

	if name != nil {
		v := strings.TrimSpace(*name)
		if v == "" {
			errs.Add("name", "cannot be empty")
		} else if utf8.RuneCountInString(v) > 100 {
			errs.Add("name", "max 100 characters")
		}
	}

	if slug != nil {
		v := strings.TrimSpace(*slug)
		if v == "" {
			errs.Add("slug", "cannot be empty")
		} else if utf8.RuneCountInString(v) > 100 {
			errs.Add("slug", "max 100 characters")
		}
	}

	if displayOrder != nil && *displayOrder < 0 {
		errs.Add("display_order", "must be zero or positive")
	}

	return errs
}

func ValidateProductCreate(name, description string, price float64, stockStatus string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(name) == "" {
		errs.Add("name", "required")
	} else if utf8.RuneCountInString(name) > 200 {
		errs.Add("name", "max 200 characters")
	}

	if strings.TrimSpace(description) == "" {
		errs.Add("description", "required")
	}

	if price <= 0 {
		errs.Add("price", "must be greater than 0")
	}

	if !isValidStockStatus(stockStatus) {
		errs.Add("stock_status", "must be in_stock, out_of_stock, or discontinued")
	}

	return errs
}

func ValidateProductUpdate(name *string, price *float64, stockStatus *string) ValidationErrors {
	var errs ValidationErrors

	if name != nil {
		v := strings.TrimSpace(*name)
		if v == "" {
			errs.Add("name", "cannot be empty")
		} else if utf8.RuneCountInString(v) > 200 {
			errs.Add("name", "max 200 characters")
		}
	}

	if price != nil && *price <= 0 {
		errs.Add("price", "must be greater than 0")
	}

	if stockStatus != nil && !isValidStockStatus(*stockStatus) {
		errs.Add("stock_status", "must be in_stock, out_of_stock, or discontinued")
	}

	return errs
}

func ValidateOrderDetails(name, phone, paymentMethod, orderType string, tableNumber *int) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(name) == "" {
		errs.Add("customer_name", "required")
	} else if utf8.RuneCountInString(name) > 200 {
		errs.Add("customer_name", "max 200 characters")
	}

	if strings.TrimSpace(phone) == "" {
		errs.Add("customer_phone", "required")
	} else if !isValidPhone(phone) {
		errs.Add("customer_phone", "invalid format")
	}

	if !isValidPaymentMethod(paymentMethod) {
		errs.Add("payment_method", "must be efectivo, tarjeta, transferencia, or otro")
	}

	if !isValidOrderType(orderType) {
		errs.Add("order_type", "must be en_local or para_llevar")
	}

	if orderType == "en_local" {
		if tableNumber == nil || *tableNumber <= 0 {
			errs.Add("table_number", "required for en_local and must be greater than 0")
		}
	}

	return errs
}

func ValidateOrderCreate(name, phone, paymentMethod, orderType string, tableNumber *int, itemCount int) ValidationErrors {
	errs := ValidateOrderDetails(name, phone, paymentMethod, orderType, tableNumber)

	if itemCount == 0 {
		errs.Add("items", "at least one item required")
	}

	return errs
}

func ValidateOrderStatus(status string) ValidationErrors {
	var errs ValidationErrors
	if status != "recibida" && status != "en_proceso" && status != "lista" && status != "entregada" {
		errs.Add("status", "must be recibida, en_proceso, lista, or entregada")
	}
	return errs
}

func ValidateRejectReason(reason string) ValidationErrors {
	var errs ValidationErrors
	if strings.TrimSpace(reason) == "" {
		errs.Add("reason", "required")
	}
	return errs
}

func isValidStockStatus(value string) bool {
	return value == "in_stock" || value == "out_of_stock" || value == "discontinued"
}

func isValidPaymentMethod(value string) bool {
	return value == "efectivo" || value == "tarjeta" || value == "transferencia" || value == "otro"
}

func isValidOrderType(value string) bool {
	return value == "en_local" || value == "para_llevar"
}

func isValidPhone(phone string) bool {
	digits := 0
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits++
			continue
		}
		if r != '+' && r != ' ' && r != '-' && r != '(' && r != ')' {
			return false
		}
	}
	return digits >= 7 && digits <= 15
}
