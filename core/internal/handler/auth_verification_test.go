package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/auenger/MEPERECO/core/internal/service"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/jackc/pgx/v5"
)

func TestVerificationEmailValidation(t *testing.T) {
	for _, email := range []string{"person@example.test", "person+tag@example.test"} {
		if !validVerificationEmail(email) {
			t.Errorf("valid email rejected: %s", email)
		}
	}
	for _, email := range []string{"", "person", "Person <person@example.test>", "person@example.test,other@example.test"} {
		if validVerificationEmail(email) {
			t.Errorf("invalid email accepted: %s", email)
		}
	}
}

// The test creates only temporary tables on its dedicated connection.
func TestEmailRegistrationPostgres(t *testing.T) {
	url := os.Getenv("TEST_AUTH_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_AUTH_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `
CREATE TEMP TABLE verification_code (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text NOT NULL,
 code text NOT NULL, expires_at timestamptz NOT NULL, used bool NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(), attempts int NOT NULL DEFAULT 0
);
CREATE TEMP TABLE "user" (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL, email text NOT NULL UNIQUE,
 avatar_url text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 onboarded_at timestamptz, onboarding_questionnaire jsonb NOT NULL DEFAULT '{}',
 cloud_waitlist_email text, cloud_waitlist_reason text, starter_content_state text,
 language text, profile_description text NOT NULL DEFAULT '', timezone text, password_hash text
);
CREATE TEMP TABLE auth_password_challenge (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text NOT NULL, purpose text NOT NULL,
 code_hash text, expires_at timestamptz NOT NULL, attempts integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(),
 token_hash text UNIQUE, token_expires_at timestamptz, UNIQUE(email,purpose)
);
CREATE TEMP TABLE auth_password_rate_limit (bucket text PRIMARY KEY, attempts int NOT NULL, expires_at timestamptz NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("SILIECO_DEV_VERIFICATION_CODE", "246813")
	h := &Handler{Queries: db.New(conn), DB: conn, TxStarter: conn, EmailService: service.NewEmailService(), cfg: Config{AllowSignup: true}}
	call := func(fn http.HandlerFunc, body any, status int) *httptest.ResponseRecorder {
		t.Helper()
		data, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data)))
		if w.Code != status {
			t.Fatalf("status=%d want=%d response=%s", w.Code, status, w.Body.String())
		}
		return w
	}
	email := "employee@example.test"
	const password = "a secure password 2026"
	const next = "another secure password 2026"
	// Ordinary login-code requests cannot create a user.
	call(h.SendCode, SendCodeRequest{Email: email}, 200)
	if _, err := h.Queries.GetUserByEmail(ctx, email); err != pgx.ErrNoRows {
		t.Fatal("login request created account")
	}
	call(h.VerifyCode, VerifyCodeRequest{Email: email, Code: "246813"}, 400)
	// The registration challenge is separate and must be completed with a password.
	call(h.SendPasswordCode, PasswordCodeRequest{Email: email, Purpose: "register"}, 200)
	if _, err := h.Queries.GetUserByEmail(ctx, email); err != pgx.ErrNoRows {
		t.Fatal("sending registration OTP created account")
	}
	call(h.SendPasswordCode, PasswordCodeRequest{Email: email, Purpose: "register"}, 429)
	call(h.VerifyPasswordCode, PasswordCodeRequest{Email: email, Purpose: "reset", Code: "246813"}, 400)
	call(h.VerifyPasswordCode, PasswordCodeRequest{Email: email, Purpose: "register", Code: "000000"}, 400)
	verified := call(h.VerifyPasswordCode, PasswordCodeRequest{Email: email, Purpose: "register", Code: "246813"}, 200)
	var challenge struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(verified.Body.Bytes(), &challenge); err != nil || challenge.Token == "" {
		t.Fatal("missing token")
	}
	if _, err := h.Queries.GetUserByEmail(ctx, email); err != pgx.ErrNoRows {
		t.Fatal("verification created account before password")
	}
	call(h.VerifyPasswordCode, PasswordCodeRequest{Email: email, Purpose: "register", Code: "246813"}, 400)
	call(h.CompletePassword, CompletePasswordRequest{Token: challenge.Token, Password: "short"}, 400)
	response := call(h.CompletePassword, CompletePasswordRequest{Token: challenge.Token, Password: password}, 200)
	var login LoginResponse
	if err := json.Unmarshal(response.Body.Bytes(), &login); err != nil || login.Token == "" || !login.User.HasPassword {
		t.Fatal("registration failed")
	}
	call(h.CompletePassword, CompletePasswordRequest{Token: challenge.Token, Password: next}, 400)
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: password}, 200)
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: next}, 401)
	// Existing users can still sign in with the login OTP even with signup disabled.
	h.cfg.AllowSignup = false
	call(h.SendCode, SendCodeRequest{Email: "new@example.test"}, 403)
	call(h.SendPasswordCode, PasswordCodeRequest{Email: "new@example.test", Purpose: "register"}, 403)
	call(h.SendCode, SendCodeRequest{Email: email}, 200)
	call(h.SendCode, SendCodeRequest{Email: email}, 429)
	response = call(h.VerifyCode, VerifyCodeRequest{Email: email, Code: "246813"}, 200)
	var existing LoginResponse
	if err := json.Unmarshal(response.Body.Bytes(), &existing); err != nil || existing.User.ID != login.User.ID {
		t.Fatal("OTP login did not preserve identity")
	}
	call(h.VerifyCode, VerifyCodeRequest{Email: email, Code: "246813"}, 400)
	// Password reset must not overwrite a different account or allow replay.
	call(h.SendPasswordCode, PasswordCodeRequest{Email: email, Purpose: "reset"}, 200)
	response = call(h.VerifyPasswordCode, PasswordCodeRequest{Email: email, Purpose: "reset", Code: "246813"}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	response = call(h.CompletePassword, CompletePasswordRequest{Token: challenge.Token, Password: next}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &existing); err != nil || existing.User.ID != login.User.ID {
		t.Fatal("reset changed identity")
	}
	call(h.CompletePassword, CompletePasswordRequest{Token: challenge.Token, Password: password}, 400)
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: password}, 401)
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: next}, 200)
	// Authenticated password changes require the previous password.
	change := func(current, password string, status int) {
		t.Helper()
		body, _ := json.Marshal(ChangePasswordRequest{CurrentPassword: current, Password: password})
		req := httptest.NewRequest(http.MethodPost, "/api/me/password", bytes.NewReader(body))
		req.Header.Set("X-User-ID", login.User.ID)
		rec := httptest.NewRecorder()
		h.ChangePassword(rec, req)
		if rec.Code != status {
			t.Fatalf("change status=%d want=%d body=%s", rec.Code, status, rec.Body.String())
		}
	}
	change("wrong password", password, 400)
	change(next, password, 200)
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: next}, 401)
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: password}, 200)
	// Passwordless existing users can set a password without creating a new identity.
	legacyEmail := "legacy@example.test"
	legacy, err := h.Queries.CreateUser(ctx, db.CreateUserParams{Name: "Legacy", Email: legacyEmail})
	if err != nil {
		t.Fatal(err)
	}
	call(h.SendCode, SendCodeRequest{Email: legacyEmail}, 200)
	call(h.VerifyCode, VerifyCodeRequest{Email: legacyEmail, Code: "246813"}, 200)
	call(h.SendPasswordCode, PasswordCodeRequest{Email: legacyEmail, Purpose: "reset"}, 200)
	response = call(h.VerifyPasswordCode, PasswordCodeRequest{Email: legacyEmail, Purpose: "reset", Code: "246813"}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	response = call(h.CompletePassword, CompletePasswordRequest{Token: challenge.Token, Password: password}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &existing); err != nil || existing.User.ID != uuidToString(legacy.ID) || !existing.User.HasPassword {
		t.Fatal("legacy account password setup changed identity")
	}
	// Limit failed password attempts even without Redis.
	for i := 0; i < 10; i++ {
		call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: "incorrect password"}, 401)
	}
	call(h.LoginPassword, PasswordAuthRequest{Email: email, Password: "incorrect password"}, 429)
}

func TestPasswordValidation(t *testing.T) {
	if !validNewPassword("a valid password") || !validNewPassword(" 密码包含空格和汉字字符 ") {
		t.Fatal("valid passwords rejected")
	}
	if validNewPassword("short") || validNewPassword(strings.Repeat("x", 73)) || validNewPassword("invalid\x00password") {
		t.Fatal("invalid password accepted")
	}
	email, password, ok := normalizePasswordAuthRequest(PasswordAuthRequest{Email: "  PERSON@example.test ", Password: "  password  "})
	if !ok || email != "person@example.test" || password != "  password  " {
		t.Fatal("password must not be trimmed")
	}
	if passwordCodeHash(email, "register", "123456") == passwordCodeHash(email, "reset", "123456") {
		t.Fatal("challenge purposes not separated")
	}
}
