Component({
  properties: {
    items: {
      type: Array,
      value: [],
      observer(items: Array<{ status: string }>) {
        const total = items.length
        const completed = items.filter((item) => item.status === 'collected' || item.status === 'not_needed').length
        this.setData({ total, completed, percent: total > 0 ? Math.round(completed / total * 100) : 0 })
      }
    }
  },
  data: { total: 0, completed: 0, percent: 0 }
})
