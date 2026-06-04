import { View, Text, TouchableOpacity } from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

export default function ChangePinScreen() {
  const router = useRouter();

  return (
    <View className="flex-1 bg-primary-50">
      <View className="flex-row items-center px-6 py-4">
        <TouchableOpacity onPress={() => router.back()} className="mr-3 p-1">
          <Ionicons name="arrow-back" size={24} color="#4C4A6E" />
        </TouchableOpacity>
        <Text className="text-2xl font-bold text-gray-900">Change PIN</Text>
      </View>

      <View className="flex-1 items-center justify-center px-6">
        <View className="bg-white rounded-xl p-8 shadow-sm items-center w-full max-w-sm">
          <Ionicons name="construct-outline" size={48} color="#9ca3af" />
          <Text className="text-xl font-bold text-gray-900 mt-4">
            Under Maintenance
          </Text>
          <Text className="text-base text-gray-500 text-center mt-2">
            The ability to change your PIN is currently under maintenance. Please check back later.
          </Text>
          <TouchableOpacity
            className="bg-primary-600 rounded-lg py-3 px-8 mt-6"
            onPress={() => router.back()}
          >
            <Text className="text-white font-semibold">Go Back</Text>
          </TouchableOpacity>
        </View>
      </View>
    </View>
  );
}
