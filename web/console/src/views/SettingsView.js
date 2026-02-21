import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import "./SettingsView.css";

import {
  apiFetch,
  applyLanguageChange,
  authState,
  clearAuth,
  formatRemainingUntil,
  localeState,
  translate,
} from "../core/context";

const SettingsView = {
  setup() {
    const t = translate;
    const router = useRouter();
    const lang = computed(() => localeState.lang);
    const nowTick = ref(Date.now());
    const loading = ref(false);
    const loggingOut = ref(false);
    const err = ref("");
    const configJSON = ref("");
    const checks = ref([]);
    let tickTimer = null;

    async function load() {
      loading.value = true;
      err.value = "";
      try {
        const [cfg, diag] = await Promise.all([apiFetch("/system/config"), apiFetch("/system/diagnostics")]);
        configJSON.value = JSON.stringify(cfg, null, 2);
        checks.value = Array.isArray(diag.checks) ? diag.checks : [];
      } catch (e) {
        err.value = e.message || t("msg_load_failed");
      } finally {
        loading.value = false;
      }
    }

    function checkStatus(c) {
      return c && c.ok ? t("status_pass") : t("status_fail");
    }

    function checkClass(c) {
      return c && c.ok ? "check-status check-pass" : "check-status check-fail";
    }

    async function logout() {
      loggingOut.value = true;
      try {
        await apiFetch("/auth/logout", { method: "POST" });
      } catch {
        // ignore logout failure
      } finally {
        clearAuth();
        router.replace("/login");
        loggingOut.value = false;
      }
    }

    const sessionLabel = computed(() => {
      void nowTick.value;
      return formatRemainingUntil(authState.expiresAt);
    });
    const sessionText = computed(() => t("topbar_ttl", { value: sessionLabel.value }));

    onMounted(() => {
      void load();
      tickTimer = window.setInterval(() => {
        nowTick.value = Date.now();
      }, 30000);
    });
    onUnmounted(() => {
      if (tickTimer !== null) {
        window.clearInterval(tickTimer);
        tickTimer = null;
      }
    });
    return {
      t,
      lang,
      loading,
      loggingOut,
      err,
      configJSON,
      checks,
      load,
      logout,
      checkStatus,
      checkClass,
      sessionText,
      onLanguageChange: applyLanguageChange,
    };
  },
  template: `
    <section>
      <h2 class="title">{{ t("settings_title") }}</h2>
      <div class="toolbar settings-toolbar">
        <div class="settings-toolbar-left">
          <QLanguageSelector :lang="lang" :presist="true" @change="onLanguageChange" />
        </div>
        <div class="settings-toolbar-right">
          <span class="settings-session">{{ sessionText }}</span>
          <QButton class="outlined" :loading="loading" @click="load">{{ t("action_refresh") }}</QButton>
          <QButton class="danger" :loading="loggingOut" @click="logout">{{ t("action_logout") }}</QButton>
        </div>
      </div>
      <QProgress v-if="loading" :infinite="true" />
      <QFence v-if="err" type="danger" icon="QIconCloseCircle" :text="err" />
      <div class="check-list">
        <div v-for="(item, idx) in checks" :key="idx" class="check-item">
          <code :class="checkClass(item)">{{ checkStatus(item) }}</code>
          <code class="check-id">{{ item.id }}</code>
          <span v-if="item.detail" class="muted check-detail">{{ item.detail }}</span>
        </div>
      </div>
      <QTextarea :modelValue="configJSON" :rows="18" :disabled="true" />
    </section>
  `,
};

export default SettingsView;
