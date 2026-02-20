import { useState, type ReactNode } from "react";

import { useAuth } from "../auth";
import { LoginScreen } from "./LoginScreen";

interface AuthGateProps {
  children: ReactNode;
}

export function AuthGate({ children }: AuthGateProps): JSX.Element {
  const { status, login } = useAuth();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState("");

  if (status === "loading") {
    return (
      <main className="auth-screen">
        <section className="auth-card card">
          <p className="muted">Checking session...</p>
        </section>
      </main>
    );
  }

  if (status === "unauthenticated") {
    return (
      <LoginScreen
        isSubmitting={isSubmitting}
        error={error}
        onSubmit={async (username, password) => {
          setError("");
          setIsSubmitting(true);
          try {
            await login(username, password);
          } catch (loginErr) {
            const message = loginErr instanceof Error ? loginErr.message : "Failed to sign in";
            setError(message);
          } finally {
            setIsSubmitting(false);
          }
        }}
      />
    );
  }

  return <>{children}</>;
}
