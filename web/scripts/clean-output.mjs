import { rmSync } from 'node:fs'
import { resolve } from 'node:path'

rmSync(resolve(import.meta.dirname, '../../internal/ui/static/assets'), {
  force: true,
  recursive: true,
})
