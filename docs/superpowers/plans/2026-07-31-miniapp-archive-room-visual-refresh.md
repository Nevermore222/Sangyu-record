# Sangyu Record Miniapp Archive Room Visual Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the approved “口述档案室” visual system to every miniapp page and eliminate the visit-capture toolbar overlap with a stable 2 x 2 grid.

**Architecture:** Preserve all existing TypeScript page logic, routes, components, and backend contracts. Add a small set of compressed local background assets, centralize common visual tokens and primitives in `app.wxss`, and keep page-specific composition in existing WXML/WXSS files. Protect the regression with source-structure tests that run in the current Vitest setup.

**Tech Stack:** Native WeChat miniapp, TypeScript, WXML, WXSS, Vitest, WeChat DevTools CLI.

## Global Constraints

- Do not change backend APIs, authentication, upload, transcription, agent, or knowledge-base behavior.
- Do not add a third-party UI library or a remote image dependency.
- Use local compressed bitmap resources only on login, workbench, and result surfaces.
- Keep forms, archive lists, visit preparation, visit capture, and reports free of decorative image backgrounds.
- Use warm paper white, ink green, and brick red; keep control radii at 8rpx or less.
- Preserve the existing route and tab structure.

---

### Task 1: Lock the visual contract with failing tests

**Files:**
- Create: `miniapp/ui-layout.test.js`
- Test: `miniapp/ui-layout.test.js`

**Interfaces:**
- Consumes: WXML and WXSS files as UTF-8 source text.
- Produces: regression checks for the capture grid, local visual assets, and approved page asset boundaries.

- [ ] **Step 1: Write the failing layout test**

```js
import { existsSync, readFileSync, statSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = (path) => readFileSync(new URL(path, import.meta.url), 'utf8')

describe('miniapp visual layout', () => {
  it('keeps all four capture actions in a two-column overflow-safe grid', () => {
    const markup = source('./pages/visit-capture/index.wxml')
    const styles = source('./pages/visit-capture/index.wxss')

    expect(markup.match(/class="capture-tool/g)).toHaveLength(4)
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
      expect(source(`./pages/${page}/index.wxss`)).toContain('/assets/')
    }

    for (const page of ['projects', 'create', 'project', 'visit-prepare', 'visit-capture', 'visit-report', 'workflow', 'profile']) {
      expect(source(`./pages/${page}/index.wxss`)).not.toContain('/assets/')
    }
  })
})
```

- [ ] **Step 2: Run the test and verify RED**

Run: `cd miniapp; npm test -- ui-layout.test.js`

Expected: FAIL because the current capture grid has four columns and `miniapp/assets/` backgrounds do not exist.

- [ ] **Step 3: Commit the failing contract test**

```powershell
git add -- miniapp/ui-layout.test.js
git commit -m "test: define miniapp visual layout contract"
```

---

### Task 2: Add compact archive-room background assets and global primitives

**Files:**
- Create: `miniapp/assets/paper-texture.png`
- Create: `miniapp/assets/login-memory.jpg`
- Create: `miniapp/assets/workbench-memory.jpg`
- Create: `miniapp/assets/result-memory.jpg`
- Modify: `miniapp/app.wxss`
- Modify: `miniapp/components/action-bar/index.wxss`
- Modify: `miniapp/components/empty-state/index.wxss`
- Modify: `miniapp/components/project-row/index.wxss`
- Modify: `miniapp/components/status-tag/index.wxss`
- Modify: `miniapp/components/asset-row/index.wxss`
- Modify: `miniapp/components/photo-grid/index.wxss`
- Test: `miniapp/ui-layout.test.js`

**Interfaces:**
- Consumes: existing component class names and global CSS variables.
- Produces: `/assets/*.jpg`, `/assets/paper-texture.png`, global tokens `--paper`, `--paper-deep`, `--ink`, `--muted`, `--line`, `--burgundy`, and `--green`.

- [ ] **Step 1: Generate four restrained bitmap assets**

Use the image generation workflow with these exact prompts, then crop and compress to the listed outputs:

