import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import {
  MAIN_KEYBOARD_REGION_ORDER,
  nextRovingIndex,
  nextVisibleRegion,
} from '../src/lib/keyboard/regionNavigation.ts'
import { resolveRequiredSelectionIndex } from '../src/lib/components/list/requiredSelection.ts'
import {
  isOptionCodeShortcut,
  resolveAppKeyboardPolicy,
} from '../src/lib/keyboard/keyboardPolicy.ts'

const modalFramePath = new URL('../src/lib/components/ui/ModalFrame.svelte', import.meta.url)
const messageListPath = new URL('../src/lib/components/list/MessageList.svelte', import.meta.url)
const conversationRowPath = new URL('../src/lib/components/list/ConversationRow.svelte', import.meta.url)
const folderPickerPath = new URL('../src/lib/components/common/FolderPickerDialog.svelte', import.meta.url)
const globalShortcutsPath = new URL('../src/lib/keyboard/globalShortcuts.ts', import.meta.url)
const keyboardStorePath = new URL('../src/lib/stores/keyboard.svelte.ts', import.meta.url)
const appPath = new URL('../src/App.svelte', import.meta.url)
const appStylesPath = new URL('../src/app.css', import.meta.url)
const activityRailPath = new URL('../src/lib/components/rail/ActivityRail.svelte', import.meta.url)
const listRowPath = new URL('../src/lib/components/kit/ListRow.svelte', import.meta.url)
const railButtonPath = new URL('../src/lib/components/rail/RailButton.svelte', import.meta.url)
const sourceSidebarPath = new URL('../src/lib/components/kit/SourceSidebar.svelte', import.meta.url)
const listPanePath = new URL('../src/lib/components/kit/ListPane.svelte', import.meta.url)
const detailPanePath = new URL('../src/lib/components/kit/DetailPane.svelte', import.meta.url)
const mailSidebarPath = new URL('../src/lib/components/sidebar/Sidebar.svelte', import.meta.url)
const folderTreeItemPath = new URL('../src/lib/components/sidebar/FolderTreeItem.svelte', import.meta.url)
const accountSectionPath = new URL('../src/lib/components/sidebar/AccountSection.svelte', import.meta.url)
const contactsSidebarPath = new URL('../src/lib/contacts/components/ContactsSidebar.svelte', import.meta.url)
const contactListPath = new URL('../src/lib/contacts/components/ContactList.svelte', import.meta.url)
const contactStorePath = new URL('../src/lib/contacts/stores/contactsView.svelte.ts', import.meta.url)
const viewerPath = new URL('../src/lib/components/viewer/ConversationViewer.svelte', import.meta.url)
const settingsPath = new URL('../src/lib/components/settings/SettingsDialog.svelte', import.meta.url)
const settingsDraftPath = new URL('../src/lib/components/settings/settingsDraft.svelte.ts', import.meta.url)
const settingsStorePath = new URL('../src/lib/stores/settings.svelte.ts', import.meta.url)
const generalSettingsPath = new URL('../src/lib/components/settings/pages/GeneralSettingsPage.svelte', import.meta.url)
const accountsTabPath = new URL('../src/lib/components/settings/AccountsTab.svelte', import.meta.url)
const settingsRowPath = new URL('../src/lib/components/settings/shared/SettingsRow.svelte', import.meta.url)
const selectTriggerPath = new URL('../src/lib/components/ui/select/select-trigger.svelte', import.meta.url)
const backupViewerPath = new URL('../src/lib/components/backup/BackupViewerDialog.svelte', import.meta.url)
const globalSearchPath = new URL('../src/lib/components/SearchOverlay.svelte', import.meta.url)
const keyboardActionStorePath = new URL('../src/lib/stores/keyboardActionMenu.svelte.ts', import.meta.url)
const keyboardActionMenuPath = new URL('../src/lib/components/keyboard/KeyboardActionMenu.svelte', import.meta.url)
const composerPath = new URL('../src/lib/components/composer/Composer.svelte', import.meta.url)
const composerEditorPath = new URL('../src/lib/components/composer/composerEditor.ts', import.meta.url)
const editorToolbarPath = new URL('../src/lib/components/composer/EditorToolbar.svelte', import.meta.url)
const attachmentListPath = new URL('../src/lib/components/viewer/AttachmentList.svelte', import.meta.url)
const emailBodyPath = new URL('../src/lib/components/viewer/EmailBody.svelte', import.meta.url)
const threeOptionDialogPath = new URL('../src/lib/components/ui/confirm-dialog/ThreeOptionDialog.svelte', import.meta.url)

test('custom modal frames close on Escape, contain Tab focus, and restore prior focus', async () => {
  const modalFrame = await readFile(modalFramePath, 'utf8')

  assert.match(modalFrame, /bind:this=\{panelEl\}/)
  assert.match(modalFrame, /onkeydown=\{handlePanelKeydown\}/)
  assert.match(modalFrame, /event\.key === 'Escape'[\s\S]*onClose\?\.\(\)/)
  assert.match(modalFrame, /event\.key !== 'Tab'/)
  assert.match(modalFrame, /previouslyFocused[\s\S]*\.focus\(\{ preventScroll: true \}\)/)
})

test('mail-list search Escape closes search and returns focus to the list', async () => {
  const messageList = await readFile(messageListPath, 'utf8')

  assert.match(
    messageList,
    /function handleSearchKeydown[\s\S]*event\.key === 'Escape'[\s\S]*clearSearch\(\)[\s\S]*listContainerRef\?\.focus\(\)/,
  )
})

