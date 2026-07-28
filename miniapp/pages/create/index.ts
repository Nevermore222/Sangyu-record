import type { CreateProjectInput } from '../../services/api'
import { api } from '../../services/client'

Page({
  data: {
    submitting: false,
    error: '',
    editions: ['精简版', '标准版', '长篇版'],
    editionIndex: 1,
    form: {
      display_name: '', birth_year: '', birth_place: '',
      long_term_residence: '', primary_occupation: ''
    }
  },

  onInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as string
    this.setData({ [`form.${field}`]: event.detail.value })
  },

  onEdition(event: WechatMiniprogram.PickerChange) {
    this.setData({ editionIndex: Number(event.detail.value) })
  },

  async submit() {
    if (this.data.submitting) return
    this.setData({ submitting: true, error: '' })
    try {
      const editions: CreateProjectInput['target_edition'][] = ['brief', 'standard', 'long']
      const project = await api.createProject({
        ...this.data.form,
        birth_year: Number(this.data.form.birth_year),
        target_edition: editions[this.data.editionIndex]
      })
      const ids = (wx.getStorageSync('recentProjectIDs') || []) as string[]
      wx.setStorageSync('recentProjectIDs', [project.id, ...ids.filter((id) => id !== project.id)])
      void wx.redirectTo({ url: `/pages/project/index?id=${project.id}` })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '创建失败', submitting: false })
    }
  }
})
