package security

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("hunter2pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "hunter2pass" {
		t.Fatal("hash must not equal the plaintext")
	}

	ok, err := VerifyPassword("hunter2pass", hash)
	if err != nil || !ok {
		t.Fatalf("verify correct password: ok=%v err=%v", ok, err)
	}

	bad, err := VerifyPassword("wrongpass", hash)
	if err != nil {
		t.Fatalf("verify wrong password err: %v", err)
	}
	if bad {
		t.Fatal("wrong password must not verify")
	}
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-valid-phc-hash"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	uid := uuid.New()
	wid := uuid.New()

	token, err := tm.IssueAccessToken(uid, wid, []Role{RoleOwner})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	claims, err := tm.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("uid = %v, want %v", claims.UserID, uid)
	}
	if claims.WorkspaceID != wid {
		t.Errorf("wid = %v, want %v", claims.WorkspaceID, wid)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != RoleOwner {
		t.Errorf("roles = %v, want [owner]", claims.Roles)
	}
}

func TestJWTRejectsWrongSecret(t *testing.T) {
	issuer := NewTokenManager("secret-a", time.Minute)
	verifier := NewTokenManager("secret-b", time.Minute)

	token, _ := issuer.IssueAccessToken(uuid.New(), uuid.New(), nil)
	if _, err := verifier.ParseAccessToken(token); err == nil {
		t.Fatal("token signed with a different secret must not verify")
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	tm := NewTokenManager("secret", -time.Minute) // already expired
	token, _ := tm.IssueAccessToken(uuid.New(), uuid.New(), nil)
	if _, err := tm.ParseAccessToken(token); err == nil {
		t.Fatal("expired token must not verify")
	}
}

func TestOpaqueTokenAndHash(t *testing.T) {
	a, err := GenerateOpaqueToken(32)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	b, _ := GenerateOpaqueToken(32)
	if a == b {
		t.Fatal("two generated tokens must differ")
	}
	if HashToken(a) != HashToken(a) {
		t.Fatal("hashing must be deterministic")
	}
	if HashToken(a) == a {
		t.Fatal("hash must differ from the token")
	}
}
