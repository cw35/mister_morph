import { computed, onMounted, reactive, ref } from "vue";

import { apiFetch, formatBytes, formatTime, safeJSON, toBool, toInt, translate } from "../core/context";

const AUDIT_ITEMS_PER_PAGE = 50;

const AuditView = {
  setup() {
    const t = translate;
    const loading = ref(false);
    const err = ref("");
    const fileItems = ref([]);
    const selectedFile = ref("");
    const lines = ref([]);
    const newerStack = ref([]);
    const meta = reactive({
      path: "",
      exists: false,
      size_bytes: 0,
      before: 0,
      from: 0,
      to: 0,
      has_older: false,
    });

    const selectedFileItem = computed(() => {
      return fileItems.value.find((item) => item.value === selectedFile.value) || fileItems.value[0] || null;
    });
    const canGoNewer = computed(() => newerStack.value.length > 0);
    const auditItems = computed(() => {
      return lines.value
        .map((line, idx) => parseAuditLine(line, idx))
        .reverse();
    });

    function normalizeAuditText(value, fallback = "-") {
      if (typeof value === "string") {
        const s = value.trim();
        return s === "" ? fallback : s;
      }
      if (typeof value === "number" && Number.isFinite(value)) {
        return String(Math.trunc(value));
      }
      return fallback;
    }

    function normalizeAuditList(value) {
      if (!Array.isArray(value)) {
        return [];
      }
      return value
        .map((it) => {
          if (typeof it === "string") {
            return it.trim();
          }
          if (it === null || it === undefined) {
            return "";
          }
          return String(it).trim();
        })
        .filter((it) => it !== "");
    }

    function humanizeAuditToken(raw) {
      const text = normalizeAuditText(raw, "");
      if (!text) {
        return "-";
      }
      return text
        .replaceAll("_", " ")
        .replace(/([a-z0-9])([A-Z])/g, "$1 $2");
    }

    function decisionBadgeType(raw) {
      switch (String(raw || "").trim().toLowerCase()) {
        case "allow":
          return "success";
        case "allow_with_redaction":
          return "warning";
        case "require_approval":
          return "warning";
        case "deny":
          return "danger";
        default:
          return "default";
      }
    }

    function riskBadgeType(raw) {
      switch (String(raw || "").trim().toLowerCase()) {
        case "low":
          return "success";
        case "medium":
          return "warning";
        case "high":
          return "danger";
        case "critical":
          return "danger";
        default:
          return "default";
      }
    }

    function decisionLabel(raw) {
      switch (String(raw || "").trim().toLowerCase()) {
        case "allow":
          return t("audit_decision_allow");
        case "allow_with_redaction":
          return t("audit_decision_redact");
        case "require_approval":
          return t("audit_decision_require_approval");
        case "deny":
          return t("audit_decision_deny");
        default:
          return humanizeAuditToken(raw);
      }
    }

    function riskLabel(raw) {
      switch (String(raw || "").trim().toLowerCase()) {
        case "low":
          return t("audit_risk_low");
        case "medium":
          return t("audit_risk_medium");
        case "high":
          return t("audit_risk_high");
        case "critical":
          return t("audit_risk_critical");
        default:
          return humanizeAuditToken(raw);
      }
    }

    function parseAuditLine(line, idx) {
      const raw = typeof line === "string" ? line : String(line ?? "");
      const parsed = safeJSON(raw, null);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        return {
          key: `${meta.from}-${idx}-raw`,
          parsed: false,
          raw,
        };
      }

      const eventID = normalizeAuditText(parsed.event_id);
      const tsRaw = normalizeAuditText(parsed.ts);
      const stepText = normalizeAuditText(parsed.step);
      const actionTypeRaw = normalizeAuditText(parsed.action_type);
      const toolName = normalizeAuditText(parsed.tool_name);
      const runID = normalizeAuditText(parsed.run_id);
      const actor = normalizeAuditText(parsed.actor);
      const approvalStatus = normalizeAuditText(parsed.approval_status);
      const summary = normalizeAuditText(parsed.action_summary_redacted);
      const reasons = normalizeAuditList(parsed.reasons);
      const reasonsText = reasons.length > 0 ? reasons.join(" | ") : "-";
      const decisionRaw = normalizeAuditText(parsed.decision, "");
      const riskRaw = normalizeAuditText(parsed.risk_level, "");

      return {
        key: `${meta.from}-${idx}-${eventID}`,
        parsed: true,
        eventID,
        tsText: tsRaw === "-" ? "-" : formatTime(tsRaw),
        actionType: humanizeAuditToken(actionTypeRaw),
        toolName,
        runID,
        stepText,
        actor,
        approvalStatus: humanizeAuditToken(approvalStatus),
        summary,
        reasonsText,
        decisionLabel: decisionLabel(decisionRaw),
        decisionType: decisionBadgeType(decisionRaw),
        riskLabel: riskLabel(riskRaw),
        riskType: riskBadgeType(riskRaw),
      };
    }

    async function loadFiles() {
      const data = await apiFetch("/audit/files");
      const items = Array.isArray(data.items) ? data.items : [];
      fileItems.value = items
        .map((it) => {
          const name = typeof it.name === "string" ? it.name.trim() : "";
          return {
            title: `${name} (${formatBytes(it.size_bytes)})`,
            value: name,
          };
        })
        .filter((it) => it.value !== "");

      const preferred = typeof data.default_file === "string" ? data.default_file.trim() : "";
      if (fileItems.value.length === 0) {
        selectedFile.value = preferred;
        return;
      }
      if (fileItems.value.find((it) => it.value === selectedFile.value)) {
        return;
      }
      if (preferred && fileItems.value.find((it) => it.value === preferred)) {
        selectedFile.value = preferred;
        return;
      }
      selectedFile.value = fileItems.value[0].value;
    }

    async function loadChunk(cursor = null, resetNewer = false) {
      loading.value = true;
      err.value = "";
      try {
        const q = new URLSearchParams();
        if (selectedFile.value) {
          q.set("file", selectedFile.value);
        }
        q.set("limit", String(AUDIT_ITEMS_PER_PAGE));
        if (cursor !== null && cursor >= 0) {
          q.set("cursor", String(cursor));
        }
        const data = await apiFetch(`/audit/logs?${q.toString()}`);
        meta.path = data.path || "";
        meta.exists = toBool(data.exists, false);
        meta.size_bytes = toInt(data.size_bytes, 0);
        meta.before = toInt(data.before, 0);
        meta.from = toInt(data.from, 0);
        meta.to = toInt(data.to, 0);
        meta.has_older = toBool(data.has_older, false);
        const fetchedLines = Array.isArray(data.lines) ? data.lines : [];
        lines.value = fetchedLines.slice(-AUDIT_ITEMS_PER_PAGE);
        if (resetNewer) {
          newerStack.value = [];
        }
      } catch (e) {
        err.value = e.message || t("msg_load_failed");
      } finally {
        loading.value = false;
      }
    }

    async function refreshLatest() {
      await loadChunk(null, true);
    }

    async function older() {
      if (loading.value || !meta.has_older) {
        return;
      }
      newerStack.value.push(meta.to);
      await loadChunk(meta.from, false);
    }

    async function newer() {
      if (loading.value || newerStack.value.length === 0) {
        return;
      }
      const cursor = newerStack.value.pop();
      if (!Number.isFinite(cursor)) {
        return;
      }
      await loadChunk(cursor, false);
    }

    async function onFileChange(item) {
      if (!item || typeof item !== "object" || typeof item.value !== "string") {
        return;
      }
      selectedFile.value = item.value;
      await loadChunk(null, true);
    }

    async function init() {
      try {
        await loadFiles();
      } catch (e) {
        err.value = e.message || t("msg_load_failed");
      }
      await loadChunk(null, true);
    }

    onMounted(init);
    return {
      t,
      loading,
      err,
      fileItems,
      selectedFileItem,
      auditItems,
      meta,
      canGoNewer,
      refreshLatest,
      older,
      newer,
      onFileChange,
      formatBytes,
    };
  },
  template: `
    <section>
      <h2 class="title">{{ t("audit_title") }}</h2>
      <div class="toolbar wrap">
        <div class="tool-item">
          <QDropdownMenu
            :items="fileItems"
            :initialItem="selectedFileItem"
            :placeholder="t('placeholder_audit_file')"
            @change="onFileChange"
          />
        </div>
        <QButton class="outlined" :loading="loading" @click="refreshLatest">{{ t("audit_latest") }}</QButton>
        <QButton class="plain" :disabled="!canGoNewer || loading" @click="newer">{{ t("audit_newer") }}</QButton>
        <QButton class="plain" :disabled="!meta.has_older || loading" @click="older">{{ t("audit_older") }}</QButton>
      </div>
      <QProgress v-if="loading" :infinite="true" />
      <QFence v-if="err" type="danger" icon="QIconCloseCircle" :text="err" />
      <div class="audit-meta">
        <code>{{ t("audit_path") }}: {{ meta.path || "-" }}</code>
        <code>{{ t("audit_size") }}: {{ formatBytes(meta.size_bytes) }}</code>
        <code>{{ t("audit_range") }}: {{ meta.from }} - {{ meta.to }}</code>
      </div>
      <div class="audit-list">
        <div v-for="item in auditItems" :key="item.key" class="audit-row">
          <template v-if="item.parsed">
            <div class="audit-item-head">
              <code class="audit-item-id">{{ item.eventID }}</code>
              <code class="audit-item-time">{{ t("audit_time") }}: {{ item.tsText }}</code>
              <QBadge :type="item.decisionType" size="sm" variant="filled">{{ item.decisionLabel }}</QBadge>
              <QBadge :type="item.riskType" size="sm" variant="filled">{{ item.riskLabel }}</QBadge>
            </div>
            <div class="audit-item-meta">
              <code>{{ t("audit_action") }}: {{ item.actionType }}</code>
              <code>{{ t("audit_tool") }}: {{ item.toolName }}</code>
              <code>{{ t("audit_run") }}: {{ item.runID }}</code>
              <code>{{ t("audit_step") }}: {{ item.stepText }}</code>
              <code v-if="item.approvalStatus !== '-'">{{ t("audit_approval") }}: {{ item.approvalStatus }}</code>
              <code v-if="item.actor !== '-'">{{ t("audit_actor") }}: {{ item.actor }}</code>
            </div>
            <code v-if="item.summary !== '-'" class="audit-summary">{{ t("audit_summary") }}: {{ item.summary }}</code>
            <code v-if="item.reasonsText !== '-'" class="audit-summary">{{ t("audit_reasons") }}: {{ item.reasonsText }}</code>
          </template>
          <template v-else>
            <div class="audit-item-head">
              <QBadge type="default" size="sm" variant="filled">{{ t("audit_raw") }}</QBadge>
            </div>
            <code class="audit-line">{{ item.raw }}</code>
          </template>
        </div>
        <p v-if="!loading && auditItems.length === 0" class="muted">{{ meta.exists ? t("audit_empty") : t("audit_no_file") }}</p>
      </div>
    </section>
  `,
};


export default AuditView;
