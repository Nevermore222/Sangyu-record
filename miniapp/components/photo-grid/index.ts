Component({
  properties: {
    items: { type: Array, value: [] }
  },
  methods: {
    preview(event: WechatMiniprogram.BaseEvent) {
      const current = event.currentTarget.dataset.path as string
      const urls = (this.properties.items as Array<{ filePath: string }>).map((item) => item.filePath)
      void wx.previewImage({ current, urls })
    }
  }
})
