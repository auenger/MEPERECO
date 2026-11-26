// Quote only values requiring shell escaping, keeping ordinary URLs readable.
export function shellArgument(value: string): string {
  return /^[a-zA-Z0-9_./:@-]+$/.test(value) ? value : "'" + value.replaceAll("'", "'\"'\"'") + "'";
}
function powershellArgument(value: string): string {
  return "'" + value.replaceAll("'", "''") + "'";
}
export function cliCommands(serverUrl?: string, appUrl?: string) {
  const app = appUrl?.trim().replace(/\/+$/, "") || "https://studio.meper.cn";
  const server = serverUrl?.trim().replace(/\/+$/, "") || app;
  const base = `${app}/cli`;
  return {
    installCmd: `curl -fsSL ${shellArgument(base + "/install.sh")} | MEPERSTUDIO_DOWNLOAD_BASE_URL=${shellArgument(base)} MEPERSTUDIO_SERVER_URL=${shellArgument(server)} bash`,
    windowsInstallCmd: `$env:MEPERSTUDIO_DOWNLOAD_BASE_URL = ${powershellArgument(base)}; $env:MEPERSTUDIO_SERVER_URL = ${powershellArgument(server)}; irm ${powershellArgument(base + "/install.ps1")} | iex`,
    setupCmd: `meperstudio setup self-host --server-url ${shellArgument(server)} --app-url ${shellArgument(app)}`,
  };
}
