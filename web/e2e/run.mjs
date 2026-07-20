import { spawn } from 'node:child_process'
import { writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const webRoot = resolve(import.meta.dirname, '..')
const playwright = resolve(webRoot, 'node_modules', '@playwright', 'test', 'cli.js')

const nextEnv = `/// <reference types="next" />
/// <reference types="next/image-types/global" />
import "./.next/dev/types/routes.d.ts";

// NOTE: This file should not be edited
// see https://nextjs.org/docs/app/api-reference/config/typescript for more information.
`

try {
  await run(process.execPath, [resolve(import.meta.dirname, 'prepare.mjs')])
  const exitCode = await run(process.execPath, [playwright, 'test'], { allowFailure: true })
  process.exitCode = exitCode
} finally {
  writeFileSync(resolve(webRoot, 'next-env.d.ts'), nextEnv)
}

function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const child = spawn(command, args, {
      cwd: webRoot,
      env: process.env,
      stdio: 'inherit',
    })
    child.once('error', reject)
    child.once('exit', (code, signal) => {
      if (signal) {
        reject(new Error(`${command} exited from signal ${signal}`))
        return
      }
      const exitCode = code ?? 1
      if (exitCode !== 0 && !options.allowFailure) {
        reject(new Error(`${command} exited with code ${exitCode}`))
        return
      }
      resolveRun(exitCode)
    })
  })
}
