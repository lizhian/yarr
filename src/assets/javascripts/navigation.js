'use strict';

var navigationMixin = {
  data: function() {
    return {
      'navigationHistory': {
        initialized: false,
        applyingPop: false,
        syncPending: false,
        layer: null,
      },
    }
  },
  beforeDestroy: function() {
    window.removeEventListener('popstate', this.handleNavigationPop)
  },
  methods: {
    hasSettingsHistory: function() {
      return this.canUseNavigationHistory() && window.history.state && !!window.history.state.settingsPage
    },
    pushSettingsHistory: function() {
      if (!this.canUseNavigationHistory()) return
      var state = this.navigationState(this.currentNavigationLayer())
      state.settingsPage = this.settingsPage()
      state.settingsStack = this.settingsStack.slice()
      state.settingsDepth = this.hasSettingsHistory() ? (window.history.state.settingsDepth || 1) + 1 : 1
      window.history.pushState(state, document.title)
    },
    replaceSettingsHistory: function() {
      if (!this.hasSettingsHistory()) return
      var state = this.navigationState(this.currentNavigationLayer())
      state.settingsPage = this.settingsPage()
      state.settingsStack = this.settingsStack.slice()
      state.settingsDepth = window.history.state.settingsDepth
      window.history.replaceState(state, document.title)
    },
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
      if (this.settings) return
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
      if (this.settings) return
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

      if (this.dialog.open) this.cancelDialog()
      if (event.state.settingsPage) {
        this.saveSettingsScroll()
        this.settingsStack = event.state.settingsStack || []
        this.restoreSettingsPage(event.state.settingsPage)
        return
      }
      if (this.settings) {
        this.saveSettingsScroll()
        this.finishClosingSettings()
        this.navigationHistory.layer = this.currentNavigationLayer()
        window.history.replaceState(this.navigationState(this.navigationHistory.layer), document.title)
        var afterClose = this.settingsAfterClose
        this.settingsAfterClose = null
        if (afterClose) afterClose()
        return
      }

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
  },
}
