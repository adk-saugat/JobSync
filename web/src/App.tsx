import { FormEvent, useEffect, useState } from "react";
import { completeSetup, fetchSession, startGoogleSignIn, type SetupCompleteResponse } from "./api";

type Step = "loading" | "signin" | "gemini" | "done" | "error";

export default function App() {
  const [step, setStep] = useState<Step>("loading");
  const [email, setEmail] = useState("");
  const [geminiKey, setGeminiKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<SetupCompleteResponse | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const oauthError = params.get("error");
    if (oauthError) {
      setError(oauthError);
      setStep("error");
      window.history.replaceState({}, "", "/");
      return;
    }

    fetchSession()
      .then((session) => {
        if (session.signed_in) {
          setEmail(session.email || "");
          setStep("gemini");
        } else {
          setStep("signin");
        }
      })
      .catch(() => setStep("signin"));
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const out = await completeSetup(geminiKey);
      setResult(out);
      setStep("done");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Setup failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="shell">
      <h1 className="brand">JobSync</h1>
      <p className="lede">
        Turn job emails into a living Google Sheet. Sign in once, paste a free Gemini key, and daily
        sync runs in the cloud.
      </p>

      <div className="flow" aria-hidden="true">
        <span className={step !== "loading" ? "on" : ""}>1 · Google</span>
        <span>·</span>
        <span className={step === "gemini" || step === "done" ? "on" : ""}>2 · Gemini</span>
        <span>·</span>
        <span className={step === "done" ? "on" : ""}>3 · Done</span>
      </div>

      {step === "loading" && (
        <section className="panel">
          <p className="muted">Checking your session…</p>
        </section>
      )}

      {step === "signin" && (
        <section className="panel">
          <div className="step-label">Step 1</div>
          <h2>Connect Gmail & Sheets</h2>
          <p>
            JobSync only reads job-related mail and writes to a tracker spreadsheet it creates for
            you.
          </p>
          {error && <div className="error">{error}</div>}
          <div className="actions">
            <button type="button" className="btn btn-primary" onClick={startGoogleSignIn}>
              Continue with Google
            </button>
          </div>
        </section>
      )}

      {step === "gemini" && (
        <section className="panel">
          <div className="step-label">Step 2</div>
          <h2>Add your Gemini key</h2>
          <p>
            Signed in as <strong>{email || "your Google account"}</strong>. Get a free key from{" "}
            <a href="https://aistudio.google.com/apikey" target="_blank" rel="noreferrer">
              Google AI Studio
            </a>
            , then paste it below.
          </p>
          {error && <div className="error">{error}</div>}
          <form onSubmit={onSubmit}>
            <div className="field">
              <label htmlFor="gemini">Gemini API key</label>
              <input
                id="gemini"
                name="gemini"
                type="password"
                autoComplete="off"
                placeholder="AIza…"
                value={geminiKey}
                onChange={(e) => setGeminiKey(e.target.value)}
                required
              />
            </div>
            <div className="actions">
              <button type="submit" className="btn btn-primary" disabled={busy || !geminiKey.trim()}>
                {busy ? "Setting up…" : "Create sheet & turn on daily sync"}
              </button>
            </div>
          </form>
        </section>
      )}

      {step === "done" && result && (
        <section className="panel success-card">
          <div className="step-label">You’re set</div>
          <h2>Daily sync is on</h2>
          <p>
            JobSync will update your sheet automatically each day. You can also keep using the CLI
            anytime.
          </p>
          <a className="sheet-link" href={result.spreadsheet_url} target="_blank" rel="noreferrer">
            Open your JobSync Tracker →
          </a>
          <p className="muted">Account: {result.account_id}</p>
        </section>
      )}

      {step === "error" && (
        <section className="panel">
          <div className="step-label">Something went wrong</div>
          <h2>Couldn’t finish Google sign-in</h2>
          {error && <div className="error">{error}</div>}
          <div className="actions">
            <button type="button" className="btn btn-primary" onClick={startGoogleSignIn}>
              Try again
            </button>
          </div>
        </section>
      )}
    </main>
  );
}
