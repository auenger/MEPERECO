import { useEffect, useRef, useState } from "react";
import { Pressable, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { TextField } from "@/components/ui/text-field";
import { api } from "@/data/api";
import { PASSWORD_POLICY, validNewPassword } from "@silieco/core/auth/password-policy";
import { router, useLocalSearchParams } from "expo-router";
import * as Haptics from "expo-haptics";
import { Text } from "@/components/ui/text";
import { OtpInput, type OtpInputRef } from "@/components/ui/otp-input";
import { Button } from "@/components/ui/button";
import { DutyScreen } from "@/components/auth/duty-screen";
import { useAuthStore } from "@/data/auth-store";
import { mapAuthError } from "@/lib/auth-error";

const CODE_LENGTH = 6;
const RESEND_COOLDOWN_SECONDS = 60;

export default function Verify() {
  const sendCode = useAuthStore((s) => s.sendCode);
  const verifyCode = useAuthStore((s) => s.verifyCode);
  const { email = "", purpose = "code" } = useLocalSearchParams<{ email?: string; purpose?: string }>();
  const acceptLogin = useAuthStore((s) => s.acceptLogin);
  const [passwordToken, setPasswordToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS);
  const [resending, setResending] = useState(false);
  const otpRef = useRef<OtpInputRef>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setInterval(() => {
      setCooldown((c) => (c <= 1 ? 0 : c - 1));
    }, 1000);
    return () => clearInterval(t);
  }, [cooldown]);

  const submit = async (value: string) => {
    if (!value || !email || submitting) return;
    void Haptics.selectionAsync();
    setSubmitting(true);
    setError(null);
    try {
      if (purpose === "register" || purpose === "reset") {
        const { token } = await api.verifyPasswordCode(email, value, purpose);
        setPasswordToken(token); setSubmitting(false); setCode(""); return;
      }
      await verifyCode(email, value);
      void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success);
      router.replace("/");
    } catch (err) {
      void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error);
      setError(mapAuthError(err, "Couldn't verify the code. Try again."));
      setSubmitting(false);
      otpRef.current?.clear();
      setCode("");
    }
  };

  const onResend = async () => {
    if (cooldown > 0 || resending || !email) return;
    void Haptics.selectionAsync();
    setResending(true);
    setError(null);
    try {
      if (purpose === "register" || purpose === "reset") await api.sendPasswordCode(email, purpose);
      else await sendCode(email);
      setCooldown(RESEND_COOLDOWN_SECONDS);
      otpRef.current?.clear();
      setCode("");
    } catch (err) {
      void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error);
      setError(mapAuthError(err, "Couldn't resend the code. Try again."));
    } finally {
      setResending(false);
    }
  };

  const complete = async () => {
    if (submitting) return;
    if (!validNewPassword(password)) { setError(`Use at least ${PASSWORD_POLICY.min} characters (up to ${PASSWORD_POLICY.maxBytes} bytes).`); return; }
    if (password !== confirmPassword) { setError("Passwords do not match."); return; }
    setSubmitting(true);setError(null);
    try { await acceptLogin(await api.completePassword(passwordToken, password)); router.replace("/"); }
    catch (err) { setError(mapAuthError(err, "Couldn't save password.")); }
    finally { setSubmitting(false); }
  };
  if (passwordToken) return <DutyScreen phase="received" title="Set a password" description={`Email verified. Use at least ${PASSWORD_POLICY.min} characters (up to ${PASSWORD_POLICY.maxBytes} bytes).`}>
    <View className="gap-2">
      <Text>New password</Text>
      <View className="relative"><TextField accessibilityLabel="New password" secureTextEntry={!showPassword} autoComplete="new-password" autoCapitalize="none" value={password} onChangeText={setPassword} className="pr-12"/><Pressable accessibilityRole="button" accessibilityLabel={showPassword ? "Hide password" : "Show password"} onPress={()=>setShowPassword(!showPassword)} className="absolute right-3 top-0 bottom-0 justify-center"><Ionicons name={showPassword?"eye-off-outline":"eye-outline"} size={20} color="#64748b"/></Pressable></View>
      <Text>Confirm password</Text><TextField accessibilityLabel="Confirm password" secureTextEntry={!showPassword} autoComplete="new-password" autoCapitalize="none" value={confirmPassword} onChangeText={setConfirmPassword}/>
      {error && <Text className="text-sm text-destructive">{error}</Text>}
    </View>
    <Button size="lg" className="bg-brand" disabled={submitting||!password||!confirmPassword} onPress={complete}><Text className="text-brand-foreground">{submitting?"Saving...":"Confirm password"}</Text></Button>
    <Button variant="ghost" disabled={submitting} onPress={()=>router.back()}><Text>Use a different email</Text></Button>
  </DutyScreen>;

  return (
    <DutyScreen
      phase="received"
      title="Check your email"
      description={`Enter the 6-digit code we sent to ${email}.`}
    >
      <View className="gap-3 items-center">
        <OtpInput
          ref={otpRef}
          numberOfDigits={CODE_LENGTH}
          value={code}
          onChange={setCode}
          onComplete={submit}
          autoFocus
          editable={!submitting}
        />
        {error ? (
          <Text className="text-sm text-destructive">{error}</Text>
        ) : null}
      </View>

      <View className="gap-3">
        <Button
          size="lg"
          className="bg-brand active:bg-brand/90"
          disabled={submitting || code.length < CODE_LENGTH}
          onPress={() => submit(code)}
        >
          <Text className="text-brand-foreground">{submitting ? "Verifying..." : "Verify"}</Text>
        </Button>

        <Pressable
          onPress={onResend}
          disabled={cooldown > 0 || resending}
          className="py-2 items-center"
        >
          <Text
            className={
              cooldown > 0 || resending
                ? "text-sm text-muted-foreground"
                : "text-sm text-primary"
            }
          >
            {resending
              ? "Sending..."
              : cooldown > 0
                ? `Resend code in ${cooldown}s`
                : "Resend code"}
          </Text>
        </Pressable>

        <Button
          variant="ghost"
          disabled={submitting}
          onPress={() => router.back()}
        >
          <Text>Use a different email</Text>
        </Button>
      </View>
    </DutyScreen>
  );
}
