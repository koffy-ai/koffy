export type BrandingArea = "center" | "admin";

export function brandingFaviconURL(area: BrandingArea, version = Date.now()) {
  return `/api/v1/branding/favicon?area=${area}&v=${version}`;
}

export function setBrandingFavicon(area: BrandingArea, version = Date.now()) {
  let icon = document.querySelector<HTMLLinkElement>('link[rel~="icon"]');
  if (!icon) {
    icon = document.createElement("link");
    icon.rel = "icon";
    document.head.appendChild(icon);
  }
  icon.removeAttribute("type");
  icon.href = brandingFaviconURL(area, version);
}
