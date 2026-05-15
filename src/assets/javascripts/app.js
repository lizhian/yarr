'use strict';

var TITLE = document.title

function scrollto(target, scroll) {
  var padding = 10
  var targetRect = target.getBoundingClientRect()
  var scrollRect = scroll.getBoundingClientRect()

  // target
  var relativeOffset = targetRect.y - scrollRect.y
  var absoluteOffset = relativeOffset + scroll.scrollTop

  if (padding <= relativeOffset && relativeOffset + targetRect.height <= scrollRect.height - padding) return

  var newPos = scroll.scrollTop
  if (relativeOffset < padding) {
    newPos = absoluteOffset - padding
  } else {
    newPos = absoluteOffset - scrollRect.height + targetRect.height + padding
  }
  scroll.scrollTop = Math.round(newPos)
}

var debounce = function(callback, wait) {
  var timeout
  return function() {
    var ctx = this, args = arguments
    clearTimeout(timeout)
    timeout = setTimeout(function() {
      callback.apply(ctx, args)
    }, wait)
  }
}

Vue.directive('scroll', {
  inserted: function(el, binding) {
    el.addEventListener('scroll', debounce(function(event) {
      binding.value(event, el)
    }, 200))
  },
})

Vue.directive('focus', {
  inserted: function(el) {
    el.focus()
  }
})

function isMobileLayout() {
  return window.matchMedia && window.matchMedia('(max-width: 767.98px)').matches
}

function isDesktopLayout() {
  return window.matchMedia && window.matchMedia('(min-width: 992px)').matches
}

var FONT_OPTIONS = [
  {name: 'lxgw-wenkai', title: '霞鹜文楷'},
  {name: 'maple-mono-nf-cn', title: 'Maple Mono NF-CN'},
]

var CONTENT_MODE_OPTIONS = [
  {name: 'normal', title: '普通'},
  {name: 'readability', title: '正文'},
  {name: 'embed', title: '嵌入'},
]

function normalizeThemeFont(font) {
  return FONT_OPTIONS.some(function(option) { return option.name == font }) ? font : 'lxgw-wenkai'
}

function normalizeContentMode(mode) {
  return CONTENT_MODE_OPTIONS.some(function(option) { return option.name == mode }) ? mode : 'normal'
}

function normalizeRSSHubSubscriptionInput(raw) {
  raw = (raw || '').trim()
  if (!raw) return {value: raw, normalized: false}

  var bilibili = normalizeBilibiliSubscriptionInput(raw)
  if (bilibili.normalized) return bilibili

  var telegram = normalizeTelegramSubscriptionInput(raw)
  if (telegram.normalized) return telegram

  return {value: raw, normalized: false}
}

function normalizeBilibiliSubscriptionInput(raw) {
  var url = parseURL(raw)
  if (!url || (url.protocol != 'http:' && url.protocol != 'https:') || url.hostname.toLowerCase() != 'space.bilibili.com') {
    return {value: raw, normalized: false}
  }
  var parts = url.pathname.replace(/^\/+|\/+$/g, '').split('/').filter(Boolean)
  if (!parts.length || !/^\d+$/.test(parts[0])) return {value: raw, normalized: false}
  if (parts.length == 1 || (parts.length == 2 && parts[1] == 'dynamic') || (parts.length == 3 && parts[1] == 'upload' && parts[2] == 'video')) {
    return {value: 'rsshub://bilibili/user/video/' + parts[0], normalized: true}
  }
  return {value: raw, normalized: false}
}

function normalizeTelegramSubscriptionInput(raw) {
  var url = parseURL(raw)
  if (!url || (url.protocol != 'http:' && url.protocol != 'https:')) return {value: raw, normalized: false}
  var host = url.hostname.toLowerCase()
  if (host != 't.me' && host != 'telegram.me') return {value: raw, normalized: false}
  var parts = url.pathname.replace(/^\/+|\/+$/g, '').split('/').filter(Boolean)
  if (parts.length == 1 && parts[0] != 's' && /^[A-Za-z0-9_]+$/.test(parts[0]) && parts[0][0] != '+') {
    return {value: 'rsshub://telegram/channel/' + parts[0], normalized: true}
  }
  if (parts.length == 2 && parts[0] == 's' && /^[A-Za-z0-9_]+$/.test(parts[1])) {
    return {value: 'rsshub://telegram/channel/' + parts[1], normalized: true}
  }
  return {value: raw, normalized: false}
}

function normalizeBilibiliQuickAddInput(raw) {
  raw = (raw || '').trim()
  if (/^\d+$/.test(raw)) {
    return {value: 'rsshub://bilibili/user/video/' + raw, normalized: true}
  }
  return normalizeBilibiliSubscriptionInput(raw)
}

function normalizeTelegramQuickAddInput(raw) {
  raw = (raw || '').trim()
  var id = raw.replace(/^@/, '')
  if (/^[A-Za-z0-9_]+$/.test(id)) {
    return {value: 'rsshub://telegram/channel/' + id, normalized: true}
  }
  return normalizeTelegramSubscriptionInput(raw)
}

function parseURL(raw) {
  try {
    return new URL(raw)
  } catch (e) {
    return null
  }
}

var ARTICLE_LIST_LAYOUTS_KEY = 'yarr.articleListLayouts.v1'

function normalizeArticleListLayout(layout) {
  return layout == 'card' ? 'card' : 'list'
}

function articleListLayoutStorageKey(feedSelected) {
  return feedSelected || 'all'
}

function readArticleListLayouts() {
  try {
    var layouts = JSON.parse(localStorage.getItem(ARTICLE_LIST_LAYOUTS_KEY) || '{}')
    return layouts && typeof layouts == 'object' && !Array.isArray(layouts) ? layouts : {}
  } catch (e) {
    return {}
  }
}

function writeArticleListLayouts(layouts) {
  try {
    localStorage.setItem(ARTICLE_LIST_LAYOUTS_KEY, JSON.stringify(layouts))
  } catch (e) {}
}

function getArticleListLayout(feedSelected) {
  return normalizeArticleListLayout(readArticleListLayouts()[articleListLayoutStorageKey(feedSelected)])
}

function setArticleListLayout(feedSelected, layout) {
  var layouts = readArticleListLayouts()
  layouts[articleListLayoutStorageKey(feedSelected)] = normalizeArticleListLayout(layout)
  writeArticleListLayouts(layouts)
}

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

