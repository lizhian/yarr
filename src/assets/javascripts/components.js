'use strict';

Vue.component('setting-field', {
  props: ['label', 'value', 'href', 'draft', 'placeholder'],
  template: `
    <div class="setting-field">
      <div class="settings-dialog-field">
        <a v-if="href" class="settings-dialog-field-label" :href="href" target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">{{ label }}</a>
        <span v-else class="settings-dialog-field-label">{{ label }}</span>
        <button v-if="!draft.editing" type="button" class="settings-dialog-field-value-action" :aria-label="'编辑' + label" @click="edit">{{ value || '未设置' }}</button>
        <div v-else class="setting-field-editor">
          <input class="form-control" type="text" :aria-label="label" v-model="draft.value" :disabled="draft.saving" :placeholder="placeholder" @keydown.enter.stop.prevent="$emit('save')" @keydown.esc.stop.prevent="cancel" v-focus>
          <div class="setting-field-actions">
            <button type="button" class="btn btn-default" :disabled="draft.saving" @click="$emit('save')">{{ draft.saving ? '保存中' : '保存' }}</button>
            <button type="button" class="btn btn-link" :disabled="draft.saving" @click="cancel">取消</button>
          </div>
          <p v-if="draft.error" class="text-danger setting-field-error" role="alert">{{ draft.error }}</p>
        </div>
      </div>
    </div>
  `,
  methods: {
    edit: function() {
      this.draft.value = this.value || ''
      this.draft.editing = true
      this.draft.error = ''
    },
    cancel: function() {
      if (this.draft.saving) return
      this.draft.value = this.value || ''
      this.draft.editing = false
      this.draft.error = ''
    },
  },
})

Vue.component('drag', {
  props: ['width'],
  template: '<div class="drag"></div>',
  mounted: function() {
    var self = this
    var startX = undefined
    var initW = undefined
    var onMouseMove = function(e) {
      var offset = e.clientX - startX
      var newWidth = initW + offset
      self.$emit('resize', newWidth)
    }
    var onMouseUp = function(e) {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }
    this.$el.addEventListener('mousedown', function(e) {
      startX = e.clientX
      initW = self.width
      document.addEventListener('mousemove', onMouseMove)
      document.addEventListener('mouseup', onMouseUp)
    })
  },
})

Vue.component('modal', {
  props: ['open'],
  template: `
    <div class="modal custom-modal" tabindex="-1" role="dialog" aria-modal="true" aria-label="确认操作" v-if="$props.open" @keydown="handleKey">
      <div class="modal-dialog">
        <div class="modal-content" ref="content">
          <div class="modal-body">
            <slot v-if="$props.open"></slot>
          </div>
        </div>
      </div>
    </div>
  `,
  data: function() {
    return {opening: false, returnFocus: null}
  },
  beforeDestroy: function() {
    document.removeEventListener('click', this.handleClick)
  },
  watch: {
    'open': function(newVal) {
      if (newVal) {
        this.opening = true
        this.returnFocus = document.activeElement
        document.addEventListener('click', this.handleClick)
        this.$nextTick(function() {
          var button = this.$el.querySelector('button')
          if (button) button.focus()
        })
      } else {
        document.removeEventListener('click', this.handleClick)
        this.$nextTick(function() {
          if (this.returnFocus && this.returnFocus.isConnected) this.returnFocus.focus({preventScroll: true})
        })
      }
    },
  },
  methods: {
    handleKey: function(event) {
      event.stopPropagation()
      if (event.key === 'Escape') {
        event.preventDefault()
        this.$emit('hide')
      }
      if (event.key !== 'Tab') return
      var buttons = this.$el.querySelectorAll('button:not(:disabled)')
      var first = buttons[0], last = buttons[buttons.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    },
    handleClick: function(e) {
      if (this.opening) {
        this.opening = false
        return
      }
      if (e.target.closest('.custom-modal') !== this.$el) return
      if (e.target.closest('.modal-content') == null) this.$emit('hide')
    },
  },
})

function dateRepr(d, maxDays) {
  var sec = (new Date().getTime() - d.getTime()) / 1000
  var neg = sec < 0
  var out = ''

  sec = Math.abs(sec)
  if (sec < 2700)  // less than 45 minutes
    out = Math.round(sec / 60) + 'm'
  else if (sec < 86400)  // less than 24 hours
    out = Math.round(sec / 3600) + 'h'
  else if (maxDays)
    out = Math.min(Math.round(sec / 86400), maxDays) + 'd'
  else if (sec < 604800)  // less than a week
    out = Math.round(sec / 86400) + 'd'
  else
    out = d.toLocaleDateString(undefined, {year: "numeric", month: "long", day: "numeric"})

  if (neg) return '-' + out
  return out
}

Vue.component('relative-time', {
  props: ['val', 'maxDays'],
  data: function() {
    var d = new Date(this.val)
    return {
      'date': d,
      'formatted': dateRepr(d, this.maxDays),
      'interval': null,
    }
  },
  template: '<time :datetime="val">{{ formatted }}</time>',
  mounted: function() {
    this.interval = setInterval(function() {
      this.formatted = dateRepr(this.date, this.maxDays)
    }.bind(this), 600000)  // every 10 minutes
  },
  destroyed: function() {
    clearInterval(this.interval)
  },
})
