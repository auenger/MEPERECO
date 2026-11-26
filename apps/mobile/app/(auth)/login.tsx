import { useCallback, useRef, useState } from "react";
import { View, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { api } from "@/data/api";
import { useReducedMotion } from "react-native-reanimated";
import { router, useFocusEffect } from "expo-router";
import * as Haptics from "expo-haptics";
import { Text } from "@/components/ui/text";
import { TextField } from "@/components/ui/text-field";
import { Button } from "@/components/ui/button";
import { DutyScreen, LETTER_FLIGHT_MS } from "@/components/auth/duty-screen";
import { useAuthStore } from "@/data/auth-store";
import { mapAuthError } from "@/lib/auth-error";

export default function Login() {
  const acceptLogin = useAuthStore((s) => s.acceptLogin);
  const [mode, setMode] = useState<"password" | "code" | "register" | "reset">("password");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const inFlight = useRef(false);
  const reducedMotion = useReducedMotion();

  useFocusEffect(useCallback(() => {
    inFlight.current = false;
    setSubmitting(false);
  }, []));

  const onSubmit = async () => {
    const trimmed = email.trim();
    if (!trimmed || inFlight.current || (mode === "password" && !password)) return;
    inFlight.current = true;
    void Haptics.selectionAsync();
    setSubmitting(true);
    setError(null);
    try {
      if (mode === "password") {
        await acceptLogin(await api.loginPassword(trimmed, password));
        router.replace("/");
        return;
      }
      await Promise.all([
        mode === "register" || mode === "reset" ? api.sendPasswordCode(trimmed, mode) : api.sendCode(trimmed),
        reducedMotion ? undefined : new Promise<void>((resolve) => setTimeout(resolve, LETTER_FLIGHT_MS)),
      ]);
      router.push({ pathname: "/verify", params: { email: trimmed, purpose: mode } });
    } catch (err) {
      void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error);
      setError(mapAuthError(err, "Unable to sign in or send the code. Try again."));
      inFlight.current = false;
      setSubmitting(false);
    }
  };

  return (
    <DutyScreen
      phase={submitting ? "sending" : "ready"}
      title={mode === "register" ? "Create an account" : mode === "reset" ? "Reset or set password" : "Welcome back"}
      description={mode === "password" ? "Sign in with your email and password." : "Verify your email to continue."}
    >
      <View className="gap-3">
        <Text className="text-sm font-medium">Email</Text>
        <TextField
          accessibilityLabel="Email"
          autoCapitalize="none"
          autoComplete="email"
          keyboardType="email-address"
          placeholder="you@example.com"
          value={email}
          onChangeText={setEmail}
          onSubmitEditing={onSubmit}
          returnKeyType="send"
          editable={!submitting}
          invalid={!!error}
        />
        {mode === "password" && <>
          <Text className="text-sm font-medium">Password</Text>
          <View className="relative">
            <TextField accessibilityLabel="Password" autoComplete="current-password" secureTextEntry={!showPassword} autoCapitalize="none" value={password} onChangeText={setPassword} editable={!submitting} className="pr-12" onSubmitEditing={onSubmit} />
            <Pressable accessibilityRole="button" accessibilityLabel={showPassword ? "Hide password" : "Show password"} onPress={() => setShowPassword(!showPassword)} className="absolute right-3 top-0 bottom-0 justify-center"><Ionicons name={showPassword ? "eye-off-outline" : "eye-outline"} size={20} color="#64748b" /></Pressable>
          </View>
        </>}
        {error ? (
          <Text className="text-sm text-destructive">{error}</Text>
        ) : null}
      </View>

      <Button
        size="lg"
        className="bg-brand active:bg-brand/90"
        disabled={submitting || !email.trim() || (mode === "password" && !password)}
        onPress={onSubmit}
      >
        <Text className="text-brand-foreground">{submitting ? "Working..." : mode === "password" ? "Sign in" : "Send code"}</Text>
      </Button>
      <View className="flex-row flex-wrap justify-between gap-3">
        {(mode === "password" ? [ ["code", "Sign in with a code"], ["reset", "Forgot password?"], ["register", "Create an account"] ] : [["password", "Sign in with password"]]).map(([next, label]) => <Pressable key={next} disabled={submitting} onPress={() => { setMode(next as typeof mode); setPassword(""); setShowPassword(false); setError(null); }}><Text className="text-sm text-primary underline">{label}</Text></Pressable>)}
      </View>
    </DutyScreen>
  );
}
