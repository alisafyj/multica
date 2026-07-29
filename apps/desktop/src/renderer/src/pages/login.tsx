import { useEffect, useState } from "react";
import { Button } from "@multica/ui/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@multica/ui/components/ui/card";
import { LogIn, Loader2 } from "lucide-react";

export function DesktopLoginPage() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => window.desktopAPI.onAuthError(setError), []);

  const signIn = async () => {
    setLoading(true);
    setError("");
    try {
      await window.desktopAPI.startSSO();
    } catch (err) {
      setError(err instanceof Error ? err.message : "SSO sign-in failed");
      setLoading(false);
    }
  };

  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle>Multica</CardTitle>
          <CardDescription>{error || "Sign in with your company account"}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button className="w-full" onClick={signIn} disabled={loading}>
            {loading ? <Loader2 className="animate-spin" /> : <LogIn />}
            Continue with SSO
          </Button>
        </CardContent>
      </Card>
    </main>
  );
}
