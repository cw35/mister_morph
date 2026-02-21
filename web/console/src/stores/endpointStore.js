import { reactive } from "vue";

const ENDPOINT_STORAGE_KEY = "mistermorph_console_endpoint_ref_v1";

const endpointState = reactive({
  items: [],
  selectedRef: "",
});

function saveSelectedEndpointRef() {
  localStorage.setItem(ENDPOINT_STORAGE_KEY, endpointState.selectedRef);
}

function setSelectedEndpointRef(ref) {
  endpointState.selectedRef = typeof ref === "string" ? ref.trim() : "";
  saveSelectedEndpointRef();
}

function hydrateEndpointSelection() {
  const ref = localStorage.getItem(ENDPOINT_STORAGE_KEY);
  endpointState.selectedRef = typeof ref === "string" ? ref.trim() : "";
}

function ensureEndpointSelection() {
  const items = Array.isArray(endpointState.items) ? endpointState.items : [];
  if (items.length === 0) {
    setSelectedEndpointRef("");
    return;
  }
  const current = endpointState.selectedRef.trim();
  if (current && items.find((item) => item.endpoint_ref === current)) {
    return;
  }
  setSelectedEndpointRef(items[0].endpoint_ref);
}

export {
  endpointState,
  setSelectedEndpointRef,
  hydrateEndpointSelection,
  ensureEndpointSelection,
};
