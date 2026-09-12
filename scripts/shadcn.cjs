#!/usr/bin/env node
const { execSync } = require("node:child_process")
const path = require("node:path")

const fix = path.join(__dirname, "shadcn-path-fix.cjs").replace(/\\/g, "/")
const nodeOptions = [process.env.NODE_OPTIONS, `--require "${fix}"`]
  .filter(Boolean)
  .join(" ")

const args = ["--yes", "shadcn@latest", ...process.argv.slice(2)]
  .map((arg) => (arg.includes(" ") ? `"${arg}"` : arg))
  .join(" ")

try {
  execSync(`npx ${args}`, {
    stdio: "inherit",
    env: { ...process.env, NODE_OPTIONS: nodeOptions },
  })
} catch (error) {
  process.exit(typeof error.status === "number" ? error.status : 1)
}
