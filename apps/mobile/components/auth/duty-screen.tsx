import { useEffect, type ReactNode } from "react";
import { Image } from "expo-image";
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, View } from "react-native";
import Animated, {
  interpolate, useAnimatedStyle, useSharedValue, withTiming,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { SvgXml } from "react-native-svg";
import { dutyAgents, studioWindow } from "@silieco/core/auth/agent-art";
import { SiliecoLogo } from "@/components/brand/silieco-logo";
import { Text } from "@/components/ui/text";

export const LETTER_FLIGHT_MS = 320;
type Phase = "ready" | "sending" | "received";

function WindowScene({ phase }: { phase: Phase }) {
  // The outgoing screen ends at the midpoint. The next screen starts there
  // and carries the same envelope into the second agent's hand.
  const flight = useSharedValue(phase === "received" ? 0.5 : 0);
  useEffect(() => {
    const destination = { ready: 0, sending: 0.5, received: 1 }[phase];
    flight.value = withTiming(destination, { duration: LETTER_FLIGHT_MS });
  }, [phase, flight]);

  const letter = useAnimatedStyle(() => ({
    transform: [
      { translateX: interpolate(flight.value, [0, 0.5, 1], [0, 26, 52]) },
      { translateY: interpolate(flight.value, [0, 0.5, 1], [0, -15, 0]) },
      { rotate: `${interpolate(flight.value, [0, 0.5, 1], [-12, -4, 9])}deg` },
    ],
  }));
  const sender = useAnimatedStyle(() => ({
    transform: [{ rotate: `${interpolate(flight.value, [0, 0.5, 1], [-4, 9, -3])}deg` }],
  }));
  const receiver = useAnimatedStyle(() => ({
    transform: [{ rotate: `${interpolate(flight.value, [0, 0.5, 0.85, 1], [4, 4, -9, -3])}deg` }],
  }));

  return (
    <View pointerEvents="none" accessibilityElementsHidden importantForAccessibility="no-hide-descendants"
      style={{ height: 120, marginBottom: 12 }}>
      <View style={{ flex: 1, overflow: "hidden", borderRadius: 15, backgroundColor: "#20365c" }}>
        <SvgXml xml={studioWindow} width="100%" height="100%" preserveAspectRatio="xMidYMid slice" />
      </View>
      <View style={{ position: "absolute", width: 200, alignSelf: "center", bottom: -16, height: 99 }}>
        <Animated.View style={[{ position: "absolute", left: 1, bottom: 0 }, sender]}>
          <SvgXml xml={dutyAgents.terminal} width={96} height={100} />
        </Animated.View>
        <Animated.View style={[{ position: "absolute", right: 0, bottom: 0 }, receiver]}>
          <SvgXml xml={dutyAgents.courier} width={96} height={100} />
        </Animated.View>
        <Animated.View style={[{ position: "absolute", left: 66, bottom: 13 }, letter]}>
          <SvgXml xml={dutyAgents.envelope} width={29} height={23} />
        </Animated.View>
      </View>
    </View>
  );
}

export function DutyScreen({ phase, title, description, children }: {
  phase: Phase;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <SafeAreaView className="flex-1 bg-background">
      <View pointerEvents="none" accessibilityElementsHidden importantForAccessibility="no-hide-descendants"
        style={StyleSheet.absoluteFill} className="opacity-[0.16] dark:opacity-30">
        <Image
          source={require("@silieco/core/brand/blue-satin.webp")}
          contentFit="cover"
          style={{ width: "100%", height: "100%" }}
          accessible={false}
        />
      </View>
      <KeyboardAvoidingView className="flex-1" behavior={Platform.OS === "ios" ? "padding" : undefined}>
        <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={{ flexGrow: 1, justifyContent: "center", padding: 20 }}>
          <View className="w-full max-w-md self-center gap-4">
            <View className="flex-row items-center gap-2">
              <SiliecoLogo size={22} /><Text className="font-semibold text-foreground">MEPERStudio</Text>
            </View>
            <WindowScene phase={phase} />
            <View className="gap-1">
              <Text accessibilityRole="header" className="text-3xl font-semibold text-foreground">{title}</Text>
              <Text className="text-sm text-muted-foreground">{description}</Text>
            </View>
            {children}
          </View>
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
