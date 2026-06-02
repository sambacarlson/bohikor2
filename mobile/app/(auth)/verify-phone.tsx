import { useState } from "react";
import { useRouter, useLocalSearchParams } from "expo-router";
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
import { api } from "@/src/lib/api";
import { setTokens } from "@/src/lib/auth";
import { useAuth } from "@/src/providers/auth-provider";

export default function VerifyPhoneScreen() {
  const router = useRouter();
  const { email } = useLocalSearchParams<{ email: string }>();
  const { refreshUser } = useAuth();

  const [countryCode, setCountryCode] = useState("+237");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [otpCode, setOtpCode] = useState("");
  const [step, setStep] = useState<"phone" | "otp">("phone");
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const [verifying, setVerifying] = useState(false);

  const fullPhone = `${countryCode}${phoneNumber}`;
  const isValidPhone = (phone: string) => /^\+[1-9]\d{6,14}$/.test(phone);

  const handleSendCode = async () => {
    setError("");
    if (!phoneNumber.trim()) {
      setError("Phone number is required");
      return;
    }
    if (!isValidPhone(fullPhone)) {
      setError("Enter a valid phone number with country code (e.g., +237 6XXXXXXXX)");
      return;
    }
    setSending(true);
    try {
      await api.post("/api/auth/send-phone-otp", { phone_number: fullPhone });
      setStep("otp");
    } catch (err: unknown) {
      if (err && typeof err === "object" && "response" in err && err.response && typeof err.response === "object" && "data" in err.response) {
        const data = (err.response as { data?: { error?: string } }).data;
        setError(data?.error || "Failed to send verification code.");
      } else {
        setError("Network error. Please try again.");
      }
    } finally {
      setSending(false);
    }
  };

  const handleVerifyOTP = async () => {
    setError("");
    if (otpCode.length !== 6) {
      setError("Please enter the full 6-digit code");
      return;
    }
    setVerifying(true);
    try {
      const { data } = await api.post("/api/auth/verify-phone-otp", {
        phone_number: fullPhone,
        code: otpCode,
        email,
      });
      await setTokens(data.data.access_token, data.data.refresh_token);
      await refreshUser();
      router.replace("/(app)/home");
    } catch (err: unknown) {
      if (err && typeof err === "object" && "response" in err && err.response && typeof err.response === "object" && "data" in err.response) {
        const data = (err.response as { data?: { error?: string } }).data;
        setError(data?.error || "Failed to complete signup. Please try again.");
      } else {
        setError("Network error. Please try again.");
      }
    } finally {
      setVerifying(false);
    }
  };

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      className="flex-1 bg-white"
    >
      <ScrollView contentContainerClassName="flex-1 justify-center px-6">
        {step === "phone" ? (
          <>
            <View className="items-center mb-8">
              <Text className="text-3xl font-bold text-gray-900">
                Verify your phone
              </Text>
              <Text className="text-gray-500 mt-2 text-center text-lg">
                Enter your phone number to receive a verification code via SMS
              </Text>
            </View>

            <View className="w-full">
              <Text className="text-gray-700 mb-2 font-medium text-base">
                Phone number
              </Text>
              <View className="flex-row gap-3">
                <TextInput
                  className="border border-gray-300 rounded-lg px-3 py-4 text-lg w-20 text-center"
                  value={countryCode}
                  onChangeText={(text) => {
                    setCountryCode(text.startsWith("+") ? text : `+${text}`);
                    setError("");
                  }}
                  keyboardType="phone-pad"
                />
                <TextInput
                  className="flex-1 border border-gray-300 rounded-lg px-4 py-4 text-lg"
                  placeholder="6XXXXXXXX"
                  keyboardType="phone-pad"
                  value={phoneNumber}
                  onChangeText={(text) => {
                    setPhoneNumber(text);
                    setError("");
                  }}
                  autoFocus
                />
              </View>

              {error ? (
                <Text className="text-red-500 mt-2 text-base">{error}</Text>
              ) : null}

              <TouchableOpacity
                className={`mt-6 rounded-xl py-4 items-center flex-row justify-center ${
                  sending ? "bg-primary-300" : "bg-primary-600"
                }`}
                onPress={handleSendCode}
                disabled={sending}
              >
                {sending ? (
                  <ActivityIndicator color="white" />
                ) : (
                  <Text className="text-white font-bold text-lg">
                    Send Code
                  </Text>
                )}
              </TouchableOpacity>
            </View>
          </>
        ) : (
          <>
            <View className="items-center mb-8">
              <Text className="text-3xl font-bold text-gray-900">
                Enter SMS code
              </Text>
              <Text className="text-gray-500 mt-2 text-center text-lg">
                We sent a 6-digit code to{"\n"}
                <Text className="font-medium text-gray-700 text-lg">
                  {fullPhone}
                </Text>
              </Text>
            </View>

            <View className="w-full">
              <Text className="text-gray-700 mb-2 font-medium text-base">
                Verification code
              </Text>
              <TextInput
                className="border border-gray-300 rounded-lg px-4 py-4 text-lg text-center tracking-widest"
                placeholder="000000"
                keyboardType="number-pad"
                maxLength={6}
                value={otpCode}
                onChangeText={(text) => {
                  setOtpCode(text.replace(/[^0-9]/g, ""));
                  setError("");
                }}
                autoFocus
              />

              {error ? (
                <Text className="text-red-500 mt-2 text-base">{error}</Text>
              ) : null}

              <TouchableOpacity
                className={`mt-6 rounded-xl py-4 items-center flex-row justify-center ${
                  verifying ? "bg-primary-300" : "bg-primary-600"
                }`}
                onPress={handleVerifyOTP}
                disabled={verifying}
              >
                {verifying ? (
                  <ActivityIndicator color="white" />
                ) : (
                  <Text className="text-white font-semibold text-lg">
                    Verify & Continue
                  </Text>
                )}
              </TouchableOpacity>

              <TouchableOpacity
                className="mt-4 py-3 items-center"
                onPress={() => {
                  setStep("phone");
                  setOtpCode("");
                  setError("");
                }}
              >
                <Text className="text-primary-600 font-medium text-lg">
                  Change phone number
                </Text>
              </TouchableOpacity>
            </View>
          </>
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}