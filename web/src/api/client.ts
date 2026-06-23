import type {
  AdminAppItem,
  AdminMetricsSummary,
  AdminUserAsset,
  AdminUserSearchItem,
  AuthBindingSummary,
  AuthConfig,
  AIModelItem,
  AIProviderItem,
  AppModelRouteItem,
  BrandingFaviconResponse,
  BrandingLogoResponse,
  CreateAPIKeyResponse,
  CreateRechargeOrderResponse,
  EntitlementItem,
  EntitlementLedgerItem,
  EntitlementMaintenanceResult,
  ListResponse,
  PaymentEventItem,
  PlanEntitlementItem,
  PlanItem,
  RechargeOrderItem,
  SubscriptionItem,
  TokenPricingItem,
  UnitPricingItem,
  UsageRequestItem,
  UserProfile,
  WeChatWidgetConfig,
  WalletLedgerItem,
  WalletSummary
} from "./types";

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export function isAuthError(error: unknown) {
  return (
    error instanceof ApiError &&
    (error.status === 401 ||
      error.code === "missing_token" ||
      error.code === "invalid_token" ||
      error.code === "unauthorized")
  );
}

const API_BASE = import.meta.env.VITE_BILLING_API_BASE || "";

export function loginWithCasdoor() {
  const returnTo = `${window.location.pathname}${window.location.search}`;
  window.location.href = `/login?return_to=${encodeURIComponent(returnTo)}`;
}

export function registerAccount() {
  const returnTo = `${window.location.pathname}${window.location.search}`;
  window.location.href = `/register?return_to=${encodeURIComponent(returnTo)}`;
}

export function startWeChatLogin(returnTo?: string) {
  const target = returnTo || `${window.location.pathname}${window.location.search}`;
  window.location.href = `/api/v1/auth/wechat/start?action=login&return_to=${encodeURIComponent(target)}`;
}

export function startWeChatBind(returnTo = "/center/security") {
  window.location.href = `/api/v1/auth/wechat/start?action=bind&return_to=${encodeURIComponent(returnTo)}`;
}

export function startWeChatPayAuth(returnTo = "/center/recharge/") {
  window.location.href = `/api/v1/auth/wechat/start?action=pay&mode=official&return_to=${encodeURIComponent(returnTo)}`;
}

export async function logout() {
  await request<{ status: string }>("/auth/logout", { method: "POST" });
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...init,
    headers
  });
  const text = await response.text();
  let data: any = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      if (response.status === 413) {
        throw new ApiError(response.status, "request_too_large", "上传内容过大，请选择 20MB 以内的图片");
      }
      throw new ApiError(
        response.status,
        "invalid_response",
        response.ok ? "服务返回了无法解析的数据" : `服务暂不可用：${response.status} ${response.statusText}`
      );
    }
  }
  if (!response.ok) {
    const error = data?.error;
    throw new ApiError(response.status, error?.code || "request_failed", error?.message || response.statusText);
  }
  return data as T;
}

