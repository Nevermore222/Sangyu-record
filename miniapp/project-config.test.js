import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

const projectConfig = JSON.parse(
  readFileSync(new URL('./project.config.json', import.meta.url), 'utf8')
)
const appConfig = JSON.parse(
  readFileSync(new URL('./app.json', import.meta.url), 'utf8')
)

describe('WeChat DevTools project configuration', () => {
  it('targets the registered production mini program', () => {
    expect(projectConfig.appid).toBe('wx336e7a90d023878f')
  })

  it('enables the TypeScript compiler plugin for TypeScript pages', () => {
    expect(projectConfig.setting.useCompilerPlugins).toContain('typescript')
  })

  it('excludes test files from upload packages', () => {
    expect(projectConfig.packOptions.ignore).toContainEqual({
      type: 'suffix',
      value: '.test.js'
    })
  })

  it('registers the project row used by archive and workbench lists', () => {
    expect(appConfig.usingComponents['project-row']).toBe('/components/project-row/index')
  })
})
