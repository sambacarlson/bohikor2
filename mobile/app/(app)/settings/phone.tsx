import { useState, useEffect, useRef } from "react";
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
import { Ionicons } from "@expo/vector-icons";
import { useAddPhone, usePhoneVerificationStatus } from "@/src/hooks/use-auth";
import { useAuth } from "@/src/providers/auth-provider";

export default function PhoneScreen() {
  const router = useRouter();
  const { refreshUser } = useAuth();

  const [countryCode, setCountryCode] = useState("+237");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [error, setError] = useState("");
  const [submitted, setSubmitted] = useState(false);

  const prevPhoneVerified = useRef(false);

  const addPhone = useAddPhone();
  const { data: verifStatus, isLoading: verifLoading } = usePhoneVerificationStatus();

  useEffect(() => {
    if (verifStatus?.phone_verified && !prevPhoneVerified.current) {
      prevPhoneVerified.current = true;
      refreshUser();
    }
  }, [verifStatus?.phone_verified, refreshUser]);

  const fullPhone = `${countryCode}${phoneNumber}`;
  const isValidPhone = (phone: string) => /^\+[1-9]\d{6,14}$/.test(phone);

  const handleSubmit = async () => {
    setError("");
    if (!phoneNumber.trim()) {
      setError("Phone number is required");
      return;
    }
    if (!isValidPhone(fullPhone)) {
      setError("Enter a valid phone number (e.g., +237 6XXXXXXXX)");
      return;
    }

    try {
      await addPhone.mutateAsync(fullPhone);
      await refreshUser();
      setSubmitted(true);
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
        setError(data?.error || "Failed to add phone number.");
      } else {
        setError("Network error. Please try again.");
      }
    }
  };

  const verification = verifStatus?.verification;
  const verifStatusText = verification
    ? verification.status === "success"
      ? "Verified"
      : verification.status === "failed"
        ? "Failed"
        : verification.status === "pending"
          ? "Processing..."
          : "Initiated"
    : null;

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      className="flex-1 bg-primary-50"
    >
      <View className="flex-row items-center px-6 py-4">
        <TouchableOpacity onPress={() => router.back()} className="mr-3 p-1">
          <Ionicons name="arrow-back" size={24} color="#4C4A6E" />
        </TouchableOpacity>
        <Text className="text-2xl font-bold text-gray-900">Phone Number</Text>
      </View>

      <ScrollView className="flex-1 px-6">
        {!submitted && !verifStatus?.phone_number ? (
          <View className="bg-white rounded-xl p-5 shadow-sm">
            <Text className="text-base text-gray-700 mb-4">
              Add your phone number to receive salary advances via mobile money.
              A small verification amount will be sent to confirm your number.
            </Text>

            <Text className="text-gray-700 mb-2 font-medium text-base">
              Phone number
            </Text>
            <View className="flex-row gap-3 mb-4">
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
              <Text className="text-red-500 mb-3 text-base">{error}</Text>
            ) : null}

            <TouchableOpacity
              className={`rounded-xl py-4 items-center ${
                phoneNumber.trim() && !addPhone.isPending
                  ? "bg-primary-600"
                  : "bg-primary-300"
              }`}
              onPress={handleSubmit}
              disabled={!phoneNumber.trim() || addPhone.isPending}
            >
              {addPhone.isPending ? (
                <ActivityIndicator color="white" />
              ) : (
                <Text className="text-white font-bold text-lg">
                  Verify Phone Number
                </Text>
              )}
            </TouchableOpacity>
          </View>
        ) : (
          <View className="bg-white rounded-xl p-5 shadow-sm">
            <Text className="text-lg font-bold text-gray-900 mb-3">
              Verification Status
            </Text>

            <View className="flex-row justify-between items-center mb-3">
              <Text className="text-base text-gray-700">Phone</Text>
              <Text className="text-base text-gray-900">
                {verifStatus?.phone_number || fullPhone}
              </Text>
            </View>

            <View className="flex-row justify-between items-center mb-3">
              <Text className="text-base text-gray-700">Verified</Text>
              <Text className="text-base text-gray-900">
                {verifStatus?.phone_verified ? "Yes" : "No"}
              </Text>
            </View>

            {verifStatusText && (
              <View className="flex-row justify-between items-center mb-3">
                <Text className="text-base text-gray-700">Transfer</Text>
                <View className={`rounded-full px-2 py-0.5 ${
                  verification?.status === "success"
                    ? "bg-green-100"
                    : verification?.status === "failed"
                      ? "bg-red-100"
                      : "bg-yellow-100"
                }`}>
                  <Text className={`text-xs font-semibold ${
                    verification?.status === "success"
                      ? "text-green-700"
                      : verification?.status === "failed"
                        ? "text-red-700"
                        : "text-yellow-700"
                  }`}>
                    {verifStatusText}
                  </Text>
                </View>
              </View>
            )}

            {verifLoading && (
              <ActivityIndicator className="mt-4" color="#7C3AED" />
            )}

            {!verifStatus?.phone_verified && verification?.status !== "pending" && verification?.status !== "initiated" && (
              <TouchableOpacity
                className="bg-primary-600 rounded-lg py-3 items-center mt-4"
                onPress={() => {
                  setSubmitted(false);
                  setPhoneNumber("");
                }}
              >
                <Text className="text-white font-semibold">Try Again</Text>
              </TouchableOpacity>
            )}
          </View>
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
