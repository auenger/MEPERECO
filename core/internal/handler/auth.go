package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/auenger/MEPERECO/core/internal/auth"
	"github.com/auenger/MEPERECO/core/internal/logger"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SignupError represents signup restriction errors
type SignupError struct {
	Message string
}

func (e SignupError) Error() string {
	return e.Message
}

var ErrSignupProhibited = SignupError{Message: "user registration is disabled on this self-hosted instance"}
var ErrEmailNotAllowed = SignupError{Message: "email address or domain not allowed on this instance"}

const devVerificationCodeEnv = "SILIECO_DEV_VERIFICATION_CODE"

// supportedLanguages mirrors `SUPPORTED_LOCALES` in packages/core/i18n/types.ts.
// Keep both lists in sync when adding a locale — the user-controlled `language`
// field round-trips through GetMe back into i18n.changeLanguage(), so without
// validation an arbitrary string would persist and echo to every device.
var supportedLanguages = map[string]struct{}{
	"en":      {},
	"zh-Hans": {},
	"ko":      {},
	"ja":      {},
}

type UserResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	HasPassword bool    `json:"has_password"`
	AvatarURL   *string `json:"avatar_url"`
	Language    *string `json:"language"`
	// Pinned IANA tz; nil = no preference (use browser-detected tz).
	Timezone                *string         `json:"timezone"`
	OnboardedAt             *string         `json:"onboarded_at"`
	OnboardingQuestionnaire json.RawMessage `json:"onboarding_questionnaire"`
	StarterContentState     *string         `json:"starter_content_state"`
	ProfileDescription      string          `json:"profile_description"`
	CreatedAt               string          `json:"created_at"`
	UpdatedAt               string          `json:"updated_at"`
}

// MaxProfileDescriptionLen caps the user-supplied profile_description body.
// Picked at 2000 chars per SILI-2406: enough room for role / stack / a few
// preferences, short enough that injecting it into every agent brief
// doesn't move the needle on prompt cost.
const MaxProfileDescriptionLen = 2000

func (h *Handler) userToResponse(u db.User) UserResponse {
	// JSONB column is []byte with DEFAULT '{}', so it's never nil at the DB
	// level. Defensive coalesce just in case a future ALTER makes the column
	// nullable and some row comes back with no default applied.
	q := u.OnboardingQuestionnaire
	if len(q) == 0 {
		q = []byte("{}")
	}
	return UserResponse{
		ID:                      uuidToString(u.ID),
		Name:                    u.Name,
		Email:                   u.Email,
		HasPassword:             u.PasswordHash.Valid && u.PasswordHash.String != "",
		AvatarURL:               h.resolveAvatarURLPtr(textToPtr(u.AvatarUrl)),
		Language:                textToPtr(u.Language),
		Timezone:                textToPtr(u.Timezone),
		OnboardedAt:             timestampToPtr(u.OnboardedAt),
		OnboardingQuestionnaire: json.RawMessage(q),
		StarterContentState:     textToPtr(u.StarterContentState),
		ProfileDescription:      u.ProfileDescription,
		CreatedAt:               timestampToString(u.CreatedAt),
		UpdatedAt:               timestampToString(u.UpdatedAt),
	}
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type SendCodeRequest struct {
	Email string `json:"email"`
}

type VerifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type PasswordAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func generateCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf[:]) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func isDevVerificationCode(code string) bool {
	if isProductionEnv() {
		return false
	}

	devCode := strings.TrimSpace(os.Getenv(devVerificationCodeEnv))
	if !isSixDigitCode(devCode) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(code), []byte(devCode)) == 1
}

func isProductionEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func isSixDigitCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (h *Handler) writePasswordLogin(w http.ResponseWriter, r *http.Request, user db.User) {
	tokenString, err := h.signSessionJWT(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	if err := auth.SetAuthCookies(w, tokenString); err != nil {
		slog.Warn("failed to set auth cookies", "error", err)
	}
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(auth.AuthTokenTTL())) {
			http.SetCookie(w, cookie)
		}
	}
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  h.userToResponse(user),
	})
}

func normalizePasswordAuthRequest(req PasswordAuthRequest) (email, password string, ok bool) {
	email = strings.ToLower(strings.TrimSpace(req.Email))
	password = req.Password
	return email, password, validVerificationEmail(email) && validLoginPassword(password)
}