```text
paper-texture.png, 512x512: subtle warm-white archival paper fibers, nearly flat lighting, no text, no border, seamless low-contrast texture
login-memory.jpg, 900x1600: softly lit Chinese family photo album and fountain pen on a plain desk, warm daylight, generous empty upper area, documentary realism, no people, no text
workbench-memory.jpg, 1200x500: archival folders, small voice recorder and one old family photograph on a clean table, soft side light, quiet documentary realism, no text
result-memory.jpg, 900x1200: completed clothbound memoir beside a single old photograph, warm daylight, quiet refined editorial photography, no text
```

Compress every file below 160 KB. Keep the image content light enough for black interface text through a solid or translucent overlay.

- [ ] **Step 2: Extend the global visual tokens and primitives**

Update `miniapp/app.wxss` so the beginning of the file contains:

```css
page {
  --ink: #242824;
  --muted: #686d68;
  --canvas: #f1f2ee;
  --paper: #fbfaf6;
  --paper-deep: #f2eee4;
  --surface: #ffffff;
  --line: #dddcd5;
  --burgundy: #7c302c;
  --green: #315c49;
  min-height: 100%;
  background: var(--canvas);
  color: var(--ink);
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  font-size: 30rpx;
  letter-spacing: 0;
}

.page {
  min-height: 100vh;
  padding: 32rpx 32rpx 180rpx;
  background: var(--paper);
  box-sizing: border-box;
}
```

Retain existing global selectors, replacing repeated literal palette values with the matching variables. Add `overflow-wrap: anywhere` to dynamic titles and labels and keep all shared radii at `8rpx` or less.

- [ ] **Step 3: Harmonize shared components**

Update the six component WXSS files to consume the same palette, fixed control heights, and visible focus/pressed states. Preserve all WXML events and properties. `project-row` and `asset-row` must keep `flex: 1; min-width: 0` on their text containers.

- [ ] **Step 4: Run the existing component-independent suite**

Run: `cd miniapp; npm test -- project-config.test.js domain/capture.test.ts services/session.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit the visual foundation**

```powershell
git add -- miniapp/assets miniapp/app.wxss miniapp/components/action-bar/index.wxss miniapp/components/empty-state/index.wxss miniapp/components/project-row/index.wxss miniapp/components/status-tag/index.wxss miniapp/components/asset-row/index.wxss miniapp/components/photo-grid/index.wxss
git commit -m "style: add archive room visual foundation"
```

---

### Task 3: Rebuild the visit-capture composition without overlap

**Files:**
- Modify: `miniapp/pages/visit-capture/index.wxml`
- Modify: `miniapp/pages/visit-capture/index.wxss`
- Test: `miniapp/ui-layout.test.js`

**Interfaces:**
- Consumes: existing `toggleRecord`, `chooseAudio`, `choosePhoto`, queue, and submit bindings.
- Produces: `.capture-intro`, `.capture-tools`, and four fixed-format `.capture-tool` buttons.

- [ ] **Step 1: Implement the two-column action grid**

Keep the four current buttons and event bindings. Wrap them under an “开始采集” section label and implement:

```css
.capture-tools {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
}

