import { FormEvent, useEffect, useState } from "react";
import {
  completeSetup,
  fetchSession,
  startGoogleSignIn,
  type SetupCompleteResponse,
  type SetupSession,
} from "./api";

type Step = "loading" | "signin" | "existing" | "gemini" | "done";

const GITHUB = "https://github.com/adk-saugat/JobSync";
const PRIVACY = `${GITHUB}/blob/main/docs/PRIVACY.md`;
const GEMINI_KEYS = "https://aistudio.google.com/apikey";

const SAMPLE_ROWS = [
  { company: "Mastercard", role: "Software Engineer Intern", status: "applied" },
  { company: "IBM", role: "Back End Developer Intern", status: "rejected" },
  { company: "Intuit", role: "Full Stack Intern", status: "assessment" },
  { company: "TikTok", role: "Software Engineer Intern", status: "interview" },
];

const SCAN_STAGES = [
  { until: 18, label: "Creating your Google Sheet…" },
  { until: 42, label: "Searching Gmail for job emails…" },
  { until: 78, label: "Reading status with Gemini…" },
  { until: 100, label: "Writing rows to your tracker…" },
] as const;

function scanStageLabel(pct: number) {
  return SCAN_STAGES.find((s) => pct < s.until)?.label ?? SCAN_STAGES[SCAN_STAGES.length - 1].label;
}

function FirstScanProgress() {
  const [pct, setPct] = useState(6);

  useEffect(() => {
    const started = performance.now();
    const cap = 92;
    const riseMs = 70_000;
    const id = window.setInterval(() => {
      const elapsed = performance.now() - started;
      setPct(Math.max(6, cap * (1 - Math.exp(-elapsed / riseMs))));
    }, 120);
    return () => window.clearInterval(id);
  }, []);

  const label = scanStageLabel(pct);
  return (
    <div className="scan-progress" role="status" aria-live="polite">
      <div
        className="scan-progress-track"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(pct)}
        aria-label={label}
      >
        <div className="scan-progress-fill" style={{ width: `${pct}%` }} />
      </div>
      <p className="scan-progress-label">{label}</p>
      <p className="scan-progress-hint">First scan can take a few minutes. Keep this tab open.</p>
    </div>
  );
}

function formatNextRun(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  return d.toLocaleString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    timeZoneName: "short",
  });
}

function oauthErrorFromUrl() {
  const raw = new URLSearchParams(window.location.search).get("error") || "";
  if (raw === "access_denied") {
    return "You canceled sign-in.";
  }
  return raw;
}

