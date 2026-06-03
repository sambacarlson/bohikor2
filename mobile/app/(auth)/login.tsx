import { useState, useEffect } from "react";
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
import { setTokens } from "@/src/lib/auth";
import { useAuth } from "@/src/providers/auth-provider";
import { useLogin } from "@/src/hooks/use-auth";

export default function LoginScreen() {
  const router = useRouter();
  const { user, refreshUser } = useAuth();

  useEffect(() => {
    if (user) {
      router.replace("/(app)/home");
    }
  }, [user, router]);

  const [email, setEmail] = useState("");
  const [pin, setPin] = useState("");
  const [error, setError] = useState("");

  const login = useLogin();

  const isValidEmail = (e: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e);

  const handleLogin = async () => {
    setError("");
    if (!email.trim()) {
      setError("Email is required");
      return;
    }
    if (!isValidEmail(email.trim())) {
      setError("Please enter a valid email address");
      return;
    }
    if (pin.length !== 5) {
      setError("PIN must be 5 digits");
      return;
    }

    try {
      const result = await login.mutateAsync({ email: email.trim(), pin });
      await setTokens(result.access_token, result.refresh_token);
      await refreshUser();
      router.replace("/(app)/home");
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
        setError(data?.error || "Invalid email or PIN.");
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
          <Text className="text-3xl font-bold text-gray-900">Bohikor</Text>
          <Text className="text-gray-500 mt-2 text-center text-lg">
            Salary Advance
          </Text>
        </View>

        <View className="w-full mb-6">
          <Text className="text-xl font-bold text-gray-900 mb-5">
            Log In
          </Text>

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

          <Text className="text-gray-700 mb-2 font-medium text-base mt-4">PIN</Text>
          <TextInput
            className="border border-gray-300 rounded-lg px-4 py-4 text-lg tracking-widest"
            placeholder="•••••"
            keyboardType="number-pad"
            maxLength={5}
            secureTextEntry
            value={pin}
            onChangeText={(text) => {
              setPin(text.replace(/[^0-9]/g, ""));
              setError("");
            }}
          />

          {error ? (
            <Text className="text-red-500 mt-2 text-base">{error}</Text>
          ) : null}

          <TouchableOpacity
            className={`mt-6 rounded-xl py-4 items-center flex-row justify-center ${
              email.trim() && pin.length === 5 && !login.isPending
                ? "bg-primary-600"
                : "bg-primary-300"
            }`}
            onPress={handleLogin}
            disabled={!email.trim() || pin.length !== 5 || login.isPending}
          >
            {login.isPending ? (
              <ActivityIndicator color="white" />
            ) : (
              <Text className="text-white font-bold text-lg">Log In</Text>
            )}
          </TouchableOpacity>

          <TouchableOpacity
            className="mt-4 py-3 items-center"
            onPress={() => router.push("/(auth)/forgot-pin")}
          >
            <Text className="text-primary-600 font-medium text-lg">
              Forgot PIN?
            </Text>
          </TouchableOpacity>
        </View>

        <View className="flex-row items-center my-4">
          <View className="flex-1 h-px bg-gray-300" />
          <Text className="mx-4 text-gray-500 font-medium text-base">or</Text>
          <View className="flex-1 h-px bg-gray-300" />
        </View>

        <View className="w-full">
          <TouchableOpacity
            className="rounded-lg py-4 items-center bg-gray-100 border border-gray-300"
            onPress={() => router.push("/(auth)/signup")}
          >
            <Text className="text-gray-900 font-bold text-lg">
              Sign Up
            </Text>
            <Text className="text-gray-500 text-base mt-1">
              Sign up with an invited email
            </Text>
          </TouchableOpacity>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
