import { useState } from "react";
import { useRouter } from "expo-router";
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
import { useForgotPin } from "@/src/hooks/use-auth";

export default function ForgotPinScreen() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");

  const forgotPin = useForgotPin();

  const isValidEmail = (e: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e);

  const handleContinue = async () => {
    setError("");
    if (!email.trim()) {
      setError("Email is required");
      return;
    }
    if (!isValidEmail(email.trim())) {
      setError("Please enter a valid email address");
      return;
    }

    try {
      await forgotPin.mutateAsync(email.trim());
      router.push({
        pathname: "/(auth)/verify-email",
        params: { email: email.trim(), purpose: "pin_reset" },
      });
    } catch (err: unknown) {
      if (
        err &&
        typeof err === "object" &&
        "response" in err &&
        err.response &&
        typeof err.response === "object" &&
        "data" in err.response
      ) {
        const data = (err.response as { data?: { error?: string } }).data;
        setError(data?.error || "Failed to send reset code.");
      } else {
        setError("Network error. Please check your connection.");
      }
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
            Forgot PIN?
          </Text>
          <Text className="text-gray-500 mt-2 text-center text-lg">
            Enter your email and we&apos;ll send you a verification code to reset your PIN
          </Text>
        </View>

        <View className="w-full">
          <Text className="text-gray-700 mb-2 font-medium text-base">Email</Text>
          <TextInput
            className="border border-gray-300 rounded-lg px-4 py-4 text-lg"
            placeholder="you@company.com"
            keyboardType="email-address"
            autoCapitalize="none"
            autoComplete="email"
            value={email}
            onChangeText={(text) => {
              setEmail(text);
              setError("");
            }}
          />

          {error ? (
            <Text className="text-red-500 mt-2 text-base">{error}</Text>
          ) : null}

          <TouchableOpacity
            className={`mt-6 rounded-xl py-4 items-center ${
              email.trim() && !forgotPin.isPending
                ? "bg-primary-600"
                : "bg-primary-300"
            }`}
            onPress={handleContinue}
            disabled={!email.trim() || forgotPin.isPending}
          >
            {forgotPin.isPending ? (
              <ActivityIndicator color="white" />
            ) : (
              <Text className="text-white font-bold text-lg">Send Reset Code</Text>
            )}
          </TouchableOpacity>

          <TouchableOpacity
            className="mt-4 py-3 items-center"
            onPress={() => router.back()}
          >
            <Text className="text-primary-600 font-medium text-lg">
              Back to Login
            </Text>
          </TouchableOpacity>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
