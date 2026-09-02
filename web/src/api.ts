export type SetupSession = {
  signed_in: boolean;
  email?: string;
};

export type SetupCompleteResponse = {
  account_id: string;
  spreadsheet_id: string;
  spreadsheet_url: string;
  status: string;
  reused_sheet?: boolean;
};

export async function fetchSession(): Promise<SetupSession> {
  const res = await fetch("/setup/session", { credentials: "include" });
  if (!res.ok) {
    return { signed_in: false };
  }
  return res.json();
}

export async function completeSetup(geminiApiKey: string): Promise<SetupCompleteResponse> {
  const res = await fetch("/setup/complete", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ gemini_api_key: geminiApiKey.trim() }),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(typeof body.error === "string" ? body.error : res.statusText || "Setup failed");
  }
  return body as SetupCompleteResponse;
}

export function startGoogleSignIn() {
  window.location.href = "/setup/oauth/start";
}
