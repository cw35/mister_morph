import { computed, reactive } from "vue";

const AUTH_STORAGE_KEY = "mistermorph_console_auth_v1";

const authState = reactive({
  token: "",
  expiresAt: "",
  account: "console",
});

const authValid = computed(() => {
  if (!authState.token || !authState.expiresAt) {
    return false;
  }
  const ts = new Date(authState.expiresAt).getTime();
  if (!Number.isFinite(ts)) {
    return false;
  }
  return ts > Date.now();
});

function saveAuth() {
  localStorage.setItem(
    AUTH_STORAGE_KEY,
    JSON.stringify({
      token: authState.token,
      expiresAt: authState.expiresAt,
      account: authState.account,
    })
  );
}

function clearAuth() {
  authState.token = "";
  authState.expiresAt = "";
  authState.account = "console";
  localStorage.removeItem(AUTH_STORAGE_KEY);
}

function hydrateAuth() {
  const raw = localStorage.getItem(AUTH_STORAGE_KEY);
  if (!raw) {
    return;
  }
  try {
    const parsed = JSON.parse(raw);
    authState.token = typeof parsed.token === "string" ? parsed.token : "";
    authState.expiresAt = typeof parsed.expiresAt === "string" ? parsed.expiresAt : "";
    authState.account = typeof parsed.account === "string" ? parsed.account : "console";
  } catch {
    clearAuth();
  }
}

export { authState, authValid, saveAuth, clearAuth, hydrateAuth };