export const api = {
  authConfig: () => request<AuthConfig>("/api/v1/auth/config"),
  me: () => request<UserProfile>("/api/v1/me"),
  updateProfile: (body: { display_name: string }) =>
    request<UserProfile>("/api/v1/me/profile", { method: "PATCH", body: JSON.stringify(body) }),
  uploadAvatar: (file: File) => {
    const body = new FormData();
    body.append("avatar", file);
    return request<UserProfile>("/api/v1/me/avatar", { method: "POST", body });
  },
  authBindings: () => request<AuthBindingSummary>("/api/v1/me/auth-bindings"),
  unbindWeChat: () => request<{ status: string }>("/api/v1/me/wechat-binding", { method: "DELETE" }),
  wallet: () => request<WalletSummary>("/api/v1/me/wallet"),
  walletLedger: (limit = 50) => request<ListResponse<WalletLedgerItem>>(`/api/v1/me/wallet/ledger?limit=${limit}`),
  subscriptions: () => request<ListResponse<SubscriptionItem>>("/api/v1/me/subscriptions"),
  entitlements: () => request<ListResponse<EntitlementItem>>("/api/v1/me/entitlements"),
  usageRequests: (limit = 50) => request<ListResponse<UsageRequestItem>>(`/api/v1/me/usage-requests?limit=${limit}`),
  entitlementLedger: (limit = 50) => request<ListResponse<EntitlementLedgerItem>>(`/api/v1/me/entitlement-ledger?limit=${limit}`),
  rechargeOrders: (limit = 50) => request<ListResponse<RechargeOrderItem>>(`/api/v1/me/recharge-orders?limit=${limit}`),
  createRechargeOrder: (body: { amount_cents: number; channel: string; description: string; openid?: string; wechat_pay_code?: string }) =>
    request<CreateRechargeOrderResponse>("/api/v1/recharge/orders", { method: "POST", body: JSON.stringify(body) }),
  sendRegisterPhoneCode: (body: { country_code: string; phone: string; human_token?: string }) =>
    request<{ status: string; debug_code?: string }>("/api/v1/auth/phone-code", {
      method: "POST",
      body: JSON.stringify(body)
    }),
  registerWithPhone: (body: {
    country_code: string;
    phone: string;
    code: string;
    password: string;
    confirm_password: string;
  }) => request<{ status: string }>("/api/v1/auth/register", { method: "POST", body: JSON.stringify(body) }),
  loginWithPassword: (body: { account: string; password: string; return_to?: string }) =>
    request<{ status: string; redirect_to: string }>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(body) }),
  wechatWidgetConfig: (params: { action: string; return_to?: string }) =>
    request<WeChatWidgetConfig>(
      `/api/v1/auth/wechat/widget-config?action=${encodeURIComponent(params.action)}&return_to=${encodeURIComponent(params.return_to || "")}`
    ),
  exchangeSession: (body: { code: string }) =>
    request<{ access_token: string; token_type: string; expires_at: string; user: UserProfile }>("/api/v1/auth/session-exchange", {
      method: "POST",
      body: JSON.stringify(body)
    }),
  sendResetPasswordCode: (body: { country_code: string; phone: string; human_token?: string }) =>
    request<{ status: string; debug_code?: string }>("/api/v1/auth/forgot-password/code", {
      method: "POST",
      body: JSON.stringify(body)
    }),
  resetPassword: (body: {
    country_code: string;
    phone: string;
    code: string;
    password: string;
    confirm_password: string;
  }) => request<{ status: string }>("/api/v1/auth/forgot-password/reset", { method: "POST", body: JSON.stringify(body) }),
  sendBindPhoneCode: (body: { country_code: string; phone: string; human_token?: string }) =>
    request<{ status: string; debug_code?: string }>("/api/v1/me/phone/code", { method: "POST", body: JSON.stringify(body) }),
  bindPhone: (body: { country_code: string; phone: string; code: string; password: string; confirm_password: string }) =>
    request<{ status: string }>("/api/v1/me/phone/bind", { method: "POST", body: JSON.stringify(body) }),
  sendChangePasswordCode: (body: { human_token?: string }) =>
    request<{ status: string; debug_code?: string }>("/api/v1/me/password/code", { method: "POST", body: JSON.stringify(body) }),
  changePassword: (body: { code: string; password: string; confirm_password: string }) =>
    request<{ status: string }>("/api/v1/me/password/change", { method: "POST", body: JSON.stringify(body) }),

  adminApps: () => request<ListResponse<AdminAppItem>>("/api/v1/admin/apps"),
  adminSaveApp: (body: { app_code: string; name: string; billing_mode: string; description: string }) =>
    request<AdminAppItem>("/api/v1/admin/apps", { method: "POST", body: JSON.stringify(body) }),
  adminCreateAPIKey: (appCode: string) =>
    request<CreateAPIKeyResponse>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/api-keys`, {
      method: "POST",
      body: "{}"
    }),
  adminPricing: (appCode: string) =>
    request<{ token_pricing: TokenPricingItem[]; unit_pricing: UnitPricingItem[] }>(
      `/api/v1/admin/apps/${encodeURIComponent(appCode)}/pricing`
    ),
  adminSavePricing: (appCode: string, body: { model_alias: string; token_amount: number; coin_amount: number }) =>
    request<{ status: string }>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/pricing`, {
      method: "POST",
      body: JSON.stringify(body)
    }),
  adminDeleteTokenPricing: (pricingID: number) =>
    request<{ status: string }>(`/api/v1/admin/pricing/token/${pricingID}`, { method: "DELETE" }),
  adminSaveUnitPricing: (
    appCode: string,
    body: { model_alias: string; unit: string; unit_amount: number; coin_amount: number }
  ) =>
    request<{ status: string }>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/unit-pricing`, {
      method: "POST",
      body: JSON.stringify(body)
    }),
  adminDeleteUnitPricing: (pricingID: number) =>
    request<{ status: string }>(`/api/v1/admin/pricing/unit/${pricingID}`, { method: "DELETE" }),
  adminPlans: (appCode: string) => request<ListResponse<PlanItem>>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/plans`),
  adminSavePlan: (appCode: string, body: { plan_code: string; name: string; period: string; price_cents: number; status: string }) =>
    request<PlanItem>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/plans`, {
      method: "POST",
      body: JSON.stringify(body)
    }),
  adminSavePlanEntitlement: (
    appCode: string,
    planCode: string,
    body: { entitlement_code: string; name: string; unit: string; monthly_quota: number }
  ) =>
    request<PlanEntitlementItem>(
      `/api/v1/admin/apps/${encodeURIComponent(appCode)}/plans/${encodeURIComponent(planCode)}/entitlements`,
      { method: "POST", body: JSON.stringify(body) }
    ),
  adminGrantSubscription: (userID: string, body: { app_code: string; plan_code: string; months: number }) =>
    request<SubscriptionItem>(`/api/v1/admin/users/${encodeURIComponent(userID)}/subscriptions`, {
      method: "POST",
      body: JSON.stringify(body)
    }),
  adminRunEntitlementMaintenance: () =>
    request<EntitlementMaintenanceResult>("/api/v1/admin/entitlements/maintenance", { method: "POST" }),
  adminUserAsset: (userID: string) => request<AdminUserAsset>(`/api/v1/admin/users/${encodeURIComponent(userID)}/asset`),
  adminUserWalletLedger: (userID: string, limit = 50) =>
    request<ListResponse<WalletLedgerItem>>(`/api/v1/admin/users/${encodeURIComponent(userID)}/wallet/ledger?limit=${limit}`),
  adminUserSubscriptions: (userID: string) =>
    request<ListResponse<SubscriptionItem>>(`/api/v1/admin/users/${encodeURIComponent(userID)}/subscriptions`),
  adminUserEntitlements: (userID: string) =>
    request<ListResponse<EntitlementItem>>(`/api/v1/admin/users/${encodeURIComponent(userID)}/entitlements`),
  adminUserEntitlementLedger: (userID: string, limit = 50) =>
    request<ListResponse<EntitlementLedgerItem>>(`/api/v1/admin/users/${encodeURIComponent(userID)}/entitlement-ledger?limit=${limit}`),
  adminUserUsageRequests: (userID: string, limit = 50) =>
    request<ListResponse<UsageRequestItem>>(`/api/v1/admin/users/${encodeURIComponent(userID)}/usage-requests?limit=${limit}`),
  adminUserRechargeOrders: (userID: string, limit = 50) =>
    request<ListResponse<RechargeOrderItem>>(`/api/v1/admin/users/${encodeURIComponent(userID)}/recharge-orders?limit=${limit}`),
  adminAdjustCoins: (userID: string, body: { amount_coins: number; remark: string }) =>
    request<WalletSummary>(`/api/v1/admin/users/${encodeURIComponent(userID)}/adjust-coins`, {
      method: "POST",
      body: JSON.stringify(body)
    }),
  adminSearchUsers: (keyword: string, limit = 20) =>
    request<ListResponse<AdminUserSearchItem>>(
      `/api/v1/admin/users/search?q=${encodeURIComponent(keyword)}&limit=${limit}`
    ),
  adminMetrics: (days: number) => request<AdminMetricsSummary>(`/api/v1/admin/metrics/summary?days=${days}`),
  adminUsageRequests: (params: URLSearchParams) => request<ListResponse<UsageRequestItem>>(`/api/v1/admin/usage-requests?${params}`),
  adminRechargeOrders: (params: URLSearchParams) => request<ListResponse<RechargeOrderItem>>(`/api/v1/admin/recharge-orders?${params}`),
  adminPaymentEvents: (params: URLSearchParams) => request<ListResponse<PaymentEventItem>>(`/api/v1/admin/payment-events?${params}`),
  adminUploadLogo: (file: File, area: "center" | "admin" = "center") => {
    const body = new FormData();
    body.append("logo", file);
    return request<BrandingLogoResponse>(`/api/v1/admin/branding/logo?area=${encodeURIComponent(area)}`, { method: "POST", body });
  },
  adminUploadFavicon: (file: File, area: "center" | "admin" = "center") => {
    const body = new FormData();
    body.append("favicon", file);
    return request<BrandingFaviconResponse>(`/api/v1/admin/branding/favicon?area=${encodeURIComponent(area)}`, {
      method: "POST",
      body
    });
  },

  aiProviders: () => request<ListResponse<AIProviderItem>>("/api/v1/admin/ai/providers"),
  saveAIProvider: (body: { provider_code: string; name: string; status: string; base_url: string }) =>
    request<AIProviderItem>("/api/v1/admin/ai/providers", { method: "POST", body: JSON.stringify(body) }),
  aiModels: () => request<ListResponse<AIModelItem>>("/api/v1/admin/ai/models"),
  saveAIModel: (body: { provider_code: string; model_alias: string; provider_model: string; capability: string; status: string }) =>
    request<AIModelItem>("/api/v1/admin/ai/models", { method: "POST", body: JSON.stringify(body) }),
  appModelRoutes: (appCode: string) =>
    request<ListResponse<AppModelRouteItem>>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/model-routes`),
  saveAppModelRoute: (appCode: string, body: { model_alias: string; status: string }) =>
    request<AppModelRouteItem>(`/api/v1/admin/apps/${encodeURIComponent(appCode)}/model-routes`, {
      method: "POST",
      body: JSON.stringify(body)
    })
};
