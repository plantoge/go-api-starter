package validator

import "testing"

type signupInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

func TestValidate_AllValid_ReturnsNil(t *testing.T) {
	in := signupInput{Email: "a@b.com", Password: "longenough"}
	if err := Validate(in); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidate_Invalid_ReturnsDetailsByJSONName(t *testing.T) {
	in := signupInput{Email: "not-an-email", Password: "short"}
	err := Validate(in)
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if _, ok := err.Details["email"]; !ok {
		t.Errorf("Details missing key 'email', got %v", err.Details)
	}
	if _, ok := err.Details["password"]; !ok {
		t.Errorf("Details missing key 'password', got %v", err.Details)
	}
}