func (h *Handler) LoginPassword(w http.ResponseWriter, r *http.Request) {
	var req PasswordAuthRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email, password, ok := normalizePasswordAuthRequest(req)
	if !ok {
		writeError(w, http.StatusBadRequest, "valid email and password are required")
		return
	}
	if !h.allowPasswordRequest(w, r, "login", email, 10, 150, 900) {
		return
	}
	user, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil && !isNotFound(err) {
		writeError(w, http.StatusServiceUnavailable, "authentication temporarily unavailable")
		return
	}
	var matches bool
	// Perform a comparable bcrypt operation for unknown accounts and passwordless users.
	if err != nil || !user.PasswordHash.Valid || user.PasswordHash.String == "" {
		// The result is deliberately discarded; this only equalizes the password check cost.
		var dummy string
		_ = h.DB.QueryRow(r.Context(), `SELECT crypt($1,gen_salt('bf',12))`, password).Scan(&dummy)
	} else {
		err = h.DB.QueryRow(r.Context(), `SELECT crypt($2,password_hash)=password_hash FROM "user" WHERE id=$1 AND password_hash IS NOT NULL`, user.ID, password).Scan(&matches)
		if err != nil && !isNotFound(err) {
			writeError(w, http.StatusServiceUnavailable, "authentication temporarily unavailable")
			return
		}
	}
	if !matches {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	h.clearPasswordLoginLimit(r.Context(), email)
	h.writePasswordLogin(w, r, user)
}

func (h *Handler) signSessionJWT(user db.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   uuidToString(user.ID),
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(auth.AuthTokenTTL()).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString(auth.JWTSecret())
}

// signupSourceFromRequest reads the attribution cookie the web frontend
// sets on the first pageview (UTM + referrer bundle). The frontend writes
// a JSON string URL-encoded into the cookie value — Go does not
// auto-decode Cookie.Value, so we have to unescape here before the string
// lands in PostHog. Missing cookie / decode failures collapse to the
// empty string; that simply omits signup_source from the event rather
// than sending percent-encoded garbage. Never fall back to r.Referer() —
// the frontend has already sanitised attribution and a raw referer can
// leak OAuth code/state from the callback URL.
//
// The cap is the server-side defence against a client that manages to set
// an oversize cookie; it matches SIGNUP_SOURCE_MAX_LEN on the frontend.
const signupSourceMaxLen = 512

func signupSourceFromRequest(r *http.Request) string {
	c, err := r.Cookie("silieco_signup_source")
	if err != nil || c == nil {
		return ""
	}
	decoded, err := url.QueryUnescape(c.Value)
	if err != nil {
		return ""
	}
	if len(decoded) > signupSourceMaxLen {
		return ""
	}
	return decoded
}

func (h *Handler) checkSignupAllowed(email string, isNewUser bool) error {
	if !isNewUser {
		return nil // existing users always allowed to log in
	}

	email = strings.ToLower(email)
	domain := ""
	if at := strings.Index(email, "@"); at > 0 {
		domain = email[at+1:]
	}

	// 1. explicit email whitelist always wins
	if len(h.cfg.AllowedEmails) > 0 && contains(h.cfg.AllowedEmails, email) {
		return nil
	}

	// 2. domain whitelist always wins
	if len(h.cfg.AllowedEmailDomains) > 0 && contains(h.cfg.AllowedEmailDomains, domain) {
		return nil
	}

	// 3. general signup flag
	if !h.cfg.AllowSignup {
		return ErrSignupProhibited
	}

	// 4. if allowlists are set but didn't match, block
	if len(h.cfg.AllowedEmailDomains) > 0 || len(h.cfg.AllowedEmails) > 0 {
		return ErrSignupProhibited
	}

	return nil
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

func validVerificationEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254
}

func (h *Handler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req SendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validVerificationEmail(email) {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}

	// Only existing users may log in with the ordinary email code. New
	// users use the separate, email-verified password registration flow.
	_, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		if !isNotFound(err) {
			writeError(w, http.StatusInternalServerError, "failed to lookup user")
			return
		}
		if h.checkSignupAllowed(email, true) != nil {
			writeError(w, http.StatusForbidden, "user registration is disabled")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
		return
	}
	// Rate limit: max 1 code per 60 seconds per email
	latest, err := h.Queries.GetLatestCodeByEmail(r.Context(), email)
	if err != nil && !isNotFound(err) {
		writeError(w, http.StatusInternalServerError, "failed to lookup verification code")
		return
	}
	if err == nil && time.Since(latest.CreatedAt.Time) < 60*time.Second {
		writeError(w, http.StatusTooManyRequests, "please wait before requesting another code")
		return
	}

	code, err := generateCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}

	storedCode, err := h.Queries.CreateVerificationCode(r.Context(), db.CreateVerificationCodeParams{
		Email:     email,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store verification code")
		return
	}

	if err := h.EmailService.SendVerificationCode(email, code); err != nil {
		// Remove the undelivered code so the user can retry immediately.
		_, _ = h.DB.Exec(r.Context(), "DELETE FROM verification_code WHERE id = $1", storedCode.ID)
		slog.Error("failed to send verification code", "email", email, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	// Best-effort cleanup of expired codes
	_ = h.Queries.DeleteExpiredVerificationCodes(r.Context())

	writeJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
}

func (h *Handler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var req VerifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)

	if !validVerificationEmail(email) || !isSixDigitCode(code) {
		writeError(w, http.StatusBadRequest, "valid email and six-digit code are required")
		return
	}

	dbCode, err := h.Queries.GetLatestVerificationCode(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	isDevCode := isDevVerificationCode(code)
	if !isDevCode && subtle.ConstantTimeCompare([]byte(code), []byte(dbCode.Code)) != 1 {
		_ = h.Queries.IncrementVerificationCodeAttempts(r.Context(), dbCode.ID)
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	consumed, err := h.Queries.MarkVerificationCodeUsed(r.Context(), dbCode.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify code")
		return
	}
	if consumed != 1 {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	user, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	tokenString, err := h.signSessionJWT(user)
	if err != nil {
		slog.Warn("login failed", append(logger.RequestAttrs(r), "error", err, "email", req.Email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set HttpOnly auth cookie (browser clients) + CSRF cookie.
	if err := auth.SetAuthCookies(w, tokenString); err != nil {
		slog.Warn("failed to set auth cookies", "error", err)
	}

	// Set CloudFront signed cookies for CDN access.
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(auth.AuthTokenTTL())) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  h.userToResponse(user),
	})
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, h.userToResponse(user))
}

type UpdateMeRequest struct {
	Name               *string `json:"name"`
	AvatarURL          *string `json:"avatar_url"`
	Language           *string `json:"language"`
	ProfileDescription *string `json:"profile_description"`
	// IANA tz to pin; "" clears back to NULL; nil leaves untouched.
	Timezone *string `json:"timezone"`
}

// MintCLIToken returns a fresh JWT for the authenticated user.
// This allows cookie-authenticated browser sessions to obtain a bearer token
// that can be handed off to the CLI via the cli_callback redirect.
func (h *Handler) MintCLIToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	tokenString, err := h.signSessionJWT(user)
	if err != nil {
		slog.Warn("cli-token: failed to sign JWT", append(logger.RequestAttrs(r), "error", err, "user_id", userID)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": tokenString})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearAuthCookies(w)
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	currentUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	name := currentUser.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
	}

	params := db.UpdateUserParams{
		ID:   currentUser.ID,
		Name: name,
	}
	if req.AvatarURL != nil {
		avatarURL, ok := h.acceptAvatarURL(w, r, *req.AvatarURL, currentUser.AvatarUrl.String)
		if !ok {
			return
		}
		params.AvatarUrl = pgtype.Text{String: avatarURL, Valid: true}
	}
	if req.Language != nil {
		lang := strings.TrimSpace(*req.Language)
		if _, ok := supportedLanguages[lang]; !ok {
			writeError(w, http.StatusBadRequest, "unsupported language")
			return
		}
		params.Language = pgtype.Text{String: lang, Valid: true}
	}
	if req.ProfileDescription != nil {
		// Count runes, not bytes: 2000 chars of Chinese must not be rejected
		// as ~6000 bytes. utf8.RuneCountInString handles invalid UTF-8 by
		// counting each bad byte as one rune, which still bounds the column.
		desc := strings.TrimSpace(*req.ProfileDescription)
		if utf8.RuneCountInString(desc) > MaxProfileDescriptionLen {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("profile_description exceeds %d characters", MaxProfileDescriptionLen))
			return
		}
		params.ProfileDescription = pgtype.Text{String: desc, Valid: true}
	}

	if req.Timezone != nil {
		// Valid=false → column untouched; Valid=true + "" → clear to
		// NULL; Valid=true + IANA → set. Three-way semantics enforced
		// in the UpdateUser SQL CASE.
		tz := strings.TrimSpace(*req.Timezone)
		if tz != "" {
			if loc, err := time.LoadLocation(tz); err != nil || loc == nil {
				writeError(w, http.StatusBadRequest, "invalid timezone")
				return
			}
		}
		params.Timezone = pgtype.Text{String: tz, Valid: true}
	}

	updatedUser, err := h.Queries.UpdateUser(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, h.userToResponse(updatedUser))
}
