import { reactive } from "vue";

const LANGUAGE_STORAGE_KEY = "quail-language";

const I18N = {
  en: {
    ttl_unknown: "unknown",
    ttl_invalid: "invalid",
    ttl_expired: "expired",
    ttl_min_left: "{m}m left",
    ttl_hour_left: "{h}h {m}m left",
    ttl_day_left: "{d}d {h}h left",

    login_required_password: "Please enter password",
    login_failed: "Login failed",
    login_button: "Login",
    login_password_placeholder: "Console password",

    dashboard_title: "Overview",
    runtime_title: "Runtime",
    group_endpoints: "Endpoints",
    group_basic: "Basic Status",
    group_model: "Model Config",
    group_channels: "Channels",
    group_runtime: "Runtime Metrics",
    stat_version: "Version",
    stat_started: "Started",
    stat_uptime: "Uptime",
    stat_health: "Health",
    stat_llm_provider: "LLM Provider",
    stat_llm_model: "LLM Model",
    stat_channels: "Channels",
    stat_go_version: "Go Version",
    stat_goroutines: "Goroutines",
    stat_heap_alloc: "Heap Alloc",
    stat_heap_sys: "Heap Sys",
    stat_heap_objects: "Heap Objects",
    stat_gc_cycles: "GC Cycles",

    tasks_title: "Tasks",
    status_all: "All",
    status_queued: "Queued",
    status_running: "Running",
    status_pending: "Pending",
    status_done: "Done",
    status_failed: "Failed",
    status_canceled: "Canceled",
    placeholder_status: "status",
    placeholder_limit: "limit",
    no_tasks: "No tasks",
    task_detail: "Detail",
    task_detail_title: "Task Detail",
    audit_title: "Audit",
    audit_file: "Audit File",
    audit_window_bytes: "Window Bytes",
    audit_latest: "Latest",
    audit_newer: "Newer",
    audit_older: "Older",
    audit_path: "Path",
    audit_size: "Size",
    audit_range: "Range",
    audit_empty: "No audit records in this window",
    audit_no_file: "Audit log file not found",
    audit_time: "Time",
    audit_decision: "Decision",
    audit_risk: "Risk",
    audit_action: "Action",
    audit_tool: "Tool",
    audit_run: "Run",
    audit_step: "Step",
    audit_actor: "Actor",
    audit_approval: "Approval",
    audit_reasons: "Reasons",
    audit_summary: "Summary",
    audit_raw: "Raw",
    audit_decision_allow: "Allow",
    audit_decision_redact: "Allow+Redact",
    audit_decision_require_approval: "Require Approval",
    audit_decision_deny: "Deny",
    audit_risk_low: "Low",
    audit_risk_medium: "Medium",
    audit_risk_high: "High",
    audit_risk_critical: "Critical",

    contacts_title: "Contacts",
    todo_title: "TODO",
    persona_title: "Persona",
    settings_title: "System",
    placeholder_select_file: "Select file",
    placeholder_audit_file: "Audit file",

    action_save: "Save",
    action_back: "Back",
    action_refresh: "Refresh",
    action_logout: "Logout",

    msg_load_failed: "Load failed",
    msg_read_failed: "Read failed",
    msg_save_failed: "Save failed",
    msg_save_success: "Saved",
    msg_file_missing_create: "File not found. Edit and save to create.",
    status_pass: "PASS",
    status_fail: "FAIL",
    no_endpoints: "No endpoints",
    endpoint_status_connected: "Connected",
    endpoint_status_disconnected: "Disconnected",
    endpoint_detail_title: "Endpoint Detail",
    endpoint_label_name: "Name",
    endpoint_label_url: "URL",
    endpoint_label_mode: "Mode",
    endpoint_label_connected: "Connected",
    endpoint_not_found: "Endpoint not found",

    nav_overview: "Overview",
    nav_runtime: "Runtime",
    nav_tasks: "Tasks",
    nav_audit: "Audit",
    nav_todo: "TODO",
    nav_contacts: "Contacts",
    nav_persona: "Persona",
    nav_settings: "Settings",
    drawer_nav: "Navigation",
    topbar_ttl: "TTL {value}",
    endpoint_placeholder: "Select endpoint",
    msg_select_endpoint: "Please select endpoint",
  },
  zh: {
    ttl_unknown: "未知",
    ttl_invalid: "无效",
    ttl_expired: "已过期",
    ttl_min_left: "剩余 {m} 分钟",
    ttl_hour_left: "剩余 {h} 小时 {m} 分钟",
    ttl_day_left: "剩余 {d} 天 {h} 小时",

    login_required_password: "请输入密码",
    login_failed: "登录失败",
    login_button: "登录",
    login_password_placeholder: "控制台密码",

    dashboard_title: "概览",
    runtime_title: "运行时",
    group_endpoints: "Endpoints",
    group_basic: "基础状态",
    group_model: "模型配置",
    group_channels: "接入渠道",
    group_runtime: "运行时指标",
    stat_version: "版本",
    stat_started: "启动时间",
    stat_uptime: "运行时长",
    stat_health: "健康状态",
    stat_llm_provider: "LLM 提供方",
    stat_llm_model: "LLM 模型",
    stat_channels: "渠道",
    stat_go_version: "Go 版本",
    stat_goroutines: "协程数",
    stat_heap_alloc: "堆已分配",
    stat_heap_sys: "堆系统占用",
    stat_heap_objects: "堆对象数",
    stat_gc_cycles: "GC 次数",

    tasks_title: "任务",
    status_all: "全部",
    status_queued: "排队中",
    status_running: "运行中",
    status_pending: "待审批",
    status_done: "已完成",
    status_failed: "失败",
    status_canceled: "已取消",
    placeholder_status: "状态",
    placeholder_limit: "数量",
    no_tasks: "无任务",
    task_detail: "详情",
    task_detail_title: "任务详情",
    audit_title: "审计",
    audit_file: "审计文件",
    audit_window_bytes: "窗口字节",
    audit_latest: "最新",
    audit_newer: "较新",
    audit_older: "较旧",
    audit_path: "路径",
    audit_size: "大小",
    audit_range: "范围",
    audit_empty: "当前窗口内无审计记录",
    audit_no_file: "未找到审计日志文件",
    audit_time: "时间",
    audit_decision: "决策",
    audit_risk: "风险",
    audit_action: "动作",
    audit_tool: "工具",
    audit_run: "Run",
    audit_step: "步骤",
    audit_actor: "操作者",
    audit_approval: "审批",
    audit_reasons: "原因",
    audit_summary: "摘要",
    audit_raw: "原始",
    audit_decision_allow: "允许",
    audit_decision_redact: "允许并脱敏",
    audit_decision_require_approval: "需要审批",
    audit_decision_deny: "拒绝",
    audit_risk_low: "低",
    audit_risk_medium: "中",
    audit_risk_high: "高",
    audit_risk_critical: "严重",

    contacts_title: "联系人",
    todo_title: "待办",
    persona_title: "人格",
    settings_title: "系统",
    placeholder_select_file: "选择文件",
    placeholder_audit_file: "选择审计文件",

    action_save: "保存",
    action_back: "返回",
    action_refresh: "刷新",
    action_logout: "退出",

    msg_load_failed: "加载失败",
    msg_read_failed: "读取失败",
    msg_save_failed: "保存失败",
    msg_save_success: "保存成功",
    msg_file_missing_create: "文件不存在，可直接编辑后保存创建",
    status_pass: "通过",
    status_fail: "失败",
    no_endpoints: "无 endpoints",
    endpoint_status_connected: "已联通",
    endpoint_status_disconnected: "未联通",
    endpoint_detail_title: "Endpoint 详情",
    endpoint_label_name: "名称",
    endpoint_label_url: "URL",
    endpoint_label_mode: "模式",
    endpoint_label_connected: "联通",
    endpoint_not_found: "未找到 endpoint",

    nav_overview: "概览",
    nav_runtime: "运行时",
    nav_tasks: "任务",
    nav_audit: "审计",
    nav_todo: "待办",
    nav_contacts: "联系人",
    nav_persona: "人格",
    nav_settings: "配置",
    drawer_nav: "导航",
    topbar_ttl: "会话 {value}",
    endpoint_placeholder: "选择 endpoint",
    msg_select_endpoint: "请先选择 endpoint",
  },
  ja: {
    ttl_unknown: "不明",
    ttl_invalid: "無効",
    ttl_expired: "期限切れ",
    ttl_min_left: "残り {m} 分",
    ttl_hour_left: "残り {h} 時間 {m} 分",
    ttl_day_left: "残り {d} 日 {h} 時間",

    login_required_password: "パスワードを入力してください",
    login_failed: "ログインに失敗しました",
    login_button: "ログイン",
    login_password_placeholder: "コンソールパスワード",

    dashboard_title: "概要",
    runtime_title: "ランタイム",
    group_endpoints: "Endpoints",
    group_basic: "基本状態",
    group_model: "モデル設定",
    group_channels: "接続チャネル",
    group_runtime: "ランタイム指標",
    stat_version: "バージョン",
    stat_started: "起動時刻",
    stat_uptime: "稼働時間",
    stat_health: "ヘルス",
    stat_llm_provider: "LLM プロバイダー",
    stat_llm_model: "LLM モデル",
    stat_channels: "チャネル",
    stat_go_version: "Go バージョン",
    stat_goroutines: "Goroutine 数",
    stat_heap_alloc: "ヒープ確保量",
    stat_heap_sys: "ヒープシステム量",
    stat_heap_objects: "ヒープオブジェクト数",
    stat_gc_cycles: "GC 回数",

    tasks_title: "タスク",
    status_all: "すべて",
    status_queued: "キュー中",
    status_running: "実行中",
    status_pending: "承認待ち",
    status_done: "完了",
    status_failed: "失敗",
    status_canceled: "キャンセル",
    placeholder_status: "状態",
    placeholder_limit: "件数",
    no_tasks: "タスクなし",
    task_detail: "詳細",
    task_detail_title: "タスク詳細",
    audit_title: "監査",
    audit_file: "監査ファイル",
    audit_window_bytes: "ウィンドウ bytes",
    audit_latest: "最新",
    audit_newer: "新しい側",
    audit_older: "古い側",
    audit_path: "パス",
    audit_size: "サイズ",
    audit_range: "範囲",
    audit_empty: "このウィンドウには監査記録がありません",
    audit_no_file: "監査ログファイルが見つかりません",
    audit_time: "時刻",
    audit_decision: "判定",
    audit_risk: "リスク",
    audit_action: "アクション",
    audit_tool: "ツール",
    audit_run: "Run",
    audit_step: "ステップ",
    audit_actor: "実行者",
    audit_approval: "承認",
    audit_reasons: "理由",
    audit_summary: "要約",
    audit_raw: "Raw",
    audit_decision_allow: "許可",
    audit_decision_redact: "許可+マスク",
    audit_decision_require_approval: "承認が必要",
    audit_decision_deny: "拒否",
    audit_risk_low: "低",
    audit_risk_medium: "中",
    audit_risk_high: "高",
    audit_risk_critical: "重大",

    contacts_title: "連絡先",
    todo_title: "TODO",
    persona_title: "ペルソナ",
    settings_title: "システム",
    placeholder_select_file: "ファイルを選択",
    placeholder_audit_file: "監査ファイルを選択",

    action_save: "保存",
    action_back: "戻る",
    action_refresh: "更新",
    action_logout: "ログアウト",

    msg_load_failed: "読み込みに失敗しました",
    msg_read_failed: "読み取りに失敗しました",
    msg_save_failed: "保存に失敗しました",
    msg_save_success: "保存しました",
    msg_file_missing_create: "ファイルがありません。編集して保存すると作成されます。",
    status_pass: "PASS",
    status_fail: "FAIL",
    no_endpoints: "endpoint なし",
    endpoint_status_connected: "接続済み",
    endpoint_status_disconnected: "未接続",
    endpoint_detail_title: "Endpoint 詳細",
    endpoint_label_name: "名前",
    endpoint_label_url: "URL",
    endpoint_label_mode: "モード",
    endpoint_label_connected: "接続",
    endpoint_not_found: "endpoint が見つかりません",

    nav_overview: "概要",
    nav_runtime: "ランタイム",
    nav_tasks: "タスク",
    nav_audit: "監査",
    nav_todo: "TODO",
    nav_contacts: "連絡先",
    nav_persona: "ペルソナ",
    nav_settings: "設定",
    drawer_nav: "ナビゲーション",
    topbar_ttl: "TTL {value}",
    endpoint_placeholder: "endpoint を選択",
    msg_select_endpoint: "先に endpoint を選択してください",
  },
};

