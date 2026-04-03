package jwt_test

import (
	"testing"
	"time"

	"chatsem/shared/pkg/jwt"

	"github.com/google/uuid"
)

const testSecret = "test-secret-key"

func TestValidateToken_Valid(t *testing.T) {
	t.Logf("[%s] stage: creating valid token", t.Name())
	claims := &jwt.Claims{
		UserID:     uuid.New(),
		ExternalID: "ext-001",
		EventID:    uuid.New(),
		Role:       "user",
	}
	token, err := jwt.CreateToken(claims, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("[%s] CreateToken: %v", t.Name(), err)
	}

	t.Logf("[%s] stage: validating token", t.Name())
	got, err := jwt.ValidateToken(token, testSecret)
	if err != nil {
		t.Fatalf("[%s] ValidateToken: %v", t.Name(), err)
	}
	if got.UserID != claims.UserID {
		t.Errorf("[%s] user_id: want %s, got %s", t.Name(), claims.UserID, got.UserID)
	}
	if got.Role != claims.Role {
		t.Errorf("[%s] role: want %s, got %s", t.Name(), claims.Role, got.Role)
	}
	t.Logf("[%s] stage: validation successful, user_id=%s", t.Name(), got.UserID)
}

func TestValidateToken_Expired(t *testing.T) {
	t.Logf("[%s] stage: creating expired token", t.Name())
	claims := &jwt.Claims{
		UserID:  uuid.New(),
		EventID: uuid.New(),
		Role:    "user",
	}
	token, err := jwt.CreateToken(claims, testSecret, -time.Second)
	if err != nil {
		t.Fatalf("[%s] CreateToken: %v", t.Name(), err)
	}

	t.Logf("[%s] stage: expecting validation failure", t.Name())
	_, err = jwt.ValidateToken(token, testSecret)
	if err == nil {
		t.Fatalf("[%s] expected error for expired token, got nil", t.Name())
	}
	t.Logf("[%s] got expected error: %v", t.Name(), err)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	t.Logf("[%s] stage: creating token with one secret, validating with another", t.Name())
	claims := &jwt.Claims{
		UserID:  uuid.New(),
		EventID: uuid.New(),
		Role:    "admin",
	}
	token, err := jwt.CreateToken(claims, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("[%s] CreateToken: %v", t.Name(), err)
	}

	_, err = jwt.ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Fatalf("[%s] expected error for wrong secret, got nil", t.Name())
	}
	t.Logf("[%s] got expected error: %v", t.Name(), err)
}