Vue.component('dropdown', {
  props: ['class', 'toggle-class', 'ref', 'drop', 'title'],
  data: function() {
    return {open: false}
  },
  template: `
    <div class="dropdown" :class="$attrs.class">
      <button ref="btn" @click="toggle" :class="btnToggleClass" :title="$props.title"><slot name="button"></slot></button>
      <div ref="menu" class="dropdown-menu" :class="{show: open}"><slot v-if="open"></slot></div>
    </div>
  `,
  computed: {
    btnToggleClass: function() {
      var c = this.$props.toggleClass || ''
      c += ' dropdown-toggle dropdown-toggle-no-caret'
      c += this.open ? ' show' : ''
      return c.trim()
    }
  },
  methods: {
    toggle: function(e) {
      this.open ? this.hide() : this.show()
    },
    show: function(e) {
      this.open = true
      this.$refs.menu.style.top = this.$refs.btn.offsetHeight + 'px'
      var drop = this.$props.drop

      if (drop === 'right') {
        this.$refs.menu.style.left = 'auto'
        this.$refs.menu.style.right = '0'
      } else if (drop === 'center') {
        this.$nextTick(function() {
          var btnWidth = this.$refs.btn.getBoundingClientRect().width
          var menuWidth = this.$refs.menu.getBoundingClientRect().width
          this.$refs.menu.style.left = '-' + ((menuWidth - btnWidth) / 2) + 'px'
        }.bind(this))
      }

      document.addEventListener('click', this.clickHandler)
    },
    hide: function() {
      this.open = false
      document.removeEventListener('click', this.clickHandler)
    },
    clickHandler: function(e) {
      var dropdown = e.target.closest('.dropdown')
      if (dropdown == null || dropdown != this.$el) return this.hide()
      if (e.target.closest('.dropdown-item') != null) return this.hide()
    }
  },
})

