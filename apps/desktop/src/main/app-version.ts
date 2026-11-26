import { app } from "electron";

// Both dev and packaged builds report the actual desktop package version.
export function getAppVersion(): string { return app.getVersion(); }
