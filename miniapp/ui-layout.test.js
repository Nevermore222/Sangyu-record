import { existsSync, readFileSync, statSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = (path) => readFileSync(new URL(path, import.meta.url), 'utf8')

describe('miniapp visual layout', () => {
  it('keeps all four capture actions in a two-column overflow-safe grid', () => {
    const markup = source('./pages/visit-capture/index.wxml')
    const styles = source('./pages/visit-capture/index.wxss')

    expect(markup.match(/<button class="capture-tool/g)).toHaveLength(4)
    expect(styles).toMatch(/\.capture-tools\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/s)
    expect(styles).toMatch(/\.capture-tool\s*\{[^}]*min-width:\s*0/s)
  })

  it('uses compressed local backgrounds only on approved presentation pages', () => {
    const allowed = ['login', 'workbench', 'result']
    const assets = ['paper-texture.png', 'login-memory.jpg', 'workbench-memory.jpg', 'result-memory.jpg']

    for (const asset of assets) {
      const path = new URL(`./assets/${asset}`, import.meta.url)
      expect(existsSync(path)).toBe(true)
      expect(statSync(path).size).toBeLessThan(160_000)
    }

    for (const page of allowed) {
      expect(source(`./pages/${page}/index.wxml`)).toContain('/assets/')
    }

    for (const page of ['projects', 'create', 'project', 'visit-prepare', 'visit-capture', 'visit-report', 'workflow', 'profile']) {
      expect(source(`./pages/${page}/index.wxml`)).not.toContain('/assets/')
      expect(source(`./pages/${page}/index.wxss`)).not.toContain('/assets/')
    }
  })
})