const localeState = reactive({
  lang: "en",
});

function normalizeLang(raw) {
  const value = String(raw || "").trim().toLowerCase();
  if (value.startsWith("zh")) {
    return "zh";
  }
  if (value.startsWith("ja")) {
    return "ja";
  }
  return "en";
}

function translate(key, vars = null) {
  const dict = I18N[localeState.lang] || I18N.en;
  let text = dict[key] || I18N.en[key] || key;
  if (!vars || typeof vars !== "object") {
    return text;
  }
  for (const [k, v] of Object.entries(vars)) {
    text = text.replaceAll(`{${k}}`, String(v));
  }
  return text;
}

function currentLocale() {
  switch (localeState.lang) {
    case "zh":
      return "zh-CN";
    case "ja":
      return "ja-JP";
    default:
      return "en-US";
  }
}

function setLanguage(lang) {
  const next = normalizeLang(lang);
  localeState.lang = next;
  localStorage.setItem(LANGUAGE_STORAGE_KEY, next);
}

function applyLanguageChange(item) {
  if (item && typeof item === "object" && "value" in item) {
    setLanguage(item.value);
    return;
  }
  setLanguage(item);
}

function hydrateLanguage() {
  const fromStorage = localStorage.getItem(LANGUAGE_STORAGE_KEY);
  if (fromStorage) {
    localeState.lang = normalizeLang(fromStorage);
    return;
  }
  localeState.lang = normalizeLang(navigator.language || "");
  localStorage.setItem(LANGUAGE_STORAGE_KEY, localeState.lang);
}

export {
  localeState,
  translate,
  currentLocale,
  setLanguage,
  applyLanguageChange,
  hydrateLanguage,
};
