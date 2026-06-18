import { mkdirSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'

const tmpDir = resolve(import.meta.dirname, '../../.tmp')
const dbPath = resolve(tmpDir, 'e2e-open-spanner.db')
const dbDriver = (process.env.OPEN_SPANNER_E2E_DB_DRIVER || 'sqlite').toLowerCase()
const exportStoragePath = process.env.OPEN_SPANNER_E2E_EXPORT_STORAGE_PATH || resolve(tmpDir, 'e2e-exports')

mkdirSync(tmpDir, { recursive: true })
rmSync(exportStoragePath, { force: true, recursive: true })
mkdirSync(exportStoragePath, { recursive: true })

if (dbDriver === 'sqlite') {
  for (const suffix of ['', '-shm', '-wal']) {
    rmSync(`${dbPath}${suffix}`, { force: true })
  }
}
