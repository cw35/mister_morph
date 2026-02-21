import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import "./LoginView.css";

import { apiFetch, applyLanguageChange, authState, loadEndpoints, localeState, saveAuth, translate } from "../core/context";

const LoginView = {
  setup() {
    const router = useRouter();
    const route = useRoute();
    const t = translate;
    const lang = computed(() => localeState.lang);
    const password = ref("");
    const busy = ref(false);
    const err = ref("");

    async function submit() {
      if (busy.value) {
        return;
      }
      if (!password.value.trim()) {
        err.value = t("login_required_password");
        return;
      }
      busy.value = true;
      err.value = "";
      try {
        const body = await apiFetch("/auth/login", {
          method: "POST",
          body: { password: password.value },
          noAuth: true,
        });
        authState.token = body.access_token || "";
        authState.expiresAt = body.expires_at || "";
        authState.account = "console";
        saveAuth();
        await loadEndpoints();
        const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/overview";
        router.replace(redirect);
      } catch (e) {
        err.value = e.message || t("login_failed");
      } finally {
        busy.value = false;
      }
    }

    return { t, lang, password, busy, err, submit, onLanguageChange: applyLanguageChange };
  },
  template: `
    <section class="login-box">
      <h1 class="login-title">Mistermorph Console</h1>
      <div class="login-language">
        <QLanguageSelector :lang="lang" :presist="true" @change="onLanguageChange" />
      </div>
      <form class="stack" @submit.prevent="submit">
        <QInput
          v-model="password"
          inputType="password"
          :placeholder="t('login_password_placeholder')"
          :disabled="busy"
          @keydown.enter.prevent="submit"
        />
        <QButton :loading="busy" class="primary" @click="submit">{{ t("login_button") }}</QButton>
        <QFence v-if="err" type="danger" icon="QIconCloseCircle" :text="err" />
      </form>
    </section>
  `,
};


export default LoginView;
