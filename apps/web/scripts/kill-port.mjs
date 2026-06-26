import { execSync } from "node:child_process";

const port = process.argv[2];
if (!port) {
  console.error("Usage: node kill-port.mjs <port>");
  process.exit(0);
}

if (process.platform !== "darwin" && process.platform !== "linux") {
  process.exit(0);
}

try {
  const stdout = execSync(`lsof -ti:${port} -sTCP:LISTEN`, {
    encoding: "utf8",
    stdio: ["pipe", "pipe", "ignore"],
  });
  for (const pid of stdout.trim().split("\n").filter(Boolean)) {
    try {
      process.kill(Number(pid), "SIGKILL");
    } catch {
      // ignore
    }
  }
} catch {
  // nothing listening on the port
}

process.exit(0);
