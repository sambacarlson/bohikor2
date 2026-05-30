import { useState } from "react";
import { View, Text, ScrollView, TouchableOpacity } from "react-native";
import { useRouter } from "expo-router";
import { useAcceptTerms } from "@/src/hooks/use-advance";
import { useAuth } from "@/src/providers/auth-provider";

const TERMS_TEXT = `By requesting a salary advance of 10,000 XAF, you agree to the following terms:

1. This is a one-time pilot advance.
2. The advance amount (10,000 XAF) plus any applicable charges will be deducted from your upcoming salary payment.
3. You may only have one active advance request at a time.
4. The advance is sent via mobile money to the phone number you provide.
5. By accepting, you authorize the deduction from your salary.

Please read these terms carefully before proceeding.`;

export default function TermsScreen() {
  const [accepted, setAccepted] = useState(false);
  const router = useRouter();
  const { user, refreshUser } = useAuth();
  const acceptTerms = useAcceptTerms();

  const handleAccept = async () => {
    try {
      await acceptTerms.mutateAsync({ version: "v1" });
      await refreshUser();
      router.back();
    } catch {
      // Error is handled by the mutation state
    }
  };

  if (user?.is_terms_accepted) {
    return (
      <View className="flex-1 bg-primary-50 items-center justify-center px-6">
        <Text className="text-xl font-bold text-gray-900 mb-2">
          Terms Already Accepted
        </Text>
        <Text className="text-base text-gray-500 text-center mb-6">
          You have already accepted the terms and conditions.
        </Text>
        <TouchableOpacity
          className="bg-primary-600 rounded-xl px-6 py-3"
          onPress={() => router.back()}
        >
          <Text className="text-white text-lg font-semibold">Go Back</Text>
        </TouchableOpacity>
      </View>
    );
  }

  return (
    <ScrollView className="flex-1 bg-primary-50">
      <View className="px-6 py-4">
        <Text className="text-3xl font-bold text-gray-900">Terms & Conditions</Text>
      </View>

      <View className="px-6 mb-6">
        <View className="bg-white rounded-xl p-6 shadow-sm">
          <Text className="text-base text-gray-700 leading-relaxed">{TERMS_TEXT}</Text>
        </View>
      </View>

      <View className="px-6 mb-6">
        <TouchableOpacity
          className={`flex-row items-center mb-6 ${accepted ? "opacity-100" : "opacity-80"}`}
          onPress={() => setAccepted(!accepted)}
          testID="terms-checkbox"
        >
          <View
            className={`w-6 h-6 rounded border-2 mr-3 items-center justify-center ${
              accepted ? "bg-primary-600 border-primary-600" : "border-gray-300"
            }`}
          >
            {accepted && <Text className="text-white text-sm">✓</Text>}
          </View>
          <Text className="text-base text-gray-700">
            I have read and accept the terms and conditions
          </Text>
        </TouchableOpacity>

        <TouchableOpacity
          className={`rounded-xl py-4 items-center ${
            accepted && !acceptTerms.isPending ? "bg-primary-600" : "bg-gray-300"
          }`}
          onPress={handleAccept}
          disabled={!accepted || acceptTerms.isPending}
          testID="accept-terms-button"
        >
          <Text className="text-white text-lg font-semibold">
            {acceptTerms.isPending ? "Accepting..." : "Accept Terms"}
          </Text>
        </TouchableOpacity>

        {acceptTerms.isError && (
          <Text className="text-red-500 text-base text-center mt-4">
            Failed to accept terms. Please try again.
          </Text>
        )}
      </View>
    </ScrollView>
  );
}
