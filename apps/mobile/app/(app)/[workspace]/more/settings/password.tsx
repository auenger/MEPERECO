import { useState } from "react";
import { Pressable, ScrollView, View } from "react-native";
import { router, Stack } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { SafeAreaView } from "react-native-safe-area-context";
import { api } from "@/data/api";
import { useAuthStore } from "@/data/auth-store";
import { Text } from "@/components/ui/text";
import { TextField } from "@/components/ui/text-field";
import { Button } from "@/components/ui/button";
import { mapAuthError } from "@/lib/auth-error";

export default function PasswordSettings() {
  const user = useAuthStore(s => s.user);
  const [mode,setMode] = useState<"current"|"send"|"code"|"reset">(user?.has_password?"current":"send");
  const [current,setCurrent]=useState(""); const [password,setPassword]=useState(""); const [confirm,setConfirm]=useState("");
  const [code,setCode]=useState("");const [token,setToken]=useState("");const [show,setShow]=useState(false);const [busy,setBusy]=useState(false);const [error,setError]=useState("");
  async function send() {if(!user||busy)return;setBusy(true);setError("");try {await api.sendPasswordCode(user.email,"reset");setMode("code");setCode("");}catch(e){setError(mapAuthError(e,"Couldn't send code."));}finally{setBusy(false);}}
  async function submit() {if(!user||busy)return;setError("");if(mode!=="code"&&(Array.from(password).length<12||password!==confirm)){setError("Use at least 12 characters and matching passwords.");return;}setBusy(true);try{
      if(mode==="code"){const r=await api.verifyPasswordCode(user.email,code,"reset");setToken(r.token);setMode("reset");return;}
      if(mode==="current"){await api.changePassword(current,password);useAuthStore.getState().setUser(await api.getMe());}
      else await useAuthStore.getState().acceptLogin(await api.completePassword(token,password));
      router.back();
    }catch(e){setError(mapAuthError(e,"Couldn't save password."));}finally{setBusy(false);}}
  function field(label: string,value: string,set: (x:string)=>void,complete: "current-password"|"new-password") {return <View className="gap-2"><Text>{label}</Text><View className="relative"><TextField accessibilityLabel={label} autoComplete={complete} secureTextEntry={!show} value={value} onChangeText={set} className="pr-12"/><Pressable accessibilityRole="button" accessibilityLabel={show?"Hide password":"Show password"} onPress={()=>setShow(!show)} className="absolute right-3 top-0 bottom-0 justify-center"><Ionicons name={show?"eye-off-outline":"eye-outline"} size={20} color="#64748b"/></Pressable></View></View>;}
  return <SafeAreaView className="flex-1 bg-background"><Stack.Screen options={{title:"Password",headerShown:true}}/><ScrollView contentContainerStyle={{padding:20,gap:16}} keyboardShouldPersistTaps="handled"><Text className="text-xl font-semibold">{user?.has_password?"Change password":"Set password"}</Text><Text className="text-muted-foreground">{user?.email}</Text>
    {mode==="send"?<Button onPress={send} disabled={busy}><Text>Send email verification code</Text></Button>:<>
      {mode==="code"?<><Text>Verification code</Text><TextField keyboardType="number-pad" maxLength={6} value={code} onChangeText={v=>setCode(v.replace(/\D/g,""))}/><Button variant="ghost" onPress={send} disabled={busy}><Text>Resend code</Text></Button></>:<>{mode==="current"&&field("Current password",current,setCurrent,"current-password")}{field("New password",password,setPassword,"new-password")}{field("Confirm password",confirm,setConfirm,"new-password")}<Text className="text-muted-foreground text-xs">Use at least 12 characters (up to 72 bytes).</Text></>}
      <Button disabled={busy||(mode==="code"?code.length!==6:!password||!confirm||(mode==="current"&&!current))} onPress={submit}><Text>{busy?"Working...":mode==="code"?"Verify":"Save password"}</Text></Button>
      {mode==="current"&&<Button variant="ghost" onPress={send} disabled={busy}><Text>Reset using email</Text></Button>}
    </>}{error!==""&&<Text className="text-destructive">{error}</Text>}</ScrollView></SafeAreaView>;
}