test('keyboard message selection is instant while pointer hover keeps its color transition', async () => {
  const [messageList, conversationRow] = await Promise.all([
    readFile(messageListPath, 'utf8'),
    readFile(conversationRowPath, 'utf8'),
  ])

  assert.match(messageList, /let selectionInputMode = \$state<'keyboard' \| 'pointer'>\('pointer'\)/)
  assert.match(messageList, /function focusConversationAtIndex[\s\S]*claimKeyboardSelection\(\)[\s\S]*selectedThreadId = conv\.threadId/)
  assert.match(messageList, /function selectPreviousWithCheck[\s\S]*claimKeyboardSelection\(\)[\s\S]*selectedThreadId = conv\.threadId/)
  assert.match(messageList, /function selectNextWithCheck[\s\S]*claimKeyboardSelection\(\)[\s\S]*selectedThreadId = conv\.threadId/)
  assert.match(messageList, /onPointerMove=\{claimPointerSelection\}/)
  assert.match(messageList, /instantSelection=\{selectionInputMode === 'keyboard'\}/)
  assert.match(conversationRow, /instantSelection = false/)
  assert.match(conversationRow, /onpointermove=\{onPointerMove\}/)
  assert.match(
    conversationRow,
    /instantSelection\s*\? 'transition-none'\s*: 'transition-colors duration-300'/,
  )
})

test('folder-picker arrows and Enter only run from its search or folder list', async () => {
  const folderPicker = await readFile(folderPickerPath, 'utf8')

  assert.match(folderPicker, /function eventTargetsFolderNavigation/)
  assert.match(folderPicker, /target === searchInput/)
  assert.match(folderPicker, /listEl\?\.contains\(target\)/)
  assert.match(
    folderPicker,
    /if \(!getEnhancedKeyboardNavigation\(\) \|\| !active \|\| !eventTargetsFolderNavigation\(e\)\) return/,
  )
})

test('enhanced keyboard policy leaves native keys alone while keeping Command-F search', () => {
  const event = (overrides = {}) => ({
    key: '',
    code: '',
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    ...overrides,
  })

  assert.equal(resolveAppKeyboardPolicy(event({ key: 'Tab' }), false), 'native')
  assert.equal(resolveAppKeyboardPolicy(event({ key: 'Tab' }), true), 'enhanced')
  assert.equal(resolveAppKeyboardPolicy(event({ key: 'f', metaKey: true }), false), 'search')
  assert.equal(resolveAppKeyboardPolicy(event({ key: 'F', ctrlKey: true }), false), 'search')
  assert.equal(resolveAppKeyboardPolicy(event({ key: 'f', metaKey: true, altKey: true }), false), 'native')
})

test('macOS Option-letter shortcuts match physical codes instead of produced symbols', () => {
  const optionSymbolEvent = (key, code) => ({
    key,
    code,
    altKey: true,
    ctrlKey: false,
    metaKey: false,
  })

  assert.equal(isOptionCodeShortcut(optionSymbolEvent('†', 'KeyT'), 'KeyT'), true)
  assert.equal(isOptionCodeShortcut(optionSymbolEvent('å', 'KeyA'), 'KeyA'), true)
  assert.equal(isOptionCodeShortcut(optionSymbolEvent('t', 'KeyY'), 'KeyT'), false)
})

