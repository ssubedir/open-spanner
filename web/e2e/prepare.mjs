import { mkdirSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'

const tmpDir = resolve(import.meta.dirname, '../../.tmp')
const dbPath = resolve(tmpDir, 'e2e-open-spanner.db')
const dbDriver = (process.env.OPEN_SPANNER_E2E_DB_DRIVER || 'sqlite').toLowerCase()

mkdirSync(tmpDir, { recursive: true })

if (dbDriver === 'sqlite') {
  for (const suffix of ['', '-shm', '-wal']) {
    rmSync(`${dbPath}${suffix}`, { force: true })
  }
}
