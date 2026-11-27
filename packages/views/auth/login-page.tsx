"use client";

import { useState, useEffect, useCallback, useRef, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@silieco/ui/components/ui/card";
import { Input } from "@silieco/ui/components/ui/input";
import { Button } from "@silieco/ui/components/ui/button";
import { Label } from "@silieco/ui/components/ui/label";
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "@silieco/ui/components/ui/input-otp";
import { useAuthStore } from "@silieco/core/auth";
import { PASSWORD_POLICY, validNewPassword } from "@silieco/core/auth/password-policy";
import { workspaceKeys } from "@silieco/core/workspace/queries";
import { api } from "@silieco/core/api";
import type { LoginResponse } from "@silieco/core/api/client";
import type { User } from "@silieco/core/types";
import { useT } from "../i18n";
import { SilkBackground } from "@silieco/ui/components/common/silk-background";
import { DutyAgents } from "./duty-agents";
import "./login.css";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface CliCallbackConfig {
  /** Validated localhost callback URL */
  url: string;
  /** Opaque state to pass back to CLI */
  state: string;
}

interface LoginPageProps {
  /** Logo element rendered above the title */
  logo?: ReactNode;
  /** Called after successful login. The workspace list is seeded into React
   *  Query before this fires, so the caller can compute a destination URL. */
  onSuccess: () => void;
  /** CLI callback config for authorizing CLI tools. */
  cliCallback?: CliCallbackConfig;
  /** Called after a token is obtained (e.g. to set cookies). */
  onTokenObtained?: () => void;
  /** Optional content below the sign-in button. */
  extra?: ReactNode;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

export function redirectToCliCallback(url: string, token: string, state: string) {
  const separator = url.includes("?") ? "&" : "?";
  window.location.href = `${url}${separator}token=${encodeURIComponent(token)}&state=${encodeURIComponent(state)}`;
}

/**
 * Validate that a CLI callback URL points to a safe host over HTTP.
 * Allows localhost and private/LAN IPs (RFC 1918) to support self-hosted setups
 * on local VMs while blocking arbitrary public hosts.
 */
export function validateCliCallback(cliCallback: string): boolean {
  try {
    const cbUrl = new URL(cliCallback);
    if (cbUrl.protocol !== "http:") return false;
    const h = cbUrl.hostname;
    if (h === "localhost" || h === "127.0.0.1") return true;
    // Allow RFC 1918 private IPs: 10.x.x.x, 172.16-31.x.x, 192.168.x.x
    if (/^10\./.test(h)) return true;
    if (/^172\.(1[6-9]|2\d|3[01])\./.test(h)) return true;
    if (/^192\.168\./.test(h)) return true;
    // Tailscale uses the RFC 6598 shared address range, 100.64.0.0/10.
    if (/^100\.(6[4-9]|[7-9]\d|1[01]\d|12[0-7])\.\d{1,3}\.\d{1,3}$/.test(h)) return true;
    return false;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function LoginPage({
  logo,
  onSuccess,
  cliCallback,
  onTokenObtained,
  extra,
}: LoginPageProps) {
  const { t } = useT("auth");
  const { t: layoutText } = useT("layout");
  const qc = useQueryClient();
  const [step, setStep] = useState<"email" | "code" | "set_password" | "cli_confirm">("email");
  const [mode, setMode] = useState<"password" | "code" | "register" | "reset">("password");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [passwordToken, setPasswordToken] = useState("");
  const [email, setEmail] = useState("");
  const registering = mode === "register";
  const [focusedField, setFocusedField] = useState<"email" | "code" | "password" | null>(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [existingUser, setExistingUser] = useState<User | null>(null);
  // Tracks how the existing session was detected so handleCliAuthorize
  // uses the matching token source (cookie → mintCliToken, localStorage → direct).
  const authSourceRef = useRef<"cookie" | "localStorage">("cookie");

  // Check for existing session when CLI callback is present.
  // Prioritises cookie auth (= current browser session) to avoid authorising
  // the CLI with a stale or mismatched localStorage token.
  useEffect(() => {
    if (!cliCallback) return;

    // Ensure no stale bearer token interferes — we want to test the cookie first.
    api.setToken(null);

    api
      .getMe()
      .then((user) => {
        authSourceRef.current = "cookie";
        setExistingUser(user);
        setStep("cli_confirm");
      })
      .catch(() => {
        // Cookie auth failed — fall back to localStorage token
        const token = localStorage.getItem("silieco_token");
        if (!token) return;

        api.setToken(token);
        api
          .getMe()
          .then((user) => {
            authSourceRef.current = "localStorage";
            setExistingUser(user);
            setStep("cli_confirm");
          })
          .catch(() => {
            api.setToken(null);
            localStorage.removeItem("silieco_token");
          });
      });
  }, [cliCallback]);

  // Cooldown timer for resend
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  const finishLogin = useCallback(async (response: LoginResponse) => {
    if (cliCallback) {
      const { token } = response;
      localStorage.setItem("silieco_token", token);
      api.setToken(token);
      onTokenObtained?.();
      redirectToCliCallback(cliCallback.url, token, cliCallback.state);
      return;
    }
    useAuthStore.getState().acceptLogin(response);
    const wsList = await api.listWorkspaces();
    qc.setQueryData(workspaceKeys.list(), wsList);
    onTokenObtained?.();
    onSuccess();
  }, [cliCallback, onTokenObtained, onSuccess, qc]);

  const switchMode = (next: typeof mode) => {
    setMode(next); setStep("email"); setPassword(""); setConfirmPassword("");
    setPasswordToken(""); setCode(""); setError(""); setFocusedField(null);
    setShowPassword(false);
  };

  const requestCode = async () => {
    if (mode === "register" || mode === "reset") await api.sendPasswordCode(email, mode);
    else await api.sendCode(email);
  };

  const handleSendCode = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!email.trim() || loading) return;
    setLoading(true); setError("");
    try {
      if (mode === "password") {
        await finishLogin(await api.loginPassword(email, password));
        return;
      }
      await requestCode(); setStep("code"); setCode(""); setCooldown(60);
    } catch (err) {
      setError(err instanceof Error ? err.message : t(($) => $.errors.send_failed));
    } finally { setLoading(false); }
  };

  const handleVerify = async (value: string) => {
    if (value.length !== 6 || loading) return;
    setLoading(true); setError("");
    try {
      if (mode === "register" || mode === "reset") {
        const { token } = await api.verifyPasswordCode(email, value, mode);
        setPasswordToken(token); setStep("set_password"); setCode("");
        return;
      }
      await finishLogin(await api.verifyCode(email, value));
    } catch (err) {
      setError(err instanceof Error ? err.message : t(($) => $.errors.code_invalid)); setCode("");
    } finally { setLoading(false); }
  };

  const handleCompletePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (loading) return;
    if (password !== confirmPassword) { setError(t(($) => $.errors.password_mismatch)); return; }
    if (!validNewPassword(password)) { setError(t(($) => $.errors.password_invalid, PASSWORD_POLICY)); return; }
    setLoading(true); setError("");
    try { await finishLogin(await api.completePassword(passwordToken, password)); setPasswordToken(""); }
    catch (err) { setError(err instanceof Error ? err.message : t(($) => $.errors.password_failed)); }
    finally { setLoading(false); }
  };

  const handleResend = async () => {
    if (cooldown > 0 || loading) return;
    setLoading(true); setError("");
    try { await requestCode(); setCooldown(60); setCode(""); }
    catch (err) { setError(err instanceof Error ? err.message : t(($) => $.errors.resend_failed)); }
    finally { setLoading(false); }
  };

  const handleCliAuthorize = async () => {
    if (!cliCallback) return;
    setLoading(true);

    try {
      let token: string;

      if (authSourceRef.current === "localStorage") {
        // Session was detected via localStorage — reuse that token directly.
        const stored = localStorage.getItem("silieco_token");
        if (!stored) throw new Error("token missing");
        token = stored;
      } else {
        // Session was detected via cookie — obtain a bearer token from the server.
        const res = await api.mintCliToken();
        token = res.token;
      }

      onTokenObtained?.();
      redirectToCliCallback(cliCallback.url, token, cliCallback.state);
    } catch {
      setError(t(($) => $.errors.cli_auth_failed));
      setExistingUser(null);
      setStep("email");
      setLoading(false);
    }
  };

  // -------------------------------------------------------------------------
  // CLI confirm step
  // -------------------------------------------------------------------------

  if (step === "cli_confirm" && existingUser) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            {logo && <div className="mx-auto mb-4">{logo}</div>}
            <CardTitle className="text-display-sm">
              {t(($) => $.cli.title)}
            </CardTitle>
            <CardDescription>
              {t(($) => $.cli.description, { email: existingUser.email })}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Button
              onClick={handleCliAuthorize}
              disabled={loading}
              className="w-full"
              size="lg"
            >
              {loading
                ? t(($) => $.cli.authorizing)
                : t(($) => $.cli.authorize)}
            </Button>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                setExistingUser(null);
                setStep("email");
              }}
            >
              {t(($) => $.cli.different_account)}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const verifying = step === "code";
  const creatingPassword = step === "set_password";
  return (
    <div className="duty-page">
      <SilkBackground className="duty-backdrop" />
      <div className="duty-brand">{logo}<span>{layoutText(($) => $.sidebar.brand_name)}</span></div>
      <section className="duty-card" aria-labelledby="duty-title">
        <DutyAgents
          looking={!verifying && focusedField === "email"}
          privateEntry={focusedField === "code" || focusedField === "password"}
          registering={registering}
          verifying={verifying || creatingPassword}
        />
        <div className="duty-content">
          <header>
            <h1 id="duty-title">
              {verifying ? t(($) => $.verify.title) : creatingPassword ? t(($) => $.password.title) : registering ? t(($) => $.signin.create) : mode === "reset" ? t(($) => $.password.reset) : t(($) => $.signin.welcome)}
            </h1>
            <p className="duty-description">
              {verifying ? t(($) => $.verify.description, { email }) : creatingPassword ? t(($) => $.password.description, PASSWORD_POLICY) : mode === "password" ? t(($) => $.password.login_description) : t(($) => $.signin.description)}
            </p>
          </header>
          {verifying ? (
            <>
              <InputOTP
                autoFocus
                aria-label={t(($) => $.verify.label)}
                aria-invalid={!!error}
                aria-describedby={error ? "duty-error" : undefined}
                onFocus={() => setFocusedField("code")}
                onBlur={() => setFocusedField(null)}
                maxLength={6}
                value={code}
                onChange={(value) => {
                  setCode(value);
                  if (value.length === 6) void handleVerify(value);
                }}
                disabled={loading}
              >
                <InputOTPGroup className="gap-2">
                  {Array.from({ length: 6 }, (_, index) => (
                    <InputOTPSlot key={index} index={index} className="h-11 w-9 rounded-lg border sm:w-11" />
                  ))}
                </InputOTPGroup>
              </InputOTP>
              {error && <p id="duty-error" role="alert" className="text-body text-destructive">{error}</p>}
              <div className="duty-actions">
                <button type="button" className="duty-switch" disabled={loading} onClick={() => {
                  setStep("email");
                  setCode("");
                  setError("");
                  setFocusedField(null);
                }}>{t(($) => $.verify.change_email)}</button>
                <button type="button" className="duty-switch" disabled={cooldown > 0 || loading} onClick={handleResend}>
                  {cooldown > 0 ? t(($) => $.verify.resend_cooldown, { seconds: cooldown }) : t(($) => $.verify.resend)}
                </button>
              </div>
            </>
          ) : creatingPassword ? (
            <form className="duty-form" onSubmit={handleCompletePassword}>
              <Label htmlFor="new-password">{t(($) => $.password.new)}</Label>
              <div className="relative">
                <Input id="new-password" type={showPassword ? "text" : "password"} autoComplete="new-password" required value={password} onChange={e => setPassword(e.target.value)} onFocus={() => setFocusedField("password")} onBlur={() => setFocusedField(null)} className="pr-11" />
                <button type="button" className="absolute inset-y-0 right-2 flex items-center justify-center text-muted-foreground hover:text-foreground" aria-label={showPassword ? t(($) => $.password.hide) : t(($) => $.password.show)} aria-pressed={showPassword} onClick={() => setShowPassword(!showPassword)}>{showPassword ? <EyeOff size={18} /> : <Eye size={18} />}</button>
              </div>
              <Label htmlFor="confirm-password">{t(($) => $.password.confirm)}</Label>
              <Input id="confirm-password" type={showPassword ? "text" : "password"} autoComplete="new-password" required value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} onFocus={() => setFocusedField("password")} onBlur={() => setFocusedField(null)} />
              {error && <p role="alert" className="text-body text-destructive">{error}</p>}
              <Button type="submit" variant="brand" disabled={loading || !password || !confirmPassword}>{loading ? t(($) => $.signin.sending) : t(($) => $.password.complete)}</Button>
              <button type="button" className="duty-switch" disabled={loading} onClick={() => switchMode(mode)}>{t(($) => $.verify.change_email)}</button>
            </form>
          ) : (
            <>
              <form className="duty-form" onSubmit={handleSendCode}>
                <Label htmlFor="login-email">{t(($) => $.common.email)}</Label>
                <Input
                  id="login-email"
                  type="email"
                  autoComplete="email"
                  required
                  readOnly={loading}
                  aria-invalid={!!error}
                  aria-describedby={error ? "duty-error" : undefined}
                  placeholder={t(($) => $.common.email_placeholder)}
                  value={email}
                  onFocus={() => setFocusedField("email")}
                  onBlur={() => setFocusedField(null)}
                  onChange={(e) => setEmail(e.target.value)}
                />
                {mode === "password" && <>
                  <Label htmlFor="login-password">{t(($) => $.password.label)}</Label>
                  <div className="relative">
                    <Input id="login-password" type={showPassword ? "text" : "password"} autoComplete="current-password" required value={password} onChange={e => setPassword(e.target.value)} readOnly={loading} onFocus={() => setFocusedField("password")} onBlur={() => setFocusedField(null)} className="pr-11" />
                    <button type="button" className="absolute inset-y-0 right-2 flex items-center justify-center text-muted-foreground hover:text-foreground" aria-label={showPassword ? t(($) => $.password.hide) : t(($) => $.password.show)} aria-pressed={showPassword} onClick={() => setShowPassword(!showPassword)}>{showPassword ? <EyeOff size={18} /> : <Eye size={18} />}</button>
                  </div>
                </>}
                {error && <p id="duty-error" role="alert" className="text-body text-destructive">{error}</p>}
                <Button type="submit" variant="brand" disabled={!email.trim() || loading || (mode === "password" && !password)}>
                  {loading ? t(($) => $.signin.sending) : mode === "password" ? t(($) => $.password.login) : t(($) => $.signin.continue)}
                </Button>
              </form>
              <div className="duty-actions">
                {mode === "password" ? <>
                  <button type="button" className="duty-switch" disabled={loading} onClick={() => switchMode("code")}>{t(($) => $.password.code_login)}</button>
                  <button type="button" className="duty-switch" disabled={loading} onClick={() => switchMode("reset")}>{t(($) => $.password.reset)}</button>
                  <button type="button" className="duty-switch" disabled={loading} onClick={() => switchMode("register")}>{t(($) => $.signin.create)}</button>
                </> : <>
                  <button type="button" className="duty-switch" disabled={loading} onClick={() => switchMode("password")}>{t(($) => $.password.login)}</button>
                  {mode === "code" && <button type="button" className="duty-switch" disabled={loading} onClick={() => switchMode("register")}>{t(($) => $.signin.create)}</button>}
                </>}
              </div>
            </>
          )}
        </div>
      </section>
      {extra}
    </div>
  );
}
