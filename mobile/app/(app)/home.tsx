import { useState } from "react";
import { View, Text, ScrollView, TouchableOpacity, Modal, ActivityIndicator, Image } from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuth } from "@/src/providers/auth-provider";
import { useCreateAdvanceRequest } from "@/src/hooks/use-advance";
import { useEligibility } from "@/src/hooks/use-eligibility";

export default function HomeScreen() {
  const { user, signOut } = useAuth();
  const router = useRouter();
  const [modalVisible, setModalVisible] = useState(false);
  const [menuVisible, setMenuVisible] = useState(false);
  const createRequest = useCreateAdvanceRequest();
  const { data: eligibility, isLoading: eligibilityLoading, refetch: refetchEligibility, isRefetching: isRefetchingEligibility } = useEligibility();

  const displayName = user?.full_name || user?.email || "User";
  const email = user?.email || "";
  const phone = user?.phone_number || "";
  const emailVerified = user?.email_verified ?? false;
  const phoneVerified = user?.phone_verified ?? false;
  const termsAccepted = user?.is_terms_accepted ?? false;
  const profileIncomplete = !phoneVerified || !user?.phone_number;

  const advanceAmount = eligibility?.advance_amount_xaf || "10,000 XAF";
  const formattedAmount = advanceAmount.includes("XAF") ? advanceAmount : `${advanceAmount} XAF`;

  const handleRequestAdvance = () => {
    if (!termsAccepted) {
      router.push("/(app)/terms" as any);
      return;
    }
    if (profileIncomplete) {
      router.push("/(app)/settings/phone" as any);
      return;
    }
    setModalVisible(true);
  };

  const handleConfirmRequest = async () => {
    try {
      await createRequest.mutateAsync();
      setModalVisible(false);
      router.push("/(app)/history" as any);
    } catch {
      // Error is shown via mutation state
    }
  };

  return (
    <View className="flex-1 bg-primary-50">
      <View className="flex-row items-center justify-between px-6 py-4">
        <View className="flex-row items-center">
          <Image
            source={require("../../assets/logo.png")}
            className="w-7 h-7 mr-2"
            resizeMode="contain"
          />
          <Text className="text-2xl font-bold text-primary-700">Bohikor</Text>
        </View>
        <TouchableOpacity
          onPress={() => setMenuVisible(true)}
          testID="menu-button"
        >
          <Ionicons name="ellipsis-vertical" size={24} color="#4C4A6E" />
        </TouchableOpacity>
      </View>

      <ScrollView className="flex-1">
        <View className="px-6 pt-2 pb-4">
          <Text className="text-2xl font-bold text-gray-900">
            {displayName}
          </Text>
        </View>

        {!termsAccepted && (
          <View className="px-6 mb-4">
            <View className="bg-yellow-50 border border-yellow-200 rounded-xl p-5">
              <Text className="text-base text-yellow-800 font-medium mb-1">
                Terms not accepted
              </Text>
              <Text className="text-sm text-yellow-700">
                You must accept the terms before requesting an advance.
              </Text>
              <TouchableOpacity
                className="mt-3 bg-yellow-600 rounded-lg py-2 px-4 self-start"
                onPress={() => router.push("/(app)/terms" as any)}
                testID="accept-terms-link"
              >
                <Text className="text-white text-sm font-semibold">Accept Terms</Text>
              </TouchableOpacity>
            </View>
          </View>
        )}

        {profileIncomplete && termsAccepted && (
          <View className="px-6 mb-4">
            <View className="bg-yellow-50 border border-yellow-200 rounded-xl p-5">
              <Text className="text-base text-yellow-800 font-medium mb-1">
                Complete your profile
              </Text>
              <Text className="text-sm text-yellow-700">
                {!user?.phone_number
                  ? "Add a phone number to receive advances via mobile money."
                  : "Verify your phone number to request an advance."}
              </Text>
              <TouchableOpacity
                className="mt-3 bg-yellow-600 rounded-lg py-2 px-4 self-start"
                onPress={() => router.push("/(app)/settings/phone" as any)}
              >
                <Text className="text-white text-sm font-semibold">
                  {!user?.phone_number ? "Add Phone Number" : "Verify Phone"}
                </Text>
              </TouchableOpacity>
            </View>
          </View>
        )}

        <View className="px-6 mb-8">
          <TouchableOpacity
            className={`rounded-xl p-6 shadow-sm items-center ${
              termsAccepted && !profileIncomplete ? "bg-primary-600" : "bg-primary-300"
            }`}
            onPress={handleRequestAdvance}
            testID="request-advance-button"
          >
            <Text className="text-white text-xl font-bold">Request Advance</Text>
            <Text className="text-primary-200 text-base mt-1">
              {eligibilityLoading ? "10,000 XAF" : formattedAmount}
            </Text>
          </TouchableOpacity>
        </View>

        <View className="px-6 mb-6">
          <View className="bg-white rounded-xl p-5 shadow-sm">
            <View className="flex-row items-center justify-between mb-4">
              <Text className="text-xl font-bold text-gray-900">
                Eligibility
              </Text>
              <TouchableOpacity
                onPress={() => refetchEligibility()}
                disabled={isRefetchingEligibility}
              >
                {isRefetchingEligibility ? (
                  <ActivityIndicator size="small" color="#4C4A6E" />
                ) : (
                  <Ionicons name="refresh" size={20} color="#4C4A6E" />
                )}
              </TouchableOpacity>
            </View>

            {eligibilityLoading ? (
              <View className="py-4 items-center">
                <ActivityIndicator size="small" color="#4C4A6E" />
              </View>
            ) : eligibility ? (
              <View className="gap-4">
                <View className="flex-row justify-between items-center">
                  <Text className="text-base text-gray-500">Advance amount</Text>
                  <Text className="text-base font-semibold text-gray-900">
                    {formattedAmount}
                  </Text>
                </View>

                <View className="flex-row justify-between items-center">
                  <Text className="text-base text-gray-500">Status</Text>
                  {eligibility.eligible ? (
                    <View className="bg-green-100 rounded-full px-3 py-1">
                      <Text className="text-green-800 text-sm font-semibold">Eligible</Text>
                    </View>
                  ) : (
                    <View className="bg-red-100 rounded-full px-3 py-1">
                      <Text className="text-red-800 text-sm font-semibold">Not eligible</Text>
                    </View>
                  )}
                </View>

                <View className="flex-row justify-between items-center">
                  <Text className="text-base text-gray-500">Remaining</Text>
                  <Text className="text-base text-gray-900">
                    {eligibility.daily_requests_remaining} daily / {eligibility.monthly_requests_remaining} monthly
                  </Text>
                </View>

                <View className="flex-row justify-between items-center">
                  <Text className="text-base text-gray-500">Request window</Text>
                  <Text className="text-base text-gray-900">
                    Days {eligibility.request_window.start_day}-{eligibility.request_window.end_day}
                  </Text>
                </View>

                {eligibility.kill_switch_active && (
                  <View className="bg-red-50 border border-red-200 rounded-lg px-3 py-2">
                    <Text className="text-red-700 text-sm font-medium">
                      Advances are temporarily disabled
                    </Text>
                  </View>
                )}

                {!eligibility.eligible && eligibility.reasons.length > 0 && (
                  <View className="bg-gray-50 rounded-lg px-3 py-2">
                    <Text className="text-sm font-medium text-gray-700 mb-1">Reasons:</Text>
                    {eligibility.reasons.map((reason, i) => (
                      <Text key={i} className="text-sm text-red-600 ml-1">
                        • {reason}
                      </Text>
                    ))}
                  </View>
                )}
              </View>
            ) : null}
          </View>
        </View>

        <View className="px-6 mb-6">
          <View className="bg-white rounded-xl p-5 shadow-sm">
            <Text className="text-xl font-bold text-gray-900 mb-5">
              Your Information
            </Text>
            <View className="gap-4">
              <View className="flex-row justify-between items-center">
                <Text className="text-base text-gray-500">Email</Text>
                <View className="flex-row items-center">
                  <Text className="text-base text-gray-900">{email}</Text>
                  <Text className="text-base ml-2">
                    {emailVerified ? "✓" : "—"}
                  </Text>
                </View>
              </View>
              <View className="flex-row justify-between items-center">
                <Text className="text-base text-gray-500">Phone</Text>
                <View className="flex-row items-center">
                  <Text className="text-base text-gray-900">{phone || "Not set"}</Text>
                  <Text className="text-base ml-2">
                    {phone && phoneVerified ? "✓" : "—"}
                  </Text>
                </View>
              </View>
              {user?.full_name && (
                <View className="flex-row justify-between items-center">
                  <Text className="text-base text-gray-500">Name</Text>
                  <Text className="text-base text-gray-900">
                    {user.full_name}
                  </Text>
                </View>
              )}
              <View className="flex-row justify-between items-center">
                <Text className="text-base text-gray-500">Status</Text>
                <Text className="text-base text-gray-900">
                  {user?.status || "—"}
                </Text>
              </View>
              <View className="flex-row justify-between items-center">
                <Text className="text-base text-gray-500">Terms</Text>
                <Text className="text-base text-gray-900">
                  {termsAccepted ? "Accepted" : "Not accepted"}
                </Text>
              </View>
            </View>
          </View>
        </View>

        <View className="px-6 mb-6">
          <TouchableOpacity
            className="bg-white rounded-xl p-5 shadow-sm flex-row items-center justify-between"
            onPress={() => router.push("/(app)/settings" as any)}
          >
            <View className="flex-row items-center">
              <Ionicons name="settings-outline" size={22} color="#4C4A6E" />
              <Text className="text-gray-900 font-semibold text-lg ml-3">Settings</Text>
            </View>
            <Ionicons name="chevron-forward" size={22} color="#9ca3af" />
          </TouchableOpacity>
        </View>

        <View className="px-6 mb-10">
          <TouchableOpacity
            className="bg-white rounded-xl p-5 shadow-sm flex-row items-center justify-between"
            onPress={() => router.push("/(app)/history" as any)}
            testID="view-history-link"
          >
            <Text className="text-gray-900 font-semibold text-lg">
              View Transaction History
            </Text>
            <Ionicons name="chevron-forward" size={22} color="#9ca3af" />
          </TouchableOpacity>
        </View>
      </ScrollView>

      {menuVisible && (
        <TouchableOpacity
          className="absolute top-0 left-0 right-0 bottom-0 z-10"
          activeOpacity={1}
          onPress={() => setMenuVisible(false)}
        >
          <View className="absolute top-24 right-6 bg-white rounded-xl shadow-lg border border-primary-200 py-2 w-48">
            <TouchableOpacity
              className="px-4 py-3 flex-row items-center"
              onPress={() => {
                setMenuVisible(false);
                router.push("/(app)/settings" as any);
              }}
            >
              <Ionicons name="settings-outline" size={18} color="#4C4A6E" />
              <Text className="text-gray-700 font-semibold ml-2">Settings</Text>
            </TouchableOpacity>
            <TouchableOpacity
              className="px-4 py-3 flex-row items-center"
              onPress={() => {
                setMenuVisible(false);
                signOut();
              }}
              testID="signout-menu-item"
            >
              <Ionicons name="log-out-outline" size={18} color="#dc2626" />
              <Text className="text-red-600 font-semibold ml-2">Sign Out</Text>
            </TouchableOpacity>
          </View>
        </TouchableOpacity>
      )}

      <Modal
        visible={modalVisible}
        transparent
        animationType="fade"
        onRequestClose={() => setModalVisible(false)}
      >
        <View className="flex-1 bg-black/50 justify-center items-center px-6">
          <View className="bg-white rounded-xl p-6 w-full max-w-sm">
            <Text className="text-xl font-bold text-gray-900 mb-2">
              Confirm Advance Request
            </Text>
            <Text className="text-base text-gray-600 mb-4">
              You are about to request a salary advance of{" "}
              <Text className="font-semibold text-gray-900">{formattedAmount}</Text>.
              This amount plus any applicable charges will be deducted from your
              upcoming salary.
            </Text>

            {createRequest.isError && (
              <Text className="text-red-500 text-sm mb-4">
                {(createRequest.error as any)?.response?.data?.error ||
                  "Failed to create request. Please try again."}
              </Text>
            )}

            <View className="flex-row justify-end gap-3">
              <TouchableOpacity
                className="px-4 py-2 rounded-lg"
                onPress={() => setModalVisible(false)}
                disabled={createRequest.isPending}
                testID="cancel-request-button"
              >
                <Text className="text-gray-600 font-semibold">Cancel</Text>
              </TouchableOpacity>
              <TouchableOpacity
                className="bg-primary-600 px-4 py-2 rounded-lg flex-row items-center"
                onPress={handleConfirmRequest}
                disabled={createRequest.isPending}
                testID="confirm-request-button"
              >
                {createRequest.isPending && (
                  <ActivityIndicator color="white" size="small" className="mr-2" />
                )}
                <Text className="text-white font-semibold">
                  {createRequest.isPending ? "Requesting..." : "Confirm"}
                </Text>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>
    </View>
  );
}
