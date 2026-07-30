import { presentProjectRow } from '../../domain/presenters'

Component({
  properties: {
    project: {
      type: Object,
      value: {},
      observer(value: { id?: string; display_name?: string; birth_year?: number; state?: string; updated_at?: string }) {
        if (!value?.id) return
        this.setData({ view: presentProjectRow(value as Required<typeof value>) })
      }
    },
    showAction: { type: Boolean, value: true }
  },
  data: {
    view: null as ReturnType<typeof presentProjectRow> | null
  },
  methods: {
    open() {
      this.triggerEvent('open', { id: this.data.view?.id })
    }
  }
})
