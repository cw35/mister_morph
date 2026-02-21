import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";

import { TASK_STATUS_META, endpointState, formatTime, runtimeApiFetch, translate } from "../core/context";

const TasksView = {
  setup() {
    const t = translate;
    const router = useRouter();
    const taskStatusItems = computed(() =>
      TASK_STATUS_META.map((item) => ({
        title: t(item.titleKey),
        value: item.value,
      }))
    );
    const statusValue = ref(TASK_STATUS_META[0].value);
    const statusItem = computed(() => {
      return taskStatusItems.value.find((item) => item.value === statusValue.value) || taskStatusItems.value[0] || null;
    });
    const limitText = ref("20");
    const items = ref([]);
    const err = ref("");
    const loading = ref(false);

    async function load() {
      loading.value = true;
      err.value = "";
      try {
        const q = new URLSearchParams();
        const v = statusValue.value || "";
        if (v) {
          q.set("status", v);
        }
        const limit = Math.max(1, Math.min(200, parseInt(limitText.value || "20", 10) || 20));
        q.set("limit", String(limit));
        const data = await runtimeApiFetch(`/tasks?${q.toString()}`);
        items.value = Array.isArray(data.items) ? data.items : [];
      } catch (e) {
        err.value = e.message || t("msg_load_failed");
      } finally {
        loading.value = false;
      }
    }

    function onStatusChange(item) {
      if (item && typeof item === "object") {
        statusValue.value = typeof item.value === "string" ? item.value : "";
      }
    }

    function openTask(id) {
      router.push(`/tasks/${id}`);
    }

    function summary(item) {
      const source = item.source || "daemon";
      const status = (item.status || "unknown").toUpperCase();
      return `[${status}] ${item.id} | ${source} | ${item.model || "-"} | ${formatTime(item.created_at)}`;
    }

    onMounted(load);
    watch(
      () => endpointState.selectedRef,
      () => {
        void load();
      }
    );
    return { t, taskStatusItems, statusItem, limitText, items, err, loading, load, onStatusChange, openTask, summary };
  },
  template: `
    <section>
      <h2 class="title">{{ t("tasks_title") }}</h2>
      <div class="toolbar wrap">
        <div class="tool-item">
          <QDropdownMenu
            :items="taskStatusItems"
            :initialItem="statusItem"
            :placeholder="t('placeholder_status')"
            @change="onStatusChange"
          />
        </div>
        <div class="tool-item">
          <QInput v-model="limitText" inputType="number" :placeholder="t('placeholder_limit')" />
        </div>
        <QButton class="outlined" :loading="loading" @click="load">{{ t("action_refresh") }}</QButton>
      </div>
      <QProgress v-if="loading" :infinite="true" />
      <QFence v-if="err" type="danger" icon="QIconCloseCircle" :text="err" />
      <div class="stack">
        <div v-for="item in items" :key="item.id" class="task-row">
          <code class="task-line">{{ summary(item) }}</code>
          <QButton class="plain" @click="openTask(item.id)">{{ t("task_detail") }}</QButton>
        </div>
        <p v-if="items.length === 0 && !loading" class="muted">{{ t("no_tasks") }}</p>
      </div>
    </section>
  `,
};


export default TasksView;
