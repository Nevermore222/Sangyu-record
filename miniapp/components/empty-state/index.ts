Component({
  properties: {
    title: { type: String, value: '暂无内容' },
    detail: { type: String, value: '' },
    actionLabel: { type: String, value: '' }
  },
  methods: {
    handleAction() {
      this.triggerEvent('action')
    }
  }
})
