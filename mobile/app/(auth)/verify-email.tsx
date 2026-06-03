import { useState, useRef, useEffect } from "react";
import { useLocalSearchParams, useRouter } from "expo-router";
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
} from "react-native";
import { useVerifyEmailOTP, useSendEmailOTP } from "@/src/hooks/use-auth";
import { setTokens } from "@/src/lib/auth";
import { api } from "@/src/lib/api";
import { useAuth } from "@/src/providers/auth-provider";

export default function VerifyEmailScreen() {
  const router = useRouter();
  const { email, purpose } = useLocalSearchParams<{
    email: string;
    purpose?: string;
  }>();
  const { refreshUser } = useAuth();
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [resendCooldown, setResendCooldown] = useState(0);
  const inputRef = useRef<TextInput>(null);

  const verifyEmailOTP = useVerifyEmailOTP();
  const sendEmailOTP = useSendEmailOTP();

  const isPinReset = purpose === "pin_reset";

  useEffect(() => {
    if (resendCooldown > 0) {
      const timer = setTimeout(() => setResendCooldown(resendCooldown - 1), 1000);
      return () => clearTimeout(timer);
    }
  }, [resendCooldown]);

  const handleVerify = async () => {
    setError("");

    if (code.length !== 6) {
      setError("Please enter the full 6-digit code");
      return;
    }

    try {
      const result = await verifyEmailOTP.mutateAsync({
        email,
        code,
        purpose: isPinReset ? "pin_reset" : "signup",
      });

      if (isPinReset && result) {
        await setTokens(result.access_token, result.refresh_token);
        await refreshUser();
        router.replace({
          pathname: "/(auth)/reset-pin",
        });
        return;
      }

      router.push({
        pathname: "/(auth)/create-pin",
        params: { email },
      });
    } catch {
      setError("Invalid or expired code. Please try again.");
      setCode("");
    }
  };

  const handleResend = async () => {
    if (resendCooldown > 0) return;
    setError("");

    try {
      if (isPinReset) {
        await api.post("/api/auth/forgot-pin", { email });
      } else {
        await sendEmailOTP.mutateAsync(email);
      }
      setResendCooldown(60);
    } catch {
      setError("Failed to resend code. Please try again.");
    }
  };

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      className="flex-1 bg-white"
    >
      <ScrollView contentContainerClassName="flex-1 justify-center px-6">
        <View className="items-center mb-8">
          <Text className="text-3xl font-bold text-gray-900">
            {isPinReset ? "Reset your PIN" : "Check your email"}
          </Text>
          <Text className="text-gray-500 mt-2 text-center text-lg">
            We sent a 6-digit code to{"\n"}
            <Text className="font-medium text-gray-700 text-lg">{email}</Text>
          </Text>
        </View>

        <View className="w-full">
          <Text className="text-gray-700 mb-2 font-medium text-base">
            Verification code
          </Text>
          <TextInput
            ref={inputRef}
            className="border border-gray-300 rounded-lg px-4 py-4 text-lg text-center tracking-widest"
            placeholder="000000"
            keyboardType="number-pad"
            maxLength={6}
            value={code}
            onChangeText={(text) => {
              setCode(text.replace(/[^0-9]/g, ""));
              setError("");
            }}
            autoFocus
          />

          {error ? (
            <Text className="text-red-500 mt-2 text-base">{error}</Text>
          ) : null}

          <TouchableOpacity
            className={`mt-6 rounded-xl py-4 items-center flex-row justify-center ${
              verifyEmailOTP.isPending ? "bg-primary-300" : "bg-primary-600"
            }`}
            onPress={handleVerify}
            disabled={verifyEmailOTP.isPending || code.length !== 6}
          >
            {verifyEmailOTP.isPending ? (
              <ActivityIndicator color="white" />
            ) : (
              <Text className="text-white font-bold text-lg">Verify</Text>
            )}
          </TouchableOpacity>

          <TouchableOpacity
            className="mt-4 py-3 items-center"
            onPress={handleResend}
            disabled={resendCooldown > 0 || sendEmailOTP.isPending}
          >
            {sendEmailOTP.isPending ? (
              <ActivityIndicator />
            ) : (
              <Text
                className={`text-lg ${
                  resendCooldown > 0
                    ? "text-gray-400"
                    : "text-primary-600 font-medium"
                }`}
              >
                {resendCooldown > 0
                  ? `Resend code in ${resendCooldown}s`
                  : "Resend code"}
              </Text>
            )}
          </TouchableOpacity>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
