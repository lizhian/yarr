var helperFunctions = {
  scrollContent: function(direction) {
    var padding = 40
    var scroll = document.querySelector('.content')
    if (!scroll) return

    var height = scroll.getBoundingClientRect().height
    var newpos = scroll.scrollTop + (height - padding) * direction

    if (typeof scroll.scrollTo == 'function') {
      scroll.scrollTo({top: newpos, left: 0, behavior: 'smooth'})
    } else {
      scroll.scrollTop = newpos
    }
  }
}
var shortcutFunctions = {
  openItemLink: function() {
    if (vm.itemSelectedDetails && vm.itemSelectedDetails.link) {
      window.open(vm.itemSelectedDetails.link, '_blank', 'noopener,noreferrer')
    }
  },
  toggleReadability: function() {
    vm.toggleReadability()
  },
  toggleItemRead: function() {
    if (vm.itemSelected != null) {
      vm.toggleItemRead(vm.itemSelectedDetails)
    }
  },
  markAllRead: function() {
    // same condition as 'Mark all read button'
    if (vm.filterSelected == 'unread' || vm.filterSelected == ''){
      vm.markItemsRead()
    }
  },
  toggleItemFavorite: function() {
    if (vm.itemSelected != null) {
      vm.toggleItemFavorite(vm.itemSelectedDetails)
    }
  },
  nextItem(){
    vm.navigateToItem(+1)
  },
  previousItem() {
    vm.navigateToItem(-1)
  },
  nextFeed(){
    vm.navigateToFeed(+1)
  },
  previousFeed() {
    vm.navigateToFeed(-1)
  },
  scrollForward: function() {
    helperFunctions.scrollContent(+1)
  },
  scrollBackward: function() {
    helperFunctions.scrollContent(-1)
  },
  closeItem: function () {
    vm.closeItem()
  },
  showAll() {
    vm.filterSelected = ''
  },
  showUnread() {
    vm.filterSelected = 'unread'
  },
  showFavorite() {
    vm.filterSelected = 'favorite'
  },
}

// Keep the shortcuts settings page in sync with these bindings.
var keybindings = {
  "o": shortcutFunctions.openItemLink,
  "i": shortcutFunctions.toggleReadability,
  "r": shortcutFunctions.toggleItemRead,
  "R": shortcutFunctions.markAllRead,
  "s": shortcutFunctions.toggleItemFavorite,
  "j": shortcutFunctions.nextItem,
  "k": shortcutFunctions.previousItem,
  "l": shortcutFunctions.nextFeed,
  "h": shortcutFunctions.previousFeed,
  "f": shortcutFunctions.scrollForward,
  "b": shortcutFunctions.scrollBackward,
  "q": shortcutFunctions.closeItem,
  "1": shortcutFunctions.showAll,
  "2": shortcutFunctions.showUnread,
  "3": shortcutFunctions.showFavorite,
}

var codebindings = {
  "KeyO": shortcutFunctions.openItemLink,
  "KeyI": shortcutFunctions.toggleReadability,
  //"r": shortcutFunctions.toggleItemRead,
  //"KeyR": shortcutFunctions.markAllRead,
  "KeyS": shortcutFunctions.toggleItemFavorite,
  "KeyJ": shortcutFunctions.nextItem,
  "KeyK": shortcutFunctions.previousItem,
  "KeyL": shortcutFunctions.nextFeed,
  "KeyH": shortcutFunctions.previousFeed,
  "KeyF": shortcutFunctions.scrollForward,
  "KeyB": shortcutFunctions.scrollBackward,
  "KeyQ": shortcutFunctions.closeItem,
  "Digit1": shortcutFunctions.showAll,
  "Digit2": shortcutFunctions.showUnread,
  "Digit3": shortcutFunctions.showFavorite,
}

function isTextBox(element) {
  return !!element.closest('input, textarea, select, [contenteditable="true"]')
}

document.addEventListener('keydown',function(event) {
  if (vm.dialog.open) return
  if (vm.settings) {
    if (event.key === 'Escape' && !isTextBox(event.target)) {
      event.preventDefault()
      vm.backSettings()
    }
    return
  }
  // Ignore while focused on text or
  // when using modifier keys (to not clash with browser behaviour)
  if (isTextBox(event.target) || event.metaKey || event.ctrlKey || event.altKey) {
    return
  }
  var keybindFunction = keybindings[event.key] || codebindings[event.code]
  if (keybindFunction) {
    event.preventDefault()
    keybindFunction()
  }
})
