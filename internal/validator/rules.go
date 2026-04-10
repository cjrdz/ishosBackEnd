package validator

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var emailLocalAllowedRe = regexp.MustCompile(`^[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+$`)

func ValidateLogin(email, password string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(email) == "" {
		errs.Add("email", "required")
	} else if !isValidEmail(email) {
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
	} else if !isValidEmail(email) {
		errs.Add("email", "invalid format")
	}

	if strings.TrimSpace(password) == "" {
		errs.Add("password", "required")
	} else if !isStrongPassword(password) {
		errs.Add("password", "must be at least 12 chars and include uppercase, lowercase, number, and symbol")
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
		} else if !isValidEmail(v) {
			errs.Add("email", "invalid format")
		}
	}

	if password != nil {
		v := strings.TrimSpace(*password)
		if v != "" && !isStrongPassword(v) {
			errs.Add("password", "must be at least 12 chars and include uppercase, lowercase, number, and symbol")
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

func isValidEmail(value string) bool {
	email := strings.TrimSpace(value)
	if email == "" || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}

	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	localPart := parts[0]
	domainPart := strings.ToLower(parts[1])
	if localPart == "" || len(localPart) > 64 || domainPart == "" || len(domainPart) > 253 {
		return false
	}
	if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") || strings.Contains(localPart, "..") {
		return false
	}
	if !emailLocalAllowedRe.MatchString(localPart) {
		return false
	}
	if strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") || strings.Contains(domainPart, "..") {
		return false
	}

	labels := strings.Split(domainPart, ".")
	if len(labels) < 2 {
		return false
	}

	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}

	return true
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

func isStrongPassword(password string) bool {
	if utf8.RuneCountInString(password) < 12 {
		return false
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSymbol := false

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSymbol
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

func ValidateFlavorCreate(name string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(name) == "" {
		errs.Add("name", "required")
	} else if utf8.RuneCountInString(name) > 100 {
		errs.Add("name", "max 100 characters")
	}

	return errs
}

func ValidateFlavorUpdate(name *string) ValidationErrors {
	var errs ValidationErrors

	if name != nil {
		v := strings.TrimSpace(*name)
		if v == "" {
			errs.Add("name", "cannot be empty")
		} else if utf8.RuneCountInString(v) > 100 {
			errs.Add("name", "max 100 characters")
		}
	}

	return errs
}

func validateAddonGroupName(groupName string, errs *ValidationErrors) {
	trimmed := strings.TrimSpace(groupName)
	if trimmed == "" {
		return
	}
	if utf8.RuneCountInString(trimmed) > 50 {
		errs.Add("group_name", "max 50 characters")
	}
}

func ValidateAddonCreate(name string, price float64, groupName string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(name) == "" {
		errs.Add("name", "required")
	} else if utf8.RuneCountInString(name) > 100 {
		errs.Add("name", "max 100 characters")
	}

	if price < 0 {
		errs.Add("price", "must be zero or greater")
	}

	validateAddonGroupName(groupName, &errs)

	return errs
}

func ValidateAddonUpdate(name *string, price *float64, groupName *string) ValidationErrors {
	var errs ValidationErrors

	if name != nil {
		v := strings.TrimSpace(*name)
		if v == "" {
			errs.Add("name", "cannot be empty")
		} else if utf8.RuneCountInString(v) > 100 {
			errs.Add("name", "max 100 characters")
		}
	}

	if price != nil && *price < 0 {
		errs.Add("price", "must be zero or greater")
	}

	if groupName != nil {
		validateAddonGroupName(*groupName, &errs)
	}

	return errs
}

func ValidateUserCreate(name, userType, phone, email, status string) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(name) == "" {
		errs.Add("name", "required")
	} else if utf8.RuneCountInString(name) > 200 {
		errs.Add("name", "max 200 characters")
	}

	if userType != "user" && userType != "company" {
		errs.Add("user_type", "must be user or company")
	}

	if strings.TrimSpace(phone) == "" {
		errs.Add("phone", "required")
	} else if !isValidPhone(phone) {
		errs.Add("phone", "invalid format")
	}

	if strings.TrimSpace(email) != "" && !isValidEmail(strings.TrimSpace(email)) {
		errs.Add("email", "invalid format")
	}

	if status != "active" && status != "inactive" {
		errs.Add("status", "must be active or inactive")
	}

	return errs
}

func ValidateUserUpdate(name, userType, phone, email, status *string) ValidationErrors {
	var errs ValidationErrors

	if name != nil {
		v := strings.TrimSpace(*name)
		if v == "" {
			errs.Add("name", "cannot be empty")
		} else if utf8.RuneCountInString(v) > 200 {
			errs.Add("name", "max 200 characters")
		}
	}

	if userType != nil {
		v := strings.TrimSpace(*userType)
		if v != "user" && v != "company" {
			errs.Add("user_type", "must be user or company")
		}
	}

	if phone != nil {
		v := strings.TrimSpace(*phone)
		if v == "" {
			errs.Add("phone", "cannot be empty")
		} else if !isValidPhone(v) {
			errs.Add("phone", "invalid format")
		}
	}

	if email != nil {
		v := strings.TrimSpace(*email)
		if v != "" && !isValidEmail(v) {
			errs.Add("email", "invalid format")
		}
	}

	if status != nil {
		v := strings.TrimSpace(*status)
		if v != "active" && v != "inactive" {
			errs.Add("status", "must be active or inactive")
		}
	}

	return errs
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
