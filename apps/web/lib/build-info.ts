export type AppBuildInfo = {
  version: string;
  commit: string;
  date: string;
};

export function getWebBuildInfo(): AppBuildInfo {
  return {
    version: process.env.NEXT_PUBLIC_APP_VERSION?.trim() || "0.1.0",
    commit: process.env.NEXT_PUBLIC_GIT_COMMIT?.trim() || "dev",
    date: process.env.NEXT_PUBLIC_BUILD_DATE?.trim() || "unknown",
  };
}
