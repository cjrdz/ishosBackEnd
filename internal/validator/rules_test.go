package validator

import "testing"

func TestIsValidEmailAcceptsWellFormedAddresses(t *testing.T) {
	t.Parallel()

	valid := []string{
		"admin@example.com",
		"john.doe+orders@sub.example.co",
		"sales_team@factory.sv",
	}

	for _, email := range valid {
		if !isValidEmail(email) {
			t.Fatalf("expected valid email: %s", email)
		}
	}
}

func TestIsValidEmailRejectsMalformedAddresses(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"",
		"plainaddress",
		"user@localhost",
		"user..dots@example.com",
		"user@-example.com",
		"user@example..com",
		"John Doe <john@example.com>",
		"user@exam_ple.com",
	}

	for _, email := range invalid {
		if isValidEmail(email) {
			t.Fatalf("expected invalid email: %s", email)
		}
	}
}
