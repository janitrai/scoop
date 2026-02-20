import { useState, type FormEvent } from "react";

interface LoginScreenProps {
  isSubmitting: boolean;
  error: string;
  onSubmit: (username: string, password: string) => Promise<void>;
}

export function LoginScreen({ isSubmitting, error, onSubmit }: LoginScreenProps): JSX.Element {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("changeme123");

  async function handleSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    await onSubmit(username, password);
  }

  return (
    <main className="auth-screen">
      <section className="auth-card card">
        <h1 className="auth-title">SCOOP</h1>
        <p className="auth-subtitle">Sign in to access stories, translations, and settings.</p>

        {error ? <p className="banner-error auth-error">{error}</p> : null}

        <form className="auth-form" onSubmit={(event) => void handleSubmit(event)}>
          <label className="field">
            <span>Username</span>
            <input
              type="text"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              disabled={isSubmitting}
            />
          </label>

          <label className="field">
            <span>Password</span>
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={isSubmitting}
            />
          </label>

          <button className="btn" type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Signing in..." : "Sign in"}
          </button>
        </form>
      </section>
    </main>
  );
}
