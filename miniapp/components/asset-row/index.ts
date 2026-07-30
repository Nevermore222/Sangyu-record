Component({
  properties: {
    item: { type: Object, value: {} },
    editable: { type: Boolean, value: false },
    removable: { type: Boolean, value: false },
    associationOptions: { type: Array, value: [] }
  },
  methods: {
    rename(event: WechatMiniprogram.InputBlur) {
      const item = this.properties.item as { localID?: string }
      this.triggerEvent('rename', { localID: item.localID, name: event.detail.value })
    },
    remove() {
      const item = this.properties.item as { localID?: string }
      this.triggerEvent('remove', { localID: item.localID })
    },
    associate(event: WechatMiniprogram.PickerChange) {
      const item = this.properties.item as { localID?: string }
      this.triggerEvent('associate', { localID: item.localID, index: Number(event.detail.value) })
    }
  }
})
