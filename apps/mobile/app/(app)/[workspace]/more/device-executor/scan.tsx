/**
 * QR pairing for the device executor. The test host shows
 * `ws://<ip>:18800/phone?code=…` as a QR (hub CLI `pair`, or the runtime
 * page); scanning it fills the hub address and pairing code and returns to
 * the executor screen. Pasting the same URL there is the fallback, so the
 * camera permission is asked for here and nowhere else.
 */
import { useEffect, useState } from "react";
import { Linking, View } from "react-native";
import { router } from "expo-router";
import { CameraView, useCameraPermissions, type BarcodeScanningResult } from "expo-camera";
import { useTranslation } from "react-i18next";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { parseHubInput } from "@/data/device-executor/protocol";
import { useDeviceExecutorStore } from "@/data/device-executor/store";

export default function DeviceExecutorScanScreen() {
  const { t } = useTranslation("device-executor");
  const [permission, requestPermission] = useCameraPermissions();
  const saveConfig = useDeviceExecutorStore((s) => s.saveConfig);
  const [handled, setHandled] = useState(false);
  const [invalid, setInvalid] = useState(false);

  useEffect(() => {
    if (permission && !permission.granted && permission.canAskAgain) void requestPermission();
  }, [permission, requestPermission]);

  const onScanned = ({ data }: BarcodeScanningResult) => {
    if (handled) return;
    const target = parseHubInput(data);
    if (!target || !target.code) {
      setInvalid(true);
      return;
    }
    setHandled(true);
    void saveConfig({ hubUrl: target.url, code: target.code }).then(() => router.back());
  };

  if (!permission) return <View className="flex-1 bg-background" />;

  if (!permission.granted) {
    return (
      <View className="flex-1 bg-background items-center justify-center px-6 gap-4">
        <Text className="text-center text-base text-foreground">{t("scan.permission_message")}</Text>
        <Button onPress={() => (permission.canAskAgain ? void requestPermission() : void Linking.openSettings())}>
          <Text>{t("scan.permission_action")}</Text>
        </Button>
      </View>
    );
  }

  return (
    <View className="flex-1 bg-black">
      <CameraView
        style={{ flex: 1 }}
        facing="back"
        barcodeScannerSettings={{ barcodeTypes: ["qr"] }}
        onBarcodeScanned={handled ? undefined : onScanned}
      />
      <View className="absolute bottom-10 left-0 right-0 items-center px-6">
        <Text className="text-center text-sm text-white">{invalid ? t("scan.invalid") : t("scan.hint")}</Text>
      </View>
    </View>
  );
}
