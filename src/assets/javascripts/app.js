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

var FONT_OPTIONS = [
  {name: 'lxgw-wenkai', title: '霞鹜文楷'},
  {name: 'maple-mono-nf-cn', title: 'Maple Mono NF-CN'},
]

function normalizeThemeFont(font) {
  return FONT_OPTIONS.some(function(option) { return option.name == font }) ? font : 'lxgw-wenkai'
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

    api.feeds.list_errors().then(function(errors) {
      vm.feed_errors = errors
    })
    this.updateMetaTheme(this.theme.name)
    this.updateBodyClass()
  },
  mounted: function() {
    this.initNavigationHistory()
  },
  data: function() {
    var s = app.settings
    return {
      'filterSelected': s.filter,
      'folders': [],
      'feeds': [],
      'feedSelected': s.feed,
      'feedListWidth': s.feed_list_width || 300,
      'feedNewChoice': [],
      'feedNewChoiceSelected': '',
      'items': [],
      'itemsHasMore': true,
      'itemSelected': null,
      'itemSelectedDetails': null,
      'itemSelectedReadability': '',
      'itemSearch': '',
      'itemSortNewestFirst': s.sort_newest_first,
      'itemListWidth': s.item_list_width || 300,
      'articleListLayout': s.article_list_layout == 'card' ? 'card' : 'list',
      'rsshubBaseUrl': s.rsshub_base_url || '',

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
        'items': false,
        'readability': false,
      },
      'feedStats': {},
      'theme': {
        'name': s.theme_name,
        'font': normalizeThemeFont(s.theme_font),
        'size': s.theme_size,
      },
      'themeColors': {
        'night': '#1f1f1f',
        'sepia': '#f4f0e5',
        'light': '#f7f7f5',
      },
      'refreshRate': s.refresh_rate,
      'toolbarDisplay': s.toolbar_display == 'icon' ? 'icon' : 'text',
      'fontOptions': FONT_OPTIONS,
      'authenticated': app.authenticated,
      'feed_errors': {},
      'navigationHistory': {
        initialized: false,
        applyingPop: false,
        syncPending: false,
        layer: null,
      },

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

      if (this.itemSelectedReadability)
        return this.itemSelectedReadability

      return this.itemSelectedDetails.content || ''
    },
    toolbarNarrow: function() {
      return this.feedListWidth < 280 || this.itemListWidth < 280
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
      api.settings.update({feed: newVal}).then(this.refreshItems.bind(this, false))
      this.itemSelected = null
      if (this.$refs.itemlist) this.$refs.itemlist.scrollTop = 0
      this.syncNavigationHistory()
    },
    'itemSelected': function(newVal, oldVal) {
      this.itemSelectedReadability = ''
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
        if (this.itemSelectedDetails.status == 'unread') {
          api.items.update(this.itemSelectedDetails.id, {status: 'read'}).then(function() {
            this.feedStats[this.itemSelectedDetails.feed_id].unread -= 1
            var itemInList = this.items.find(function(i) { return i.id == item.id })
            if (itemInList) itemInList.status = 'read'
            this.itemSelectedDetails.status = 'read'
          }.bind(this))
        }
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
      api.settings.update({article_list_layout: newVal})
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
    refreshStats: function(loopMode) {
      return api.status().then(function(data) {
        if (loopMode && !vm.itemSelected) vm.refreshItems()

        vm.loading.feeds = data.running
        if (data.running) {
          setTimeout(vm.refreshStats.bind(vm, true), 500)
        }
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
        return
      }

      var query = this.getItemsQuery()
      if (loadMore) {
        query.after = vm.items[vm.items.length-1].id
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
    itemImage: function(item) {
      var link = (item.media_links || []).find(function(link) {
        return link.type === 'image' && link.url
      })
      return link && link.url
    },
    toggleArticleListLayout: function() {
      this.articleListLayout = this.articleListLayout == 'card' ? 'list' : 'card'
    },
    markItemsRead: function() {
      var query = this.getItemsQuery()
      api.items.mark_read(query).then(function() {
        vm.items = []
        vm.itemsPage = {'cur': 1, 'num': 1}
        vm.itemSelected = null
        vm.itemsHasMore = false
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
    createFeed: function(event) {
      var form = event.target
      var data = {
        url: form.querySelector('input[name=url]').value,
        folder_id: parseInt(form.querySelector('select[name=folder_id]').value) || null,
      }
      if (this.feedNewChoiceSelected) {
        data.url = this.feedNewChoiceSelected
      }
      this.loading.newfeed = true
      api.feeds.create(data).then(function(result) {
        if (result.status === 'success') {
          vm.refreshFeeds()
          vm.refreshStats()
          vm.settings = ''
          vm.feedSelected = 'feed:' + result.feed.id
        } else if (result.status === 'multiple') {
          vm.feedNewChoice = result.choice
          vm.feedNewChoiceSelected = result.choice[0].url
        } else if (result.status === 'error') {
          vm.alertDialog(result.message || '无法添加订阅源。')
        } else {
          vm.alertDialog('未在给定 URL 找到订阅源。')
        }
        vm.loading.newfeed = false
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
    toggleReadability: function() {
      if (this.itemSelectedReadability) {
        this.itemSelectedReadability = null
        return
      }
      var item = this.itemSelectedDetails
      if (!item) return
      if (item.link) {
        this.loading.readability = true
        api.crawl(item.link).then(function(data) {
          vm.itemSelectedReadability = data && data.content
          vm.loading.readability = false
        })
      }
    },
    showSettings: function(settings) {
      this.settings = settings

      if (settings === 'create') {
        vm.feedNewChoice = []
        vm.feedNewChoiceSelected = ''
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
    feedLinkHref: function(link) {
      if (!link) return ''
      if (link.indexOf('rsshub://') !== 0) return link
      if (!this.rsshubBaseUrl) return ''
      return this.rsshubBaseUrl.replace(/\/+$/, '') + '/' + link.replace('rsshub://', '').replace(/^\/+/, '')
    },
    resizeFeedList: function(width) {
      this.feedListWidth = Math.min(Math.max(200, width), 700)
    },
    resizeItemList: function(width) {
      this.itemListWidth = Math.min(Math.max(200, width), 700)
    },
    resetFeedChoice: function() {
      this.feedNewChoice = []
      this.feedNewChoiceSelected = ''
    },
    incrFont: function(x) {
      this.theme.size = +(this.theme.size + (0.1 * x)).toFixed(1)
    },
    fetchAllFeeds: function() {
      if (this.loading.feeds) return
      api.feeds.refresh().then(function() {
        vm.refreshStats()
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
