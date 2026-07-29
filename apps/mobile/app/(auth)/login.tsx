import { useEffect, useState } from "react";
import { View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router } from "expo-router";
import {
  makeRedirectUri,
  ResponseType,
  useAuthRequest,
} from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import * as Haptics from "expo-haptics";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { MulticaLogo } from "@/components/brand/multica-logo";
import { useAuthStore } from "@/data/auth-store";
import { mapAuthError } from "@/lib/auth-error";

WebBrowser.maybeCompleteAuthSession();

const API_URL = process.env.EXPO_PUBLIC_API_URL?.replace(/\/+$/, "");
if (!API_URL) throw new Error("EXPO_PUBLIC_API_URL is not set");

const redirectUri = makeRedirectUri({
  native: "multica://auth/mobile-callback",
  scheme: "multica",
  path: "auth/mobile-callback",
});

export default function Login() {
  const loginWithSSO = useAuthStore((state) => state.loginWithSSO);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [request, response, promptAsync] = useAuthRequest(
    {
      clientId: "mobile",
      redirectUri,
      responseType: ResponseType.Code,
      usePKCE: true,
    },
    {
      authorizationEndpoint: `${API_URL}/auth/sso/authorize`,
      tokenEndpoint: `${API_URL}/auth/sso/token`,
    },
  );

  useEffect(() => {
    if (!response) return;
    if (response.type !== "success") {
      if (response.type !== "dismiss" && response.type !== "cancel") {
        setError("SSO sign-in was not completed");
      }
      setSubmitting(false);
      return;
    }
    const code = response.params.code;
    const verifier = request?.codeVerifier;
    if (!code || !verifier) {
      setError("SSO callback was incomplete");
      setSubmitting(false);
      return;
    }
    void loginWithSSO(code, verifier, redirectUri)
      .then(() => {
        void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success);
        router.replace("/");
      })
      .catch((err) => {
        void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error);
        setError(mapAuthError(err, "Couldn't complete SSO sign-in."));
        setSubmitting(false);
      });
  }, [loginWithSSO, request, response]);

  const signIn = () => {
    setError(null);
    setSubmitting(true);
    void Haptics.selectionAsync();
    void promptAsync().catch((err) => {
      setError(mapAuthError(err, "Couldn't open SSO sign-in."));
      setSubmitting(false);
    });
  };

  return (
    <SafeAreaView className="flex-1 bg-background">
      <View className="flex-1 justify-center px-6 gap-6">
        <View className="items-center gap-3">
          <MulticaLogo size={32} />
          <View className="gap-1 items-center">
            <Text className="text-2xl font-semibold text-foreground">Multica</Text>
            <Text className="text-sm text-muted-foreground text-center">
              Sign in with your company account
            </Text>
          </View>
        </View>
        {error ? <Text className="text-sm text-destructive text-center">{error}</Text> : null}
        <Button size="lg" disabled={!request || submitting} onPress={signIn}>
          <Text>{submitting ? "Signing in..." : "Continue with SSO"}</Text>
        </Button>
      </View>
    </SafeAreaView>
  );
}
