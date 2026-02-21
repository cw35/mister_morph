import { createRouter, createWebHistory } from "vue-router";

import { BASE_PATH, apiFetch, authState, authValid, clearAuth, saveAuth } from "../core/context";
import {
  AuditView,
  ContactsFilesView,
  DashboardView,
  LoginView,
  OverviewView,
  PersonaFilesView,
  SettingsView,
  TasksView,
  TaskDetailView,
  TODOFilesView,
} from "../views";

const routes = [
  { path: "/login", component: LoginView },
  { path: "/overview", component: OverviewView },
  { path: "/dashboard", component: DashboardView },
  { path: "/tasks", component: TasksView },
  { path: "/tasks/:id", component: TaskDetailView },
  { path: "/audit", component: AuditView },
  { path: "/todo-files", component: TODOFilesView },
  { path: "/contacts-files", component: ContactsFilesView },
  { path: "/persona-files", component: PersonaFilesView },
  { path: "/settings", component: SettingsView },
  { path: "/", redirect: "/overview" },
];

const router = createRouter({
  history: createWebHistory(BASE_PATH + "/"),
  routes,
});

const NAV_ITEMS_META = [
  { id: "/dashboard", titleKey: "nav_runtime", icon: "QIconSpeedoMeter" },
  { id: "/tasks", titleKey: "nav_tasks", icon: "QIconInbox" },
  { id: "/audit", titleKey: "nav_audit", icon: "QIconFingerprint" },
  { id: "/todo-files", titleKey: "nav_todo", icon: "QIconBookOpen" },
  { id: "/contacts-files", titleKey: "nav_contacts", icon: "QIconUsers" },
  { id: "/persona-files", titleKey: "nav_persona", icon: "QIconUserCircle" },
  { id: "/settings", titleKey: "nav_settings", icon: "QIconSettings" },
];

router.beforeEach(async (to) => {
  if (to.path === "/login") {
    return true;
  }
  if (!authValid.value) {
    return { path: "/login", query: { redirect: to.fullPath } };
  }
  try {
    const me = await apiFetch("/auth/me");
    authState.account = me.account || "console";
    authState.expiresAt = me.expires_at || authState.expiresAt;
    saveAuth();
  } catch {
    clearAuth();
    return { path: "/login", query: { redirect: to.fullPath } };
  }
  return true;
});

export { router, NAV_ITEMS_META };
