const de = {
  // App-level
  app: {
    agentDone: 'Agent fertig – #{number}',
    terminalError: 'Terminal-Fehler (Session {id}): {msg}',
    mergeConflicts: 'Merge-Konflikte erkannt{op}',
    conflictFiles: '{count} Datei(en) mit Konflikten',
    maxPanes: 'Max. {max} Terminals pro Tab erreicht.',
    notifyDone: '{name} - Fertig',
    notifyDoneBody: 'Claude ist fertig. Prompt bereit.',
    notifyInput: '{name} - Eingabe nötig',
    notifyInputBody: 'Claude wartet auf Bestätigung.',
  },

  // TabBar
  tabBar: {
    rename: 'Tab umbenennen:',
    newTab: 'Neuer Tab (Ctrl+T)',
  },

  // Toolbar
  toolbar: {
    noDir: '(kein Verzeichnis)',
    changeDirEnabled: 'Arbeitsverzeichnis ändern: {dir}',
    changeDirDisabled: 'Alle Terminals schließen um Verzeichnis zu ändern ({dir})',
    paneCount: '{count} / {max} Terminals',
    maxReached: 'Max. {max} Terminals erreicht',
    newTerminal: 'Neues Terminal (Ctrl+N)',
    commandPalette: 'Befehlspalette',
    files: 'Dateien (Ctrl+B)',
    audioOn: 'Audio einschalten',
    audioOff: 'Audio stumm schalten',
    settings: 'Einstellungen',
  },

  // Sidebar
  sidebar: {
    unpin: 'Sidebar lösen',
    pin: 'Sidebar anpinnen',
    search: 'Suchen...',
    noResults: 'Keine Ergebnisse',
  },

  // Footer
  footer: {
    commitJustNow: 'Letzter Commit: gerade eben',
    commitMinutes: 'Letzter Commit: {min}m',
    commitHours: 'Letzter Commit: {h}h {m}m',
    conflicts: '\u26A0 {count} Konflikt(e){op}',
    updateAvailable: 'Update v{version} verfügbar',
  },

  // LaunchDialog
  launch: {
    titleIssue: 'Agent für #{number}',
    titleNew: 'Neues Terminal',
    claudeNotFound: 'Claude CLI nicht gefunden.',
    settingsLink: 'Einstellungen',
    codexNotFound: 'Codex CLI nicht gefunden.',
    codexInstall: 'npm i -g @openai/codex',
    geminiNotFound: 'Gemini CLI nicht gefunden.',
    geminiInstall: 'npm i -g @google/gemini-cli',
    shell: 'Shell',
    shellDesc: 'Standard-Terminal',
    claude: 'Claude Code',
    claudeDesc: 'Normal-Modus',
    claudeYolo: 'Claude YOLO',
    claudeYoloDesc: 'Alle Berechtigungen',
    codex: 'Codex',
    codexDesc: 'OpenAI Codex CLI',
    codexAuto: 'Codex Auto',
    codexAutoDesc: 'Full-Auto Modus',
    gemini: 'Gemini',
    geminiDesc: 'Google Gemini CLI',
    geminiYolo: 'Gemini Sandbox',
    geminiYoloDesc: 'Sandbox-Modus',
    modelLabel: 'Claude Modell:',
    cancel: 'Abbrechen (Esc)',
  },

  // SettingsDialog
  settings: {
    title: 'Einstellungen',
    theme: 'Theme',
    themeDesc: 'Farbschema der gesamten Oberfläche.',
    terminalColor: 'Terminal-Farbe',
    terminalColorDesc: 'Bestimmt Akzentfarbe, Cursor und fokussierte Rahmen.',
    fontFamily: 'Schriftart',
    fontFamilyDesc: 'Monospace-Schriftart für alle Terminals.',
    fontDefault: 'Standard (Cascadia Code, Fira Code, ...)',
    fontNotInstalled: '(nicht installiert)',
    fontSize: 'Schriftgröße',
    fontSizeDesc: 'Basis-Schriftgröße in Pixel. Ctrl+Scroll zum Zoomen pro Pane.',
    logging: 'Logging',
    loggingDesc: 'Schreibt detaillierte Protokolle in eine Datei. Wird automatisch deaktiviert nach 3 stabilen Starts.',
    active: 'Aktiv',
    inactive: 'Inaktiv',
    openLogDir: 'Log-Ordner öffnen',
    worktrees: 'Git Worktrees',
    worktreesDesc: 'Erstellt pro Issue ein isoliertes Arbeitsverzeichnis statt nur einen Branch zu wechseln.',
    cliTools: 'CLI-Tools',
    cliToolsDesc: 'Aktivierte Tools erscheinen in der Terminal-Auswahl (Ctrl+N).',
    claudePathDesc: 'Pfad zur Claude Code CLI. Leer lassen für automatische Erkennung.',
    claudePlaceholder: 'claude (automatisch)',
    browse: 'Durchsuchen',
    detect: 'Erkennen',
    found: 'Gefunden: {path}',
    notFound: 'Nicht gefunden',
    codexPathDesc: 'Pfad zur Codex CLI. Installation: npm i -g @openai/codex',
    codexPlaceholder: 'codex (automatisch)',
    codexNotFound: 'Nicht gefunden — npm i -g @openai/codex',
    geminiPathDesc: 'Pfad zur Gemini CLI. Installation: npm i -g @google/gemini-cli',
    geminiPlaceholder: 'gemini (automatisch)',
    geminiNotFound: 'Nicht gefunden — npm i -g @google/gemini-cli',
    audio: 'Audio',
    audioDesc: 'Akustische Benachrichtigungen wenn ein Agent fertig ist oder Eingabe braucht.',
    audioWhenFocused: 'Auch bei fokussiertem Fenster',
    volume: 'Lautstärke',
    doneSound: 'Fertig-Sound',
    doneSoundDefault: 'Standard (Synthesizer)',
    inputSound: 'Eingabe-Sound',
    errorSound: 'Fehler-Sound',
    preview: 'Vorschau',
    reset: 'Standard',
    cancel: 'Abbrechen',
    save: 'Speichern',
  },

  // CommandPalette
  commandPalette: {
    title: 'Befehlspalette',
    placeholder: 'Befehl / Text',
    namePlaceholder: 'Name (z.B. Run Tests)',
    save: 'Speichern',
    cancel: 'Abbrechen',
    edit: 'Bearbeiten',
    delete: 'Löschen',
    addNew: '+ Neuen Befehl anlegen',
    add: 'Hinzufügen',
  },

  // ProjectDialog
  project: {
    title: 'Projekt hinzufügen',
    subtitle: 'Wähle wie du ein Projekt öffnen möchtest',
    openFolder: 'Vorhandenen Ordner öffnen',
    openFolderDesc: 'Ein bestehendes Projekt auf diesem Computer öffnen',
    createNew: 'Neues Projekt erstellen',
    createNewDesc: 'Einen neuen Projektordner anlegen',
    namePrompt: 'Projektname:',
  },

  // TerminalPane
  terminal: {
    processExited: 'Prozess beendet',
    restart: 'Neu starten',
    close: 'Schließen',
    devServer: 'Dev Server',
    urlOpened: '{url} geöffnet',
    urlDetected: '{url} erkannt',
  },

  // PaneTitlebar
  titlebar: {
    doubleClickRename: 'Doppelklick zum Umbenennen',
    issueActions: 'Issue-Aktionen',
    commitPush: 'Commit & Push',
    createPR: 'PR erstellen',
    closeIssue: 'Issue schließen',
    pipelineQueue: 'Pipeline Queue',
    maximize: 'Maximize',
    closePane: 'Close',
  },

  // TerminalSearch
  search: {
    placeholder: 'Suchen... (Enter=weiter, Shift+Enter=zurück)',
    previous: 'Vorheriger (Shift+Enter)',
    next: 'Nächster (Enter)',
    close: 'Schließen (Esc)',
  },

  // CrashDialog
  crash: {
    title: 'Instabilität erkannt',
    description: 'Die letzten zwei Sitzungen wurden nicht sauber beendet. Möchtest du das Logging aktivieren, um die Ursache zu finden?',
    hint: 'Das Log wird automatisch deaktiviert, sobald 3 Sitzungen wieder stabil laufen.',
    dismiss: 'Nein, danke',
    enable: 'Logging aktivieren',
  },

  // BranchConflictDialog
  branchConflict: {
    title: 'Branch-Konflikt',
    current: 'Aktuell:',
    target: 'Ziel:',
    dirtyWarning: 'Uncommitted Changes vorhanden — Branch-Wechsel nicht möglich.',
    switchBranch: 'Branch wechseln',
    switchDesc: 'Zum Issue-Branch wechseln{dirty}',
    stay: 'Im Branch bleiben',
    stayDesc: 'Session ohne Branch-Wechsel starten',
    createWorktree: 'Worktree erstellen',
    worktreeDesc: 'Isoliertes Verzeichnis für Issue #{number}',
    cancel: 'Abbrechen (Esc)',
  },

  // IssueDialog
  issue: {
    editTitle: 'Issue #{number} bearbeiten',
    createTitle: 'Neues Issue',
    titleRequired: 'Titel ist erforderlich',
    saveError: 'Fehler beim Speichern',
    titleLabel: 'Titel',
    descLabel: 'Beschreibung',
    descPlaceholder: 'Beschreibung (Markdown)...',
    labelsLabel: 'Labels',
    statusLabel: 'Status',
    saveHint: 'Ctrl+Enter zum Speichern',
    cancel: 'Abbrechen',
    saving: 'Speichere...',
    save: 'Speichern',
    create: 'Erstellen',
  },

  // IssuesView
  issues: {
    ghNotFound: 'GitHub CLI nicht gefunden',
    ghInstall: 'Bitte gh installieren:',
    notAuthenticated: 'Nicht angemeldet',
    loginPrompt: 'Bitte anmelden:',
    filterOpen: 'Open',
    filterClosed: 'Closed',
    filterAll: 'Alle',
    refresh: 'Aktualisieren',
    createIssue: 'Neues Issue',
    filterPlaceholder: 'Issues filtern...',
    loading: 'Laden...',
    noIssues: 'Keine Issues',
    agentActivity: 'Agent: {activity}',
    openInBrowser: 'Im Browser öffnen',
    launchForIssue: 'Claude für dieses Issue starten',
  },

  // ContextMenu
  contextMenu: {
    copy: 'Kopieren',
    paste: 'Einfügen',
    selectAll: 'Alles auswählen',
    search: 'Suchen',
    clearTerminal: 'Terminal leeren',
    newTerminal: 'Neues Terminal',
  },

  // QueuePanel
  queue: {
    title: 'Pipeline Queue',
    clearDone: 'Clear done ({count})',
    placeholder: 'Prompt eingeben... (Enter = Add)',
    empty: 'Queue ist leer. Prompts werden nacheinander abgearbeitet.',
    remove: 'Entfernen',
    pending: '{count} wartend',
  },

  // SourceControlView
  sourceControl: {
    conflicts: 'Konflikte',
    modified: 'Modified',
    added: 'Added',
    untracked: 'Untracked',
    deleted: 'Deleted',
    renamed: 'Renamed',
    mergeInProgress: '{op} in Bearbeitung',
    noChanges: 'Keine Änderungen',
    copied: 'kopiert!',
    copyPath: 'Pfad kopieren',
  },

  // FileTreeItem
  fileTree: {
    copyPath: 'Pfad kopieren',
    removeFavorite: 'Favorit entfernen',
    addFavorite: 'Als Favorit markieren',
    copied: 'kopiert!',
  },

  // FavoritesSection
  favorites: {
    noFavorites: 'Keine Favoriten',
    removeFavorite: 'Favorit entfernen',
  },

  // PaneGrid
  paneGrid: {
    empty: 'Kein Terminal offen.',
    emptyHint: 'Drücke Ctrl+N oder klicke + New Terminal (max. {max} pro Tab)',
  },

  // PaneErrorBoundary
  paneError: {
    title: 'Pane-Fehler',
    restart: 'Neu starten',
    close: 'Schließen',
  },

  // FilePreview
  filePreview: {
    loading: 'Laden...',
    binaryFile: 'Binärdatei kann nicht angezeigt werden',
    openInEditor: 'Im Editor öffnen',
    edit: 'Bearbeiten',
    close: 'Schließen',
    openInBrowser: 'Im Browser öffnen',
  },

  // IssueDetail
  issueDetail: {
    toggleState: 'Status ändern',
    edit: 'Bearbeiten',
    comments: 'Kommentare ({count})',
    commentPlaceholder: 'Kommentar schreiben...',
    sending: 'Sende...',
    send: 'Senden',
  },

  // SetupDialog
  setup: {
    title: 'Willkommen bei Multiterminal',
    subtitle: 'Richte dein Setup ein',
    languageLabel: 'Sprache',
    cliToolsLabel: 'CLI-Tools aktivieren',
    cliToolsDesc: 'Wähle welche AI-Tools du verwenden möchtest.',
    finishButton: 'Los geht\'s',
    skipButton: 'Überspringen',
  },

  // Common
  common: {
    cancel: 'Abbrechen',
    save: 'Speichern',
    close: 'Schließen',
    loading: 'Laden...',
  },
} as const;

export default de;
