import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";

import { endpointState, loadEndpoints, setSelectedEndpointRef, toBool, translate } from "../core/context";

const OverviewView = {
  setup() {
    const t = translate;
    const router = useRouter();
    const err = ref("");
    const loading = ref(false);
    let refreshTimer = null;
    const endpointRows = computed(() =>
      endpointState.items.map((item) => ({
        endpoint_ref: item.endpoint_ref || "",
        name: item.name || item.endpoint_ref || "-",
        url: item.url || "-",
        connected: toBool(item.connected, false),
      }))
    );

    function openEndpoint(item) {
      if (!item || typeof item.endpoint_ref !== "string" || !item.endpoint_ref) {
        return;
      }
      setSelectedEndpointRef(item.endpoint_ref);
      router.push("/dashboard");
    }

    async function load() {
      loading.value = true;
      err.value = "";
      try {
        await loadEndpoints();
      } catch (e) {
        err.value = e.message || t("msg_load_failed");
      } finally {
        loading.value = false;
      }
    }

    onMounted(() => {
      void load();
      refreshTimer = window.setInterval(() => {
        void load();
      }, 60000);
    });
    onUnmounted(() => {
      if (refreshTimer !== null) {
        window.clearInterval(refreshTimer);
        refreshTimer = null;
      }
    });
    return { t, err, loading, endpointRows, openEndpoint };
  },
  template: `
    <section>
      <QProgress v-if="loading" :infinite="true" />
      <QFence v-if="err" type="danger" icon="QIconCloseCircle" :text="err" />
      <div class="stat-groups">
        <section class="stat-group">
          <h3 class="stat-group-title">{{ t("group_endpoints") }}</h3>
          <div class="endpoint-overview-list">
            <div
              v-for="item in endpointRows"
              :key="item.endpoint_ref"
              class="endpoint-overview-item frame clickable"
              tabindex="0"
              role="button"
              @click="openEndpoint(item)"
              @keydown.enter.prevent="openEndpoint(item)"
              @keydown.space.prevent="openEndpoint(item)"
            >
              <div class="endpoint-overview-head my-2">
                <span class="channel-runtime-dot">
                  <QBadge
                    :type="item.connected ? 'success' : 'default'"
                    size="md"
                    variant="filled"
                    :dot="true"
                  />
                </span>
                <code class="endpoint-overview-name">{{ item.name }}</code>
              </div>
              <code class="endpoint-overview-url">{{ item.url }}</code>
            </div>
            <p v-if="endpointRows.length === 0 && !loading" class="muted">{{ t("no_endpoints") }}</p>
          </div>
        </section>
      </div>
    </section>
  `,
};


export default OverviewView;
