import { useState } from "react";
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
import { useCreatePin } from "@/src/hooks/use-auth";
import { setTokens } from "@/src/lib/auth";
import { useAuth } from "@/src/providers/auth-provider";

export default function CreatePinScreen() {
  const router = useRouter();
  const { email } = useLocalSearchParams<{ email: string }>();
  const { refreshUser } = useAuth();

  const [pin, setPin] = useState("");
  const [confirmPin, setConfirmPin] = useState("");
  const [error, setError] = useState("");

  const createPin = useCreatePin();

  const handleCreatePin = async () => {
    setError("");
    if (pin.length !== 5) {
      setError("PIN must be 5 digits");
      return;
    }
    if (pin !== confirmPin) {
      setError("PINs do not match");
      return;
    }

    try {
      const result = await createPin.mutateAsync({ email, pin });
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
        setError(data?.error || "Failed to create PIN. Please try again.");
      } else {
        setError("Network error. Please try again.");
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
            Create your PIN
          </Text>
          <Text className="text-gray-500 mt-2 text-center text-lg">
            Choose a 5-digit PIN to secure your account
          </Text>
        </View>

        <View className="w-full">
          <Text className="text-gray-700 mb-2 font-medium text-base">
            Enter PIN
          </Text>
          <TextInput
            className="border border-gray-300 rounded-lg px-4 py-4 text-lg text-center tracking-widest"
            placeholder="•••••"
            keyboardType="number-pad"
            maxLength={5}
            secureTextEntry
            value={pin}
            onChangeText={(text) => {
              setPin(text.replace(/[^0-9]/g, ""));
              setError("");
            }}
            autoFocus
          />

          <Text className="text-gray-700 mb-2 font-medium text-base mt-4">
            Confirm PIN
          </Text>
          <TextInput
            className="border border-gray-300 rounded-lg px-4 py-4 text-lg text-center tracking-widest"
            placeholder="•••••"
            keyboardType="number-pad"
            maxLength={5}
            secureTextEntry
            value={confirmPin}
            onChangeText={(text) => {
              setConfirmPin(text.replace(/[^0-9]/g, ""));
              setError("");
            }}
          />

          {error ? (
            <Text className="text-red-500 mt-2 text-base">{error}</Text>
          ) : null}

          <TouchableOpacity
            className={`mt-6 rounded-xl py-4 items-center flex-row justify-center ${
              pin.length === 5 && confirmPin.length === 5 && !createPin.isPending
                ? "bg-primary-600"
                : "bg-primary-300"
            }`}
            onPress={handleCreatePin}
            disabled={pin.length !== 5 || confirmPin.length !== 5 || createPin.isPending}
          >
            {createPin.isPending ? (
              <ActivityIndicator color="white" />
            ) : (
              <Text className="text-white font-bold text-lg">Create PIN</Text>
            )}
          </TouchableOpacity>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
