import { onMounted, ref } from "vue";

import { apiFetch, translate } from "../core/context";

const PersonaFilesView = {
  setup() {
    const t = translate;
    const loading = ref(false);
    const saving = ref(false);
    const err = ref("");
    const ok = ref("");
    const fileItems = ref([
      { title: "IDENTITY.md", name: "IDENTITY.md" },
      { title: "SOUL.md", name: "SOUL.md" },
    ]);
    const selectedFile = ref(fileItems.value[0]);
    const content = ref("");

    async function loadFiles() {
      const data = await apiFetch("/persona/files");
      const items = Array.isArray(data.items) ? data.items : [];
      if (items.length === 0) {
        return;
      }
      fileItems.value = items.map((it) => ({
        title: it.name || "",
        name: it.name || "",
      }));
      if (!fileItems.value.find((x) => x.name === selectedFile.value?.name)) {
        selectedFile.value = fileItems.value[0];
      }
    }

    async function loadContent(name) {
      loading.value = true;
      err.value = "";
      ok.value = "";
      try {
        const data = await apiFetch(`/persona/files/${encodeURIComponent(name)}`);
        content.value = data.content || "";
      } catch (e) {
        if (e && e.status === 404) {
          content.value = "";
          ok.value = t("msg_file_missing_create");
          return;
        }
        err.value = e.message || t("msg_read_failed");
      } finally {
        loading.value = false;
      }
    }

    async function save() {
      saving.value = true;
      err.value = "";
      ok.value = "";
      try {
        await apiFetch(`/persona/files/${encodeURIComponent(selectedFile.value.name)}`, {
          method: "PUT",
          body: { content: content.value },
        });
        ok.value = t("msg_save_success");
      } catch (e) {
        err.value = e.message || t("msg_save_failed");
      } finally {
        saving.value = false;
      }
    }

    async function onFileChange(item) {
      if (!item || typeof item !== "object" || !item.name) {
        return;
      }
      selectedFile.value = item;
      await loadContent(item.name);
    }

    async function init() {
      await loadFiles();
      await loadContent(selectedFile.value.name);
    }

    onMounted(init);
    return { t, loading, saving, err, ok, fileItems, selectedFile, content, onFileChange, save };
  },
  template: `
    <section>
      <h2 class="title">{{ t("persona_title") }}</h2>
      <div class="toolbar wrap">
        <div class="tool-item">
          <QDropdownMenu
            :items="fileItems"
            :initialItem="selectedFile"
            :placeholder="t('placeholder_select_file')"
            @change="onFileChange"
          />
        </div>
        <QButton class="primary" :loading="saving" @click="save">{{ t("action_save") }}</QButton>
      </div>
      <QProgress v-if="loading" :infinite="true" />
      <QFence v-if="err" type="danger" icon="QIconCloseCircle" :text="err" />
      <QFence v-if="ok" type="success" icon="QIconCheckCircle" :text="ok" />
      <QTextarea v-model="content" :rows="22" />
    </section>
  `,
};


export default PersonaFilesView;