.capture-tool {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: space-between;
  width: 100%;
  min-width: 0;
  height: 156rpx;
  margin: 0;
  padding: 24rpx;
  border: 1rpx solid var(--line);
  border-radius: 8rpx;
  background: #fff;
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 600;
  line-height: 1.25;
  text-align: left;
  box-sizing: border-box;
}
```

Give the record action a brick-red accent, keep icon boxes at a stable `40rpx`, and add `overflow-wrap: anywhere` to the label. Reduce the notes textarea to a controlled `220rpx` height and add bottom spacing so the fixed action bar cannot cover it.

- [ ] **Step 2: Run the focused layout test and verify GREEN for the grid test**

Run: `cd miniapp; npm test -- ui-layout.test.js`

Expected: the capture-grid test passes; the asset-boundary test still fails until Task 4 references the assets.

- [ ] **Step 3: Commit the capture-page fix**

```powershell
git add -- miniapp/pages/visit-capture/index.wxml miniapp/pages/visit-capture/index.wxss
git commit -m "fix: prevent capture actions from overlapping"
```

---

### Task 4: Apply the archive-room system to every remaining page

**Files:**
- Modify: `miniapp/pages/login/index.wxml`
- Modify: `miniapp/pages/login/index.wxss`
- Modify: `miniapp/pages/workbench/index.wxml`
- Modify: `miniapp/pages/workbench/index.wxss`
- Modify: `miniapp/pages/projects/index.wxss`
- Modify: `miniapp/pages/profile/index.wxss`
- Modify: `miniapp/pages/create/index.wxss`
- Modify: `miniapp/pages/project/index.wxss`
- Modify: `miniapp/pages/visit-prepare/index.wxss`
- Modify: `miniapp/pages/visit-report/index.wxss`
- Modify: `miniapp/pages/workflow/index.wxss`
- Modify: `miniapp/pages/result/index.wxml`
- Modify: `miniapp/pages/result/index.wxss`
- Test: `miniapp/ui-layout.test.js`

**Interfaces:**
- Consumes: local `/assets/` resources and existing page data/event bindings.
- Produces: consistent presentation across all eleven registered pages without route or behavior changes.

- [ ] **Step 1: Add presentation-only image layers to approved pages**

Add an `image` with `mode="aspectFill"` and `aria-hidden="true"` only in login, workbench, and result WXML. Use these exact classes and sources:

```xml
<image class="login-backdrop" src="/assets/login-memory.jpg" mode="aspectFill" aria-hidden="true"></image>
<image class="workbench-backdrop" src="/assets/workbench-memory.jpg" mode="aspectFill" aria-hidden="true"></image>
<image class="result-backdrop" src="/assets/result-memory.jpg" mode="aspectFill" aria-hidden="true"></image>
```

Place each image in a positioned wrapper with an opaque or near-opaque paper overlay behind all text. Do not add asset references to any other page.

- [ ] **Step 2: Harmonize operational pages**

Apply the global tokens to projects, profile, create, project, visit-prepare, visit-report, and workflow WXSS. Keep their existing WXML and bindings. Use a consistent spacing sequence (`12rpx`, `20rpx`, `28rpx`, `40rpx`), keep headings compact, and retain full-width list bands rather than converting sections into nested cards.

- [ ] **Step 3: Complete the presentation pages**

Style login as a quiet cover with a stable bottom authentication state, workbench as a compact daily ledger with its image limited to the header band, and result as a finished-book presentation with the background behind the existing book preview. Ensure all foreground content remains readable when images fail to load.

- [ ] **Step 4: Run the focused visual contract and typecheck**

Run: `cd miniapp; npm test -- ui-layout.test.js; npm run typecheck`

Expected: PASS for both asset-boundary tests and TypeScript compilation.

- [ ] **Step 5: Commit the page refresh**

```powershell
git add -- miniapp/pages/login miniapp/pages/workbench miniapp/pages/projects/index.wxss miniapp/pages/profile/index.wxss miniapp/pages/create/index.wxss miniapp/pages/project/index.wxss miniapp/pages/visit-prepare/index.wxss miniapp/pages/visit-report/index.wxss miniapp/pages/workflow/index.wxss miniapp/pages/result
git commit -m "style: refresh miniapp pages with archive room theme"
```

---

### Task 5: Verify the complete miniapp and package it in DevTools

**Files:**
- Modify only if verification reveals a visual regression in files already listed above.

**Interfaces:**
- Consumes: completed miniapp visual refresh.
- Produces: passing automated suite and a DevTools-compilable package at 320px and standard iPhone widths.

- [ ] **Step 1: Run the complete automated suite**

Run: `cd miniapp; npm test; npm run typecheck`

Expected: all Vitest files pass and TypeScript exits with code 0.

- [ ] **Step 2: Build with WeChat DevTools**

Run:

```powershell
& 'D:\微信开发者工具\微信web开发者工具\cli.bat' build-npm --project 'D:\Sangyu-record\miniapp' --port 9420 --lang zh
```

Expected: DevTools prints `build-npm` success with no compilation errors.

- [ ] **Step 3: Inspect fixed-format screens**

In WeChat DevTools, inspect `pages/visit-capture/index` at 320px and the standard iPhone simulator. Confirm four capture buttons remain two columns with no clipped text; the notes and upload summary scroll fully above the fixed action bar. Inspect login, workbench, archives, project, visit preparation, report, workflow, result, and profile for text clipping and image contrast.

- [ ] **Step 4: Run the final diff guard and commit verification fixes**

Run: `git diff --check; git status --short`

Expected: no whitespace errors; `private.wx336e7a90d023878f.key` remains untracked and unstaged.

If verification required edits to the files already listed in this plan, stage only those files and commit:

```powershell
git commit -m "fix: polish miniapp responsive layouts"
```
