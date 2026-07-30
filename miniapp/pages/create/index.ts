import type { CreateProjectInput } from '../../services/api'
import { api } from '../../services/client'

Page({
  data: {
    step: 1,
    submitting: false,
    error: '',
    createdProjectID: '',
    editions: ['精简版', '标准版', '长篇版'],
    editionIndex: 1,
    confirmedBy: 'elder' as 'elder' | 'guardian',
    form: {
      display_name: '',
      birth_year: '',
      birth_place: '',
      long_term_residence: '',
      primary_occupation: ''
    }
  },

  onInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as string
    this.setData({ [`form.${field}`]: event.detail.value, error: '' })
  },

  onEdition(event: WechatMiniprogram.PickerChange) {
    this.setData({ editionIndex: Number(event.detail.value) })
  },

  onConsent(event: WechatMiniprogram.RadioGroupChange) {
    this.setData({ confirmedBy: event.detail.value as 'elder' | 'guardian' })
  },

  nextStep() {
    const error = this.validateBasics()
    if (error) {
      this.setData({ error })
      return
    }
    this.setData({ step: 2, error: '' })
  },

  previousStep() { this.setData({ step: 1, error: '' }) },

  validateBasics(): string {
    const year = Number(this.data.form.birth_year)
    if (!this.data.form.display_name.trim()) return '请填写姓名或档案代号'
    if (!Number.isInteger(year) || year < 1900 || year > new Date().getFullYear()) return '请填写有效的出生年份'
    if (!this.data.form.birth_place.trim()) return '请填写出生地'
    if (!this.data.form.long_term_residence.trim()) return '请填写长期生活地区'
    return ''
  },

  async submit() {
    if (this.data.submitting) return
    this.setData({ submitting: true, error: '' })
    try {
      let projectID = this.data.createdProjectID
      if (!projectID) {
        const editions: CreateProjectInput['target_edition'][] = ['brief', 'standard', 'long']
        const project = await api.createProject({
          display_name: this.data.form.display_name.trim(),
          birth_year: Number(this.data.form.birth_year),
          birth_place: this.data.form.birth_place.trim(),
          long_term_residence: this.data.form.long_term_residence.trim(),
          primary_occupation: this.data.form.primary_occupation.trim(),
          target_edition: editions[this.data.editionIndex]
        })
        projectID = project.id
        this.setData({ createdProjectID: projectID })
      }
      await api.confirmConsent(projectID, this.data.confirmedBy)
      await wx.redirectTo({ url: `/pages/project/index?id=${projectID}` })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '档案建立失败', submitting: false })
    }
  }
})
