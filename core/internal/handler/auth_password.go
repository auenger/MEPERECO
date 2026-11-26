package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/auenger/MEPERECO/core/internal/analytics"
	"github.com/auenger/MEPERECO/core/internal/auth"
	obsmetrics "github.com/auenger/MEPERECO/core/internal/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const passwordMinRunes = 12
const passwordMaxBytes = 72 // bcrypt's input length limit; never truncate.

type PasswordCodeRequest struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
	Code    string `json:"code,omitempty"`
}
type CompletePasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
}

func validNewPassword(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) >= passwordMinRunes && len(s) <= passwordMaxBytes && !strings.ContainsRune(s, 0)
}
func validLoginPassword(s string) bool {
	return utf8.ValidString(s) && len(s) > 0 && len(s) <= passwordMaxBytes && !strings.ContainsRune(s, 0)
}
func validPasswordPurpose(s string) bool { return s == "register" || s == "reset" }
func passwordCodeHash(email, purpose, code string) string {
	mac := hmac.New(sha256.New, auth.JWTSecret())
	mac.Write([]byte("password-code\x00" + email + "\x00" + purpose + "\x00" + code))
	return hex.EncodeToString(mac.Sum(nil))
}
func passwordTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func newPasswordToken() (string, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func validPasswordToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

func (h *Handler) passwordBucket(ctx context.Context, bucket string, limit, seconds int) (bool, error) {
	// Hash bucket names to avoid retaining email addresses in the limiter table.
	bucket = passwordTokenHash(bucket)
	var attempts int
	err := h.DB.QueryRow(ctx, `INSERT INTO auth_password_rate_limit (bucket,attempts,expires_at)
 VALUES ($1,1,now()+($2::integer * interval '1 second')) ON CONFLICT(bucket) DO UPDATE SET
 attempts=CASE WHEN auth_password_rate_limit.expires_at<=now() THEN 1 ELSE LEAST(auth_password_rate_limit.attempts+1,1000000) END,
 expires_at=CASE WHEN auth_password_rate_limit.expires_at<=now() THEN EXCLUDED.expires_at ELSE auth_password_rate_limit.expires_at END RETURNING attempts`, bucket, seconds).Scan(&attempts)
	return attempts <= limit, err
}
func (h *Handler) allowPasswordRequest(w http.ResponseWriter, r *http.Request, kind, email string, emailLimit, ipLimit, seconds int) bool {
	ip := h.clientIPForRateLimit(r)
	// Honor forwarded addresses only from explicitly configured trusted proxies.
	for _, key := range []struct {
		bucket string
		limit  int
	}{{kind + ":ip:" + ip, ipLimit}, {kind + ":email:" + email, emailLimit}} {
		allowed, err := h.passwordBucket(r.Context(), key.bucket, key.limit, seconds)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "authentication temporarily unavailable")
			return false
		}
		if !allowed {
			writeError(w, http.StatusTooManyRequests, "too many attempts; please try again later")
			return false
		}
	}
	return true
}
func (h *Handler) clearPasswordLoginLimit(ctx context.Context, email string) {
	_, _ = h.DB.Exec(ctx, `DELETE FROM auth_password_rate_limit WHERE bucket=$1`, passwordTokenHash("login:email:"+email))
}

func (h *Handler) SendPasswordCode(w http.ResponseWriter, r *http.Request) {
	var req PasswordCodeRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validVerificationEmail(email) || !validPasswordPurpose(req.Purpose) {
		writeError(w, 400, "valid email and password purpose are required")
		return
	}
	if !h.allowPasswordRequest(w, r, "send-password", email, 8, 100, 3600) {
		return
	}
	_, err := h.Queries.GetUserByEmail(r.Context(), email)
	exists := err == nil
	if err != nil && !isNotFound(err) {
		writeError(w, 500, "failed to lookup user")
		return
	}
	if req.Purpose == "register" {
		if exists {
			writeError(w, 409, "account already exists; sign in or reset your password")
			return
		}
		if err = h.checkSignupAllowed(email, true); err != nil {
			writeError(w, 403, "registration is not available for this email")
			return
		}
	} else if !exists {
		// Same response as a delivered recovery email; no account enumeration.
		writeJSON(w, 200, map[string]string{"message": "Verification code sent"})
		return
	}
	code, err := generateCode()
	if err != nil {
		writeError(w, 500, "failed to generate verification code")
		return
	}
	var id pgtype.UUID
	err = h.DB.QueryRow(r.Context(), `INSERT INTO auth_password_challenge(email,purpose,code_hash,expires_at)
 VALUES($1,$2,$3,now()+interval '10 minutes') ON CONFLICT(email,purpose) DO UPDATE SET
 id=gen_random_uuid(),code_hash=EXCLUDED.code_hash,expires_at=EXCLUDED.expires_at,attempts=0,created_at=now(),token_hash=NULL,token_expires_at=NULL
 WHERE auth_password_challenge.created_at<=now()-interval '60 seconds' RETURNING id`, email, req.Purpose, passwordCodeHash(email, req.Purpose, code)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 429, "please wait before requesting another code")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to store verification code")
		return
	}
	if err = h.EmailService.SendVerificationCode(email, code); err != nil {
		_, _ = h.DB.Exec(r.Context(), `DELETE FROM auth_password_challenge WHERE id=$1`, id)
		slog.Error("failed to send password verification code", "error", err)
		writeError(w, 500, "failed to send verification code")
		return
	}
	// Best-effort bounded cleanup of expired challenges and rate counters.
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM auth_password_challenge WHERE expires_at<now()-interval '1 hour' AND (token_expires_at IS NULL OR token_expires_at<now())`)
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM auth_password_rate_limit WHERE expires_at<now()-interval '1 hour'`)
	writeJSON(w, 200, map[string]string{"message": "Verification code sent"})
}

