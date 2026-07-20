import { useEffect } from 'react'

export function useInitialLoad(load: () => Promise<void>) {
  useEffect(() => {
    const id = window.setTimeout(() => {
      void load()
    }, 0)

    return () => window.clearTimeout(id)
  }, [load])
}
