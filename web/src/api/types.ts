export type ListResponse<T> = { items: T[] };

export type UserProfile = {
  id: number;
  casdoor_user_id: string;
  name: string;
  display_name: string;
  display_name_custom: boolean;
  email: string;
  phone: string;
  avatar_url: string;
  avatar_custom: boolean;
  owner: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
};

export type WalletSummary = {
  balance_coins: number;
  reserved_coins: number;
  available_coins: number;
};

export type WalletLedgerItem = {
  id: number;
  direction: string;
  reason: string;
  amount_coins: number;
  balance_after: number;
  remark: string;
  created_at: string;
};

export type SubscriptionItem = {
  id: number;
  app_code: string;
  app_name: string;
  plan_code: string;
  plan_name: string;
  status: string;
  starts_at: string;
  ends_at: string;
};

export type EntitlementItem = {
  id: number;
  app_code: string;
  app_name: string;
  entitlement_code: string;
  unit: string;
  period_month: string;
  quota: number;
  used: number;
  reserved: number;
  available: number;
};

export type EntitlementLedgerItem = {
  id: number;
  app_code: string;
  app_name: string;
  entitlement_code: string;
  unit: string;
  direction: string;
  amount: number;
  used_after: number;
  reserved_after: number;
  usage_request_id?: number;
  created_at: string;
};

export type UsageRequestItem = {
  id: number;
  app_code: string;
  app_name: string;
  user_id?: string;
  status: string;
  billing_mode: string;
  model_alias: string;
  provider: string;
  provider_model: string;
  provider_job_id: string;
  estimated_total_tokens: number;
  estimated_images: number;
  estimated_video_seconds: number;
  estimated_business_units: number;
  actual_prompt_tokens: number;
  actual_completion_tokens: number;
  actual_total_tokens: number;
  actual_images: number;
  actual_video_seconds: number;
  actual_business_units: number;
  reserved_coins: number;
  charged_coins: number;
  reserved_units: number;
  charged_units: number;
  error_code: string;
  error_message: string;
  created_at: string;
  authorized_at?: string;
  committed_at?: string;
  cancelled_at?: string;
};

export type RechargeOrderItem = {
  id: number;
  order_no: string;
  user_id?: string;
  provider: string;
  amount_cents: number;
  coins: number;
  status: string;
  provider_trade_no: string;
  paid_at?: string;
  created_at: string;
  updated_at: string;
};

export type PaymentEventItem = {
  id: number;
  provider: string;
  event_id: string;
  event_type: string;
  order_no: string;
  provider_trade_no: string;
  processed_at?: string;
  created_at: string;
};

export type AdminAppItem = {
  id: number;
  app_code: string;
  name: string;
  status: string;
  billing_mode: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type TokenPricingItem = {
  id: number;
  app_code: string;
  model_alias: string;
  token_amount: number;
  coin_amount: number;
  status: string;
  effective_from: string;
  created_at: string;
  updated_at: string;
};

export type UnitPricingItem = {
  id: number;
  app_code: string;
  model_alias: string;
  unit: string;
  unit_amount: number;
  coin_amount: number;
  status: string;
  effective_from: string;
  created_at: string;
  updated_at: string;
};

export type PlanEntitlementItem = {
  id: number;
  plan_code: string;
  entitlement_code: string;
  name: string;
  unit: string;
  monthly_quota: number;
  created_at: string;
  updated_at: string;
};

export type PlanItem = {
  id: number;
  app_code: string;
  plan_code: string;
  name: string;
  period: string;
  price_cents: number;
  status: string;
  entitlements?: PlanEntitlementItem[];
  created_at: string;
  updated_at: string;
};

export type AdminUserAsset = {
  user: UserProfile;
  wallet: WalletSummary;
};

export type AdminUserSearchItem = {
  casdoor_user_id: string;
  name: string;
  display_name: string;
  email: string;
  phone: string;
  is_admin: boolean;
};

export type AdminMetricsSummary = {
  days: number;
  since: string;
  until: string;
  usage_by_app: Array<{
    app_code: string;
    app_name: string;
    request_count: number;
    committed_count: number;
    cancelled_count: number;
    failed_count: number;
    charged_coins: number;
    charged_units: number;
    actual_total_tokens: number;
    actual_images: number;
    actual_video_seconds: number;
    actual_business_units: number;
  }>;
  recharge_by_status: Array<{
    status: string;
    order_count: number;
    amount_cents: number;
    coins: number;
  }>;
};

export type BrandingLogoResponse = {
  status: string;
  area: "center" | "admin";
  logo_url: string;
  size_bytes: number;
  width: number;
  height: number;
};

export type BrandingFaviconResponse = {
  status: string;
  area: "center" | "admin";
  favicon_url: string;
  size_bytes: number;
  width: number;
  height: number;
};

export type AuthConfig = {
  provider: string;
  site_key: string;
  enabled: boolean;
};

export type WeChatWidgetConfig = {
  app_id: string;
  app_type: string;
  scope: string;
  redirect_uri: string;
  state: string;
  stylelite: string;
  auth_url: string;
};

export type AuthBindingSummary = {
  phone_bound: boolean;
  wechat_bound: boolean;
  wechat_official_bound: boolean;
  wechat_nickname: string;
  wechat_avatar_url: string;
};

export type WeChatJSPayment = {
  channel: "wechat_jsapi";
  appId: string;
  timeStamp: string;
  nonceStr: string;
  package: string;
  signType: string;
  paySign: string;
};

export type PaymentMethod = "wechat" | "alipay";

export type AIProviderItem = {
  id: number;
  provider_code: string;
  name: string;
  status: string;
  base_url: string;
  created_at: string;
  updated_at: string;
};

export type AIModelItem = {
  id: number;
  provider_code: string;
  provider_name: string;
  model_alias: string;
  provider_model: string;
  capability: string;
  status: string;
  created_at: string;
  updated_at: string;
};

export type AppModelRouteItem = {
  id: number;
  app_code: string;
  model_alias: string;
  provider_code: string;
  provider_model: string;
  capability: string;
  status: string;
  created_at: string;
};

export type CreateRechargeOrderResponse = {
  order_no: string;
  provider: string;
  amount_cents: number;
  coins: number;
  status: string;
  payment: Record<string, unknown>;
};

export type CreateAPIKeyResponse = {
  app_code: string;
  key: string;
  prefix: string;
};

export type EntitlementMaintenanceResult = {
  expired_subscriptions: number;
  created_balances: number;
  updated_balances: number;
};