Vue.component('modal', {
  props: ['open'],
  template: `
    <div class="modal custom-modal" tabindex="-1" role="dialog" aria-modal="true" v-if="$props.open">
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
    return {opening: false}
  },
  watch: {
    'open': function(newVal) {
      if (newVal) {
        this.opening = true
        document.addEventListener('click', this.handleClick)
      } else {
        document.removeEventListener('click', this.handleClick)
      }
    },
  },
  methods: {
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

function dateRepr(d) {
  var sec = (new Date().getTime() - d.getTime()) / 1000
  var neg = sec < 0
  var out = ''

  sec = Math.abs(sec)
  if (sec < 2700)  // less than 45 minutes
    out = Math.round(sec / 60) + 'm'
  else if (sec < 86400)  // less than 24 hours
    out = Math.round(sec / 3600) + 'h'
  else if (sec < 604800)  // less than a week
    out = Math.round(sec / 86400) + 'd'
  else
    out = d.toLocaleDateString(undefined, {year: "numeric", month: "long", day: "numeric"})

  if (neg) return '-' + out
  return out
}

Vue.component('relative-time', {
  props: ['val'],
  data: function() {
    var d = new Date(this.val)
    return {
      'date': d,
      'formatted': dateRepr(d),
      'interval': null,
    }
  },
  template: '<time :datetime="val">{{ formatted }}</time>',
  mounted: function() {
    this.interval = setInterval(function() {
      this.formatted = dateRepr(this.date)
    }.bind(this), 600000)  // every 10 minutes
  },
  destroyed: function() {
    clearInterval(this.interval)
  },
})

var vm = new Vue({
  created: function() {
    this.refreshStats()
      .then(this.refreshFeeds.bind(this))
      .then(this.refreshItems.bind(this, false))

    this.scheduleStatusPoll(60000)

    api.feeds.list_errors().then(function(errors) {
      vm.feed_errors = errors
    })
    this.updateMetaTheme(this.theme.name)
    this.updateBodyClass()
  },
  mounted: function() {
    this.initNavigationHistory()
    if (this.$refs.itemlist) {
      this.$refs.itemlist.addEventListener('scroll', this.handleItemListScroll, {passive: true})
    }
  },
  beforeDestroy: function() {
    clearTimeout(this.statusPollTimeout)
    if (this.$refs.itemlist) {
      this.$refs.itemlist.removeEventListener('scroll', this.handleItemListScroll)
    }
  },
  data: function() {
    var s = app.settings
    return {
      'filterSelected': s.filter,
      'folders': [],
      'feeds': [],
      'feedSelected': s.feed,
      'feedListWidth': s.feed_list_width || 300,
      'feedIconErrors': {},
      'feedNewChoice': [],
      'feedNewChoiceSelected': '',
      'feedNewContentMode': 'normal',
      'feedDeleteSelectedIds': [],
      'items': [],
      'itemsHasMore': true,
      'itemsAutoReadSeen': {},
      'itemsAutoReadPending': {},
      'itemListLastScrollTop': 0,
      'itemSelected': null,
      'itemSelectedDetails': null,
      'itemSelectedReadability': '',
      'itemSelectedReadabilityError': '',
      'itemSelectedContentMode': 'normal',
      'itemSearch': '',
      'itemSortNewestFirst': s.sort_newest_first,
      'itemListWidth': s.item_list_width || 300,
      'articleListLayout': getArticleListLayout(s.feed),
      'articleListLayoutApplying': false,
      'rsshubBaseUrl': s.rsshub_base_url || '',
      'authConfig': {
        enabled: app.authenticated,
        username: '',
      },
      'authForm': {
        enabled: app.authenticated,
        username: '',
        password: '',
      },

      'filteredFeedStats': {},
      'filteredFolderStats': {},
      'filteredTotalStats': null,

      'settings': '',
      'settingsFeed': null,
      'settingsFolder': null,
      'dialog': {
        open: false,
        type: 'alert',
        title: '',
        message: '',
        inputValue: '',
        inputType: 'text',
        confirmText: '确定',
        cancelText: '取消',
        danger: false,
        resolve: null,
      },
      'loading': {
        'feeds': 0,
        'newfeed': false,
        'deletefeeds': false,
        'items': false,
        'readability': false,
        'backup': false,
        'icons': false,
        'feedIcon': null,
      },
      'feedStats': {},
      'theme': {
        'name': s.theme_name,
        'font': normalizeThemeFont(s.theme_font),
        'size': s.theme_size,
      },
      'themeColors': {
        'night': '#1f1f1f',
        'sepia': '#f2e6bd',
        'light': '#f7f7f5',
      },
      'refreshRate': s.refresh_rate,
      'toolbarDisplay': s.toolbar_display == 'icon' ? 'icon' : 'text',
      'fontOptions': FONT_OPTIONS,
      'contentModeOptions': CONTENT_MODE_OPTIONS,
      'authenticated': app.authenticated,
      'feed_errors': {},
      'navigationHistory': {
        initialized: false,
        applyingPop: false,
        syncPending: false,
        layer: null,
      },
      'statusPollTimeout': null,

      'refreshRateOptions': [
        { title: "0", value: 0 },
        { title: "1m", value: 1 },
        { title: "5m", value: 5 },
        { title: "10m", value: 10 },
        { title: "30m", value: 30 },
        { title: "1h", value: 60 },
      ],
    }
  },
  computed: {
    foldersWithFeeds: function() {
      var feedsByFolders = this.feeds.reduce(function(folders, feed) {
        if (!folders[feed.folder_id])
          folders[feed.folder_id] = [feed]
        else
          folders[feed.folder_id].push(feed)
        return folders
      }, {})
      var folders = this.folders.slice().map(function(folder) {
        folder.feeds = feedsByFolders[folder.id]
        return folder
      })
      folders.push({id: null, feeds: feedsByFolders[null]})
      return folders
    },
    feedDeleteGroups: function() {
      return this.foldersWithFeeds
        .filter(function(folder) {
          return folder.feeds && folder.feeds.length
        })
        .map(function(folder) {
          return {
            id: folder.id,
            title: folder.id ? folder.title : '无文件夹',
            feeds: folder.feeds,
          }
        })
    },
    feedsById: function() {
      return this.feeds.reduce(function(acc, f) { acc[f.id] = f; return acc }, {})
    },
    foldersById: function() {
      return this.folders.reduce(function(acc, f) { acc[f.id] = f; return acc }, {})
    },
    current: function() {
      var parts = (this.feedSelected || '').split(':', 2)
      var type = parts[0]
      var guid = parts[1]

      var folder = {}, feed = {}

      if (type == 'feed')
        feed = this.feedsById[guid] || {}
      if (type == 'folder')
        folder = this.foldersById[guid] || {}

      return {type: type, feed: feed, folder: folder}
    },
    itemSelectedContent: function() {
      if (!this.itemSelected) return ''

      if (this.itemSelectedContentMode == 'readability')
        return this.itemSelectedReadability

      return this.itemSelectedDetails.content || ''
    },
    toolbarNarrow: function() {
      return this.feedListWidth < 280 || this.itemListWidth < 280
    },
    toolbarFeedActionsNarrow: function() {
      return this.feedListWidth < 260
    },
    toolbarFeedActionsMinimal: function() {
      return this.feedListWidth < 230
    },
    showBottomMarkItemsRead: function() {
      return this.filterSelected == 'unread' &&
        this.items.length > 0 &&
        !this.itemsHasMore &&
        !this.loading.items
    },
  },
  watch: {
    'theme': {
      deep: true,
      handler: function(theme) {
        this.updateMetaTheme(theme.name)
        this.updateBodyClass()
        api.settings.update({
          theme_name: theme.name,
          theme_font: theme.font,
          theme_size: theme.size,
        })
      },
    },
    'feedStats': {
      deep: true,
      handler: debounce(function() {
        var title = TITLE
        var unreadCount = Object.values(this.feedStats).reduce(function(acc, stat) {
          return acc + stat.unread
        }, 0)
        if (unreadCount) {
          title += ' ('+unreadCount+')'
        }
        document.title = title
        this.computeStats()
      }, 500),
    },
    'filterSelected': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({filter: newVal}).then(this.refreshItems.bind(this, false))
      this.itemSelected = null
      this.computeStats()
      this.syncNavigationHistory()
    },
    'feedSelected': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      var layout = getArticleListLayout(newVal)
      if (this.articleListLayout != layout) {
        this.articleListLayoutApplying = true
        this.articleListLayout = layout
      }
      api.settings.update({feed: newVal}).then(this.refreshItems.bind(this, false))
      this.itemSelected = null
      if (this.$refs.itemlist) this.$refs.itemlist.scrollTop = 0
      this.syncNavigationHistory()
    },
    'itemSelected': function(newVal, oldVal) {
      this.itemSelectedReadability = ''
      this.itemSelectedReadabilityError = ''
      this.itemSelectedContentMode = 'normal'
      this.loading.readability = false
      if (newVal === null) {
        this.itemSelectedDetails = null
        this.syncNavigationHistory()
        return
      }
      if (this.$refs.content) this.$refs.content.scrollTop = 0
      this.syncNavigationHistory()

      api.items.get(newVal).then(function(item) {
        if (this.itemSelected !== newVal) return
        this.itemSelectedDetails = item
        this.itemSelectedContentMode = normalizeContentMode((this.feedsById[item.feed_id] || {}).content_mode)
        this.loadSelectedContentMode()
        this.markItemRead(this.itemSelectedDetails)
      }.bind(this)).catch(function() {
        if (this.itemSelected === newVal) this.itemSelected = null
      }.bind(this))
    },
    'itemSearch': debounce(function(newVal) {
      this.refreshItems()
    }, 500),
    'itemSortNewestFirst': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({sort_newest_first: newVal}).then(vm.refreshItems.bind(this, false))
    },
    'feedListWidth': debounce(function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({feed_list_width: newVal})
    }, 1000),
    'itemListWidth': debounce(function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({item_list_width: newVal})
    }, 1000),
    'refreshRate': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({refresh_rate: newVal})
    },
    'toolbarDisplay': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({toolbar_display: newVal})
    },
    'articleListLayout': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      if (this.articleListLayoutApplying) {
        this.articleListLayoutApplying = false
        return
      }
      setArticleListLayout(this.feedSelected, newVal)
    },
  },
  methods: {
    currentNavigationLayer: function() {
      if (this.itemSelected !== null) return 'item'
      if (this.feedSelected !== null) return 'items'
      return 'feeds'
    },
    navigationLayerRank: function(layer) {
      return {
        feeds: 0,
        items: 1,
        item: 2,
      }[layer]
    },
    navigationState: function(layer) {
      return {
        yarr: true,
        layer: layer,
        feedSelected: this.feedSelected,
        itemSelected: this.itemSelected,
      }
    },
    canUseNavigationHistory: function() {
      return isMobileLayout() &&
        window.history &&
        typeof window.history.pushState === 'function' &&
        typeof window.history.replaceState === 'function'
    },
    initNavigationHistory: function() {
      if (this.navigationHistory.initialized || !this.canUseNavigationHistory()) return

      var layer = this.currentNavigationLayer()
      window.history.replaceState(this.navigationState('feeds'), document.title)

      if (this.navigationLayerRank(layer) >= this.navigationLayerRank('items')) {
        window.history.pushState(this.navigationState('items'), document.title)
      }
      if (layer === 'item') {
        window.history.pushState(this.navigationState('item'), document.title)
      }

      this.navigationHistory.initialized = true
      this.navigationHistory.layer = layer
      window.addEventListener('popstate', this.handleNavigationPop)
    },
    syncNavigationHistory: function() {
      if (this.navigationHistory.applyingPop) return
      if (!this.navigationHistory.initialized) this.initNavigationHistory()
      if (!this.navigationHistory.initialized || !this.canUseNavigationHistory()) return
      if (this.navigationHistory.syncPending) return

      this.navigationHistory.syncPending = true
      this.$nextTick(function() {
        this.navigationHistory.syncPending = false
        this.applyNavigationHistorySync()
      })
    },
    applyNavigationHistorySync: function() {
      if (this.navigationHistory.applyingPop) return
      if (!this.navigationHistory.initialized || !this.canUseNavigationHistory()) return

      var oldLayer = this.navigationHistory.layer
      var newLayer = this.currentNavigationLayer()

      if (oldLayer === newLayer) {
        window.history.replaceState(this.navigationState(newLayer), document.title)
        return
      }

      var oldRank = this.navigationLayerRank(oldLayer)
      var newRank = this.navigationLayerRank(newLayer)

      if (newRank > oldRank) {
        if (oldLayer === 'feeds' && this.navigationLayerRank(newLayer) >= this.navigationLayerRank('items')) {
          window.history.pushState(this.navigationState('items'), document.title)
        }
        if (newLayer === 'item') {
          window.history.pushState(this.navigationState('item'), document.title)
        }
      } else {
        this.navigationHistory.applyingPop = true
        window.history.go(newRank - oldRank)
        setTimeout(function() {
          this.navigationHistory.applyingPop = false
        }.bind(this), 500)
      }

      this.navigationHistory.layer = newLayer
    },
    handleNavigationPop: function(event) {
      if (!isMobileLayout()) return
      if (!event.state || !event.state.yarr) return

      this.navigationHistory.applyingPop = true
      this.navigationHistory.layer = event.state.layer || 'feeds'

      if (event.state.layer === 'feeds') {
        this.itemSelected = null
        this.feedSelected = null
      } else if (event.state.layer === 'items') {
        this.feedSelected = event.state.feedSelected
        this.itemSelected = null
      } else if (event.state.layer === 'item') {
        this.feedSelected = event.state.feedSelected
        this.itemSelected = event.state.itemSelected
      }

      this.$nextTick(function() {
        this.navigationHistory.applyingPop = false
      })
    },
    closeItem: function() {
      if (this.itemSelected === null) return
      if (this.navigationHistory.initialized && this.canUseNavigationHistory()) {
        window.history.back()
        return
      }
      this.itemSelected = null
    },
    showFeedList: function() {
      if (this.feedSelected === null) return
      if (this.navigationHistory.initialized && this.canUseNavigationHistory() && this.currentNavigationLayer() === 'items') {
        window.history.back()
        return
      }
      this.itemSelected = null
      this.feedSelected = null
    },
    updateMetaTheme: function(theme) {
      document.querySelector("meta[name='theme-color']").content = this.themeColors[theme]
    },
    updateBodyClass: function() {
      document.body.classList.value = 'theme-' + this.theme.name + ' font-' + this.theme.font
    },
    themeTitle: function(theme) {
      return {
        light: '浅色',
        sepia: '护眼',
        night: '夜间',
      }[theme] || theme
    },
    scheduleStatusPoll: function(delay) {
      clearTimeout(this.statusPollTimeout)
      this.statusPollTimeout = setTimeout(function() {
        vm.refreshStats()
      }, delay)
    },
    refreshStats: function(loopMode) {
      return api.status().then(function(data) {
        if (loopMode && !vm.itemSelected) vm.refreshItems()

        vm.loading.feeds = data.running
        vm.scheduleStatusPoll(data.running ? 500 : 60000)
        vm.feedStats = data.stats.reduce(function(acc, stat) {
          acc[stat.feed_id] = stat
          return acc
        }, {})

        api.feeds.list_errors().then(function(errors) {
          vm.feed_errors = errors
        })
      })
    },
    getItemsQuery: function() {
      var query = {}
      if (this.feedSelected) {
        var parts = this.feedSelected.split(':', 2)
        var type = parts[0]
        var guid = parts[1]
        if (type == 'feed') {
          query.feed_id = guid
        } else if (type == 'folder') {
          query.folder_id = guid
        }
      }
      if (this.filterSelected) {
        query.status = this.filterSelected
      }
      if (this.itemSearch) {
        query.search = this.itemSearch
      }
      if (!this.itemSortNewestFirst) {
        query.oldest_first = true
      }
      return query
    },
    refreshFeeds: function() {
      return Promise
        .all([api.folders.list(), api.feeds.list()])
        .then(function(values) {
          vm.folders = values[0]
          vm.feeds = values[1]
        })
    },
    refreshItems: function(loadMore = false) {
      if (this.feedSelected === null) {
        vm.items = []
        vm.itemsHasMore = false
        vm.resetItemListAutoRead()
        return
      }

      var query = this.getItemsQuery()
      if (loadMore) {
        query.after = vm.items[vm.items.length-1].id
      } else {
        this.resetItemListAutoRead()
      }

      this.loading.items = true
      return api.items.list(query).then(function(data) {
        if (loadMore) {
          vm.items = vm.items.concat(data.list)
        } else {
          vm.items = data.list
        }
        vm.itemsHasMore = data.has_more
        vm.loading.items = false

        // load more if there's some space left at the bottom of the item list.
        vm.$nextTick(function() {
          vm.updateItemListAutoReadSeen()
          if (vm.itemsHasMore && !vm.loading.items && vm.itemListCloseToBottom()) {
            vm.refreshItems(true)
          }
        })
      })
    },
    itemListCloseToBottom: function() {
      // approx. vertical space at the bottom of the list (loading el & paddings) when 1rem = 16px
      var bottomSpace = 70
      var scale = (parseFloat(getComputedStyle(document.documentElement).fontSize) || 16) / 16

      var el = this.$refs.itemlist

      if (el.scrollHeight === 0) return false  // element is invisible (responsive design)

      var closeToBottom = (el.scrollHeight - el.scrollTop - el.offsetHeight) < bottomSpace * scale
      return closeToBottom
    },
    loadMoreItems: function(event, el) {
      if (!this.itemsHasMore) return
      if (this.loading.items) return
      if (this.itemListCloseToBottom()) return this.refreshItems(true)
      if (this.itemSelected && this.itemSelected === this.items[this.items.length - 1].id) return this.refreshItems(true)
    },
    handleItemListScroll: function(event) {
      this.markScrolledItemsRead(event.currentTarget)
    },
    resetItemListAutoRead: function() {
      this.itemsAutoReadSeen = {}
      this.itemsAutoReadPending = {}
      this.itemListLastScrollTop = this.$refs.itemlist ? this.$refs.itemlist.scrollTop : 0
    },
    canAutoReadItemList: function(el) {
      if (!el || el.scrollHeight === 0) return false
      if (isMobileLayout()) return this.currentNavigationLayer() === 'items'
      return isDesktopLayout()
    },
    updateItemListAutoReadSeen: function(el) {
      el = el || this.$refs.itemlist
      if (!this.canAutoReadItemList(el)) return
      if (this.filterSelected != '' && this.filterSelected != 'unread') return

      var scrollRect = el.getBoundingClientRect()
      var labels = el.querySelectorAll('.selectgroup[data-item-id]')
      for (var i = 0; i < labels.length; i++) {
        var label = labels[i]
        var item = this.items.find(function(item) { return item.id == label.dataset.itemId })
        if (!item || item.status != 'unread') continue

        var labelRect = label.getBoundingClientRect()
        var visibleHeight = Math.min(labelRect.bottom, scrollRect.bottom) - Math.max(labelRect.top, scrollRect.top)
        if (visibleHeight >= labelRect.height / 2) {
          this.itemsAutoReadSeen[item.id] = true
        }
      }
    },
    markScrolledItemsRead: function(el) {
      el = el || this.$refs.itemlist
      if (!el) return

      var scrollTop = el.scrollTop
      var scrollingDown = scrollTop > this.itemListLastScrollTop
      this.itemListLastScrollTop = scrollTop

      if (!this.canAutoReadItemList(el)) return
      if (this.filterSelected != '' && this.filterSelected != 'unread') return

      var scrollRect = el.getBoundingClientRect()
      var labels = el.querySelectorAll('.selectgroup[data-item-id]')
      if (scrollingDown) {
        for (var i = 0; i < labels.length; i++) {
          var label = labels[i]
          if (label.getBoundingClientRect().bottom > scrollRect.top) continue

          var item = this.items.find(function(item) { return item.id == label.dataset.itemId })
          if (!item || item.status != 'unread') continue
          if (!this.itemsAutoReadSeen[item.id]) continue

          this.markItemRead(item)
        }
      }

      this.updateItemListAutoReadSeen(el)
    },
    itemImage: function(item) {
      var link = (item.media_links || []).find(function(link) {
        return link.type === 'image' && link.url
      })
      return link && link.url
    },
    toggleArticleListLayout: function() {
      this.articleListLayout = this.articleListLayout == 'card' ? 'list' : 'card'
    },
    feedIconErrored: function(feed) {
      return !!this.feedIconErrors[feed.id + ':' + (feed.icon_url || '')]
    },
    markFeedIconErrored: function(feed) {
      this.$set(this.feedIconErrors, feed.id + ':' + (feed.icon_url || ''), true)
    },
    markItemsRead: function() {
      var query = this.getItemsQuery()
      api.items.mark_read(query).then(function() {
        vm.items = []
        vm.itemsPage = {'cur': 1, 'num': 1}
        vm.itemSelected = null
        vm.itemsHasMore = false
        vm.resetItemListAutoRead()
        vm.refreshStats()
      })
    },
    toggleFolderExpanded: function(folder) {
      folder.is_expanded = !folder.is_expanded
      api.folders.update(folder.id, {is_expanded: folder.is_expanded})
    },
    formatDate: function(datestr) {
      var options = {
        year: "numeric", month: "long", day: "numeric",
        hour: '2-digit', minute: '2-digit',
      }
      return new Date(datestr).toLocaleDateString(undefined, options)
    },
    moveFeed: function(feed, folder) {
      var folder_id = folder ? folder.id : null
      api.feeds.update(feed.id, {folder_id: folder_id}).then(function() {
        feed.folder_id = folder_id
        vm.refreshStats()
      })
    },
    moveFeedToNewFolder: function(feed) {
      this.promptDialog('请输入文件夹名称：').then(function(title) {
        if (!title) return
        api.folders.create({'title': title}).then(function(folder) {
          api.feeds.update(feed.id, {folder_id: folder.id}).then(function() {
            feed.folder_id = folder.id
            vm.settings = ''
            vm.refreshFeeds().then(function() {
              vm.refreshStats()
            })
          })
        })
      })
    },
    createNewFeedFolder: function() {
      this.promptDialog('请输入文件夹名称：').then(function(title) {
        if (!title) return
        api.folders.create({'title': title}).then(function(result) {
          vm.refreshFeeds().then(function() {
            vm.$nextTick(function() {
              if (vm.$refs.newFeedFolder) {
                vm.$refs.newFeedFolder.value = result.id
              }
            })
          })
        })
      })
    },
    renameFolder: function(folder) {
      this.promptDialog('请输入新名称', folder.title).then(function(newTitle) {
        if (!newTitle) return
        api.folders.update(folder.id, {title: newTitle}).then(function() {
          folder.title = newTitle
          this.folders.sort(function(a, b) {
            return a.title.localeCompare(b.title)
          })
        }.bind(this))
      }.bind(this))
    },
    deleteFolder: function(folder) {
      this.confirmDialog('确定删除文件夹「' + folder.title + '」吗？', '删除文件夹').then(function(confirmed) {
        if (!confirmed) return
        api.folders.delete(folder.id).then(function() {
          vm.settings = ''
          vm.settingsFolder = null
          vm.feedSelected = null
          vm.refreshStats()
          vm.refreshFeeds()
        })
      })
    },
    updateFeedLink: function(feed) {
      this.promptDialog('请输入订阅源链接', feed.feed_link).then(function(newLink) {
        if (!newLink) return
        api.feeds.update(feed.id, {feed_link: newLink}).then(function() {
          feed.feed_link = newLink
        })
      })
    },
    renameFeed: function(feed) {
      this.promptDialog('请输入新名称', feed.title).then(function(newTitle) {
        if (!newTitle) return
        api.feeds.update(feed.id, {title: newTitle}).then(function() {
          feed.title = newTitle
        })
      })
    },
    updateFeedContentSelector: function(feed) {
      this.promptDialog('请输入正文选择器', feed.content_selector || '').then(function(selector) {
        if (selector === null) return
        api.feeds.update(feed.id, {content_selector: selector}).then(function(res) {
          if (res.ok) {
            feed.content_selector = selector.trim()
          } else {
            vm.alertDialog('正文选择器格式不支持。')
          }
        })
      })
    },
    updateFeedContentMode: function(feed, mode) {
      mode = normalizeContentMode(mode)
      api.feeds.update(feed.id, {content_mode: mode}).then(function(res) {
        if (res.ok) {
          feed.content_mode = mode
        } else {
          vm.alertDialog('内容方式不支持。')
        }
      })
    },
    normalizeContentMode: normalizeContentMode,
    trimValue: function(value) {
      return (value || '').trim()
    },
    isHTTPURL: function(value) {
      return /^https?:\/\//i.test(this.trimValue(value))
    },
    updateFeedIconURL: function(feed) {
      this.promptDialog('请输入图标链接', feed.icon_url || '').then(function(iconURL) {
        if (iconURL === null) return
        api.feeds.update(feed.id, {icon_url: iconURL}).then(function(res) {
          if (res.ok) {
            feed.icon_url = iconURL.trim()
          } else {
            vm.alertDialog('订阅源图标链接必须是 HTTP(S) URL。')
          }
        })
      })
    },
    refreshFeedIcon: function(feed) {
      if (!feed || this.loading.feedIcon === feed.id) return
      this.loading.feedIcon = feed.id
      api.feeds.refresh_icon(feed.id).then(function(updatedFeed) {
        if (updatedFeed && updatedFeed.id) {
          feed.icon_url = updatedFeed.icon_url
        }
        vm.feedIconErrors = {}
      }).then(function() {
        vm.loading.feedIcon = null
      }, function() {
        vm.loading.feedIcon = null
      })
    },
    deleteFeed: function(feed) {
      this.confirmDialog('确定删除订阅源「' + feed.title + '」吗？', '删除订阅源').then(function(confirmed) {
        if (!confirmed) return
        api.feeds.delete(feed.id).then(function() {
          vm.settings = ''
          vm.settingsFeed = null
          vm.feedSelected = null
          vm.refreshStats()
          vm.refreshFeeds()
        })
      })
    },
    deleteSelectedFeeds: function() {
      var ids = this.feedDeleteSelectedIds.slice()
      if (!ids.length || this.loading.deletefeeds) return

      this.confirmDialog('确定删除 ' + ids.length + ' 个订阅源吗？', '删除订阅源').then(function(confirmed) {
        if (!confirmed) return

        vm.loading.deletefeeds = true
        Promise.all(ids.map(function(id) {
          return api.feeds.delete(id).then(function(res) {
            return res.ok
          }).catch(function() {
            return false
          })
        })).then(function(results) {
          var failed = results.some(function(ok) { return !ok })
          vm.settings = ''
          vm.settingsFeed = null
          vm.feedDeleteSelectedIds = []
          vm.feedSelected = null
          vm.itemSelected = null
          return vm.refreshFeeds().then(function() {
            vm.refreshStats()
            if (failed) vm.alertDialog('部分订阅源删除失败。')
          })
        }).then(function() {
          vm.loading.deletefeeds = false
        }).catch(function() {
          vm.loading.deletefeeds = false
          vm.alertDialog('部分订阅源删除失败。')
        })
      })
    },
    createFeed: function(event) {
      var form = event.target
      var data = {
        url: normalizeRSSHubSubscriptionInput(form.querySelector('input[name=url]').value).value,
        folder_id: parseInt(form.querySelector('select[name=folder_id]').value) || null,
        content_selector: form.querySelector('input[name=content_selector]').value,
        content_mode: this.feedNewContentMode,
      }
      if (this.feedNewChoiceSelected) {
        data.url = this.feedNewChoiceSelected
      }
      this.createFeedFromData(data, true)
    },
    createFeedFromData: function(data, allowChoice) {
      this.loading.newfeed = true
      api.feeds.create(data).then(function(result) {
        if (result.status === 'success') {
          vm.refreshFeeds()
          vm.refreshStats()
          vm.settings = ''
          vm.feedSelected = 'feed:' + result.feed.id
        } else if (allowChoice && result.status === 'multiple') {
          vm.feedNewChoice = result.choice
          vm.feedNewChoiceSelected = result.choice[0].url
        } else if (result.status === 'error') {
          vm.alertDialog(result.message || '无法添加订阅源。')
        } else if (result.error) {
          vm.alertDialog(result.error)
        } else {
          vm.alertDialog('未在给定 URL 找到订阅源。')
        }
        vm.loading.newfeed = false
      })
    },
    createRSSHubFeed: function(kind) {
      var config = {
        bilibili: {
          prompt: '请输入 Bilibili UID 或空间链接',
          normalize: normalizeBilibiliQuickAddInput,
        },
        telegram: {
          prompt: '请输入 Telegram 频道 ID 或 t.me 链接',
          normalize: normalizeTelegramQuickAddInput,
        },
      }[kind]
      if (!config) return

      this.promptDialog(config.prompt).then(function(value) {
        var normalized = config.normalize((value || '').trim())
        if (!normalized.value) return
        if (!normalized.normalized) {
          vm.alertDialog('无法识别 UID/频道 ID。')
          return
        }

        var folderId = vm.current.feed.folder_id || vm.current.folder.id || null
        vm.createFeedFromData({
          url: normalized.value,
          folder_id: folderId,
        }, false)
      })
    },
    toggleItemStatus: function(item, targetstatus, fallbackstatus) {
      var oldstatus = item.status
      var newstatus = item.status !== targetstatus ? targetstatus : fallbackstatus

      var updateStats = function(status, incr) {
        if ((status == 'unread') || (status == 'starred')) {
          this.feedStats[item.feed_id][status] += incr
        }
      }.bind(this)

      api.items.update(item.id, {status: newstatus}).then(function() {
        updateStats(oldstatus, -1)
        updateStats(newstatus, +1)

        var itemInList = this.items.find(function(i) { return i.id == item.id })
        if (itemInList) itemInList.status = newstatus
        item.status = newstatus
      }.bind(this))
    },
    markItemRead: function(item) {
      if (!item || item.status != 'unread') return Promise.resolve()
      if (this.itemsAutoReadPending[item.id]) return Promise.resolve()

      this.itemsAutoReadPending[item.id] = true

      return api.items.update(item.id, {status: 'read'}).then(function() {
        var feedStats = this.feedStats[item.feed_id]
        if (feedStats && feedStats.unread > 0) feedStats.unread -= 1

        var itemInList = this.items.find(function(i) { return i.id == item.id })
        if (itemInList) itemInList.status = 'read'
        if (this.itemSelectedDetails && this.itemSelectedDetails.id == item.id) {
          this.itemSelectedDetails.status = 'read'
        }
        item.status = 'read'
      }.bind(this)).catch(function() {
        // Keep the article unread; a later scroll or selection can retry.
      }).then(function() {
        delete this.itemsAutoReadPending[item.id]
      }.bind(this))
    },
    toggleItemStarred: function(item) {
      this.toggleItemStatus(item, 'starred', 'read')
    },
    toggleItemRead: function(item) {
      this.toggleItemStatus(item, 'unread', 'read')
    },
    importOPML: function(event) {
      var input = event.target
      var form = document.querySelector('#opml-import-form')
      this.settings = ''
      api.upload_opml(form).then(function() {
        input.value = ''
        vm.refreshFeeds()
        vm.refreshStats()
      })
    },
    logout: function() {
      api.logout().then(function() {
        document.location.reload()
      })
    },
    loadAuthConfig: function() {
      return api.auth.get().then(function(config) {
        vm.authConfig = config
        vm.authForm.enabled = config.enabled
        vm.authForm.username = config.username || ''
        vm.authForm.password = ''
        vm.authenticated = config.enabled
      })
    },
    updateAuthConfig: function() {
      var payload = {
        enabled: this.authForm.enabled,
        username: this.authForm.username,
        password: this.authForm.password,
      }
      if (!payload.enabled) {
        this.confirmDialog('关闭访问认证后将清空已保存的用户名和密码。', '关闭访问认证').then(function(confirmed) {
          if (!confirmed) return
          api.auth.update({enabled: false}).then(function(res) {
            if (res.ok) document.location.reload()
            else vm.alertDialog('未能关闭访问认证。')
          })
        })
        return
      }
      api.auth.update(payload).then(function(res) {
        if (res.ok) {
          document.location.reload()
        } else {
          vm.alertDialog('用户名和密码不能为空。')
        }
      })
    },
    backupData: function() {
      if (this.loading.backup) return
      this.loading.backup = true
      api.backups.create().then(function(result) {
        vm.alertDialog(vm.backupSummaryMessage(result), '备份完成')
      }).catch(function() {
        vm.alertDialog('备份失败。')
      }).then(function() {
        vm.loading.backup = false
      })
    },
    backupSummaryMessage: function(result) {
      var tableCounts = result.table_counts || {}
      var tableNames = Object.keys(tableCounts).sort()
      var lines = [
        '订阅源：' + (result.feed_count || 0) + ' 个',
        '备份目录：' + (result.path || ''),
        '',
        '表数据：',
      ]
      tableNames.forEach(function(name) {
        lines.push(name + '：' + tableCounts[name] + ' 行')
      })
      return lines.join('\n')
    },
    toggleReadability: function() {
      this.setItemSelectedContentMode(this.itemSelectedContentMode == 'readability' ? 'normal' : 'readability')
    },
    setItemSelectedContentMode: function(mode) {
      this.itemSelectedContentMode = normalizeContentMode(mode)
      this.loadSelectedContentMode()
    },
    loadSelectedContentMode: function() {
      if (this.itemSelectedContentMode == 'readability') {
        this.loadItemSelectedReadability()
      }
    },
    loadItemSelectedReadability: function() {
      var item = this.itemSelectedDetails
      if (!item) return
      if (!item.link) {
        this.itemSelectedReadability = ''
        this.itemSelectedReadabilityError = '当前文章没有原文链接，无法获取正文。'
        return
      }
      this.loading.readability = true
      this.itemSelectedReadabilityError = ''
      var itemId = item.id
      api.crawl(item.link, item.feed_id).then(function(data) {
        if (vm.itemSelected !== itemId) return
        vm.itemSelectedReadability = data && data.content || ''
        if (!vm.itemSelectedReadability) {
          vm.itemSelectedReadabilityError = '未能获取正文。'
        }
      }).catch(function() {
        if (vm.itemSelected !== itemId) return
        vm.itemSelectedReadability = ''
        vm.itemSelectedReadabilityError = '未能获取正文。'
      }).then(function() {
        if (vm.itemSelected !== itemId) return
        vm.loading.readability = false
      })
    },
    showSettings: function(settings) {
      this.settings = settings

      if (settings === 'create') {
        vm.feedNewChoice = []
        vm.feedNewChoiceSelected = ''
        vm.feedNewContentMode = 'normal'
      } else if (settings === 'deletefeeds') {
        vm.feedDeleteSelectedIds = []
      } else if (settings === 'auth') {
        vm.loadAuthConfig()
      }
    },
    showFeedSettings: function(feed) {
      this.settingsFeed = feed
      this.settingsFolder = null
      this.settings = 'feed'
    },
    showFolderSettings: function(folder) {
      this.settingsFolder = folder
      this.settingsFeed = null
      this.settings = 'folder'
    },
    updateRSSHubBaseUrl: function(event) {
      var value = event.target.querySelector('[name=rsshub_base_url]').value
      api.settings.update({rsshub_base_url: value}).then(function(res) {
        if (res.ok) {
          api.settings.get().then(function(settings) {
            vm.rsshubBaseUrl = settings.rsshub_base_url || ''
            vm.settings = ''
          })
        } else {
          vm.alertDialog('RSSHub 基础链接列表必须每行都是 HTTP(S) URL；以 # 开头的停用地址也必须是合法 URL。')
        }
      })
    },
    openDialog: function(options) {
      return new Promise(function(resolve) {
        this.dialog = {
          open: true,
          type: options.type || 'alert',
          title: options.title || '提示',
          message: options.message || '',
          inputValue: options.inputValue || '',
          inputType: options.inputType || 'text',
          confirmText: options.confirmText || '确定',
          cancelText: options.cancelText || '取消',
          danger: !!options.danger,
          resolve: resolve,
        }
      }.bind(this))
    },
    resolveDialog: function(value) {
      if (!this.dialog.open) return
      var resolve = this.dialog.resolve
      this.dialog.open = false
      this.dialog.resolve = null
      if (resolve) resolve(value)
    },
    submitDialog: function() {
      if (this.dialog.type === 'prompt') {
        this.resolveDialog(this.dialog.inputValue)
      } else {
        this.resolveDialog(true)
      }
    },
    cancelDialog: function() {
      if (this.dialog.type === 'confirm') return this.resolveDialog(false)
      if (this.dialog.type === 'prompt') return this.resolveDialog(null)
      this.resolveDialog(true)
    },
    alertDialog: function(message, title) {
      return this.openDialog({
        type: 'alert',
        title: title || '提示',
        message: message,
      })
    },
    confirmDialog: function(message, title) {
      return this.openDialog({
        type: 'confirm',
        title: title || '确认',
        message: message,
        confirmText: '确定',
        cancelText: '取消',
        danger: true,
      })
    },
    promptDialog: function(message, value) {
      return this.openDialog({
        type: 'prompt',
        title: '输入',
        message: message,
        inputValue: value || '',
        confirmText: '确定',
        cancelText: '取消',
      })
    },
    resizeFeedList: function(width) {
      this.feedListWidth = Math.min(Math.max(200, width), 700)
    },
    resizeItemList: function(width) {
      this.itemListWidth = Math.min(Math.max(200, width), 700)
    },
    resetColumnWidths: function() {
      var appWidth = this.$el.getBoundingClientRect().width
      this.feedListWidth = Math.round(appWidth / 5)
      this.itemListWidth = Math.round(appWidth * 3 / 10)
    },
    resetFeedChoice: function() {
      this.feedNewChoice = []
      this.feedNewChoiceSelected = ''
    },
    incrFont: function(x) {
      this.theme.size = +(this.theme.size + (0.1 * x)).toFixed(1)
    },
    fetchAllFeeds: function() {
      this.resetColumnWidths()
      if (this.loading.feeds) return
      api.feeds.refresh().then(function() {
        vm.refreshStats()
      })
    },
    refreshFeedIcons: function() {
      if (this.loading.icons) return
      this.loading.icons = true
      api.feeds.refresh_icons().then(function() {
        vm.feedIconErrors = {}
        return vm.refreshFeeds()
      }).then(function() {
        vm.loading.icons = false
      }, function() {
        vm.loading.icons = false
      })
    },
    computeStats: function() {
      var filter = this.filterSelected
      if (!filter) filter = 'unread'

      var statsFeeds = {}, statsFolders = {}, statsTotal = 0

      for (var i = 0; i < this.feeds.length; i++) {
        var feed = this.feeds[i]
        if (!this.feedStats[feed.id]) continue

        var n = vm.feedStats[feed.id][filter] || 0

        if (!statsFolders[feed.folder_id]) statsFolders[feed.folder_id] = 0

        statsFeeds[feed.id] = n
        statsFolders[feed.folder_id] += n
        statsTotal += n
      }

      this.filteredFeedStats = statsFeeds
      this.filteredFolderStats = statsFolders
      this.filteredTotalStats = statsTotal
    },
    // navigation helper, navigate relative to selected item
    navigateToItem: function(relativePosition) {
      let vm = this
      if (vm.itemSelected == null) {
        // if no item is selected, select first
        if (vm.items.length !== 0) vm.itemSelected = vm.items[0].id
        return
      }

      var itemPosition = vm.items.findIndex(function(x) { return x.id === vm.itemSelected })
      if (itemPosition === -1) {
        if (vm.items.length !== 0) vm.itemSelected = vm.items[0].id
        return
      }

      var newPosition = itemPosition + relativePosition
      if (newPosition < 0 || newPosition >= vm.items.length) return

      vm.itemSelected = vm.items[newPosition].id

      vm.$nextTick(function() {
        var scroll = document.querySelector('#item-list-scroll')

        var handle = scroll.querySelector('input[type=radio]:checked')
        var target = handle && handle.parentElement

        if (target && scroll) scrollto(target, scroll)

        vm.loadMoreItems()
      })
    },
    // navigation helper, navigate relative to selected feed
    navigateToFeed: function(relativePosition) {
      let vm = this
      const navigationList = this.foldersWithFeeds
        .filter(folder => !folder.id || !vm.mustHideFolder(folder))
        .map((folder) => {
          if (this.mustHideFolder(folder)) return []
          const folds = folder.id ? [`folder:${folder.id}`] : []
          const feeds = (folder.is_expanded || !folder.id)
            ? (folder.feeds || []).filter(f => !vm.mustHideFeed(f)).map(f => `feed:${f.id}`)
            : []
          return folds.concat(feeds)
        })
        .flat()
      navigationList.unshift('')

      var currentFeedPosition = navigationList.indexOf(vm.feedSelected)

      if (currentFeedPosition == -1) {
        vm.feedSelected = ''
        return
      }

      var newPosition = currentFeedPosition+relativePosition
      if (newPosition < 0 || newPosition >= navigationList.length) return

      vm.feedSelected = navigationList[newPosition]

      vm.$nextTick(function() {
        var scroll = document.querySelector('#feed-list-scroll')

        var handle = scroll.querySelector('input[type=radio]:checked')
        var target = handle && handle.parentElement

        if (target && scroll) scrollto(target, scroll)
      })
    },
    mustHideFolder: function (folder) {
      return this.filterSelected
        && !(this.current.folder.id == folder.id || this.current.feed.folder_id == folder.id)
        && !this.filteredFolderStats[folder.id]
        && (!this.itemSelectedDetails || (this.feedsById[this.itemSelectedDetails.feed_id] || {}).folder_id != folder.id)
    },
    mustHideFeed: function (feed) {
      return this.filterSelected
        && !(this.current.feed.id == feed.id)
        && !this.filteredFeedStats[feed.id]
        && (!this.itemSelectedDetails || this.itemSelectedDetails.feed_id != feed.id)
    },
  }
})

vm.$mount('#app')
