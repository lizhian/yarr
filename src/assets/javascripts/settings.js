'use strict';

function requireSettingResponse(response) {
  if (!response.ok) throw new Error('保存失败，请检查输入或重试。')
  return response
}

var settingsMixin = {
  data: function() {
    return {
      'feedNewChoice': [],
      'feedNewChoiceSelected': '',
      'feedNewFolderId': null,
      'feedNewContentMode': 'normal',
      'feedNewRankingMode': 'off',
      'feedDeleteSelectedIds': [],
      'authConfig': {
        enabled: app.authenticated,
        username: '',
      },
      'authForm': {
        enabled: app.authenticated,
        username: '',
        password: '',
      },

      settings: '',
      settingsFeed: null,
      settingsFolder: null,
      dialog: {open: false, title: '', message: '', resolve: null},
      settingsStack: [],
      settingsScroll: {},
      settingsDrafts: {},
      settingsNotices: {},
      settingsPending: {},
      settingsReturnFocus: null,
      settingsAfterClose: null,
      settingsNewURL: '',
      settingsNewSelector: '',
      settingsRSSHubDraft: null,
      settingsClosing: false,
      settingsAuthLoaded: false,
    }
  },
  computed: {
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
    currentFeedRefreshDetail: function() {
      if (!this.settingsFeed) return null
      return this.feedRefreshDetails[this.settingsFeed.id] || null
    },
    currentFeedLastRefreshedAt: function() {
      if (!this.settingsFeed) return ''
      return feedRefreshTime(this.settingsFeed, this.feedRefreshDetails, 'last_refreshed_at')
    },
    currentFeedLastRefreshSucceededAt: function() {
      if (!this.settingsFeed) return ''
      return feedRefreshTime(this.settingsFeed, this.feedRefreshDetails, 'last_refresh_succeeded_at')
    },
    showFeedContentSelector: function() {
      return this.settingsFeed && normalizeContentMode(this.settingsFeed.content_mode) == 'readability'
    },
    settingsTitle: function() {
      if (this.settings === 'feed') return this.settingsFeed ? this.settingsFeed.title : '订阅源设置'
      if (this.settings === 'folder') return this.settingsFolder ? this.settingsFolder.title : '文件夹设置'
      return {menu: '系统设置', create: '新订阅源', deletefeeds: '删除订阅源', auth: '访问认证',
        rsshub: 'RSSHub 基础链接列表', rsshubdetails: 'RSSHub 刷新详情', shortcuts: '键盘快捷键'}[this.settings] || '设置'
    },
    settingsKey: function() {
      return this.settings + ':' + (this.settingsFeed ? this.settingsFeed.id : this.settingsFolder ? this.settingsFolder.id : '')
    },
    settingsNotice: function() {
      return this.settingsNotices[this.settingsKey] || ''
    },
  },
  methods: {
    resetFeedChoice: function() {
      this.feedNewChoice = []
      this.feedNewChoiceSelected = ''
    },
    normalizeFontSize: function(value) { return normalizeAppFontSize(value) },
    saveBrowserSetting: function(name, value) {
      var target = {
        themeName: [this.theme, 'name', THEME_NAME_KEY],
        themeFont: [this.theme, 'font', THEME_FONT_KEY],
        appFontSize: [this, 'appFontSize', APP_FONT_SIZE_KEY],
        toolbarDisplay: [this, 'toolbarDisplay', TOOLBAR_DISPLAY_KEY],
      }[name]
      if (!writeLocalSetting(target[2], value)) {
        this.notifySettings('浏览器无法保存设置，请检查存储权限。')
        return
      }
      target[0][target[1]] = value
      this.notifySettings('')
    },
    settingsPage: function() {
      return {name: this.settings, feedId: this.settingsFeed && this.settingsFeed.id,
        folderId: this.settingsFolder && this.settingsFolder.id}
    },
    saveSettingsScroll: function() {
      if (this.settings && this.$refs.settingsScroll) {
        this.settingsScroll[this.settingsKey] = this.$refs.settingsScroll.scrollTop
      }
    },
    restoreSettingsPage: function(page) {
      this.settings = page.name
      this.settingsFeed = page.feedId ? this.feeds.find(function(feed) { return feed.id === page.feedId }) : null
      this.settingsFolder = page.folderId ? this.folders.find(function(folder) { return folder.id === page.folderId }) : null
      if ((page.feedId && !this.settingsFeed) || (page.folderId && !this.settingsFolder)) {
        this.settings = 'menu'
        this.settingsStack = []
        this.notifySettings('该订阅源或文件夹已不存在。')
      }
      this.$nextTick(function() {
        if (this.$refs.settingsScroll) this.$refs.settingsScroll.scrollTop = this.settingsScroll[this.settingsKey] || 0
        if (this.$refs.settingsHeading) this.$refs.settingsHeading.focus({preventScroll: true})
      })
    },
    showSettings: function(name, feed, folder) {
      this.saveSettingsScroll()
      if (!this.settings) {
        this.settingsReturnFocus = document.activeElement
        this.initNavigationHistory()
      } else {
        this.settingsStack.push(this.settingsPage())
      }
      this.restoreSettingsPage({name: name, feedId: feed && feed.id, folderId: folder && folder.id})
      if (name === 'create' && !this.settingsNewURL) {
        this.feedNewFolderId = this.current.feed.folder_id || this.current.folder.id || null
      }
      if (name === 'rsshub' && this.settingsRSSHubDraft === null) this.settingsRSSHubDraft = this.rsshubBaseUrl
      if (name === 'auth') this.loadAuthConfig()
      if (name === 'rsshubdetails') this.refreshStats()
      this.pushSettingsHistory()
    },
    showFeedSettings: function(feed) {
      this.showSettings('feed', feed)
    },
    showFolderSettings: function(folder) {
      this.showSettings('folder', null, folder)
    },
    showCurrentSettings: function() {
      if (this.current.type === 'feed' && this.current.feed.id) this.showFeedSettings(this.current.feed)
      if (this.current.type === 'folder' && this.current.folder.id) this.showFolderSettings(this.current.folder)
    },
    backSettings: function() {
      if (this.dialog.open) return this.cancelDialog()
      if (!this.settingsStack.length) return this.closeSettings()
      this.saveSettingsScroll()
      if (this.hasSettingsHistory()) return window.history.back()
      this.restoreSettingsPage(this.settingsStack.pop())
    },
    closeSettings: function(afterClose) {
      if (this.settingsClosing) return
      if (this.dialog.open) return this.cancelDialog()
      this.saveSettingsScroll()
      this.authForm.password = ''
      if (this.hasSettingsHistory()) {
        this.settingsClosing = true
        this.settingsAfterClose = typeof afterClose === 'function' ? afterClose : null
        window.history.go(-(window.history.state.settingsDepth || this.settingsStack.length + 1))
        return
      }
      this.finishClosingSettings()
      if (typeof afterClose === 'function') afterClose()
    },
    finishClosingSettings: function() {
      this.settingsClosing = false
      this.settings = ''
      this.settingsFeed = null
      this.settingsFolder = null
      this.settingsStack = []
      this.authForm.password = ''
      this.$nextTick(function() {
        var focus = this.settingsReturnFocus
        if (focus && focus.isConnected) focus.focus({preventScroll: true})
      })
    },
    selectArticle: function(id) {
      var select = function() { this.itemSelected = id }.bind(this)
      if (this.settings) this.closeSettings(select)
      else select()
    },
    reconcileSettingsTargets: function() {
      var page = this.settingsPage()
      if (page.feedId) this.settingsFeed = this.feeds.find(function(feed) { return feed.id === page.feedId }) || null
      if (page.folderId) this.settingsFolder = this.folders.find(function(folder) { return folder.id === page.folderId }) || null
      if ((page.feedId && !this.settingsFeed) || (page.folderId && !this.settingsFolder)) {
        this.restoreSettingsPage({name: 'menu'})
        this.notifySettings('该订阅源或文件夹已不存在。')
        this.replaceSettingsHistory()
      }
    },
    settingsDraft: function(key, value) {
      if (!this.settingsDrafts[key]) {
        this.$set(this.settingsDrafts, key, {value: value || '', editing: false, saving: false, error: ''})
      }
      return this.settingsDrafts[key]
    },
    notifySettings: function(message, key) {
      this.$set(this.settingsNotices, key || this.settingsKey, message)
    },
    runSettingsAction: function(key, action, success, message) {
      if (this.settingsPending[key]) return Promise.resolve(false)
      var pageKey = this.settingsKey
      this.$set(this.settingsPending, key, true)
      this.notifySettings('', pageKey)
      return Promise.resolve().then(action).then(function(result) {
        return Promise.resolve(success && success(result)).then(function() {
          var notice = typeof message === 'function' ? message(result) : message
          this.notifySettings(notice === undefined ? '已保存。' : notice, pageKey)
          return true
        }.bind(this))
      }.bind(this)).catch(function(error) {
        this.notifySettings(error.message || '操作失败，请重试。', pageKey)
        return false
      }.bind(this)).then(function(ok) {
        this.$delete(this.settingsPending, key)
        return ok
      }.bind(this))
    },
    saveGlobalSetting: function(property, field, value) {
      var payload = {}
      payload[field] = value
      return this.runSettingsAction(field, function() {
        return api.settings.update(payload).then(requireSettingResponse)
      }, function() { this[property] = value }.bind(this))
    },
    saveFeedSetting: function(feed, field, value) {
      var payload = {}
      payload[field] = value
      return api.feeds.update(feed.id, payload).then(requireSettingResponse).then(function() {
        var live = this.feeds.find(function(candidate) { return candidate.id === feed.id })
        if (!live) throw new Error('该订阅源已不存在。')
        live[field] = value
        feed[field] = value
        if (field === 'icon_url') live.custom_icon = feed.custom_icon = !!value
      }.bind(this))
    },
    saveTextSetting: function(kind, object, field) {
      var key = kind + ':' + object.id + ':' + field
      var draft = this.settingsDraft(key, object[field])
      if (draft.saving) return
      var value = draft.value.trim()
      if (!value && (field === 'title' || field === 'feed_link')) {
        draft.error = '此项不能为空。'
        return
      }
      draft.saving = true
      draft.error = ''
      var request = kind === 'feed' ? this.saveFeedSetting(object, field, value) :
        api.folders.update(object.id, {title: value}).then(requireSettingResponse).then(function() {
          var folder = this.folders.find(function(folder) { return folder.id === object.id })
          if (!folder) throw new Error('该文件夹已不存在。')
          folder.title = object.title = value
          this.folders.sort(function(a, b) { return a.title.localeCompare(b.title) })
        }.bind(this))
      return request.then(function() {
        draft.value = value
        draft.editing = false
      }).catch(function(error) {
        draft.error = error.message || '保存失败，请重试。'
      }).then(function() { draft.saving = false })
    },
    updateFeedContentMode: function(feed, mode) {
      return this.runSettingsAction('feed:' + feed.id + ':content_mode', function() {
        return this.saveFeedSetting(feed, 'content_mode', normalizeContentMode(mode))
      }.bind(this))
    },
    updateFeedRankingMode: function(feed, mode) {
      return this.runSettingsAction('feed:' + feed.id + ':ranking_mode', function() {
        return this.saveFeedSetting(feed, 'ranking_mode', mode)
      }.bind(this))
    },
    normalizeContentMode: function(mode) { return normalizeContentMode(mode) },
    trimValue: function(value) { return (value || '').trim() },
    isHTTPURL: function(value) { return /^https?:\/\//i.test(this.trimValue(value)) },
    moveFeed: function(feed, folder) {
      return this.runSettingsAction('feed:' + feed.id + ':folder_id', function() {
        return this.saveFeedSetting(feed, 'folder_id', folder ? folder.id : null)
      }.bind(this), function() { this.refreshStats() }.bind(this), '已移动。')
    },
    createSettingsFolder: function(feed) {
      var key = feed ? 'feed:' + feed.id + ':newfolder' : 'create:newfolder'
      var draft = this.settingsDraft(key, '')
      var title = draft.value.trim()
      if (!title) { draft.error = '文件夹名称不能为空。'; return }
      if (draft.saving) return
      draft.saving = true
      draft.error = ''
      return api.folders.create({title: title}).then(function(folder) {
        if (!folder || !folder.id) throw new Error('无法创建文件夹。')
        return this.refreshFeeds().then(function() {
          if (feed) return this.saveFeedSetting(feed, 'folder_id', folder.id)
          this.feedNewFolderId = folder.id
        }.bind(this))
      }.bind(this)).then(function() {
        draft.value = ''
        draft.editing = false
        this.refreshStats()
      }.bind(this)).catch(function(error) {
        draft.error = error.message || '创建失败，请重试。'
      }).then(function() { draft.saving = false })
    },
    refreshFeedIcon: function(feed) {
      if (this.loading.feedIcon === feed.id) return
      this.loading.feedIcon = feed.id
      return this.runSettingsAction('icon:' + feed.id, function() {
        return api.feeds.refresh_icon(feed.id).then(function(updated) {
          if (!updated || updated.id !== feed.id) throw new Error('更新图标失败。')
          var live = this.feeds.find(function(candidate) { return candidate.id === feed.id })
          if (live) { live.icon_url = updated.icon_url; live.custom_icon = updated.custom_icon }
          this.feedIconErrors = {}
        }.bind(this))
      }.bind(this), null, '图标已更新。').then(function() { this.loading.feedIcon = null }.bind(this))
    },
    refreshFeed: function(feed) {
      if (this.loading.feed === feed.id) return
      this.loading.feed = feed.id
      return this.runSettingsAction('refresh:' + feed.id, function() {
        return api.feeds.refresh_one(feed.id).then(requireSettingResponse)
      }, function() { return this.refreshStats(true) }.bind(this), '已开始刷新。')
        .then(function() { this.loading.feed = null }.bind(this))
    },
    afterSettingsDeletion: function(ids, folderId) {
      if (this.itemSelectedDetails && ids.indexOf(this.itemSelectedDetails.feed_id) !== -1) this.itemSelected = null
      if (ids.some(function(id) { return this.feedSelected === 'feed:' + id }.bind(this)) ||
          (folderId && this.feedSelected === 'folder:' + folderId)) this.feedSelected = ''
      Object.keys(this.settingsDrafts).forEach(function(key) {
        if (ids.some(function(id) { return key.indexOf('feed:' + id + ':') === 0 }) ||
            (folderId && key.indexOf('folder:' + folderId + ':') === 0)) this.$delete(this.settingsDrafts, key)
      }.bind(this))
      return this.refreshFeeds().then(function() {
        this.refreshStats()
        this.settingsFeed = null
        this.settingsFolder = null
        this.settings = 'menu'
        // Keep the history depth so closing still returns to the original reader layer.
        this.replaceSettingsHistory()
        this.$nextTick(function() {
          var section = this.$refs.subscriptionSettings
          if (section) section.scrollIntoView({block: 'start'})
        })
      }.bind(this))
    },
    deleteFeed: function(feed) {
      return this.confirmDialog('确定删除订阅源「' + feed.title + '」吗？', '删除订阅源').then(function(confirmed) {
        if (!confirmed) return
        return this.runSettingsAction('delete:' + feed.id, function() {
          return api.feeds.delete(feed.id).then(requireSettingResponse)
        }, function() { return this.afterSettingsDeletion([feed.id]) }.bind(this))
      }.bind(this))
    },
    deleteFolder: function(folder) {
      return this.confirmDialog('确定删除文件夹「' + folder.title + '」吗？', '删除文件夹').then(function(confirmed) {
        if (!confirmed) return
        return this.runSettingsAction('deletefolder:' + folder.id, function() {
          return api.folders.delete(folder.id).then(requireSettingResponse)
        }, function() { return this.afterSettingsDeletion([], folder.id) }.bind(this))
      }.bind(this))
    },
    deleteSelectedFeeds: function() {
      var ids = this.feedDeleteSelectedIds.slice()
      if (!ids.length || this.loading.deletefeeds) return
      return this.confirmDialog('确定删除 ' + ids.length + ' 个订阅源吗？', '删除订阅源').then(function(confirmed) {
        if (!confirmed) return
        this.loading.deletefeeds = true
        return Promise.all(ids.map(function(id) {
          return api.feeds.delete(id).then(function(res) { return res.ok }).catch(function() { return false })
        })).then(function(results) {
          var deleted = ids.filter(function(id, index) { return results[index] })
          this.feedDeleteSelectedIds = ids.filter(function(id, index) { return !results[index] })
          if (!this.feedDeleteSelectedIds.length) return this.afterSettingsDeletion(deleted)
          if (this.itemSelectedDetails && deleted.indexOf(this.itemSelectedDetails.feed_id) !== -1) this.itemSelected = null
          return this.refreshFeeds().then(function() {
            this.refreshStats()
            this.notifySettings('部分订阅源删除失败，已保留失败项，可重试。')
          }.bind(this))
        }.bind(this)).catch(function() {
          this.notifySettings('删除后同步失败，请重试。')
        }.bind(this)).then(function() { this.loading.deletefeeds = false }.bind(this))
      }.bind(this))
    },
    createFeed: function() {
      return this.createFeedFromData({
        url: this.feedNewChoiceSelected || normalizeRSSHubSubscriptionInput(this.settingsNewURL).value,
        folder_id: this.feedNewFolderId,
        content_selector: this.feedNewContentMode === 'readability' ? this.settingsNewSelector : '',
        content_mode: this.feedNewContentMode,
        ranking_mode: this.feedNewRankingMode,
      }, true)
    },
    createFeedFromData: function(data, allowChoice) {
      if (this.loading.newfeed) return
      var pageKey = this.settingsKey
      this.loading.newfeed = true
      return api.feeds.create(data).then(function(result) {
        if (result.status === 'success') {
          this.refreshFeeds()
          this.refreshStats()
          this.notifySettings('已添加订阅源「' + result.feed.title + '」。', pageKey)
          this.resetFeedChoice()
        } else if (allowChoice && result.status === 'multiple') {
          this.feedNewChoice = result.choice
          this.feedNewChoiceSelected = result.choice[0].url
        } else {
          throw new Error(result.message || result.error || '未在给定 URL 找到订阅源。')
        }
      }.bind(this)).catch(function(error) {
        this.notifySettings(error.message || '无法添加订阅源。', pageKey)
      }.bind(this)).then(function() { this.loading.newfeed = false }.bind(this))
    },
    createRSSHubFeed: function(kind) {
      var draft = this.settingsDraft('rsshub:' + kind, '')
      var normalize = kind === 'bilibili' ? normalizeBilibiliQuickAddInput : normalizeTelegramQuickAddInput
      var result = normalize(draft.value)
      if (!result.normalized) { draft.error = '无法识别 UID/频道 ID。'; return }
      draft.error = ''
      return this.createFeedFromData({url: result.value,
        folder_id: this.current.feed.folder_id || this.current.folder.id || null}, false)
    },
    importOPML: function(event) {
      var input = event.target
      if (!input.files.length) return
      return this.runSettingsAction('opml', function() {
        return api.upload_opml(input.form).then(requireSettingResponse)
      }, function() {
        input.value = ''
        this.refreshStats()
        return this.refreshFeeds()
      }.bind(this), '导入完成。')
    },
    logout: function() {
      api.logout().then(function() { document.location.reload() })
    },
    loadAuthConfig: function() {
      return this.runSettingsAction('auth-load', function() { return api.auth.get() }, function(config) {
        this.authConfig = config
        this.authenticated = config.enabled
        if (this.settings === 'auth' && !this.settingsAuthLoaded) {
          this.authForm.enabled = config.enabled
          this.authForm.username = config.username || ''
          this.authForm.password = ''
          this.settingsAuthLoaded = true
        }
      }.bind(this), '')
    },
    updateAuthConfig: function() {
      if (this.settingsPending.auth || this.settingsPending['auth-load']) return
      var payload = Object.assign({}, this.authForm)
      var save = function() {
        return this.runSettingsAction('auth', function() {
          return api.auth.update(payload).then(requireSettingResponse)
        }, function() {
          this.authenticated = payload.enabled
          this.authConfig = {enabled: payload.enabled, username: payload.enabled ? payload.username : ''}
          this.authForm.password = ''
        }.bind(this))
      }.bind(this)
      if (payload.enabled) return save()
      return this.confirmDialog('关闭访问认证后将清空已保存的用户名和密码。', '关闭访问认证')
        .then(function(confirmed) { if (confirmed) return save() })
    },
    updateRSSHubBaseUrl: function() {
      var value = this.settingsRSSHubDraft
      return this.runSettingsAction('rsshub', function() {
        return api.settings.update({rsshub_base_url: value}).then(requireSettingResponse).then(api.settings.get)
      }, function(settings) {
        this.rsshubBaseUrl = settings.rsshub_base_url || ''
        if (this.settingsRSSHubDraft === value) this.settingsRSSHubDraft = this.rsshubBaseUrl
      }.bind(this))
    },
    backupData: function() {
      if (this.loading.backup) return
      this.loading.backup = true
      return this.runSettingsAction('backup', function() {
        return api.backups.create().then(function(result) {
          if (result.error) throw new Error(result.error)
          return result
        })
      }, null, this.backupSummaryMessage).then(function() { this.loading.backup = false }.bind(this))
    },
    backupSummaryMessage: function(result) {
      return ['备份完成', '订阅源：' + (result.feed_count || 0) + ' 个', '备份目录：' + (result.path || ''),
        '文件：'].concat(result.files || []).join('\n')
    },
    confirmDialog: function(message, title) {
      if (this.dialog.open) return Promise.resolve(false)
      return new Promise(function(resolve) {
        this.dialog = {open: true, title: title || '确认', message: message, resolve: resolve}
      }.bind(this))
    },
    resolveDialog: function(value) {
      if (!this.dialog.open) return
      var resolve = this.dialog.resolve
      this.dialog.open = false
      this.dialog.resolve = null
      if (resolve) resolve(value)
    },
    submitDialog: function() { this.resolveDialog(true) },
    cancelDialog: function() { this.resolveDialog(false) },
  },
}
