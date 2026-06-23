import { Button, ConfigProvider, Drawer, Layout, Menu, Typography } from "antd";
import {
  Bot,
  Boxes,
  CreditCard,
  Gauge,
  KeyRound,
  LayoutDashboard,
  Menu as MenuIcon,
  ReceiptText,
  Settings,
  UserRound,
  WalletCards
} from "lucide-react";
import { useEffect, useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { AccountHeader } from "../components/AccountHeader";
import { AuthState } from "../components/AuthState";
import { setBrandingFavicon } from "../components/branding";

const { Header, Sider, Content } = Layout;

const centerMenuItems = [
  { key: "/center", icon: <WalletCards size={17} />, label: "首页" },
  { key: "/center/recharge", icon: <CreditCard size={17} />, label: "充值" },
  { key: "/center/security", icon: <KeyRound size={17} />, label: "账号安全" }
];

const adminMenuItems = [
  { key: "/admin", icon: <Gauge size={17} />, label: "运营概览" },
  { key: "/admin/apps", icon: <KeyRound size={17} />, label: "应用与定价" },
  { key: "/admin/plans", icon: <Boxes size={17} />, label: "套餐权益" },
  { key: "/admin/users", icon: <UserRound size={17} />, label: "用户资产" },
  { key: "/admin/usage", icon: <LayoutDashboard size={17} />, label: "调用记录" },
  { key: "/admin/payments", icon: <ReceiptText size={17} />, label: "支付记录" },
  { key: "/admin/ai", icon: <Bot size={17} />, label: "AI 配置" },
  { key: "/admin/settings", icon: <Settings size={17} />, label: "系统设置" }
];

const centerTheme = {
  token: {
    colorPrimary: "#00d4aa",
    colorInfo: "#0080ff",
    colorBgLayout: "#0a1628",
    colorBgContainer: "rgba(255, 255, 255, 0.04)",
    colorBorder: "rgba(255, 255, 255, 0.12)",
    colorText: "rgba(255, 255, 255, 0.92)",
    colorTextSecondary: "rgba(255, 255, 255, 0.66)",
    colorTextTertiary: "rgba(255, 255, 255, 0.46)",
    borderRadius: 12,
    fontFamily: 'AlibabaSans, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
  },
  components: {
    Alert: {
      colorInfoBg: "rgba(0, 128, 255, 0.10)",
      colorInfoBorder: "rgba(0, 128, 255, 0.28)"
    },
    Button: {
      borderRadius: 999,
      primaryShadow: "0 14px 32px rgba(0, 128, 255, 0.26)"
    },
    Card: {
      borderRadiusLG: 18,
      colorBorderSecondary: "rgba(255, 255, 255, 0.10)",
      colorBgContainer: "rgba(255, 255, 255, 0.045)"
    },
    Modal: {
      contentBg: "#0f1b2d",
      headerBg: "#0f1b2d",
      titleColor: "#ffffff"
    },
    Table: {
      borderColor: "rgba(255, 255, 255, 0.08)",
      headerBg: "rgba(255, 255, 255, 0.06)",
      headerColor: "rgba(255, 255, 255, 0.72)",
      rowHoverBg: "rgba(0, 212, 170, 0.08)"
    }
  }
};

export function ShellLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const isAdminArea = location.pathname.startsWith("/admin");
  const selected = `/${location.pathname.split("/").filter(Boolean).slice(0, 2).join("/")}`;
  const menuItems = isAdminArea ? adminMenuItems : centerMenuItems;
  const title = isAdminArea ? "管理后台" : "用户中心";
  const [logoVersion, setLogoVersion] = useState(Date.now());
  const [faviconVersion, setFaviconVersion] = useState(Date.now());
  const [centerMenuOpen, setCenterMenuOpen] = useState(false);
  const logoArea = isAdminArea ? "admin" : "center";
  const logoURL = `/api/v1/branding/logo?area=${logoArea}&v=${logoVersion}`;

  useEffect(() => {
    const refreshLogo = () => setLogoVersion(Date.now());
    window.addEventListener("koffy-logo-updated", refreshLogo);
    return () => window.removeEventListener("koffy-logo-updated", refreshLogo);
  }, []);

  useEffect(() => {
    const refreshFavicon = () => setFaviconVersion(Date.now());
    window.addEventListener("koffy-favicon-updated", refreshFavicon);
    return () => window.removeEventListener("koffy-favicon-updated", refreshFavicon);
  }, []);

  useEffect(() => {
    setCenterMenuOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    document.title = isAdminArea ? "管理后台" : "用户中心";
    setBrandingFavicon(isAdminArea ? "admin" : "center", faviconVersion);
  }, [faviconVersion, isAdminArea]);

  const renderMenu = () => (
    <Menu
      className="shell-menu"
      mode="inline"
      theme={isAdminArea ? "light" : "dark"}
      selectedKeys={[selected]}
      items={menuItems}
      onClick={({ key }) => {
        if (typeof key === "string") navigate(key);
      }}
    />
  );

  const shell = (
    <Layout className={`app-shell ${isAdminArea ? "admin-shell" : "center-shell"}`}>
      <Sider
        width={isAdminArea ? 232 : 264}
        breakpoint={isAdminArea ? "lg" : undefined}
        collapsedWidth={isAdminArea ? 0 : undefined}
        className="shell-sider"
        theme={isAdminArea ? "light" : "dark"}
      >
        <div className="brand">
          <img className="brand-logo" src={logoURL} alt="Koffy" />
        </div>
        {renderMenu()}
      </Sider>
      <Layout>
        <Header className="shell-header">
          {!isAdminArea ? (
            <Button
              className="center-menu-button"
              type="text"
              icon={<MenuIcon size={22} />}
              onClick={() => setCenterMenuOpen(true)}
              aria-label="打开菜单"
            />
          ) : null}
          <Typography.Title level={4} style={{ margin: 0 }}>
            {title}
          </Typography.Title>
          <AccountHeader />
        </Header>
        <Content className="content-wrap">
          <AuthState adminOnly={isAdminArea}>
            <Outlet />
          </AuthState>
        </Content>
      </Layout>
      {!isAdminArea ? (
        <Drawer
          rootClassName="center-mobile-drawer"
          classNames={{
            header: "center-mobile-drawer-header",
            body: "center-mobile-drawer-body",
            content: "center-mobile-drawer-content"
          }}
          placement="left"
          width="100%"
          open={centerMenuOpen}
          onClose={() => setCenterMenuOpen(false)}
          title={<img className="drawer-logo" src={logoURL} alt="Koffy" />}
        >
          {renderMenu()}
        </Drawer>
      ) : null}
    </Layout>
  );

  return isAdminArea ? shell : <ConfigProvider theme={centerTheme}>{shell}</ConfigProvider>;
}
