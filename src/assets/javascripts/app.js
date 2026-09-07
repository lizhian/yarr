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

var PULL_REFRESH_DISTANCE = 64
var PULL_REFRESH_MAX_OFFSET = 96

function pullRefreshEnabled(config) {
  return config &&
    isMobileLayout() &&
    !config.loading &&
    typeof config.action == 'function'
}

function resetPullRefresh(el, state) {
  state.tracking = false
  state.pulling = false
  state.refreshing = false
  state.distance = 0
  el.style.removeProperty('--pull-refresh-offset')
  el.classList.remove('pull-refresh-pulling')
  el.classList.remove('pull-refresh-ready')
  el.classList.remove('pull-refresh-refreshing')
}

Vue.directive('pull-refresh', {
  inserted: function(el, binding) {
    var state = {
      config: binding.value || {},
      startY: 0,
      distance: 0,
      tracking: false,
      pulling: false,
      refreshing: false,
    }

    state.touchstart = function(event) {
      if (!pullRefreshEnabled(state.config)) return
      if (state.refreshing || event.touches.length != 1 || el.scrollTop > 0) return

      state.startY = event.touches[0].clientY
      state.distance = 0
      state.tracking = true
      state.pulling = false
    }

    state.touchmove = function(event) {
      if (!state.tracking) return
      if (!pullRefreshEnabled(state.config) || event.touches.length != 1) return resetPullRefresh(el, state)

      var distance = event.touches[0].clientY - state.startY
      if (distance <= 0) return resetPullRefresh(el, state)
      if (el.scrollTop > 0) return resetPullRefresh(el, state)

      if (event.cancelable) event.preventDefault()

      state.pulling = true
      state.distance = Math.min(PULL_REFRESH_MAX_OFFSET, distance * 0.5)
      el.style.setProperty('--pull-refresh-offset', Math.round(state.distance) + 'px')
      el.classList.add('pull-refresh-pulling')
      el.classList.toggle('pull-refresh-ready', state.distance >= PULL_REFRESH_DISTANCE)
    }

    state.touchend = function() {
      if (!state.tracking) return

      if (!state.pulling || state.distance < PULL_REFRESH_DISTANCE || !pullRefreshEnabled(state.config)) {
        resetPullRefresh(el, state)
        return
      }

      state.tracking = false
      state.refreshing = true
      el.style.setProperty('--pull-refresh-offset', PULL_REFRESH_DISTANCE + 'px')
      el.classList.remove('pull-refresh-pulling')
      el.classList.add('pull-refresh-refreshing')

      Promise.resolve(state.config.action()).then(function() {
        resetPullRefresh(el, state)
      }, function() {
        resetPullRefresh(el, state)
      })
    }

    state.touchcancel = function() {
      resetPullRefresh(el, state)
    }

    el._pullRefreshState = state
    el.addEventListener('touchstart', state.touchstart, {passive: true})
    el.addEventListener('touchmove', state.touchmove, {passive: false})
    el.addEventListener('touchend', state.touchend)
    el.addEventListener('touchcancel', state.touchcancel)
  },
  update: function(el, binding) {
    if (!el._pullRefreshState) return
    el._pullRefreshState.config = binding.value || {}
  },
  unbind: function(el) {
    var state = el._pullRefreshState
    if (!state) return
    el.removeEventListener('touchstart', state.touchstart)
    el.removeEventListener('touchmove', state.touchmove)
    el.removeEventListener('touchend', state.touchend)
    el.removeEventListener('touchcancel', state.touchcancel)
    delete el._pullRefreshState
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

var FEED_SORT_OPTIONS = [
  {name: 'name', title: '名称'},
  {name: 'time', title: '时间'},
  {name: 'count', title: '数量'},
]

var STATUS_POLL_INTERVAL = 10000

function normalizeThemeFont(font) {
  return FONT_OPTIONS.some(function(option) { return option.name == font }) ? font : 'lxgw-wenkai'
}

function normalizeContentMode(mode) {
  return CONTENT_MODE_OPTIONS.some(function(option) { return option.name == mode }) ? mode : 'normal'
}

function normalizeFeedSort(sort) {
  return FEED_SORT_OPTIONS.some(function(option) { return option.name == sort }) ? sort : 'time'
}

function compareFeedNames(a, b) {
  return (a.title || '').localeCompare(b.title || '') || a.id - b.id
}

function compareFeeds(sort, stats, a, b) {
  if (sort == 'time') {
    var timeA = Date.parse(a.latest_item_arrived_at || '') || 0
    var timeB = Date.parse(b.latest_item_arrived_at || '') || 0
    if (timeA != timeB) return timeB - timeA
  }
  if (sort == 'count') {
    var countA = (stats[a.id] || {}).unread || 0
    var countB = (stats[b.id] || {}).unread || 0
    if (countA != countB) return countB - countA
  }
  return compareFeedNames(a, b)
}

function feedRefreshTime(feed, refreshDetails, field) {
  var detail = refreshDetails[feed.id]
  return detail && detail[field] || feed[field] || ''
}

function latestFeedRefreshTime(feeds, refreshDetails, folderID, field) {
  var latest = ''
  var latestTime = 0
  feeds.forEach(function(feed) {
    if (feed.folder_id != folderID) return
    var value = feedRefreshTime(feed, refreshDetails, field)
    var timestamp = Date.parse(value) || 0
    if (timestamp > latestTime) {
      latest = value
      latestTime = timestamp
    }
  })
  return latest
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
  var uid = parseBilibiliUIDPrefix(raw)
  if (uid) return {value: 'rsshub://bilibili/user/video/' + uid, normalized: true}

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
  var match = raw.match(/^(\d+)$/)
  if (match) {
    return {value: 'rsshub://bilibili/user/video/' + match[1], normalized: true}
  }
  return normalizeBilibiliSubscriptionInput(raw)
}

function parseBilibiliUIDPrefix(raw) {
  var match = (raw || '').trim().match(/^uid\s*:\s*(\d+)$/i)
  return match ? match[1] : null
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
var FEED_SELECTED_KEY = 'yarr.feedSelected.v1'
var APP_FONT_SIZE_KEY = 'yarr.appFontSize.v1'
var THEME_NAME_KEY = 'yarr.themeName.v1'
var THEME_FONT_KEY = 'yarr.themeFont.v1'
var TOOLBAR_DISPLAY_KEY = 'yarr.toolbarDisplay.v1'
var APP_FONT_SIZE_DEFAULT = 15
var APP_FONT_SIZE_MIN = 10
var APP_FONT_SIZE_MAX = 30

function readFeedSelected() {
  try {
    var raw = localStorage.getItem(FEED_SELECTED_KEY)
    if (raw === null) return ''
    var feedSelected = JSON.parse(raw)
    return feedSelected === null || typeof feedSelected == 'string' ? feedSelected : ''
  } catch (e) {
    return ''
  }
}

function writeFeedSelected(feedSelected) {
  try {
    localStorage.setItem(FEED_SELECTED_KEY, JSON.stringify(feedSelected === null ? null : (feedSelected || '')))
  } catch (e) {}
}

function readLocalSetting(key, fallback, normalize) {
  try {
    var raw = localStorage.getItem(key)
    return raw === null ? normalize(fallback) : normalize(JSON.parse(raw))
  } catch (e) {
    return normalize(fallback)
  }
}

function writeLocalSetting(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
    return true
  } catch (e) {
    return false
  }
}

function normalizeThemeName(theme) {
  return ['light', 'sepia', 'night'].indexOf(theme) >= 0 ? theme : 'light'
}

function normalizeToolbarDisplay(display) {
  return display == 'icon' ? 'icon' : 'text'
}

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

function normalizeAppFontSize(size) {
  size = Number(size)
  return isFinite(size) && Math.floor(size) == size && size >= APP_FONT_SIZE_MIN && size <= APP_FONT_SIZE_MAX ? size : APP_FONT_SIZE_DEFAULT
}

function readAppFontSize() {
  try {
    return normalizeAppFontSize(localStorage.getItem(APP_FONT_SIZE_KEY))
  } catch (e) {
    return APP_FONT_SIZE_DEFAULT
  }
}

function applyAppFontSize(size) {
  document.documentElement.style.setProperty('font-size', normalizeAppFontSize(size) + 'px', 'important')
}

function getArticleListLayout(feedSelected) {
  return normalizeArticleListLayout(readArticleListLayouts()[articleListLayoutStorageKey(feedSelected)])
}

function setArticleListLayout(feedSelected, layout) {
  var layouts = readArticleListLayouts()
  layouts[articleListLayoutStorageKey(feedSelected)] = normalizeArticleListLayout(layout)
  writeArticleListLayouts(layouts)
}

var vm = new Vue({
  mixins: [settingsMixin, navigationMixin],
  created: function() {
    applyAppFontSize(this.appFontSize)
    this.refreshStats()
    Promise.all([
      this.refreshFeeds(),
      this.refreshItems(false),
    ])

    this.scheduleStatusPoll(STATUS_POLL_INTERVAL)

    api.feeds.list_errors().then(function(errors) {
      vm.feed_errors = errors
    })
    this.updateMetaTheme(this.theme.name)
    this.updateBodyClass()
  },
  mounted: function() {
    if (!app.settings.feed_list_width || !app.settings.item_list_width) {
      this.resetColumnWidths()
    }
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
    var feedSelected = readFeedSelected()
    return {
      'filterSelected': s.filter,
      'folders': [],
      'feeds': [],
      'feedSelected': feedSelected,
      'feedListWidth': s.feed_list_width || 300,
      'feedSort': normalizeFeedSort(s.feed_sort),
      'feedSortOptions': FEED_SORT_OPTIONS,
      'feedIconErrors': {},
      'items': [],
      'itemsHasMore': true,
      'itemsCursor': null,
      'itemsRequestSeq': 0,
      'itemsAutoReadSeen': {},
      'itemsAutoReadPending': {},
      'itemListLastScrollTop': 0,
      'itemSelected': null,
      'itemSelectedDetails': null,
      'itemSelectedReadability': '',
      'itemSelectedReadabilityError': '',
      'itemSelectedContentMode': 'normal',
      'itemUnreadFirstAll': s.unread_first !== false,
      'itemSortNewestFirstAll': s.sort_newest_first !== false,
      'itemListWidth': s.item_list_width || 300,
      'articleListLayout': getArticleListLayout(feedSelected),
      'articleListLayoutApplying': false,
      'rsshubBaseUrl': s.rsshub_base_url || '',
      'rsshubDetails': [],
      'rsshubFailures': {stats: [], feeds: []},
      'feedRefreshDetails': {},
      'feedRefreshDetailsInitialized': false,
      'autoReadScrollAll': !!s.auto_read_scroll,
      'filteredFeedStats': {},
      'filteredFolderStats': {},
      'filteredTotalStats': null,

      'loading': {
        'feeds': 0,
        'newfeed': false,
        'deletefeeds': false,
        'items': false,
        'itemDetails': false,
        'readability': false,
        'readingState': false,
        'backup': false,
        'icons': false,
        'feed': null,
        'feedIcon': null,
      },
      'feedStats': {},
      'theme': {
        'name': readLocalSetting(THEME_NAME_KEY, s.theme_name, normalizeThemeName),
        'font': readLocalSetting(THEME_FONT_KEY, s.theme_font, normalizeThemeFont),
      },
      'appFontSize': readAppFontSize(),
      'themeColors': {
        'night': '#1f1f1f',
        'sepia': '#f2e6bd',
        'light': '#f5f6f6',
      },
      'refreshRate': s.refresh_rate,
      'backupEnabled': !!s.backup_enabled,
      'toolbarDisplay': readLocalSetting(TOOLBAR_DISPLAY_KEY, s.toolbar_display, normalizeToolbarDisplay),
      'fontOptions': FONT_OPTIONS,
      'contentModeOptions': CONTENT_MODE_OPTIONS,
      'rankingModeOptions': [
        { title: '停用', name: 'off' },
        { title: '有图', name: 'with_image' },
        { title: '无图', name: 'without_image' },
      ],
      'authenticated': app.authenticated,
      'feed_errors': {},
      'statusPollTimeout': null,
      'logoRefreshAnimating': false,

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
      var feedStats = this.feedStats
      var feedSort = this.feedSort
      var feedsByFolders = this.feeds.reduce(function(folders, feed) {
        if (!folders[feed.folder_id])
          folders[feed.folder_id] = [feed]
        else
          folders[feed.folder_id].push(feed)
        return folders
      }, {})
      Object.keys(feedsByFolders).forEach(function(folderID) {
        feedsByFolders[folderID].sort(function(a, b) {
          return compareFeeds(feedSort, feedStats, a, b)
        })
      })
      var folders = this.folders.slice().map(function(folder) {
        folder.feeds = feedsByFolders[folder.id]
        return folder
      })
      folders.push({id: null, feeds: feedsByFolders[null]})
      return folders
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
    currentLastRefreshedAt: function() {
      var current = this.current
      if (current.type == 'feed') return feedRefreshTime(current.feed, this.feedRefreshDetails, 'last_refreshed_at')
      if (current.type != 'folder' || !current.folder.id) return ''
      return latestFeedRefreshTime(this.feeds, this.feedRefreshDetails, current.folder.id, 'last_refreshed_at')
    },
    currentLastRefreshSucceededAt: function() {
      var current = this.current
      if (current.type == 'feed') return feedRefreshTime(current.feed, this.feedRefreshDetails, 'last_refresh_succeeded_at')
      if (current.type != 'folder' || !current.folder.id) return ''
      return latestFeedRefreshTime(this.feeds, this.feedRefreshDetails, current.folder.id, 'last_refresh_succeeded_at')
    },
    currentLastRefreshSucceeded: function() {
      var refreshedAt = Date.parse(this.currentLastRefreshedAt) || 0
      var succeededAt = Date.parse(this.currentLastRefreshSucceededAt) || 0
      return refreshedAt > 0 && succeededAt >= refreshedAt
    },
    autoReadScroll: function() {
      var current = this.current
      if (current.type == 'feed') {
        // 订阅源列表尚未加载时沿用默认关闭，避免短暂触发滚动标已读
        if (!current.feed.id) return false
        return !!current.feed.auto_read_scroll
      }
      if (current.type == 'folder') {
        if (!current.folder.id) return false
        return !!current.folder.auto_read_scroll
      }
      return !!this.autoReadScrollAll
    },
    itemUnreadFirst: function() {
      var current = this.current
      if (current.type == 'feed') return current.feed.id ? current.feed.unread_first !== false : true
      if (current.type == 'folder') return current.folder.id ? current.folder.unread_first !== false : true
      return this.itemUnreadFirstAll
    },
    itemSortNewestFirst: function() {
      var current = this.current
      if (current.type == 'feed') return current.feed.id ? current.feed.sort_newest_first !== false : true
      if (current.type == 'folder') return current.folder.id ? current.folder.sort_newest_first !== false : true
      return this.itemSortNewestFirstAll
    },
    itemSelectedContent: function() {
      if (!this.itemSelectedDetails) return ''

      if (this.itemSelectedContentMode == 'readability')
        return this.itemSelectedReadability

      return this.itemSelectedDetails.content || ''
    },
    showItemListSkeleton: function() {
      return this.loading.items && this.items.length == 0 && this.feedSelected !== null
    },
    itemListSkeletonRows: function() {
      return this.articleListLayout == 'card' ? 3 : 6
    },
    showItemDetailsSkeleton: function() {
      return this.loading.itemDetails || (!!this.itemSelectedDetails && this.itemSelectedContentMode == 'readability' && this.loading.readability)
    },
    showItemDetailsMeta: function() {
      return !!this.itemSelectedDetails
    },
    itemSelectedLink: function() {
      return this.itemSelectedDetails && this.itemSelectedDetails.link || ''
    },
    itemAtStart: function() {
      return !this.items.length || this.itemSelected == this.items[0].id
    },
    itemAtEnd: function() {
      return !this.items.length || this.itemSelected == this.items[this.items.length - 1].id
    },
    toolbarNarrow: function() {
      return this.feedListWidth < 280 || this.itemListWidth < 280
    },
    markItemsReadBoundaryId: function() {
      if (this.filterSelected != '' || !this.itemUnreadFirst) return null
      if (!this.items.some(function(item) { return item.status == 'unread' })) return null

      for (var i = 1; i < this.items.length; i++) {
        if (this.items[i - 1].listStatus == 'unread' && this.items[i].listStatus == 'read') {
          return this.items[i].id
        }
      }
      return null
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
      if (!this.settings) this.itemSelected = null
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
      writeFeedSelected(newVal)
      this.refreshItems(false)
      if (this.$refs.itemlist) this.$refs.itemlist.scrollTop = 0
      this.syncNavigationHistory()
    },
    'itemSelected': function(newVal, oldVal) {
      this.itemSelectedReadability = ''
      this.itemSelectedReadabilityError = ''
      this.itemSelectedContentMode = 'normal'
      this.loading.readability = false
      this.loading.itemDetails = false
      if (newVal === null) {
        this.itemSelectedDetails = null
        this.syncNavigationHistory()
        return
      }
      this.itemSelectedDetails = null
      this.loading.itemDetails = true
      if (this.$refs.content) this.$refs.content.scrollTop = 0
      this.syncNavigationHistory()

      api.items.get(newVal).then(function(item) {
        if (this.itemSelected !== newVal) return
        this.itemSelectedDetails = item
        this.loading.itemDetails = false
        this.itemSelectedContentMode = normalizeContentMode((this.feedsById[item.feed_id] || {}).content_mode)
        this.loadSelectedContentMode()
        this.$nextTick(this.refreshRankingTimes)
        this.markItemRead(this.itemSelectedDetails)
      }.bind(this)).catch(function() {
        if (this.itemSelected === newVal) {
          this.loading.itemDetails = false
          this.itemSelected = null
        }
      }.bind(this))
    },
    'feedListWidth': debounce(function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({feed_list_width: newVal})
    }, 1000),
    'itemListWidth': debounce(function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      api.settings.update({item_list_width: newVal})
    }, 1000),
    'appFontSize': function(newVal, oldVal) {
      if (oldVal === undefined) return  // do nothing, initial setup
      applyAppFontSize(newVal)
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
    hasRSSHubFeedDetails: function() {
      return this.rsshubDetails.some(function(detail) {
        return detail.details && detail.details.length
      })
    },
    hasRSSHubFailureStats: function() {
      return this.rsshubFailures && this.rsshubFailures.stats && this.rsshubFailures.stats.length > 0
    },
    hasRSSHubFailedFeeds: function() {
      return this.rsshubFailures && this.rsshubFailures.feeds && this.rsshubFailures.feeds.length > 0
    },
    hasRSSHubFailures: function() {
      return this.hasRSSHubFailureStats() || this.hasRSSHubFailedFeeds()
    },
    scheduleStatusPoll: function(delay) {
      clearTimeout(this.statusPollTimeout)
      this.statusPollTimeout = setTimeout(function() {
        vm.refreshStats()
      }, delay)
    },
    triggerLogoRefreshAnimation: function() {
      this.logoRefreshAnimating = false
      this.$nextTick(function() {
        vm.logoRefreshAnimating = true
      })
    },
    refreshStats: function(loopMode) {
      return api.status().then(function(data) {
        if (loopMode && !vm.itemSelected) vm.refreshItems()

        vm.loading.feeds = data.running
        if (data.running > 0) vm.triggerLogoRefreshAnimation()
        vm.rsshubDetails = data.rsshub_details || []
        vm.rsshubFailures = data.rsshub_failures || {stats: [], feeds: []}
        var nextFeedRefreshDetails = data.feed_refresh_details || {}
        var shouldRefreshFeeds = vm.feedRefreshDetailsInitialized && Object.keys(nextFeedRefreshDetails).some(function(feedID) {
          var detail = nextFeedRefreshDetails[feedID]
          var previous = vm.feedRefreshDetails[feedID]
          return detail.new_items > 0 && (!previous || detail.last_refreshed_at != previous.last_refreshed_at)
        })
        vm.feedRefreshDetails = nextFeedRefreshDetails
        vm.feedRefreshDetailsInitialized = true
        if (shouldRefreshFeeds) vm.refreshFeeds()
        vm.scheduleStatusPoll(STATUS_POLL_INTERVAL)
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
      if (this.filterSelected == 'unread') query.status = 'unread'
      if (this.filterSelected == 'favorite') query.favorite = true
      query.unread_first = this.itemUnreadFirst
      if (!this.itemSortNewestFirst) {
        query.oldest_first = true
      }
      return query
    },
    feedSelectionExists: function(feedSelected) {
      if (feedSelected === null || feedSelected === '') return true

      var parts = (feedSelected || '').split(':', 2)
      var type = parts[0]
      var guid = parts[1]

      if (type == 'feed') return !!this.feedsById[guid]
      if (type == 'folder') return !!this.foldersById[guid]
      return false
    },
    ensureFeedSelectionExists: function() {
      if (this.feedSelectionExists(this.feedSelected)) return
      this.feedSelected = ''
      writeFeedSelected('')
    },
    refreshFeeds: function() {
      return Promise
        .all([api.folders.list(), api.feeds.list()])
        .then(function(values) {
          vm.folders = values[0]
          vm.feeds = values[1]
          vm.ensureFeedSelectionExists()
          vm.reconcileSettingsTargets()
        })
    },
    refreshItems: function(loadMore = false) {
      var requestSeq = ++this.itemsRequestSeq
      if (this.feedSelected === null) {
        vm.items = []
        vm.itemsHasMore = false
        vm.itemsCursor = null
        vm.loading.items = false
        vm.resetItemListAutoRead()
        return
      }

      var query = this.getItemsQuery()
      if (loadMore) {
        if (!this.itemsCursor) return
        query.after = this.itemsCursor.after
        query.after_unread = this.itemsCursor.unread
      } else {
        this.resetItemListAutoRead()
        this.items = []
        this.itemsHasMore = false
        this.itemsCursor = null
      }

      this.loading.items = true
      return api.items.list(query).then(function(data) {
        if (requestSeq != vm.itemsRequestSeq) return

        // Preserve the server-sorted group when an item becomes read in place.
        data.list.forEach(function(item) {
          item.listStatus = item.status
        })
        if (loadMore) {
          var loaded = vm.items.reduce(function(ids, item) { ids[item.id] = true; return ids }, {})
          vm.items = vm.items.concat(data.list.filter(function(item) { return !loaded[item.id] }))
        } else {
          vm.items = data.list
        }
        vm.itemsCursor = data.next_after === null ? null : {after: data.next_after, unread: data.next_after_unread}
        vm.itemsHasMore = data.has_more
        vm.loading.items = false

        // load more if there's some space left at the bottom of the item list.
        vm.$nextTick(function() {
          vm.updateItemListAutoReadSeen()
          if (vm.itemsHasMore && !vm.loading.items && vm.itemListCloseToBottom()) {
            vm.refreshItems(true)
          }
        })
      }).catch(function(err) {
        if (requestSeq == vm.itemsRequestSeq) {
          vm.loading.items = false
          throw err
        }
      })
    },
    itemListCloseToBottom: function() {
      var el = this.$refs.itemlist

      if (!el || el.scrollHeight === 0) return false  // element is invisible (responsive design)

      var scale = (parseFloat(getComputedStyle(document.documentElement).fontSize) || 16) / 16
      var prefetchDistance = Math.max(320 * scale, el.offsetHeight * 0.75)

      var closeToBottom = (el.scrollHeight - el.scrollTop - el.offsetHeight) < prefetchDistance
      return closeToBottom
    },
    itemListPrefetchTriggerVisible: function(el) {
      el = el || this.$refs.itemlist
      if (!el || el.scrollHeight === 0) return false

      var labels = el.querySelectorAll('.selectgroup[data-item-id]')
      if (!labels.length) return false

      var triggerIndex = Math.max(0, labels.length - 10)
      var triggerLabel = labels[triggerIndex]
      if (!triggerLabel) return false

      var scrollRect = el.getBoundingClientRect()
      var labelRect = triggerLabel.getBoundingClientRect()
      return labelRect.bottom > scrollRect.top && labelRect.top < scrollRect.bottom
    },
    loadMoreItems: function(event, el) {
      if (!this.itemsHasMore) return
      if (this.loading.items) return
      if (this.itemListPrefetchTriggerVisible(el)) return this.refreshItems(true)
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
      if (!this.autoReadScroll) return false
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
    toggleAutoReadScroll: function() {
      var enabled = !this.autoReadScroll
      var current = this.current
      if (current.type == 'feed' && current.feed.id) {
        var feed = current.feed
        api.feeds.update(feed.id, {auto_read_scroll: enabled}).then(function() {
          feed.auto_read_scroll = enabled
        })
        return
      }
      if (current.type == 'folder' && current.folder.id) {
        var folder = current.folder
        api.folders.update(folder.id, {auto_read_scroll: enabled}).then(function() {
          folder.auto_read_scroll = enabled
        })
        return
      }
      // "全部" 文章列表：绑定全局设置
      api.settings.update({auto_read_scroll: enabled}).then(function() {
        vm.autoReadScrollAll = enabled
      })
    },
    setItemOrder: function(field, value) {
      var current = this.current
      var payload = {}
      payload[field] = value
      if (current.type == 'feed' && current.feed.id) {
        return api.feeds.update(current.feed.id, payload).then(function() {
          current.feed[field] = value
          vm.refreshItems(false)
        })
      }
      if (current.type == 'folder' && current.folder.id) {
        return api.folders.update(current.folder.id, payload).then(function() {
          current.folder[field] = value
          vm.refreshItems(false)
        })
      }
      return api.settings.update(payload).then(function() {
        if (field == 'unread_first') vm.itemUnreadFirstAll = value
        if (field == 'sort_newest_first') vm.itemSortNewestFirstAll = value
        vm.refreshItems(false)
      })
    },
    feedIconErrored: function(feed) {
      return !!this.feedIconErrors[feed.id + ':' + (feed.icon_url || '')]
    },
    markFeedIconErrored: function(feed) {
      this.$set(this.feedIconErrors, feed.id + ':' + (feed.icon_url || ''), true)
    },
    markItemsRead: function() {
      var query = this.getItemsQuery()
      var keepCurrentItems = this.filterSelected == ''
      var feedSelected = this.feedSelected
      var filterSelected = this.filterSelected
      var itemUnreadFirst = this.itemUnreadFirst
      var itemSortNewestFirst = this.itemSortNewestFirst
      api.items.mark_read(query).then(function() {
        var sameItemList = vm.feedSelected == feedSelected &&
          vm.filterSelected == filterSelected &&
          vm.itemUnreadFirst == itemUnreadFirst &&
          vm.itemSortNewestFirst == itemSortNewestFirst
        var shouldShowFeedList = sameItemList &&
          filterSelected == 'unread' &&
          isMobileLayout() &&
          vm.currentNavigationLayer() == 'items'

        if (keepCurrentItems) {
          if (sameItemList) {
            vm.items.forEach(function(item) {
              if (item.status == 'unread') item.status = 'read'
            })
            if (vm.itemSelectedDetails && vm.itemSelectedDetails.status == 'unread') {
              vm.itemSelectedDetails.status = 'read'
            }
          }
        } else if (sameItemList) {
          vm.items = []
          vm.itemSelected = null
          vm.itemsHasMore = false
        }
        if (sameItemList) vm.resetItemListAutoRead()
        if (shouldShowFeedList) vm.showFeedList()
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
    toggleItemStatus: function(item, targetstatus, fallbackstatus) {
      var oldstatus = item.status
      var newstatus = item.status !== targetstatus ? targetstatus : fallbackstatus

      var updateStats = function(status, incr) {
        if (status == 'unread') {
          var feedStats = this.feedStats[item.feed_id]
          if (feedStats) feedStats[status] += incr
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
    toggleItemFavorite: function(item) {
      var favorite = !item.favorite
      api.items.update(item.id, {favorite: favorite}).then(function() {
        var feedStats = this.feedStats[item.feed_id]
        if (feedStats) feedStats.favorite += favorite ? 1 : -1
        var itemInList = this.items.find(function(i) { return i.id == item.id })
        if (itemInList) itemInList.favorite = favorite
        item.favorite = favorite
      }.bind(this))
    },
    toggleItemRead: function(item) {
      this.toggleItemStatus(item, 'unread', 'read')
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
      } else {
        this.$nextTick(this.refreshRankingTimes)
      }
    },
    refreshRankingTimes: function() {
      if (!this.$refs.content) return
      var nodes = this.$refs.content.querySelectorAll('.bilibili-ranking-meta time[datetime]')
      Array.prototype.forEach.call(nodes, function(node) {
        var date = new Date(node.getAttribute('datetime'))
        if (!isNaN(date.getTime())) node.textContent = dateRepr(date)
      })
    },
    onContentClick: function(e) {
      var meta = e.target.closest('.bilibili-ranking-meta')
      if (!meta || !this.$refs.content.contains(meta)) return
      var details = meta.nextElementSibling
      if (!details || !details.classList.contains('bilibili-ranking-details')) return
      details.open = !details.open
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
        vm.$nextTick(vm.refreshRankingTimes)
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
    refreshCurrentItems: function() {
      if (this.loading.items) return Promise.resolve()
      return this.refreshItems(false)
    },
    syncReadingState: function() {
      if (this.loading.readingState) return Promise.resolve()
      this.loading.readingState = true
      return this.refreshStats()
        .then(function() {
          return vm.refreshItems(false)
        })
        .then(function() {
          vm.loading.readingState = false
        }, function() {
          vm.loading.readingState = false
        })
    },
    resetColumnsAndSyncReadingState: function() {
      this.resetColumnWidths()
      return this.syncReadingState()
    },
    refreshAllFeeds: function() {
      if (this.loading.feeds) return Promise.resolve()
      return this.runSettingsAction('refresh-all', function() {
        return api.feeds.refresh().then(requireSettingResponse)
      }, function() { return this.refreshStats() }.bind(this), '已开始刷新。')
    },
    refreshFeedIcons: function() {
      if (this.loading.icons) return
      this.loading.icons = true
      return this.runSettingsAction('icons', function() {
        return api.feeds.refresh_icons().then(requireSettingResponse)
      }, function() {
        this.feedIconErrors = {}
        return this.refreshFeeds()
      }.bind(this), '图标已更新。').then(function() { this.loading.icons = false }.bind(this))
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
