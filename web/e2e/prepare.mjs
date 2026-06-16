import { mkdirSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'

const tmpDir = resolve(import.meta.dirname, '../../.tmp')
const dbPath = resolve(tmpDir, 'e2e-open-spanner.db')

mkdirSync(tmpDir, { recursive: true })

for (const suffix of ['', '-shm', '-wal']) {
  rmSync(`${dbPath}${suffix}`, { force: true })
}
