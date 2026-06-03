import { View, Text, ScrollView, TouchableOpacity } from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuth } from "@/src/providers/auth-provider";

export default function SettingsScreen() {
  const router = useRouter();
  const { user, signOut } = useAuth();

  const phoneDisplay = user?.phone_number || "Not set";
  const phoneVerified = user?.phone_verified ?? false;
  const termsAccepted = user?.is_terms_accepted ?? false;

  return (
    <View className="flex-1 bg-primary-50">
      <View className="flex-row items-center px-6 py-4">
        <TouchableOpacity onPress={() => router.back()} className="mr-3 p-1">
          <Ionicons name="arrow-back" size={24} color="#4C4A6E" />
        </TouchableOpacity>
        <Text className="text-2xl font-bold text-gray-900">Settings</Text>
      </View>

      <ScrollView className="flex-1 px-6">
        <View className="bg-white rounded-xl p-5 shadow-sm mb-4">
          <Text className="text-lg font-bold text-gray-900 mb-4">
            Phone Number
          </Text>
          <View className="flex-row justify-between items-center mb-3">
            <Text className="text-base text-gray-700">{phoneDisplay}</Text>
            {user?.phone_number && (
              <View className={`rounded-full px-2 py-0.5 ${phoneVerified ? "bg-green-100" : "bg-yellow-100"}`}>
                <Text className={`text-xs font-semibold ${phoneVerified ? "text-green-700" : "text-yellow-700"}`}>
                  {phoneVerified ? "Verified" : "Pending"}
                </Text>
              </View>
            )}
          </View>
          <TouchableOpacity
            className="bg-primary-600 rounded-lg py-3 items-center mt-2"
            onPress={() => router.push("/(app)/settings/phone" as any)}
          >
            <Text className="text-white font-semibold">
              {user?.phone_number ? "Change Phone Number" : "Add Phone Number"}
            </Text>
          </TouchableOpacity>
        </View>

        <View className="bg-white rounded-xl p-5 shadow-sm mb-4">
          <Text className="text-lg font-bold text-gray-900 mb-4">
            Security
          </Text>
          <TouchableOpacity
            className="flex-row justify-between items-center py-3"
            onPress={() => router.push("/(app)/settings/change-pin" as any)}
          >
            <Text className="text-base text-gray-700">Change PIN</Text>
            <Ionicons name="chevron-forward" size={20} color="#9ca3af" />
          </TouchableOpacity>
        </View>

        <View className="bg-white rounded-xl p-5 shadow-sm mb-4">
          <Text className="text-lg font-bold text-gray-900 mb-4">
            Legal
          </Text>
          <TouchableOpacity
            className="flex-row justify-between items-center py-3"
            onPress={() => router.push("/(app)/terms" as any)}
          >
            <Text className="text-base text-gray-700">
              Terms & Conditions {termsAccepted ? "(Accepted)" : ""}
            </Text>
            <Ionicons name="chevron-forward" size={20} color="#9ca3af" />
          </TouchableOpacity>
        </View>

        <TouchableOpacity
          className="bg-white rounded-xl p-5 shadow-sm mb-10 flex-row items-center"
          onPress={signOut}
        >
          <Ionicons name="log-out-outline" size={20} color="#dc2626" />
          <Text className="text-red-600 font-semibold ml-2">Sign Out</Text>
        </TouchableOpacity>
      </ScrollView>
    </View>
  );
}