func (h *Handler) VerifyPasswordCode(w http.ResponseWriter, r *http.Request) {
	var req PasswordCodeRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)
	if !validVerificationEmail(email) || !validPasswordPurpose(req.Purpose) || !isSixDigitCode(code) {
		writeError(w, 400, "invalid or expired code")
		return
	}
	if !h.allowPasswordRequest(w, r, "verify-password", email, 20, 150, 900) {
		return
	}
	var id pgtype.UUID
	var hash string
	err := h.DB.QueryRow(r.Context(), `SELECT id,code_hash FROM auth_password_challenge WHERE email=$1 AND purpose=$2 AND code_hash IS NOT NULL AND expires_at>now() AND attempts<5`, email, req.Purpose).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 400, "invalid or expired code")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to verify code")
		return
	}
	if !isDevVerificationCode(code) && subtle.ConstantTimeCompare([]byte(hash), []byte(passwordCodeHash(email, req.Purpose, code))) != 1 {
		_, _ = h.DB.Exec(r.Context(), `UPDATE auth_password_challenge SET attempts=attempts+1 WHERE id=$1 AND code_hash IS NOT NULL`, id)
		writeError(w, 400, "invalid or expired code")
		return
	}
	token, err := newPasswordToken()
	if err != nil {
		writeError(w, 500, "failed to verify code")
		return
	}
	tag, err := h.DB.Exec(r.Context(), `UPDATE auth_password_challenge SET code_hash=NULL,token_hash=$2,token_expires_at=now()+interval '10 minutes' WHERE id=$1 AND code_hash IS NOT NULL AND expires_at>now() AND attempts<5`, id, passwordTokenHash(token))
	if err != nil {
		writeError(w, 500, "failed to verify code")
		return
	}
	if tag.RowsAffected() != 1 {
		writeError(w, 400, "invalid or expired code")
		return
	}
	writeJSON(w, 200, map[string]string{"token": token})
}

func (h *Handler) CompletePassword(w http.ResponseWriter, r *http.Request) {
	var req CompletePasswordRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if !validPasswordToken(req.Token) || !validNewPassword(req.Password) {
		writeError(w, 400, "a valid verification token and a password of at least 12 characters (max 72 bytes) are required")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to complete password setup")
		return
	}
	defer tx.Rollback(r.Context())
	var email, purpose string
	err = tx.QueryRow(r.Context(), `UPDATE auth_password_challenge SET token_hash=NULL,token_expires_at=NULL WHERE token_hash=$1 AND token_expires_at>now() AND code_hash IS NULL RETURNING email,purpose`, passwordTokenHash(req.Token)).Scan(&email, &purpose)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 400, "verification expired; verify your email again")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to complete password setup")
		return
	}
	var id pgtype.UUID
	if purpose == "register" {
		if err = h.checkSignupAllowed(email, true); err != nil {
			writeError(w, 403, "registration is not available for this email")
			return
		}
		name := strings.SplitN(email, "@", 2)[0]
		err = tx.QueryRow(r.Context(), `INSERT INTO "user" (name,email,password_hash) VALUES($1,$2,crypt($3,gen_salt('bf',12))) ON CONFLICT(email) DO NOTHING RETURNING id`, name, email, req.Password).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 409, "account already exists; sign in or reset your password")
			return
		}
	} else if purpose == "reset" {
		err = tx.QueryRow(r.Context(), `UPDATE "user" SET password_hash=crypt($2,gen_salt('bf',12)),updated_at=now() WHERE email=$1 RETURNING id`, email, req.Password).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 400, "verification expired; verify your email again")
			return
		}
	} else {
		writeError(w, 400, "invalid verification purpose")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to save password")
		return
	}
	user, err := h.Queries.WithTx(tx).GetUser(r.Context(), id)
	if err != nil {
		writeError(w, 500, "failed to load account")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to save password")
		return
	}
	h.clearPasswordLoginLimit(r.Context(), email)
	if purpose == "register" {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.Signup(uuidToString(user.ID), user.Email, signupSourceFromRequest(r)))
	}
	h.writePasswordLogin(w, r, user)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ChangePasswordRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || !validLoginPassword(req.CurrentPassword) || !validNewPassword(req.Password) {
		writeError(w, 400, "current password and a new password of at least 12 characters (max 72 bytes) are required")
		return
	}
	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, 404, "user not found")
		return
	}
	if !h.allowPasswordRequest(w, r, "login", user.Email, 10, 150, 900) {
		return
	}
	tag, err := h.DB.Exec(r.Context(), `UPDATE "user" SET password_hash=crypt($3,gen_salt('bf',12)),updated_at=now() WHERE id=$1 AND password_hash IS NOT NULL AND crypt($2,password_hash)=password_hash`, user.ID, req.CurrentPassword, req.Password)
	if err != nil {
		writeError(w, 500, "failed to change password")
		return
	}
	if tag.RowsAffected() != 1 {
		writeError(w, 400, "invalid current password; use email verification to reset it")
		return
	}
	h.clearPasswordLoginLimit(r.Context(), user.Email)
	writeJSON(w, 200, map[string]string{"message": "Password changed"})
}
