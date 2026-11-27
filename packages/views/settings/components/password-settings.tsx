"use client";

import { useState, type FormEvent } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Input } from "@silieco/ui/components/ui/input";
import { Button } from "@silieco/ui/components/ui/button";
import { Label } from "@silieco/ui/components/ui/label";
import { api } from "@silieco/core/api";
import { useAuthStore } from "@silieco/core/auth";
import { PASSWORD_POLICY, validNewPassword } from "@silieco/core/auth/password-policy";
import { useT } from "../../i18n";
import { SettingsCard, SettingsSection } from "./settings-layout";

export function PasswordSettings() {
  const { t } = useT("auth");
  const user = useAuthStore(s => s.user);
  const [mode, setMode] = useState<"idle" | "current" | "code" | "reset">("idle");
  const [current, setCurrent] = useState("");
  const [code, setCode] = useState("");
  const [token, setToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [visible, setVisible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  function clear() { setMode("idle"); setCurrent(""); setCode(""); setToken(""); setPassword(""); setConfirmation(""); setVisible(false); setError(""); }
  if (!user) return null;
  async function send() {
    if (!user || busy) return;
    setBusy(true);setError("");setSuccess(false);
    try { await api.sendPasswordCode(user.email,"reset"); setMode("code");setCode("");setToken("");setCurrent(""); }
    catch (err) { setError(err instanceof Error ? err.message : t($=>$.errors.send_failed)); }
    finally { setBusy(false); }
  }
  async function submit(e: FormEvent) {
    e.preventDefault(); if (!user || busy) return;
    setError("");setSuccess(false);
    if (mode !== "code" && !validNewPassword(password)) {setError(t($=>$.errors.password_invalid, PASSWORD_POLICY)); return;}
    if (mode !== "code" && password !== confirmation) {setError(t($=>$.errors.password_mismatch)); return;}
    setBusy(true);
    try {
      if (mode === "code") {const result=await api.verifyPasswordCode(user.email,code,"reset");setToken(result.token);setMode("reset");setCode("");return;}
      if (mode === "current") {await api.changePassword(current,password);await useAuthStore.getState().refreshMe();}
      else if (mode === "reset") {const result=await api.completePassword(token,password);useAuthStore.getState().acceptLogin(result);}
      clear();setSuccess(true);
    } catch(err) {setError(err instanceof Error?err.message:t($=>$.errors.password_failed));}
    finally {setBusy(false);}
  }
  const passwordInput = (id: string,value: string,onChange: (value:string)=>void, autocomplete: string) => <div className="relative"><Input id={id} type={visible?"text":"password"} autoComplete={autocomplete} value={value} onChange={e=>onChange(e.target.value)} className="pr-10" required/><button type="button" className="absolute inset-y-0 right-2 flex items-center text-muted-foreground" aria-label={visible?t($=>$.password.hide):t($=>$.password.show)} aria-pressed={visible} onClick={()=>setVisible(!visible)}>{visible?<EyeOff size={18}/>:<Eye size={18}/>}</button></div>;
  return <SettingsSection title={t($=>$.password.label)}><SettingsCard><div className="flex flex-col gap-3 p-4">
    {success && <p role="status" className="text-sm text-muted-foreground">{t($=>$.password.updated)}</p>}
    {mode === "idle" ? <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={busy} onClick={()=>{setSuccess(false); if(user.has_password)setMode("current");else void send();}}>{user.has_password?t($=>$.password.change):t($=>$.password.title)}</Button>{user.has_password&&<Button variant="ghost" disabled={busy} onClick={send}>{t($=>$.password.reset)}</Button>}</div> : <form className="grid gap-3 max-w-sm" onSubmit={submit}>
      {mode === "code" ? <><p className="text-sm">{t($=>$.verify.description,{email:user.email})}</p><Label htmlFor="settings-password-code">{t($=>$.verify.label)}</Label><Input id="settings-password-code" inputMode="numeric" autoComplete="one-time-code" maxLength={6} value={code} onChange={e=>setCode(e.target.value.replace(/\D/g,""))} required/><Button type="button" variant="ghost" disabled={busy} onClick={send}>{t($=>$.verify.resend)}</Button></> : <>
      {mode==="current"&&<><Label htmlFor="settings-current-password">{t($=>$.password.current)}</Label>{passwordInput("settings-current-password",current,setCurrent,"current-password")}</>}
      <p className="text-sm text-muted-foreground">{t($=>$.password.description, PASSWORD_POLICY)}</p><Label htmlFor="settings-new-password">{t($=>$.password.new)}</Label>{passwordInput("settings-new-password",password,setPassword,"new-password")}<Label htmlFor="settings-confirm-password">{t($=>$.password.confirm)}</Label>{passwordInput("settings-confirm-password",confirmation,setConfirmation,"new-password")}</>}
      <div className="flex gap-2"><Button type="submit" variant="brand" disabled={busy||(mode==="code"?code.length!==6:!password||!confirmation||(mode==="current"&&!current))}>{busy?t($=>$.signin.sending):t($=>$.password.complete)}</Button><Button type="button" variant="ghost" disabled={busy} onClick={clear}>{t($=>$.common.back)}</Button></div>
    </form>}
    {error&&<p role="alert" className="text-sm text-destructive">{error}</p>}
  </div></SettingsCard></SettingsSection>;
}
