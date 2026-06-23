import { useMutation } from "@tanstack/react-query";
import { Button, Card, message, Space, Typography, Upload } from "antd";
import type { UploadFile } from "antd";
import { ImageUp } from "lucide-react";
import { useState } from "react";
import { api } from "../../api/client";
import { brandingFaviconURL, type BrandingArea } from "../../components/branding";

const brandingSettings: Array<{
  area: BrandingArea;
  title: string;
  description: string;
}> = [
  {
    area: "center",
    title: "用户中心",
    description: "用于用户中心、登录页、注册页和忘记密码页。"
  },
  {
    area: "admin",
    title: "管理后台",
    description: "用于管理后台，建议 Logo 使用适合浅色背景的版本。"
  }
];

export function AdminSettingsPage() {
  const [assetVersion, setAssetVersion] = useState(Date.now());
  const refreshAssets = (eventName: "koffy-logo-updated" | "koffy-favicon-updated") => {
    setAssetVersion(Date.now());
    window.dispatchEvent(new Event(eventName));
  };

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">系统设置</h1>
      </div>
      <Card title="品牌资源">
        <div className="settings-logo-list">
          {brandingSettings.map((setting) => (
            <BrandingSettingCard key={setting.area} setting={setting} version={assetVersion} onUploaded={refreshAssets} />
          ))}
        </div>
      </Card>
    </div>
  );
}

function BrandingSettingCard({
  setting,
  version,
  onUploaded
}: {
  setting: (typeof brandingSettings)[number];
  version: number;
  onUploaded: (eventName: "koffy-logo-updated" | "koffy-favicon-updated") => void;
}) {
  return (
    <div className="settings-logo-item">
      <div className="settings-logo-copy">
        <Typography.Title level={4}>{setting.title}</Typography.Title>
        <Typography.Text type="secondary">{setting.description}</Typography.Text>
      </div>
      <BrandingAssetUploader area={setting.area} kind="logo" version={version} onUploaded={onUploaded} />
      <BrandingAssetUploader area={setting.area} kind="favicon" version={version} onUploaded={onUploaded} />
    </div>
  );
}

function BrandingAssetUploader({
  area,
  kind,
  version,
  onUploaded
}: {
  area: BrandingArea;
  kind: "logo" | "favicon";
  version: number;
  onUploaded: (eventName: "koffy-logo-updated" | "koffy-favicon-updated") => void;
}) {
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const label = kind === "logo" ? "Logo" : "网站图标 (favicon)";
  const upload = useMutation<{ width: number; height: number; size_bytes: number }, Error, File>({
    mutationFn: async (file: File) =>
      kind === "logo" ? await api.adminUploadLogo(file, area) : await api.adminUploadFavicon(file, area),
    onSuccess: (result) => {
      message.success(`${label} 已更新：${result.width}*${result.height}，${Math.ceil(result.size_bytes / 1024)}KB`);
      setFileList([]);
      onUploaded(kind === "logo" ? "koffy-logo-updated" : "koffy-favicon-updated");
    },
    onError: (error) => message.error(error instanceof Error ? error.message : `${label} 上传失败`)
  });
  const selectedFile = fileList[0]?.originFileObj;
  const assetURL =
    kind === "logo"
      ? `/api/v1/branding/logo?area=${area}&v=${version}`
      : brandingFaviconURL(area, version);

  return (
    <div className="settings-logo-grid">
      <div className={kind === "logo" ? "logo-preview" : "favicon-preview"}>
        <img src={assetURL} alt={`${area} ${label}`} />
      </div>
      <Space direction="vertical" size={14}>
        <Typography.Text strong>{label}</Typography.Text>
        <Upload
          accept="image/png,image/jpeg,image/gif"
          maxCount={1}
          fileList={fileList}
          beforeUpload={() => false}
          onChange={({ fileList: nextFileList }) => setFileList(nextFileList)}
        >
          <Button icon={<ImageUp size={16} />}>选择图片</Button>
        </Upload>
        <Typography.Text type="secondary">
          {kind === "logo" ? "建议使用 702*180 的透明背景 PNG 图片。" : "建议使用正方形 PNG 图片，上传后统一处理为 128*128。"}
        </Typography.Text>
        <Button type="primary" disabled={!selectedFile} loading={upload.isPending} onClick={() => selectedFile && upload.mutate(selectedFile)}>
          更换 {label}
        </Button>
      </Space>
    </div>
  );
}
