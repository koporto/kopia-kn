import { BrowserWindow, ipcMain } from "electron";
import { spawn } from "child_process";
import fs from "fs";
import path from "path";
import log from "electron-log";
import { defaultServerBinary, publicPath } from "./utils.js";
import { configDir } from "./config.js";

function destinationCandidates() {
  const extras = [];
  if (process.env.KNOCKOUT_DESTINATION_FILE) {
    extras.push(process.env.KNOCKOUT_DESTINATION_FILE);
  }
  extras.push(path.join(configDir(), "knockout-destination.json"));
  extras.push(path.join(publicPath(), "knockout-destination.json"));
  extras.push(path.join(process.resourcesPath || "", "knockout-destination.json"));
  extras.push(
    path.join(path.dirname(process.execPath), "knockout-destination.json"),
  );
  return extras.filter(Boolean);
}

export function findDestinationFile() {
  for (const p of destinationCandidates()) {
    try {
      if (fs.existsSync(p) && fs.statSync(p).isFile()) {
        return p;
      }
    } catch {
      // keep searching
    }
  }
  return "";
}

export function isKnockoutEnrolled() {
  return fs.existsSync(path.join(configDir(), "knockout-site.json"));
}

function runKnockoutSetup(payload) {
  return new Promise((resolve) => {
    const exe = defaultServerBinary();
    if (!exe) {
      resolve({ error: "Knockout Backup engine was not found." });
      return;
    }

    const args = [
      "knockout",
      "setup",
      "--site-id",
      payload.siteId,
      "--profile",
      payload.profile || "workstation",
      "--json",
    ];
    const dest = findDestinationFile();
    if (dest) {
      args.push("--destination", dest);
    }

    const env = {
      ...process.env,
      KOPIA_PASSWORD: payload.passcode,
    };

    log.info("knockout setup", exe, args.join(" "));
    const child = spawn(exe, args, { env });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d) => {
      stdout += d;
    });
    child.stderr.on("data", (d) => {
      stderr += d;
    });
    child.on("error", (err) => {
      resolve({ error: err.message });
    });
    child.on("close", (code) => {
      if (code !== 0) {
        const line =
          stderr.trim().split("\n").filter(Boolean).pop() ||
          stdout.trim() ||
          "Enrollment failed";
        resolve({ error: line });
        return;
      }
      try {
        resolve(JSON.parse(stdout));
      } catch {
        resolve({ error: "Enrollment finished but the result was not readable." });
      }
    });
  });
}

export function registerKnockoutIPC() {
  ipcMain.handle("knockout-setup", async (_event, payload) => {
    return runKnockoutSetup(payload || {});
  });
  ipcMain.handle("knockout-destination-status", async () => {
    return { destinationFile: findDestinationFile(), enrolled: isKnockoutEnrolled() };
  });
}

export function showKnockoutSetupWindow() {
  return new Promise((resolve) => {
    const win = new BrowserWindow({
      title: "Knockout Backup",
      width: 780,
      height: 760,
      autoHideMenuBar: true,
      webPreferences: {
        preload: path.join(publicPath(), "preload.js"),
      },
    });

    win.loadFile(path.join(publicPath(), "knockout-setup.html"));
    win.on("closed", () => resolve(isKnockoutEnrolled()));
  });
}
