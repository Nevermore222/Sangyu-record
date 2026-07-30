import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

const projectConfig = JSON.parse(
  readFileSync(new URL('./project.config.json', import.meta.url), 'utf8')
)

describe('WeChat DevTools project configuration', () => {
  it('enables the TypeScript compiler plugin for TypeScript pages', () => {
    expect(projectConfig.setting.useCompilerPlugins).toContain('typescript')
  })

  it('excludes test files from upload packages', () => {
    expect(projectConfig.packOptions.ignore).toContainEqual({
      type: 'suffix',
      value: '.test.js'
    })
  })
})