test('enhanced keyboard setting is save-only, defaults on, and updates the runtime store', async () => {
  const [settingsDraft, settingsStore, generalSettings, keyboardStore] = await Promise.all([
    readFile(settingsDraftPath, 'utf8'),
    readFile(settingsStorePath, 'utf8'),
    readFile(generalSettingsPath, 'utf8'),
    readFile(keyboardStorePath, 'utf8'),
  ])

  assert.match(generalSettings, /bind:checked=\{draft\.enhancedKeyboardNavigation\}/)
  assert.match(settingsDraft, /GetEnhancedKeyboardNavigation\(\)/)
  assert.match(settingsDraft, /this\.enhancedKeyboardNavigation = keyboardNavigation \?\? true/)
  assert.match(settingsDraft, /await SetEnhancedKeyboardNavigation\(this\.enhancedKeyboardNavigation\)/)
  assert.match(settingsDraft, /updateEnhancedKeyboardNavigationStore\(this\.enhancedKeyboardNavigation\)/)
  assert.match(settingsDraft, /enhancedKeyboardNavigation: this\.enhancedKeyboardNavigation/)
  assert.match(settingsStore, /let enhancedKeyboardNavigation = \$state<boolean>\(true\)/)
  assert.match(settingsStore, /enhancedKeyboardNavigation = keyboardNavigation \?\? true/)
  assert.match(
    keyboardStore,
    /export function isMainKeyboardScope\(\): boolean \{[\s\S]*getEnhancedKeyboardNavigation\(\) && keyboardScope === 'main'/,
  )
})

test('current-region action menu exposes visible controls without changing Tab routing', async () => {
  const [globalShortcuts, actionStore, actionMenu, composer, toolbar, attachments] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(keyboardActionStorePath, 'utf8'),
    readFile(keyboardActionMenuPath, 'utf8'),
    readFile(composerPath, 'utf8'),
    readFile(editorToolbarPath, 'utf8'),
    readFile(attachmentListPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /e\.key === 'F10' && e\.shiftKey[\s\S]*ctx\.openRegionActionMenu\(\)/)
  assert.match(actionStore, /data-keyboard-action-context/)
  assert.match(actionStore, /showForRegion\(region: string\)/)
  assert.match(actionStore, /requestAnimationFrame\(\(\) => action\.element\.click\(\)\)/)
  assert.match(actionMenu, /event\.key === 'ArrowDown'[\s\S]*moveSelection\(1\)/)
  assert.match(actionMenu, /event\.key === 'ArrowUp'[\s\S]*moveSelection\(-1\)/)
  assert.match(actionMenu, /event\.key === 'Enter'[\s\S]*activate\(filteredActions\[selectedIndex\]\)/)
  assert.match(composer, /keyboardActionMenu\.showForRoot\(composerRootElement\)/)
  assert.match(toolbar, /data-keyboard-toolbar-selected/)
  assert.match(toolbar, /e\.key === 'Enter' \|\| e\.key === ' '/)
  assert.match(attachments, /data-keyboard-action-context=\{att\.filename\}/)
  assert.match(attachments, /group-focus-within:opacity-100/)
})

test('turning enhanced keyboard navigation off guards every region owner but not search', async () => {
  const [
    globalShortcuts,
    activityRail,
    sourceSidebar,
    listPane,
    detailPane,
    viewer,
    composer,
    settings,
    backupViewer,
    emailBody,
    composerEditor,
    toolbar,
    threeOptionDialog,
  ] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
    readFile(sourceSidebarPath, 'utf8'),
    readFile(listPanePath, 'utf8'),
    readFile(detailPanePath, 'utf8'),
    readFile(viewerPath, 'utf8'),
    readFile(composerPath, 'utf8'),
    readFile(settingsPath, 'utf8'),
    readFile(backupViewerPath, 'utf8'),
    readFile(emailBodyPath, 'utf8'),
    readFile(composerEditorPath, 'utf8'),
    readFile(editorToolbarPath, 'utf8'),
    readFile(threeOptionDialogPath, 'utf8'),
  ])

  assert.ok(
    globalShortcuts.indexOf("keyboardPolicy === 'search'")
      < globalShortcuts.indexOf('if (ctx.showComposer)'),
  )
  assert.match(globalShortcuts, /keyboardPolicy === 'native'[\s\S]*return/)
  for (const source of [activityRail, sourceSidebar, listPane, detailPane, viewer, composer, settings, backupViewer]) {
    assert.match(source, /getEnhancedKeyboardNavigation\(\)/)
  }
  assert.match(
    emailBody,
    /enhancedKeyboardNavigation && e\.altKey[\s\S]*getEnhancedKeyboardNavigation\(\)/,
  )
  assert.match(
    composerEditor,
    /isEnhancedKeyboardNavigationEnabled\?\.\(\) === false[\s\S]*return false/,
  )
  assert.match(toolbar, /if \(!getEnhancedKeyboardNavigation\(\)\)/)
  assert.match(
    threeOptionDialog,
    /if \(!getEnhancedKeyboardNavigation\(\) && !isTab\) return/,
  )
})

test('four visible regions cycle forward, backward, and across hidden regions', () => {
  assert.deepEqual(MAIN_KEYBOARD_REGION_ORDER, ['featureNav', 'sidebar', 'messageList', 'viewer'])
  assert.equal(nextVisibleRegion('featureNav', 1, MAIN_KEYBOARD_REGION_ORDER), 'sidebar')
  assert.equal(nextVisibleRegion('viewer', 1, MAIN_KEYBOARD_REGION_ORDER), 'featureNav')
  assert.equal(nextVisibleRegion('featureNav', -1, MAIN_KEYBOARD_REGION_ORDER), 'viewer')
  assert.equal(nextVisibleRegion('viewer', -1, ['featureNav', 'messageList', 'viewer']), 'messageList')
  assert.equal(nextVisibleRegion('featureNav', 1, ['featureNav', 'viewer']), 'viewer')
})

test('main Tab routing uses one focused region and preserves input Tab', async () => {
  const [globalShortcuts, keyboardStore, app, activityRail] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(keyboardStorePath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /e\.key === 'Tab' && !inInput[\s\S]*e\.shiftKey\) focusPreviousPane\(\)[\s\S]*focusNextPane\(\)/)
  assert.match(keyboardStore, /let focusedPane = \$state<FocusablePane>/)
  assert.match(keyboardStore, /dataset\.keyboardRegionVisible === 'false'/)
  assert.match(keyboardStore, /nextVisibleRegion\(focusedPane, -1, getVisiblePanes\(\)\)/)
  assert.match(app, /data-keyboard-region="sidebar"[\s\S]*data-keyboard-region="messageList"[\s\S]*data-keyboard-region="viewer"/)
  assert.match(activityRail, /data-keyboard-region="featureNav"/)
})