export default function App() {
  const [error, setError] = useState(oauthErrorFromUrl);
  const [step, setStep] = useState<Step>("loading");
  const [email, setEmail] = useState("");
  const [geminiKey, setGeminiKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<SetupCompleteResponse | null>(null);
  const [existing, setExisting] = useState<SetupSession | null>(null);

  useEffect(() => {
    fetchSession()
      .then((session) => {
        if (session.signed_in) {
          setEmail(session.email || "");
          setError("");
          if (session.registered && session.spreadsheet_url) {
            setExisting(session);
            setStep("existing");
          } else {
            setStep("gemini");
          }
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
    <div className="site">
      <header className="nav">
        <a className="logo" href="/">
          JobSync
        </a>
        <nav>
          <a href="#how">How it works</a>
          <a href="#privacy">Privacy</a>
          <a className="nav-cta" href="#setup">
            Get started
          </a>
        </nav>
      </header>

      <main>
        <section className="hero">
          <div className="hero-copy">
            <p className="eyebrow">Gmail → Gemini → Google Sheet</p>
            <h1>Your applications, updated while you sleep.</h1>
            <p className="lede">
              JobSync reads job emails, figures out the status, and writes them to a tracker
              spreadsheet you own. Set it up once. It runs every night in the cloud.
            </p>
            <ul className="bullets">
              <li>Read-only Gmail. Never sends or deletes mail.</li>
              <li>Creates a JobSync Tracker sheet in your Drive.</li>
              <li>Applied, assessment, interview, rejected, offer.</li>
            </ul>
          </div>

          <aside className="hero-visual" aria-label="Example tracker">
            <div className="sheet-card">
              <div className="sheet-bar">
                <span>JobSync Tracker</span>
                <span className="live">Updates daily</span>
              </div>
              <table>
                <thead>
                  <tr>
                    <th>Company</th>
                    <th>Role</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {SAMPLE_ROWS.map((row) => (
                    <tr key={row.company}>
                      <td>{row.company}</td>
                      <td>{row.role}</td>
                      <td>
                        <span className={`pill pill-${row.status}`}>{row.status}</span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </aside>
        </section>

        <section id="setup" className="setup-band">
          <div className="setup-intro">
            <h2>Get started in two steps</h2>
            <p>Sign in with Google, paste a free Gemini key, and daily sync turns on.</p>
          </div>
          <div className="setup-card">
            {step === "loading" && <p className="muted">Checking your session…</p>}

            {step === "signin" && (
              <>
                <p className="step">Step 1 of 2</p>
                <h3>Connect Gmail & Sheets</h3>
                <p>
                  JobSync asks for Gmail read access and permission to create and update your
                  tracker spreadsheet.
                </p>
                {error && <p className="error">{error}</p>}
                <button type="button" onClick={startGoogleSignIn}>
                  Continue with Google
                </button>
              </>
            )}

            {step === "existing" && existing && (
              <>
                <p className="step">Already set up</p>
                <h3>Daily sync is on</h3>
                <p>
                  Signed in as <strong>{email || "your Google account"}</strong>.{" "}
                  {existing.next_run
                    ? `Next sync: ${formatNextRun(existing.next_run)}.`
                    : "Your tracker updates every night."}
                </p>
                <a className="btn-link" href={existing.spreadsheet_url} target="_blank" rel="noreferrer">
                  Open your tracker
                </a>
                <p className="muted small">
                  New Gemini key?{" "}
                  <button type="button" className="link-button" onClick={() => setStep("gemini")}>
                    Replace it
                  </button>
                </p>
              </>
            )}

            {step === "gemini" && busy && (
              <>
                <p className="step">Setting up</p>
                <h3>Creating sheet and scanning mail</h3>
                <p>
                  This first pass reads job emails from the last 30 days. Daily sync runs later on
                  its own.
                </p>
                <FirstScanProgress />
              </>
            )}

            {step === "gemini" && !busy && (
              <>
                <p className="step">Step 2 of 2</p>
                <h3>Add your Gemini key</h3>
                <p>
                  Signed in as <strong>{email || "your Google account"}</strong>. Get a free key from{" "}
                  <a href={GEMINI_KEYS} target="_blank" rel="noreferrer">
                    Google AI Studio
                  </a>
                  , then paste it below.
                </p>
                {error && <p className="error">{error}</p>}
                <form onSubmit={onSubmit}>
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
                  <button type="submit" disabled={!geminiKey.trim()}>
                    Create sheet & scan mail
                  </button>
                </form>
              </>
            )}

            {step === "done" && result && (
              <>
                <p className="step">You’re set</p>
                <h3>Daily sync is on</h3>
                <p>{setupResultCopy(result)}</p>
                <a className="btn-link" href={result.spreadsheet_url} target="_blank" rel="noreferrer">
                  Open your tracker
                </a>
              </>
            )}
          </div>
        </section>

        <section id="how" className="how">
          <h2>How it works</h2>
          <ol className="steps">
            <li>
              <h3>Search Gmail</h3>
              <p>
                A tight query looks for apply, interview, assessment, rejection, and offer mail from
                the last 30 days — not promotions.
              </p>
            </li>
            <li>
              <h3>Classify with Gemini</h3>
              <p>
                The model extracts company, role, and status. Low-confidence and non-job mail is
                ignored.
              </p>
            </li>
            <li>
              <h3>Write the sheet</h3>
              <p>
                Matching rows update in place. Status only moves forward, so a rejection won’t wipe
                an offer.
              </p>
            </li>
          </ol>
        </section>

        <section id="privacy" className="privacy">
          <h2>What JobSync can access</h2>
          <div className="privacy-grid">
            <div>
              <h3>Gmail, read-only</h3>
              <p>Search and read job-related messages. It cannot send, label, or delete mail.</p>
            </div>
            <div>
              <h3>Google Sheets</h3>
              <p>Creates a JobSync Tracker in your Drive and updates that sheet only.</p>
            </div>
            <div>
              <h3>Your Gemini key</h3>
              <p>Stored encrypted and used only when a sync runs. You can revoke it anytime.</p>
            </div>
          </div>
          <p className="privacy-note">
            Full details are in the{" "}
            <a href={PRIVACY} target="_blank" rel="noreferrer">
              privacy policy
            </a>
            .
          </p>
        </section>
      </main>

      <footer className="footer">
        <span>JobSync</span>
        <a href={PRIVACY} target="_blank" rel="noreferrer">
          Privacy
        </a>
        <a href={GITHUB} target="_blank" rel="noreferrer">
          GitHub
        </a>
      </footer>
    </div>
  );
}

function setupResultCopy(result: SetupCompleteResponse): string {
  const filled = (result.emails_created ?? 0) + (result.emails_updated ?? 0);
  const sheet = result.reused_sheet
    ? "Using your existing tracker for this Gmail."
    : "Created your JobSync Tracker spreadsheet.";

  if (result.quota_exhausted) {
    return `${sheet} Gemini’s free-tier limit was hit — remaining mail waits until the next nightly sync.`;
  }
  if (result.sync_error) {
    return `${sheet} Daily sync is on. The first scan will retry tonight.`;
  }
  if (filled > 0) {
    const noun = filled === 1 ? "application" : "applications";
    return `${sheet} Added ${filled} ${noun} from recent mail. It will keep updating overnight.`;
  }
  if (result.sync_status) {
    return `${sheet} No new updates in recent mail. Daily sync will keep watching.`;
  }
  return `${sheet} Daily sync is on.`;
}
