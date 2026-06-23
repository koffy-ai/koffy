import { createBrowserRouter, Navigate } from "react-router-dom";
import { ShellLayout } from "./layouts/ShellLayout";
import { AdminAIPage } from "./pages/admin/AdminAIPage";
import { AdminAppsPage } from "./pages/admin/AdminAppsPage";
import { AdminOverviewPage } from "./pages/admin/AdminOverviewPage";
import { AdminPaymentsPage } from "./pages/admin/AdminPaymentsPage";
import { AdminPlansPage } from "./pages/admin/AdminPlansPage";
import { AdminSettingsPage } from "./pages/admin/AdminSettingsPage";
import { AdminUsagePage } from "./pages/admin/AdminUsagePage";
import { AdminUsersPage } from "./pages/admin/AdminUsersPage";
import { AccountSecurityPage } from "./pages/center/AccountSecurityPage";
import { CenterHomePage } from "./pages/center/CenterHomePage";
import { ForgotPasswordPage } from "./pages/ForgotPasswordPage";
import { LoginPage } from "./pages/LoginPage";
import { RechargePage } from "./pages/center/RechargePage";
import { RegisterPage } from "./pages/RegisterPage";

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  { path: "/register", element: <RegisterPage /> },
  { path: "/forgot-password", element: <ForgotPasswordPage /> },
  {
    path: "/",
    element: <ShellLayout />,
    children: [
      { index: true, element: <Navigate to="/center" replace /> },
      { path: "center", element: <CenterHomePage /> },
      { path: "center/recharge", element: <RechargePage /> },
      { path: "center/recharge/", element: <RechargePage /> },
      { path: "center/security", element: <AccountSecurityPage /> },
      { path: "admin", element: <AdminOverviewPage /> },
      { path: "admin/apps", element: <AdminAppsPage /> },
      { path: "admin/plans", element: <AdminPlansPage /> },
      { path: "admin/users", element: <AdminUsersPage /> },
      { path: "admin/usage", element: <AdminUsagePage /> },
      { path: "admin/payments", element: <AdminPaymentsPage /> },
      { path: "admin/ai", element: <AdminAIPage /> },
      { path: "admin/settings", element: <AdminSettingsPage /> }
    ]
  }
]);