test('feature navigation arrows directly activate the next destination', async () => {
  const [activityRail, railButton] = await Promise.all([
    readFile(activityRailPath, 'utf8'),
    readFile(railButtonPath, 'utf8'),
  ])
  const moveSelection = activityRail.match(
    /function moveSelection\(delta: number\) \{([\s\S]*?)\n {2}\}/,
  )?.[1] ?? ''

  assert.match(moveSelection, /const nextFeature =/)
  assert.match(moveSelection, /activateSelection\(nextFeature\)/)
  assert.doesNotMatch(moveSelection, /selectedFeature\s*=/)
  assert.match(activityRail, /function activateSelection\(feature: string\)/)
  assert.match(activityRail, /function select\(name: string\)[\s\S]*setActivePane\(name\)/)
  assert.match(activityRail, /function selectSettings\(\)[\s\S]*onOpenSettings\?\.\(\)/)
  assert.match(activityRail, /active=\{selectedFeature === 'mail'\}/)
  assert.match(activityRail, /active=\{selectedFeature === pane\.id\}/)
  assert.match(activityRail, /selectedFeature === 'settings' \? 'border-l-primary/)
  assert.doesNotMatch(activityRail, /selected=\{selectedFeature/)
  assert.doesNotMatch(railButton, /selected/)
})

test('region indicator stays top-only while selections use component backgrounds', async () => {
  const [styles, keyboardStore, app, activityRail, listRow] = await Promise.all([
    readFile(appStylesPath, 'utf8'),
    readFile(keyboardStorePath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
    readFile(listRowPath, 'utf8'),
  ])

  assert.doesNotMatch(styles, /pane-focus-flash/)
  assert.doesNotMatch(keyboardStore, /flashingPane|flashTimeoutId|triggerFlash|isPaneFlashing/)
  assert.match(styles, /\.keyboard-region \{[\s\S]*border-top: 3px solid transparent/)
  assert.match(styles, /\.keyboard-region\[data-region-active='true'\][\s\S]*#f97316/)
  assert.doesNotMatch(styles, /keyboard-selected-item|inset 0 0 0 2px #cbd5e1/)
  assert.match(app, /isMainKeyboardScope\(\) && getActivePane\(\) === 'mail' && getFocusedPane\(\) === 'messageList'/)
  assert.match(activityRail, /selectedFeature === 'settings' \? 'border-l-primary bg-accent\/40 text-primary'/)
  assert.match(listRow, /selected[\s\S]*bg-primary\/20/)
  assert.doesNotMatch(activityRail + listRow, /keyboard-selected-item/)
})

test('mouse interaction claims each main region', async () => {
  const [app, activityRail, sourceSidebar, contactList] = await Promise.all([
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
    readFile(sourceSidebarPath, 'utf8'),
    readFile(contactListPath, 'utf8'),
  ])

  assert.match(app, /onmousedown=\{\(event\) => handlePaneMouseDown\('sidebar', event\)\}/)
  assert.match(app, /onmousedown=\{\(\) => handlePaneClick\('messageList'\)\}/)
  assert.match(app, /onmousedown=\{\(\) => handlePaneClick\('viewer'\)\}/)
  assert.match(activityRail, /onmousedown=\{claimRegion\}/)
  assert.match(sourceSidebar, /onmousedown=\{handleMouseDown\}/)
  assert.match(contactList, /function claimListRegion[\s\S]*setFocusedPane\('messageList'\)/)
  assert.match(contactList, /onmousedown=\{claimListRegion\}/)
  assert.match(contactList, /data-keyboard-region-visible=\{!isResponsive\(\) \|\| getResponsiveView\(\) === 'default'\}/)
})

test('contacts sidebar arrows include the All Contacts and Refresh action group', async () => {
  const [sourceSidebar, contactsSidebar] = await Promise.all([
    readFile(sourceSidebarPath, 'utf8'),
    readFile(contactsSidebarPath, 'utf8'),
  ])

  assert.equal(nextRovingIndex('ArrowUp', 1, 6, true), 0)
  assert.equal(nextRovingIndex('ArrowDown', 0, 6, true), 1)
  assert.equal(nextRovingIndex('ArrowUp', 0, 6, true), 5)
  assert.equal(nextRovingIndex('ArrowDown', 5, 6, true), 0)
  assert.match(sourceSidebar, /headerActionsFocused = \$bindable\(false\)/)
  assert.match(sourceSidebar, /function move\(step: 1 \| -1\)[\s\S]*nextRovingIndex[\s\S]*focusHeaderActions/)
  assert.match(sourceSidebar, /headerActionsFocused[\s\S]*e\.key === 'ArrowLeft'[\s\S]*moveHeaderAction/)
  assert.match(sourceSidebar, /function activateCurrent[\s\S]*activateHeaderAction/)
  assert.match(contactsSidebar, /bind:headerActionsFocused[\s\S]*bind:selectedHeaderActionId/)
  assert.match(contactsSidebar, /data-source-sidebar-header-action="all"/)
  assert.match(contactsSidebar, /data-source-sidebar-header-action="refresh"/)
  assert.match(contactsSidebar, /tabindex="-1"/)
  assert.match(
    contactsSidebar,
    /headerActionsFocused && selectedHeaderActionId === 'all'\s*\? 'bg-primary text-primary-foreground hover:bg-primary\/90'/,
  )
  assert.doesNotMatch(
    contactsSidebar,
    /!headerActionsFocused \|\| selectedHeaderActionId === 'all'/,
  )
})

test('mail sidebar rows leave DOM focus on the keyboard region', async () => {
  const [app, folderTreeItem, accountSection] = await Promise.all([
    readFile(appPath, 'utf8'),
    readFile(folderTreeItemPath, 'utf8'),
    readFile(accountSectionPath, 'utf8'),
  ])

  assert.match(
    app,
    /function handlePaneMouseDown[\s\S]*pane !== 'sidebar'[\s\S]*closest\('\[data-sidebar-item\]'\)[\s\S]*event\.preventDefault\(\)[\s\S]*region\?\.focus\(\{ preventScroll: true \}\)/,
  )
  assert.match(folderTreeItem, /tabindex="-1"[\s\S]*data-sidebar-item="folder"/)
  assert.match(folderTreeItem, /transition-colors focus:outline-none/)
  assert.match(accountSection, /tabindex="-1"[\s\S]*data-sidebar-item="account-header"/)
  assert.match(accountSection, /transition-colors focus:outline-none/)
  assert.doesNotMatch(folderTreeItem + accountSection, /keyboard-selected-item/)
})

test('mail sidebar only highlights the focused Compose or Sync action', async () => {
  const sidebar = await readFile(mailSidebarPath, 'utf8')

  assert.match(
    sidebar,
    /sidebarActionsFocused && selectedSidebarAction === 'compose'\s*\? 'bg-primary text-primary-foreground hover:bg-primary\/90'/,
  )
  assert.doesNotMatch(
    sidebar,
    /!sidebarActionsFocused \|\| selectedSidebarAction === 'compose'/,
  )
  assert.match(
    sidebar,
    /sidebarActionsFocused && selectedSidebarAction === 'sync'\s*\? 'bg-primary text-primary-foreground hover:bg-primary\/90'/,
  )
})

test('input Escape returns to its region while search scope Tab remains unchanged', async () => {
  const [globalShortcuts, globalSearch, backupViewer, contactList] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(globalSearchPath, 'utf8'),
    readFile(backupViewerPath, 'utf8'),
    readFile(contactListPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /if \(inInput\)[\s\S]*e\.key === 'Escape'[\s\S]*focusCurrentPane\(\)/)
  assert.match(globalSearch, /e\.key === 'Tab'[\s\S]*moveScope\(e\.shiftKey \? -1 : 1\)/)
  assert.match(backupViewer, /event\.key === 'Tab'[\s\S]*moveSearchScope\(event\.shiftKey \? -1 : 1\)/)
  assert.match(contactList, /e\.key === 'Escape'[\s\S]*focusPane\('messageList'\)/)
})

test('ConversationViewer no longer owns Tab message navigation', async () => {
  const viewer = await readFile(viewerPath, 'utf8')

  assert.doesNotMatch(viewer, /e\.key === 'Tab'/)
  assert.match(viewer, /Region-level Tab[\s\S]*global shortcut dispatcher/)
  assert.match(viewer, /isInputElement\(e\.target\) \|\| isInputElement\(document\.activeElement\)/)
})

test('backup message list uses roving tabindex and complete directional navigation', async () => {
  const backupViewer = await readFile(backupViewerPath, 'utf8')

  assert.equal(nextRovingIndex('ArrowDown', 0, 3), 1)
  assert.equal(nextRovingIndex('ArrowUp', 0, 3), 0)
  assert.equal(nextRovingIndex('Home', 2, 3), 0)
  assert.equal(nextRovingIndex('End', 0, 3), 2)
  assert.match(backupViewer, /tabindex=\{selectedMessageKey === message\.key \? 0 : -1\}/)
  assert.match(backupViewer, /\['ArrowUp', 'ArrowDown', 'Home', 'End'\]/)
  assert.match(backupViewer, /scrollIntoView\(\{ block, behavior: 'smooth' \}\)/)
  assert.match(backupViewer, /event\.key === 'Enter'[\s\S]*selectMessage\(selectedMessageKey\)/)
  assert.match(backupViewer, /handleMessageListFocus[\s\S]*visibleMessages\[0\][\s\S]*selectMessage\(first\.key\)[\s\S]*focusMessageRow\(first\.key\)/)
  assert.match(backupViewer, /void selectMessage\(next\.key\)[\s\S]*scrollMessageIntoView\(next\.key\)[\s\S]*focusMessageRow\(next\.key\)/)
})

test('ordinary Escape cannot clear the selected mail detail', async () => {
  const [globalShortcuts, app] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(appPath, 'utf8'),
  ])

  assert.doesNotMatch(globalShortcuts, /clearConversation/)
  assert.match(globalShortcuts, /e\.key === 'Escape'[\s\S]*hasCheckedMessages\(\)[\s\S]*clearChecked\(\)/)
  assert.doesNotMatch(app, /clearConversation:/)
})

test('mail selection remains required across refresh, folder switch, and deletion', async () => {
  const messageList = await readFile(messageListPath, 'utf8')

  assert.equal(resolveRequiredSelectionIndex([], 'missing'), -1)
  assert.equal(resolveRequiredSelectionIndex(['a', 'b'], 'b'), 1)
  assert.equal(resolveRequiredSelectionIndex(['a', 'b'], 'missing'), 0)
  assert.equal(resolveRequiredSelectionIndex(['a', 'c'], 'deleted-b', 1), 1)
  assert.equal(resolveRequiredSelectionIndex(['a'], 'deleted-b', 1), 0)
  assert.match(messageList, /resolveRequiredSelectionIndex\([\s\S]*conversations\.map/)
  assert.match(messageList, /loadConversations\(totalLoaded, autoSelectNext \? currentIndex : 0\)/)
  assert.match(messageList, /folderChanged \|\| selectionChanged[\s\S]*emitConversationSelection/)
  assert.match(messageList, /conversations\.length === 0[\s\S]*onEmptyFolder\?\.\(\)/)
})

test('settings dialog cycles two regions, traps focus, and restores the feature rail without outlining the settings button', async () => {
  const [settings, app, activityRail] = await Promise.all([
    readFile(settingsPath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
  ])

  assert.match(settings, /settingsRegion = \$state<'navigation' \| 'content'>/)
  assert.match(settings, /event\.key === 'Tab' && settingsContentMode === 'browse'[\s\S]*settingsRegion === 'navigation' \? 'content' : 'navigation'/)
  assert.match(settings, /event\.key === 'Escape' && settingsContentMode === 'input'[\s\S]*finishSettingsInput\(\)/)
  assert.match(settings, /setKeyboardScope\('settings'\)[\s\S]*setKeyboardScope\('main'\)/)
  assert.match(settings, /onOpenAutoFocus=\{handleOpenAutoFocus\}/)
  assert.match(app, /requestAnimationFrame\(\(\) => activityRailRef\?\.focusSettings\(\)\)/)
  const restoreFocusBody = activityRail.match(
    /export function focusSettings\(\) \{([\s\S]*?)\n\s{2}\}/,
  )?.[1] ?? ''
  assert.match(restoreFocusBody, /selectedFeature = 'settings'/)
  assert.match(restoreFocusBody, /setFocusedPane\('featureNav'\)/)
  assert.match(restoreFocusBody, /railEl\?\.focus\(\{ preventScroll: true \}\)/)
  assert.doesNotMatch(activityRail, /settingsButtonEl/)
})

test('leaving restored Settings focus selects the feature matching the visible main pane', async () => {
  const activityRail = await readFile(activityRailPath, 'utf8')

  assert.match(
    activityRail,
    /const focusedPane = getFocusedPane\(\)[\s\S]*focusedPane !== 'featureNav'[\s\S]*selectedFeature = activePane/,
  )
  assert.match(
    activityRail,
    /export function focusSettings\(\)[\s\S]*selectedFeature = 'settings'[\s\S]*setFocusedPane\('featureNav'\)/,
  )
})

test('settings navigation arrows immediately activate the highlighted category', async () => {
  const settings = await readFile(settingsPath, 'utf8')
  const navigationKeyHandler = settings.match(
    /const currentIndex = navigation\.findIndex[\s\S]*?if \(nextIndex >= 0\)([^\n]*)/,
  )?.[1] ?? ''

  assert.match(navigationKeyHandler, /selectNavigationPage\(navigation\[nextIndex\]\.id\)/)
  assert.doesNotMatch(navigationKeyHandler, /selectedNavigationPage\s*=/)
})

test('settings keyboard navigation moves the category background with its active text', async () => {
  const settings = await readFile(settingsPath, 'utf8')

  assert.match(settings, /settingsNavigationInputMode = \$state<'keyboard' \| 'pointer'>\('pointer'\)/)
  assert.match(
    settings,
    /\['ArrowUp', 'ArrowDown', 'Home', 'End'\][\s\S]*settingsNavigationInputMode = 'keyboard'[\s\S]*selectNavigationPage\(navigation\[nextIndex\]\.id\)/,
  )
  assert.match(settings, /onpointermove=\{\(\) => \{ settingsNavigationInputMode = 'pointer' \}\}/)
  assert.match(settings, /activePage === item\.id[\s\S]*'bg-background\/70 text-primary'/)
  assert.match(
    settings,
    /settingsNavigationInputMode === 'pointer'[\s\S]*hover:bg-background\/70 hover:text-foreground/,
  )
})

test('settings content browse mode selects actual controls and enters native input state only on activation', async () => {
  const [settings, settingsRow] = await Promise.all([
    readFile(settingsPath, 'utf8'),
    readFile(settingsRowPath, 'utf8'),
  ])

  assert.equal(nextRovingIndex('ArrowDown', 4, 5, true), 0)
  assert.equal(nextRovingIndex('ArrowUp', 0, 5, true), 4)
  assert.equal(nextRovingIndex('ArrowDown', 0, 0, true), -1)
  assert.match(settingsRow, /data-settings-control-row/)
  assert.doesNotMatch(settingsRow, /settings-control-selected/)
  assert.doesNotMatch(settingsRow, /data-\[settings-control-selected=true\]:bg-primary\/15/)
  assert.match(settings, /settingsContentMode = \$state<'browse' \| 'input'>\('browse'\)/)
  assert.match(settings, /button:not\(:disabled\)/)
  assert.match(settings, /function getSettingsControls/)
  assert.match(settings, /function getSettingsContentItems/)
  assert.match(settings, /function selectSettingsControl/)
  assert.match(settings, /settingsKeyboardSelected = 'true'/)
  assert.match(settings, /data-settings-keyboard-selected='true'/)
  assert.match(settings, /nextRovingIndex\(event\.key as RovingNavigationKey,[\s\S]*settingsContentItems\.length,\s*true/)
  assert.match(settings, /function activateSelectedSettingsControl/)
  assert.match(settings, /settingsContentMode = 'input'/)
  assert.match(settings, /control\.focus\(\{ preventScroll: true \}\)/)
  assert.match(settings, /activateSelectedSettingsControl\(activationKey: 'Enter' \| ' '\)/)
  assert.match(settings, /activateSelectedSettingsControl\(event\.key\)/)
  assert.match(settings, /scrollIntoView\(\{ block: 'nearest' \}\)/)
  assert.match(settings, /onfocusin=\{handleContentFocusIn\}/)
  assert.match(settings, /onmousedown=\{handleContentMouseDown\}/)
})

test('settings select activation uses native keydown and keeps one blue indicator while open', async () => {
  const [settings, selectTrigger] = await Promise.all([
    readFile(settingsPath, 'utf8'),
    readFile(selectTriggerPath, 'utf8'),
  ])

  assert.match(selectTrigger, /data-keyboard-input="true"/)
  assert.match(selectTrigger, /data-keyboard-select-trigger="true"/)
  assert.match(
    selectTrigger,
    /data-keyboard-select-trigger='true'\]\[data-state='open'\][\s\S]*box-shadow:[\s\S]*hsl\(var\(--ring\)\)/,
  )
  assert.doesNotMatch(selectTrigger, /data-state='open'[\s\S]*box-shadow: none/)
  assert.match(settings, /function isSettingsSelectTrigger[\s\S]*data-keyboard-select-trigger/)
  assert.match(
    settings,
    /function beginSettingsInput\([\s\S]*settingsContentMode = 'input'[\s\S]*control\.focus\(\{ preventScroll: true \}\)[\s\S]*clearSettingsKeyboardSelection\(\)/,
  )
  assert.match(
    settings,
    /isSettingsSelectTrigger\(control\)[\s\S]*control\.dispatchEvent\([\s\S]*new KeyboardEvent\('keydown',[\s\S]*key: activationKey/,
  )
  assert.doesNotMatch(settings, /control\.matches\('\[role="combobox"\]'\)/)
  assert.match(
    settings,
    /function selectSettingsControlForTarget\(target: EventTarget \| null, showSelection = true\)[\s\S]*if \(showSelection\) selectSettingsControl\([\s\S]*else clearSettingsKeyboardSelection\(\)/,
  )
  assert.match(
    settings,
    /function handleContentFocusIn[\s\S]*selectSettingsControlForTarget\(event\.target, settingsContentMode === 'browse'\)/,
  )
  assert.match(
    settings,
    /if \(!\['ArrowUp', 'ArrowDown', 'Home', 'End'\]\.includes\(event\.key\)\) return[\s\S]*contentRegionEl\?\.focus\(\{ preventScroll: true \}\)[\s\S]*selectSettingsControl\(nextIndex\)/,
  )
})

test('settings lets an open select handle Escape before returning to control browse mode', async () => {
  const settings = await readFile(settingsPath, 'utf8')

  assert.match(settings, /onkeydowncapture=\{handleSettingsKeydown\}/)
  assert.doesNotMatch(settings, /onkeydown=\{handleSettingsKeydown\}/)
  assert.match(
    settings,
    /event\.key === 'Tab' && settingsContentMode === 'browse'[\s\S]*settingsRegion === 'navigation' \? 'content' : 'navigation'/,
  )
  assert.match(settings, /function observeSelectLifecycle/)
  assert.match(settings, /attributeFilter: \['aria-expanded', 'data-state'\]/)
  assert.match(settings, /sawOpen[\s\S]*finishSettingsInput\(\)/)
  assert.match(
    settings,
    /event\.key === 'Escape' && settingsContentMode === 'input'[\s\S]*isSettingsSelectTrigger\(activeControl\)[\s\S]*isOpenSelect\(activeControl\)[\s\S]*return[\s\S]*finishSettingsInput\(\)/,
  )
  assert.match(settings, /function finishSettingsInput[\s\S]*settingsContentMode = 'browse'/)
  assert.match(settings, /function finishSettingsInput[\s\S]*contentRegionEl\?\.focus\(\{ preventScroll: true \}\)/)
})

test('account row actions use left-right within a row and up-down between account groups', async () => {
  const [settings, accountsTab] = await Promise.all([
    readFile(settingsPath, 'utf8'),
    readFile(accountsTabPath, 'utf8'),
  ])

  assert.match(accountsTab, /data-settings-horizontal-group="account-actions"/)
  assert.match(accountsTab, /data-settings-horizontal-context=\{acc\.id\}/)
  for (const action of ['test', 'move-up', 'move-down', 'edit', 'delete']) {
    assert.match(accountsTab, new RegExp(`data-settings-horizontal-action="${action}"`))
  }
  assert.match(
    accountsTab,
    /await accountStore\.reorderAccounts\(ids\)[\s\S]*onAccountOrderChanged\?\.\(accountId, 'move-up'\)/,
  )
  assert.match(
    accountsTab,
    /await accountStore\.reorderAccounts\(ids\)[\s\S]*onAccountOrderChanged\?\.\(accountId, 'move-down'\)/,
  )
  assert.match(settings, /function getSettingsHorizontalGroups/)
  assert.match(settings, /function getSettingsHorizontalGroupControls/)
  assert.match(
    settings,
    /function restoreSettingsHorizontalControlAfterRender[\s\S]*dataset\.settingsHorizontalContext === context[\s\S]*resolveHorizontalActionIndex[\s\S]*selectSettingsControlElement\(target\)/,
  )
  assert.match(
    settings,
    /selectedHorizontalGroup[\s\S]*\['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'\]\.includes\(event\.key\)/,
  )
  assert.match(
    settings,
    /event\.key === 'ArrowLeft' \|\| event\.key === 'ArrowRight'[\s\S]*selectHorizontalGroupControl/,
  )
  assert.match(
    settings,
    /event\.key === 'ArrowUp' \|\| event\.key === 'ArrowDown'[\s\S]*selectAdjacentHorizontalGroup/,
  )
})

test('enhanced keyboard explanation is exposed through a question-mark popover', async () => {
  const [generalSettings, settingsRow] = await Promise.all([
    readFile(generalSettingsPath, 'utf8'),
    readFile(settingsRowPath, 'utf8'),
  ])

  assert.match(generalSettings, /help=\{\$_\('settingsGeneral\.enhancedKeyboardNavigationDescription'\)\}/)
  assert.doesNotMatch(
    generalSettings,
    /description=\{\$_\('settingsGeneral\.enhancedKeyboardNavigationDescription'\)\}/,
  )
  assert.match(settingsRow, /help\?: string/)
  assert.match(settingsRow, /<Popover\.Root>/)
  assert.match(settingsRow, /<Popover\.Trigger[\s\S]*data-settings-help-trigger/)
  assert.match(settingsRow, /icon="lucide:circle-help"/)
  assert.match(settingsRow, /<Popover\.Content[\s\S]*\{help\}/)
})

test('settings footer uses horizontal actions and both actions return vertically to controls', async () => {
  const settings = await readFile(settingsPath, 'utf8')

  assert.match(settings, /data-settings-footer-actions/)
  assert.match(settings, /data-settings-footer-action="cancel"/)
  assert.match(settings, /data-settings-footer-action="save"/)
  assert.match(
    settings,
    /Button data-settings-footer-action="save" variant="outline"/,
  )
  assert.match(
    settings,
    /getSettingsFooterActions\(\)[\s\S]*?\.map\(\(element\): SettingsContentItem => \(\{ kind: 'footer', element \}\)\)/,
  )
  assert.doesNotMatch(settings, /selectedSettingsFooterActionIndex/)
  assert.doesNotMatch(settings, /function selectSettingsFooterAction/)
  assert.match(
    settings,
    /settingsControlForTarget\(target\)[\s\S]*settingsFooterActionForTarget\(target\)[\s\S]*contentItems\.indexOf\(footerItem\)/,
  )
  assert.match(settings, /selectedItem\.kind === 'footer'[\s\S]*selectedItem\.element\.click\(\)/)
  assert.match(
    settings,
    /selectedItem\?\.kind === 'footer'[\s\S]*\['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'\]\.includes\(event\.key\)/,
  )
  assert.match(
    settings,
    /event\.key === 'ArrowUp' \|\| event\.key === 'ArrowDown'[\s\S]*controlItems[\s\S]*event\.key === 'ArrowUp' \? controlItems\.at\(-1\) : controlItems\[0\][\s\S]*selectSettingsControl\(settingsContentItems\.indexOf\(targetControl\)\)/,
  )
  assert.match(
    settings,
    /event\.key === 'ArrowLeft' \? 'ArrowUp' : 'ArrowDown'[\s\S]*footerItems\.length[\s\S]*selectSettingsControl\(settingsContentItems\.indexOf\(footerItems\[nextFooterIndex\]\)\)/,
  )
  assert.match(
    settings,
    /data-settings-keyboard-selected='true'[\s\S]*outline: 2px solid hsl\(var\(--primary\)\)/,
  )
  assert.match(
    settings,
    /data-settings-footer-action\]\[data-settings-keyboard-selected='true'\][\s\S]*outline-offset: 2px/,
  )
  assert.match(
    settings,
    /data-settings-footer-action='save'\]\[data-settings-keyboard-selected='true'\][\s\S]*background-color: hsl\(var\(--primary\)\)[\s\S]*color: hsl\(var\(--primary-foreground\)\)/,
  )
})

test('settings saved baselines remain reactive so the dirty indicator clears with Save', async () => {
  const settingsDraft = await readFile(settingsDraftPath, 'utf8')

  assert.match(settingsDraft, /private original = \$state\(''\)/)
  assert.match(settingsDraft, /private originalBackup = \$state\(''\)/)
  assert.match(
    settingsDraft,
    /private capture\(\): void \{[\s\S]*this\.original = JSON\.stringify\(this\.snapshot\(\)\)[\s\S]*this\.originalBackup = JSON\.stringify\(this\.backupSnapshot\(\)\)/,
  )
})

test('IME composition and dialog priority stay ahead of region shortcuts', async () => {
  const [globalShortcuts, settings, backupViewer] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(settingsPath, 'utf8'),
    readFile(backupViewerPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /e\.isComposing \|\| e\.keyCode === 229/)
  assert.ok(globalShortcuts.indexOf('if (ctx.showSearchOverlay) return') < globalShortcuts.indexOf("e.key === 'Tab'"))
  assert.ok(globalShortcuts.indexOf('if (isDialogGuardActive()) return') < globalShortcuts.indexOf("e.key === 'Tab'"))
  assert.match(
    settings,
    /!getEnhancedKeyboardNavigation\(\)[\s\S]*event\.isComposing[\s\S]*event\.keyCode === 229[\s\S]*showRestartDialog/,
  )
  assert.match(backupViewer, /event\.isComposing \|\| event\.keyCode === 229/)
})

test('contact arrows update detail without forcing responsive activation', async () => {
  const [contactList, contactStore] = await Promise.all([
    readFile(contactListPath, 'utf8'),
    readFile(contactStorePath, 'utf8'),
  ])

  assert.match(contactList, /onSelect=\{\(id\) => focusContact\(id\)\}/)
  assert.match(contactList, /onActivate=\{\(id\) => activateContact\(id\)\}/)
  assert.match(contactStore, /focusContact[\s\S]*selectContact\(id, false\)/)
  assert.match(contactStore, /activateContact[\s\S]*selectContact\(id, true\)/)
})
